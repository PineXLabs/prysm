package backfill

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db/filesystem"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/startup"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/sync"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/verification"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
)

type workerId int

type p2pWorker struct {
	id   workerId
	todo chan batch
	done chan batch
	p2p  p2p.P2P
	v    *verifier
	c    *startup.Clock
	cm   sync.ContextByteVersions
	ncv  verification.NewColumnVerifier
	cfs  *filesystem.ColumnStorage
}

func (w *p2pWorker) run(ctx context.Context) {
	for {
		select {
		case b := <-w.todo:
			log.WithFields(b.logFields()).WithField("backfillWorker", w.id).Debug("Backfill worker received batch")
			if b.state == batchColumnSync {
				w.done <- w.handleColumns(ctx, b)
			} else {
				w.done <- w.handleBlocks(ctx, b)
			}
		case <-ctx.Done():
			log.WithField("backfillWorker", w.id).Info("Backfill worker exiting after context canceled")
			return
		}
	}
}

func (w *p2pWorker) handleBlocks(ctx context.Context, b batch) batch {
	cs := w.c.CurrentSlot()
	blobRetentionStart, err := sync.BlobRPCMinValidSlot(cs)
	if err != nil {
		return b.withRetryableError(errors.Wrap(err, "configuration issue, could not compute minimum blob retention slot"))
	}
	b.blockPid = b.busy
	start := time.Now()
	results, err := sync.SendBeaconBlocksByRangeRequest(ctx, w.c, w.p2p, b.blockPid, b.blockRequest(), blockValidationMetrics)
	dlt := time.Now()
	backfillBatchTimeDownloadingBlocks.Observe(float64(dlt.Sub(start).Milliseconds()))
	if err != nil {
		log.WithError(err).WithFields(b.logFields()).Debug("Batch requesting failed")
		return b.withRetryableError(err)
	}
	vb, err := w.v.verify(results)
	backfillBatchTimeVerifying.Observe(float64(time.Since(dlt).Milliseconds()))
	if err != nil {
		log.WithError(err).WithFields(b.logFields()).Debug("Batch validation failed")
		return b.withRetryableError(err)
	}
	// This is a hack to get the rough size of the batch. This helps us approximate the amount of memory needed
	// to hold batches and relative sizes between batches, but will be inaccurate when it comes to measuring actual
	// bytes downloaded from peers, mainly because the p2p messages are snappy compressed.
	bdl := 0
	for i := range vb {
		bdl += vb[i].SizeSSZ()
	}
	backfillBlocksApproximateBytes.Add(float64(bdl))
	log.WithFields(b.logFields()).WithField("dlbytes", bdl).Debug("Backfill batch block bytes downloaded")
	_, subnetMap, err := p2p.RetrieveColumnSubnets(w.p2p)
	if err != nil {
		return b.withRetryableError(err)
	}

	bs, err := newColumnSync(cs, vb, &columnSyncConfig{retentionStart: blobRetentionStart, ncv: w.ncv, store: w.cfs}, func(slot primitives.Slot, root [32]byte) map[uint64]struct{} {
		return subnetMap
	})
	if err != nil {
		return b.withRetryableError(err)
	}
	return b.withResults(vb, bs)
}

//func (w *p2pWorker) handleBlobs(ctx context.Context, b batch) batch {
//	b.columnPid = b.busy
//	start := time.Now()
//	// we don't need to use the response for anything other than metrics, because blobResponseValidation
//	// adds each of them to a batch AvailabilityStore once it is checked.
//	blobs, err := sync.SendBlobsByRangeRequest(ctx, w.c, w.p2p, b.columnPid, w.cm, b.blobRequest(), b.blobResponseValidator(), blobValidationMetrics)
//	if err != nil {
//		b.bs = nil
//		return b.withRetryableError(err)
//	}
//	dlt := time.Now()
//	backfillBatchTimeDownloadingBlobs.Observe(float64(dlt.Sub(start).Milliseconds()))
//	if len(blobs) > 0 {
//		// All blobs are the same size, so we can compute 1 and use it for all in the batch.
//		sz := blobs[0].SizeSSZ() * len(blobs)
//		backfillBlobsApproximateBytes.Add(float64(sz))
//		log.WithFields(b.logFields()).WithField("dlbytes", sz).Debug("Backfill batch blob bytes downloaded")
//	}
//	return b.postBlobSync()
//}

func (w *p2pWorker) handleColumns(ctx context.Context, b batch) batch {
	b.columnPid = b.busy
	start := time.Now()
	// we don't need to use the response for anything other than metrics, because columnResponseValidation
	// adds each of them to a batch AvailabilityStore once it is checked.
	columns, err := sync.SendColumnsByRangeRequest(ctx, w.c, w.p2p, b.columnPid, w.cm, b.columnRequest(), b.columnResponseValidator(), columnValidationMetrics)
	if err != nil {
		b.cs = nil
		return b.withRetryableError(err)
	}
	dlt := time.Now()
	backfillBatchTimeDownloadingColumns.Observe(float64(dlt.Sub(start).Milliseconds()))
	if len(columns) > 0 {
		// All columns are the same size, so we can compute 1 and use it for all in the batch.
		sz := columns[0].SizeSSZ() * len(columns)
		backfillBlobsApproximateBytes.Add(float64(sz))
		log.WithFields(b.logFields()).WithField("dlbytes", sz).Debug("Backfill batch blob bytes downloaded")
	}
	return b.postColumnSync()
}

func newP2pWorker(id workerId, p p2p.P2P, todo, done chan batch, c *startup.Clock, v *verifier, cm sync.ContextByteVersions, ncv verification.NewColumnVerifier, cfs *filesystem.ColumnStorage) *p2pWorker {
	return &p2pWorker{
		id:   id,
		todo: todo,
		done: done,
		p2p:  p,
		v:    v,
		c:    c,
		cm:   cm,
		ncv:  ncv,
		cfs:  cfs,
	}
}
