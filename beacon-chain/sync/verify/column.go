package verify

import (
	"github.com/pkg/errors"
	fieldparams "github.com/prysmaticlabs/prysm/v5/config/fieldparams"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
	"github.com/prysmaticlabs/prysm/v5/runtime/version"
)

var (
	errColumnVerification              = errors.New("unable to verify columns")
	ErrIncorrectColumnIndex            = errors.New("incorrect column index")
	ErrColumnBlockMisaligned           = errors.Wrap(errColumnVerification, "root of block header in column sidecar does not match block root")
	ErrMismatchedColumnCommitments     = errors.Wrap(errColumnVerification, "commitments at given slot, root and index do not match")
	ErrMismatchedCommitmentsCount      = errors.Wrap(errColumnVerification, "the count of commitments in column sidecar does not match block")
	ErrMismatchedColumnCommitmentsHash = errors.Wrap(errColumnVerification, "commitmentsHash in column sidecar does not match block commitmentsHash")
)

// ColumnAlignsWithBlock verifies if the column aligns with the block.
func ColumnAlignsWithBlock(column blocks.ROColumn, block blocks.ROBlock) error {
	if block.Version() < version.Deneb {
		return nil
	}
	if column.Index >= fieldparams.MaxColumnsPerBlock {
		return errors.Wrapf(ErrIncorrectColumnIndex, "index %d exceeds MAX_COLUMNS_PER_BLOCK %d", column.Index, fieldparams.MaxColumnsPerBlock)
	}

	if column.BlockRoot() != block.Root() {
		return ErrColumnBlockMisaligned
	}

	// Verify commitment byte values match
	// TODO: verify commitment inclusion proof - actually replace this with a better rpc blob verification stack altogether.
	commits, err := block.Block().Body().BlobKzgCommitments()
	if err != nil {
		return err
	}

	if len(column.BlobKzgCommitments) != len(commits) {
		return errors.Wrapf(ErrMismatchedCommitmentsCount, "column commitments count %#x != block commitments count %#x, for block root %#x at slot %d ", len(column.BlobKzgCommitments), len(commits), block.Root(), column.Slot())
	}

	for i := 0; i < len(column.BlobKzgCommitments); i++ {
		blockCommitment := bytesutil.ToBytes48(commits[i])
		blobCommitment := bytesutil.ToBytes48(column.BlobKzgCommitments[i])
		if blobCommitment != blockCommitment {
			return errors.Wrapf(ErrMismatchedColumnCommitments, "commitment %#x != block commitment %#x, at index %d for block root %#x at slot %d ", blobCommitment, blockCommitment, column.Index, block.Root(), column.Slot())
		}
	}

	return nil
}
