package sync

import (
	"context"
	"fmt"

	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/feed"
	opfeed "github.com/prysmaticlabs/prysm/v5/beacon-chain/core/feed/operation"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"google.golang.org/protobuf/proto"
)

func (s *Service) columnSubscriber(ctx context.Context, msg proto.Message) error {
	b, ok := msg.(blocks.VerifiedROColumn)
	if !ok {
		return fmt.Errorf("message was not type blocks.ROColumn, type=%T", msg)
	}

	s.setSeenColumnIndex(b.Slot(), b.ProposerIndex(), b.Index)

	if err := s.cfg.chain.ReceiveColumn(ctx, b); err != nil {
		return err
	}

	s.cfg.operationNotifier.OperationFeed().Send(&feed.Event{
		Type: opfeed.ColumnSidecarReceived,
		Data: &opfeed.ColumnSidecarReceivedData{
			Column: &b,
		},
	})

	return nil
}
