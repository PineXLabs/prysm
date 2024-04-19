package blockchain

import (
	"context"

	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
)

// SendNewColumnEvent sends a message to the ColumnNotifier channel that the blob
// for the block root `root` is ready in the database
func (s *Service) sendNewColumnEvent(root [32]byte, index uint64) {
	s.columnNotifiers.notifyIndex(root, index)
}

// ReceiveColumn saves the column to database and sends the new event
func (s *Service) ReceiveColumn(ctx context.Context, c blocks.VerifiedROColumn) error {
	if err := s.columnStorage.Save(c); err != nil {
		return err
	}

	s.sendNewColumnEvent(c.BlockRoot(), c.Index)
	return nil
}
