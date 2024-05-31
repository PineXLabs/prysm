package verification

import "github.com/pkg/errors"

const (
	RequireBlobIndexInBounds Requirement = iota
	RequireNotFromFutureSlot
	RequireSlotAboveFinalized
	RequireValidProposerSignature
	RequireSidecarParentSeen
	RequireSidecarParentValid
	RequireSidecarParentSlotLower
	RequireSidecarDescendsFromFinalized
	RequireSidecarInclusionProven
	RequireSidecarKzgProofVerified
	RequireSidecarProposerExpected
	RequireColumnIndexInBounds
	RequireColumnSidecarParentSeen
	RequireColumnSidecarParentValid
	RequireColumnSidecarParentSlotLower
	RequireColumnSidecarDescendsFromFinalized
	RequireColumnSidecarInclusionProven
	RequireColumnSidecarKzgProofVerified
	RequireColumnSidecarProposerExpected
)

var allSidecarRequirements = []Requirement{
	RequireBlobIndexInBounds,
	RequireNotFromFutureSlot,
	RequireSlotAboveFinalized,
	RequireValidProposerSignature,
	RequireSidecarParentSeen,
	RequireSidecarParentValid,
	RequireSidecarParentSlotLower,
	RequireSidecarDescendsFromFinalized,
	RequireSidecarInclusionProven,
	RequireSidecarKzgProofVerified,
	RequireSidecarProposerExpected,
}

var allColumnSidecarRequirements = []Requirement{
	RequireColumnIndexInBounds,
	RequireNotFromFutureSlot,
	RequireSlotAboveFinalized,
	RequireValidProposerSignature,
	RequireColumnSidecarParentSeen,
	RequireColumnSidecarParentValid,
	RequireColumnSidecarParentSlotLower,
	RequireColumnSidecarDescendsFromFinalized,
	RequireColumnSidecarInclusionProven,
	RequireColumnSidecarKzgProofVerified,
	RequireColumnSidecarProposerExpected,
}

var (
	ErrBlobInvalid   = errors.New("blob failed verification")
	ErrColumnInvalid = errors.New("column failed verification")
	// ErrBlobIndexInvalid means RequireBlobIndexInBounds failed.
	ErrBlobIndexInvalid = errors.Wrap(ErrBlobInvalid, "incorrect blob sidecar index")
	// ErrColumnIndexInvalid means RequireColumnIndexInBounds failed.
	ErrColumnIndexInvalid = errors.Wrap(ErrColumnInvalid, "incorrect column sidecar index")
	// ErrFromFutureSlot means RequireSlotNotTooEarly failed.
	ErrFromFutureSlot = errors.Wrap(ErrBlobInvalid, "slot is too far in the future")
	// ErrSlotNotAfterFinalized means RequireSlotAboveFinalized failed.
	ErrSlotNotAfterFinalized = errors.Wrap(ErrBlobInvalid, "slot <= finalized checkpoint")
	// ErrInvalidProposerSignature means RequireValidProposerSignature failed.
	ErrInvalidProposerSignature = errors.Wrap(ErrBlobInvalid, "proposer signature could not be verified")
	// ErrSidecarParentNotSeen means RequireSidecarParentSeen failed.
	ErrSidecarParentNotSeen = errors.Wrap(ErrBlobInvalid, "parent root has not been seen")
	// ErrColumnSidecarParentNotSeen means RequireColumnSidecarParentSeen failed.
	ErrColumnSidecarParentNotSeen = errors.Wrap(ErrColumnInvalid, "parent root has not been seen")
	// ErrSidecarParentInvalid means RequireSidecarParentValid failed.
	ErrSidecarParentInvalid = errors.Wrap(ErrBlobInvalid, "parent block is not valid")
	// ErrColumnSidecarParentInvalid means RequireColumnSidecarParentValid failed.
	ErrColumnSidecarParentInvalid = errors.Wrap(ErrColumnInvalid, "parent block is not valid")
	// ErrSlotNotAfterParent means RequireSidecarParentSlotLower failed.
	ErrSlotNotAfterParent = errors.Wrap(ErrBlobInvalid, "slot <= slot")
	// ErrSidecarNotFinalizedDescendent means RequireSidecarDescendsFromFinalized failed.
	ErrSidecarNotFinalizedDescendent = errors.Wrap(ErrBlobInvalid, "blob parent is not descended from the finalized block")
	// ErrSidecarInclusionProofInvalid means RequireSidecarInclusionProven failed.
	ErrSidecarInclusionProofInvalid = errors.Wrap(ErrBlobInvalid, "sidecar inclusion proof verification failed")
	// ErrSidecarKzgProofInvalid means RequireSidecarKzgProofVerified failed.
	ErrSidecarKzgProofInvalid = errors.Wrap(ErrBlobInvalid, "sidecar kzg commitment proof verification failed")
	// ErrSidecarUnexpectedProposer means RequireSidecarProposerExpected failed.
	ErrSidecarUnexpectedProposer = errors.Wrap(ErrBlobInvalid, "sidecar was not proposed by the expected proposer_index")
	// ErrColumnSidecarNotFinalizedDescendent means RequireColumnSidecarDescendsFromFinalized failed.
	ErrColumnSidecarNotFinalizedDescendent = errors.Wrap(ErrColumnInvalid, "column parent is not descended from the finalized block")
	// ErrColumnSidecarInclusionProofInvalid means RequireColumnSidecarInclusionProven failed.
	ErrColumnSidecarInclusionProofInvalid = errors.Wrap(ErrColumnInvalid, "column sidecar inclusion proof verification failed")
	// ErrColumnSidecarKzgProofInvalid means RequireColumnSidecarKzgProofVerified failed.
	ErrColumnSidecarKzgProofInvalid = errors.Wrap(ErrColumnInvalid, "column sidecar kzg commitment proof verification failed")
	// ErrColumnSidecarUnexpectedProposer means RequireColumnSidecarProposerExpected failed.
	ErrColumnSidecarUnexpectedProposer = errors.Wrap(ErrColumnInvalid, "column sidecar was not proposed by the expected proposer_index")
)
