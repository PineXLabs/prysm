package das

import (
	"context"

	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
)

// AvailabilityStore describes a component that can verify and save sidecars for a given block, and confirm previously
// verified and saved sidecars.
// Persist guarantees that the sidecar will be available to perform a DA check
// for the life of the beacon node process.
// IsDataAvailable guarantees that all blobs committed to in the block have been
// durably persisted before returning a non-error value.
type AvailabilityStore interface {
	IsDataAvailable(ctx context.Context, current primitives.Slot, b blocks.ROBlock) error
	Persist(current primitives.Slot, sc ...blocks.ROBlob) error
}

// ColumnAvailabilityStore describes a component that can verify and save column sidecars for a given block, and confirm previously
// verified and saved column sidecars.
// Persist guarantees that the column sidecar will be available to perform a DA check
// for the life of the beacon node process.
// IsDataAvailable guarantees that all columns committed to in the block have been
// durably persisted before returning a non-error value.
type ColumnAvailabilityStore interface {
	IsDataAvailable(ctx context.Context, current primitives.Slot, b blocks.ROBlock) error
	Persist(current primitives.Slot, sc ...blocks.ROColumn) error
}
