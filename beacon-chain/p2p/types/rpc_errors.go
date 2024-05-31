package types

import "errors"

var (
	ErrWrongForkDigestVersion = errors.New("wrong fork digest version")
	ErrInvalidEpoch           = errors.New("invalid epoch")
	ErrInvalidFinalizedRoot   = errors.New("invalid finalized root")
	ErrInvalidSequenceNum     = errors.New("invalid sequence number provided")
	ErrGeneric                = errors.New("internal service error")

	ErrRateLimited          = errors.New("rate limited")
	ErrIODeadline           = errors.New("i/o deadline exceeded")
	ErrInvalidRequest       = errors.New("invalid range, step or count")
	ErrBlobLTMinRequest     = errors.New("blob slot < minimum_request_epoch")
	ErrColumnLTMinRequest   = errors.New("column slot < minimum_request_epoch")
	ErrMaxBlobReqExceeded   = errors.New("requested more than MAX_REQUEST_BLOB_SIDECARS")
	ErrMaxColumnReqExceeded = errors.New("requested more than MAX_REQUEST_COLUMN_SIDECARS")
	ErrResourceUnavailable  = errors.New("resource requested unavailable")
)
