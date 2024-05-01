package backfill

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/das"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db/filesystem"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/verification"
	fieldparams "github.com/prysmaticlabs/prysm/v5/config/fieldparams"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
)

type columnSummary struct {
	blockRoot [32]byte
	index     uint64
	//commitment [48]byte
	commitments [][]byte
	signature   [fieldparams.BLSSignatureLength]byte
}

type columnSyncConfig struct {
	retentionStart primitives.Slot
	ncv            verification.NewColumnVerifier
	store          *filesystem.ColumnStorage
}

func newColumnSync(current primitives.Slot, vbs verifiedROBlocks, cfg *columnSyncConfig) (*columnSync, error) {
	expected, err := vbs.columnIdents(cfg.retentionStart)
	if err != nil {
		return nil, err
	}
	cbv := newColumnBatchVerifier(cfg.ncv)
	as := das.NewColumnLazilyPersistentStore(cfg.store, cbv)
	return &columnSync{current: current, expected: expected, cbv: cbv, store: as}, nil
}

type columnVerifierMap map[[32]byte][fieldparams.MaxColumnsPerBlock]verification.ColumnVerifier

type columnSync struct {
	store    das.ColumnAvailabilityStore
	expected []columnSummary
	next     int
	cbv      *columnBatchVerifier
	current  primitives.Slot
}

func (cs *columnSync) columnsNeeded() int {
	return len(cs.expected) - cs.next
}

func (cs *columnSync) validateNext(rb blocks.ROColumn) error {
	if cs.next >= len(cs.expected) {
		return errUnexpectedResponseSize
	}
	next := cs.expected[cs.next]
	cs.next += 1
	// Get the super cheap verifications out of the way before we init a verifier.
	if next.blockRoot != rb.BlockRoot() {
		return errors.Wrapf(errUnexpectedResponseContent, "next expected root=%#x, saw=%#x", next.blockRoot, rb.BlockRoot())
	}
	if next.index != rb.Index {
		return errors.Wrapf(errUnexpectedResponseContent, "next expected root=%#x, saw=%#x for root=%#x", next.index, rb.Index, next.blockRoot)
	}
	//if next.commitment != bytesutil.ToBytes48(rb.KzgCommitment) {
	//	return errors.Wrapf(errUnexpectedResponseContent, "next expected commitment=%#x, saw=%#x for root=%#x", next.commitment, rb.KzgCommitment, rb.BlockRoot())
	//}

	if bytesutil.ToBytes96(rb.SignedBlockHeader.Signature) != next.signature {
		return verification.ErrInvalidProposerSignature
	}
	v := cs.cbv.newVerifier(rb)
	if err := v.ColumnIndexInBounds(); err != nil {
		return err
	}
	v.SatisfyRequirement(verification.RequireValidProposerSignature)
	if err := v.SidecarInclusionProven(); err != nil {
		return err
	}
	if err := v.SidecarKzgProofVerified(); err != nil {
		return err
	}
	if err := cs.store.Persist(cs.current, rb); err != nil {
		return err
	}

	return nil
}

func newColumnBatchVerifier(ncv verification.NewColumnVerifier) *columnBatchVerifier {
	return &columnBatchVerifier{newColumnVerifier: ncv, verifiers: make(columnVerifierMap)}
}

type columnBatchVerifier struct {
	newColumnVerifier verification.NewColumnVerifier
	verifiers         columnVerifierMap
}

func (cbv *columnBatchVerifier) newVerifier(rb blocks.ROColumn) verification.ColumnVerifier {
	m := cbv.verifiers[rb.BlockRoot()]
	m[rb.Index] = cbv.newColumnVerifier(rb, verification.BackfillColumnSidecarRequirements)
	cbv.verifiers[rb.BlockRoot()] = m
	return m[rb.Index]
}

func (cbv *columnBatchVerifier) VerifiedROColumns(_ context.Context, blk blocks.ROBlock, _ []blocks.ROColumn) ([]blocks.VerifiedROColumn, error) {
	m, ok := cbv.verifiers[blk.Root()]
	if !ok {
		return nil, errors.Wrapf(verification.ErrMissingVerification, "no record of verifiers for root %#x", blk.Root())
	}

	if len(m) != fieldparams.MaxColumnsPerBlock {
		return nil, errors.Wrapf(errUnexpectedColumnCount, "error column count [%d] for root %#x", len(m), blk.Root())
	}

	c, err := blk.Block().Body().BlobKzgCommitments()
	if err != nil {
		return nil, errors.Wrapf(errUnexpectedCommitment, "error reading commitments from block root %#x", blk.Root())
	}
	vcs := make([]blocks.VerifiedROColumn, fieldparams.MaxColumnsPerBlock)

	for k, column := range m {
		vc, err := column.VerifiedROColumn()
		if err != nil {
			return nil, err
		}
		if len(vc.BlobKzgCommitments) != len(c) {
			return nil, errors.Wrapf(errUnexpectedColumnCommitment, "error reading commitments from block root %#x", blk.Root())
		}

		for i := 0; i < len(vc.BlobKzgCommitments); i++ {
			blockCommitment := bytesutil.ToBytes48(c[i])
			blobCommitment := bytesutil.ToBytes48(vc.BlobKzgCommitments[i])
			if blobCommitment != blockCommitment {
				return nil, errors.Wrapf(errBatchVerifierMismatch, "commitments do not match, verified=%#x da check=%#x for root %#x", vc.BlobKzgCommitments[i], c[i], vc.BlockRoot())
			}
		}
		vcs[k] = vc
	}

	return vcs, nil
}

var _ das.ColumnBatchVerifier = &columnBatchVerifier{}
