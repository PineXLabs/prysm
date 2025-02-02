package filesystem

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	blobBuckets     = []float64{3, 5, 7, 9, 11, 13}
	blobSaveLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "blob_storage_save_latency",
		Help:    "Latency of BlobSidecar storage save operations in milliseconds",
		Buckets: blobBuckets,
	})
	blobFetchLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "blob_storage_get_latency",
		Help:    "Latency of BlobSidecar storage get operations in milliseconds",
		Buckets: blobBuckets,
	})
	blobsPrunedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blob_pruned",
		Help: "Number of BlobSidecar files pruned.",
	})
	blobsWrittenCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blob_written",
		Help: "Number of BlobSidecar files written",
	})
	blobDiskCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blob_disk_count",
		Help: "Approximate number of blob files in storage",
	})
	blobDiskSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blob_disk_bytes",
		Help: "Approximate number of bytes occupied by blobs in storage",
	})
	columnBuckets     = []float64{3, 5, 7, 9, 11, 13} //todo: may needs to be revised...
	columnSaveLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "column_storage_save_latency",
		Help:    "Latency of columnSidecar storage save operations in milliseconds",
		Buckets: columnBuckets,
	})
	columnFetchLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "column_storage_get_latency",
		Help:    "Latency of columnSidecar storage get operations in milliseconds",
		Buckets: columnBuckets,
	})
	columnsPrunedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "column_pruned",
		Help: "Number of columnSidecar files pruned.",
	})
	columnsWrittenCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "column_written",
		Help: "Number of columnSidecar files written",
	})
	columnDiskCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "column_disk_count",
		Help: "Approximate number of column files in storage",
	})
	columnDiskSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "column_disk_bytes",
		Help: "Approximate number of bytes occupied by columns in storage",
	})
)
