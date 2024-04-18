package sync

import (
	"context"
	"fmt"
	"os"
	"path"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prysmaticlabs/prysm/v5/config/features"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
	"github.com/prysmaticlabs/prysm/v5/io/file"
	"github.com/sirupsen/logrus"
)

// todo: to complete
func (s *Service) validateColumn(ctx context.Context, pid peer.ID, msg *pubsub.Message) (pubsub.ValidationResult, error) {
	//....
	return pubsub.ValidationAccept, nil
}

// Returns true if the column with the same slot, proposer index, and column index has been seen before.
func (s *Service) hasSeenColumnIndex(slot primitives.Slot, proposerIndex primitives.ValidatorIndex, index uint64) bool {
	s.seenColumnLock.RLock()
	defer s.seenColumnLock.RUnlock()
	b := append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(proposerIndex))...)
	b = append(b, bytesutil.Bytes32(index)...)
	_, seen := s.seenColumnCache.Get(string(b))
	return seen
}

// Sets the column with the same slot, proposer index, and column index as seen.
func (s *Service) setSeenColumnIndex(slot primitives.Slot, proposerIndex primitives.ValidatorIndex, index uint64) {
	s.seenColumnLock.Lock()
	defer s.seenColumnLock.Unlock()
	b := append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(proposerIndex))...)
	b = append(b, bytesutil.Bytes32(index)...)
	s.seenColumnCache.Add(string(b), true)
}

func columnFields(b blocks.ROColumn) logrus.Fields {
	return logrus.Fields{
		"slot":            b.Slot(),
		"proposerIndex":   b.ProposerIndex(),
		"blockRoot":       fmt.Sprintf("%#x", b.BlockRoot()),
		"commitmentsHash": fmt.Sprintf("%#x", b.CommitmentsHash),
		"index":           b.Index,
	}
}

func computeSubnetForColumnSidecar(index uint64) uint64 {
	return index % params.BeaconConfig().ColumnsidecarSubnetCount
}

// saveInvalidColumnToTemp as a block ssz. Writes to temp directory.
func saveInvalidColumnToTemp(b blocks.ROColumn) {
	if !features.Get().SaveInvalidColumn {
		return
	}
	filename := fmt.Sprintf("column_sidecar_%#x_%d_%d.ssz", b.BlockRoot(), b.Slot(), b.Index)
	fp := path.Join(os.TempDir(), filename)
	log.Warnf("Writing invalid column sidecar to disk at %s", fp)
	enc, err := b.MarshalSSZ()
	if err != nil {
		log.WithError(err).Error("Failed to ssz encode column sidecar")
		return
	}
	if err := file.WriteFile(fp, enc); err != nil {
		log.WithError(err).Error("Failed to write to disk")
	}
}
