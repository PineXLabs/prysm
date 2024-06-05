package das

import (
	"context"
	"fmt"

	errors "github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db/filesystem"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/verification"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/runtime/logging"
	"github.com/prysmaticlabs/prysm/v5/runtime/version"
	"github.com/prysmaticlabs/prysm/v5/time/slots"
	log "github.com/sirupsen/logrus"
)

// LazilyPersistentStore is an implementation of AvailabilityStore to be used when batch syncing.
// This implementation will hold any blobs passed to Persist until the IsDataAvailable is called for their
// block, at which time they will undergo full verification and be saved to the disk.
type ColumnLazilyPersistentStore struct {
	store         *filesystem.ColumnStorage
	cache         *columnCache
	verifier      ColumnBatchVerifier
	filterFactory func(slot primitives.Slot, root [32]byte) map[uint64]struct{}
}

var _ ColumnAvailabilityStore = &ColumnLazilyPersistentStore{}

// ColumnBatchVerifier enables LazyAvailabilityStore to manage the verification process
// going from ROColumn->VerifiedROColumn, while avoiding the decision of which individual verifications
// to run and in what order. Since ColumnLazilyPersistentStore always tries to verify and save columns only when
// they are all available, the interface takes a slice of columns, enabling the implementation to optimize
// batch verification.
type ColumnBatchVerifier interface {
	VerifiedROColumns(ctx context.Context, blk blocks.ROBlock, sc []blocks.ROColumn) ([]blocks.VerifiedROColumn, error)
}

// NewColumnLazilyPersistentStore creates a new ColumnLazilyPersistentStore. This constructor should always be used
// when creating a ColumnLazilyPersistentStore because it needs to initialize the cache under the hood.
func NewColumnLazilyPersistentStore(store *filesystem.ColumnStorage, verifier ColumnBatchVerifier,
	filterFactory func(slot primitives.Slot, root [32]byte) map[uint64]struct{}) *ColumnLazilyPersistentStore {
	return &ColumnLazilyPersistentStore{
		store:         store,
		cache:         newColumnCache(),
		verifier:      verifier,
		filterFactory: filterFactory,
	}
}

// Persist adds columns to the working column cache. columns stored in this cache will be persisted
// for at least as long as the node is running. Once IsDataAvailable succeeds, all columns referenced
// by the given block are guaranteed to be persisted for the remainder of the retention period.
func (s *ColumnLazilyPersistentStore) Persist(current primitives.Slot, sc ...blocks.ROColumn) error {
	if len(sc) == 0 {
		return nil
	}
	if len(sc) > 1 {
		first := sc[0].BlockRoot()
		for i := 1; i < len(sc); i++ {
			if first != sc[i].BlockRoot() {
				return errMixedRoots
			}
		}
	}
	if !params.WithinColumnDAPeriod(slots.ToEpoch(sc[0].Slot()), slots.ToEpoch(current)) {
		return nil
	}
	key := keyFromColumnSidecar(sc[0])
	entry := s.cache.ensure(key)
	filter := s.filterFactory(key.slot, key.root)
	for i := range sc {
		col := &sc[i]
		if _, ok := filter[col.Index]; !ok {
			continue
		}
		//log.Debugf("Persisting a column to cache, key.slot %d, key.root %#x, i is %d", key.slot, key.root, i)
		if err := entry.stash(&sc[i]); err != nil {
			return err
		}
	}
	return nil
}

// IsDataAvailable returns nil if all the commitments in the given block are persisted to the db and have been verified.
// BlobSidecars already in the db are assumed to have been previously verified against the block.
func (s *ColumnLazilyPersistentStore) IsDataAvailable(ctx context.Context, current primitives.Slot, b blocks.ROBlock) error {
	blockCommitments, err := commitmentsToColumnCheck(b, current)
	if err != nil {
		return errors.Wrapf(err, "could check data availability for block %#x", b.Root())
	}
	// Return early for blocks that are pre-deneb or which do not have any commitments.
	if blockCommitments.count() == 0 {
		return nil
	}

	key := keyFromBlock(b)
	entry := s.cache.ensure(key)
	defer s.cache.delete(key)
	root := b.Root()

	// Verify we have all the expected sidecars, and fail fast if any are missing or inconsistent.
	// We don't try to salvage problematic batches because this indicates a misbehaving peer and we'd rather
	// ignore their response and decrease their peer score.
	filter := s.filterFactory(current, b.Root())
	log.WithField("column filter", filter).Debug("verifying column availability")
	sidecars, err := entry.filter(root, filter, blockCommitments)
	if err != nil {
		return errors.Wrap(err, "incomplete ColumnSidecar batch")
	}
	// Do thorough verifications of each ColumnSidecar for the block.
	// Same as above, we don't save ColumnSidecars if there are any problems with the batch.
	vscs, err := s.verifier.VerifiedROColumns(ctx, b, sidecars)
	if err != nil {
		var me verification.VerificationMultiError
		ok := errors.As(err, &me)
		if ok {
			fails := me.Failures()
			lf := make(log.Fields, len(fails))
			for i := range fails {
				lf[fmt.Sprintf("fail_%d", i)] = fails[i].Error()
			}
			log.WithFields(lf).WithFields(logging.BlockFieldsFromColumn(sidecars[0])).
				Debug("invalid ColumnSidecars received")
		}
		return errors.Wrapf(err, "invalid ColumnSidecars received for block %#x", root)
	}
	// Ensure that each ColumnSidecar is written to disk.
	for i := range vscs {
		if err := s.store.Save(vscs[i]); err != nil {
			return errors.Wrapf(err, "failed to save ColumnSidecar index %d for block %#x", vscs[i].Index, root)
		}
	}
	// All ColumnSidecars are persisted - da check succeeds.
	return nil
}

func commitmentsToColumnCheck(b blocks.ROBlock, current primitives.Slot) (columnSafeCommitmentArray, error) {
	var ar columnSafeCommitmentArray
	if b.Version() < version.Deneb {
		return ar, nil
	}
	// We are only required to check within MIN_EPOCHS_FOR_COLUMN_SIDECARS_REQUESTS
	if !params.WithinColumnDAPeriod(slots.ToEpoch(b.Block().Slot()), slots.ToEpoch(current)) {
		return ar, nil
	}
	kc, err := b.Block().Body().BlobKzgCommitments()
	if err != nil {
		return ar, err
	}
	if len(kc) > len(ar) {
		return ar, errIndexOutOfBounds
	}
	copy(ar[:], kc)
	return ar, nil
}
