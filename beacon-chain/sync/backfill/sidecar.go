package backfill

import (
	"github.com/pkg/errors"
)

var (
	errUnexpectedResponseSize     = errors.New("received more blobs than expected for the requested range")
	errUnexpectedCommitment       = errors.New("BlobSidecar commitment does not match block")
	errUnexpectedResponseContent  = errors.New("BlobSidecar response does not include expected values in expected order")
	errBatchVerifierMismatch      = errors.New("the list of blocks passed to the availability check does not match what was verified")
	errUnexpectedColumnCount      = errors.New("ColumnSidecar does not match expected count")
	errUnexpectedColumnCommitment = errors.New("ColumnSidecar commitment does not match block")
)
