package blocks

import (
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
)

var errNilBeaconBlockHeader = errors.New("received nil beacon block header")

// ROColumn represents a read-only blob sidecar with its block root.
type ROColumn struct {
	*ethpb.ColumnSidecar
	root [32]byte
}

// NewROColumnWithRoot creates a new ROColumn with a given root.
func NewROColumnWithRoot(b *ethpb.ColumnSidecar, root [32]byte) (ROColumn, error) {
	if b == nil {
		return ROColumn{}, errNilBlock
	}
	return ROColumn{ColumnSidecar: b, root: root}, nil
}

// NewROColumn creates a new ROColumn by computing the HashTreeRoot of the header.
func NewROColumn(b *ethpb.ColumnSidecar) (ROColumn, error) {
	if b == nil {
		return ROColumn{}, errNilBlock
	}
	if b.SignedBlockHeader == nil || b.SignedBlockHeader.Header == nil {
		return ROColumn{}, errNilBeaconBlockHeader
	}
	root, err := b.SignedBlockHeader.Header.HashTreeRoot()
	if err != nil {
		return ROColumn{}, err
	}
	return ROColumn{ColumnSidecar: b, root: root}, nil
}

// BlockRoot returns the root of the block.
func (b *ROColumn) BlockRoot() [32]byte {
	return b.root
}

// Slot returns the slot of the blob sidecar.
func (b *ROColumn) Slot() primitives.Slot {
	return b.SignedBlockHeader.Header.Slot
}

// ParentRoot returns the parent root of the blob sidecar.
func (b *ROColumn) ParentRoot() [32]byte {
	return bytesutil.ToBytes32(b.SignedBlockHeader.Header.ParentRoot)
}

// ParentRootSlice returns the parent root as a byte slice.
func (b *ROColumn) ParentRootSlice() []byte {
	return b.SignedBlockHeader.Header.ParentRoot
}

// BodyRoot returns the body root of the blob sidecar.
func (b *ROColumn) BodyRoot() [32]byte {
	return bytesutil.ToBytes32(b.SignedBlockHeader.Header.BodyRoot)
}

// ProposerIndex returns the proposer index of the blob sidecar.
func (b *ROColumn) ProposerIndex() primitives.ValidatorIndex {
	return b.SignedBlockHeader.Header.ProposerIndex
}

// BlockRootSlice returns the block root as a byte slice. This is often more convenient/concise
// than setting a tmp var to BlockRoot(), just so that it can be sliced.
func (b *ROColumn) BlockRootSlice() []byte {
	return b.root[:]
}

// ROColumnSlice is a custom type for a []ROColumn, allowing methods to be defined that act on a slice of ROColumn.
type ROColumnSlice []ROColumn

// Protos is a helper to make a more concise conversion from []ROColumn->[]*ethpb.ColumnSidecar.
func (s ROColumnSlice) Protos() []*ethpb.ColumnSidecar {
	pb := make([]*ethpb.ColumnSidecar, len(s))
	for i := range s {
		pb[i] = s[i].ColumnSidecar
	}
	return pb
}

// VerifiedROColumn represents an ROColumn that has undergone full verification (eg block sig, inclusion proof, commitment check).
type VerifiedROColumn struct {
	ROColumn
}

// NewVerifiedROColumn "upgrades" an ROColumn to a VerifiedROColumn. This method should only be used by the verification package.
func NewVerifiedROColumn(rob ROColumn) VerifiedROColumn {
	return VerifiedROColumn{ROColumn: rob}
}
