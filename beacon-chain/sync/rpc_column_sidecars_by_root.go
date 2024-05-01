package sync

import (
	"context"
	"fmt"
	"sort"
	"time"

	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p/types"
	"github.com/prysmaticlabs/prysm/v5/cmd/beacon-chain/flags"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
	"github.com/prysmaticlabs/prysm/v5/monitoring/tracing"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// columnSidecarByRootRPCHandler handles the /eth2/beacon_chain/req/column_sidecars_by_root/1/ RPC request.
// spec: https://github.com/ethereum/consensus-specs/column/a7e45db9ac2b60a33e144444969ad3ac0aae3d4c/specs/deneb/p2p-interface.md#columnsidecarsbyroot-v1
func (s *Service) columnSidecarByRootRPCHandler(ctx context.Context, msg interface{}, stream libp2pcore.Stream) error {
	ctx, span := trace.StartSpan(ctx, "sync.columnSidecarByRootRPCHandler")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, ttfbTimeout)
	defer cancel()
	SetRPCStreamDeadlines(stream)
	log := log.WithField("handler", p2p.ColumnSidecarsByRootName[1:]) // slice the leading slash off the name var
	ref, ok := msg.(*types.ColumnSidecarsByRootReq)
	if !ok {
		return errors.New("message is not type ColumnSidecarsByRootReq")
	}

	columnIdents := *ref
	if err := validateColumnByRootRequest(columnIdents); err != nil {
		s.cfg.p2p.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		s.writeErrorResponseToStream(responseCodeInvalidRequest, err.Error(), stream)
		return err
	}
	// Sort the identifiers so that requests for the same column root will be adjacent, minimizing db lookups.
	sort.Sort(columnIdents)

	//batchSize := flags.Get().BlobBatchLimit
	batchSize := flags.Get().ColumnBatchLimit
	var ticker *time.Ticker
	if len(columnIdents) > batchSize {
		ticker = time.NewTicker(time.Second)
	}

	// Compute the oldest slot we'll allow a peer to request, based on the current slot.
	cs := s.cfg.clock.CurrentSlot()
	minReqSlot, err := ColumnRPCMinValidSlot(cs)
	if err != nil {
		return errors.Wrapf(err, "unexpected error computing min valid column request slot, current_slot=%d", cs)
	}

	for i := range columnIdents {
		if err := ctx.Err(); err != nil {
			closeStream(stream, log)
			return err
		}

		// Throttle request processing to no more than batchSize/sec.
		if i != 0 && i%batchSize == 0 && ticker != nil {
			<-ticker.C
		}
		s.rateLimiter.add(stream, 1)
		root, idx := bytesutil.ToBytes32(columnIdents[i].BlockRoot), columnIdents[i].Index
		sc, err := s.cfg.columnStorage.Get(root, idx)
		if err != nil {
			if db.IsNotFound(err) {
				log.WithError(err).WithFields(logrus.Fields{
					"root":  fmt.Sprintf("%#x", root),
					"index": idx,
				}).Debugf("Peer requested column sidecar by root not found in db")
				continue
			}
			log.WithError(err).Errorf("unexpected db error retrieving ColumnSidecar, root=%x, index=%d", root, idx)
			s.writeErrorResponseToStream(responseCodeServerError, types.ErrGeneric.Error(), stream)
			return err
		}

		// If any root in the request content references a block earlier than minimum_request_epoch,
		// peers MAY respond with error code 3: ResourceUnavailable or not include the column in the response.
		// note: we are deviating from the spec to allow requests for columns that are before minimum_request_epoch,
		// up to the beginning of the retention period.
		if sc.Slot() < minReqSlot {
			s.writeErrorResponseToStream(responseCodeResourceUnavailable, types.ErrColumnLTMinRequest.Error(), stream)
			log.WithError(types.ErrColumnLTMinRequest).
				Debugf("requested column for block %#x before minimum_request_epoch", columnIdents[i].BlockRoot)
			return types.ErrColumnLTMinRequest
		}

		SetStreamWriteDeadline(stream, defaultWriteDuration)
		if chunkErr := WriteColumnSidecarChunk(stream, s.cfg.chain, s.cfg.p2p.Encoding(), sc); chunkErr != nil {
			log.WithError(chunkErr).Debug("Could not send a chunked response")
			s.writeErrorResponseToStream(responseCodeServerError, types.ErrGeneric.Error(), stream)
			tracing.AnnotateError(span, chunkErr)
			return chunkErr
		}
	}
	closeStream(stream, log)
	return nil
}

func validateColumnByRootRequest(columnIdents types.ColumnSidecarsByRootReq) error {
	if uint64(len(columnIdents)) > params.BeaconConfig().MaxRequestColumnSidecars {
		return types.ErrMaxColumnReqExceeded
	}
	return nil
}
