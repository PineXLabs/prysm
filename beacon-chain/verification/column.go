package verification

import (
	"context"

	"github.com/pkg/errors"
	forkchoicetypes "github.com/prysmaticlabs/prysm/v5/beacon-chain/forkchoice/types"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/state"
	fieldparams "github.com/prysmaticlabs/prysm/v5/config/fieldparams"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
	"github.com/prysmaticlabs/prysm/v5/runtime/logging"
	"github.com/prysmaticlabs/prysm/v5/time/slots"
	log "github.com/sirupsen/logrus"
)

// GossipColumnSidecarRequirements defines the set of requirements that ColumnSidecars received on gossip
// must satisfy in order to upgrade an ROColumn to a VerifiedROColumn.
var GossipColumnSidecarRequirements = requirementList(allColumnSidecarRequirements).excluding()

// SpectestColumnSidecarRequirements is used by the forkchoice spectests when verifying columns used in the on_block tests.
// The only requirements we exclude for these tests are the parent validity and seen tests, as these are specific to
// gossip processing and require the bad block cache that we only use there.
var SpectestColumnSidecarRequirements = requirementList(GossipColumnSidecarRequirements).excluding(
	RequireColumnSidecarParentSeen, RequireColumnSidecarParentValid)

// InitsyncColumnSidecarRequirements is the list of verification requirements to be used by the init-sync service
// for batch-mode syncing. Because we only perform batch verification as part of the IsDataAvailable method
// for columns after the block has been verified, and the columns to be verified are keyed in the cache by the
// block root, the list of required verifications is much shorter than gossip.
var InitsyncColumnSidecarRequirements = requirementList(GossipColumnSidecarRequirements).excluding(
	RequireNotFromFutureSlot,
	RequireSlotAboveFinalized,
	RequireColumnSidecarParentSeen,
	RequireColumnSidecarParentValid,
	RequireColumnSidecarParentSlotLower,
	RequireColumnSidecarDescendsFromFinalized,
	RequireColumnSidecarProposerExpected,
)

// BackfillColumnSidecarRequirements is the same as InitsyncColumnSidecarRequirements.
var BackfillColumnSidecarRequirements = requirementList(InitsyncColumnSidecarRequirements).excluding()

// PendingQueueColumnSidecarRequirements is the same as InitsyncColumnSidecarRequirements, used by the pending blocks queue.
var PendingQueueColumnSidecarRequirements = requirementList(InitsyncColumnSidecarRequirements).excluding()

type ROColumnVerifier struct {
	*sharedResources
	results                *results
	column                 blocks.ROColumn
	parent                 state.BeaconState
	verifyColumnCommitment rocolumnCommitmentVerifier
}

type rocolumnCommitmentVerifier func(...blocks.ROColumn) error

var _ ColumnVerifier = &ROColumnVerifier{}

// VerifiedROColumn "upgrades" the wrapped ROColumn to a VerifiedROColumn.
// If any of the verifications ran against the column failed, or some required verifications
// were not run, an error will be returned.
func (bv *ROColumnVerifier) VerifiedROColumn() (blocks.VerifiedROColumn, error) {
	if bv.results.allSatisfied() {
		return blocks.NewVerifiedROColumn(bv.column), nil
	}
	return blocks.VerifiedROColumn{}, bv.results.errors(ErrColumnInvalid)
}

// SatisfyRequirement allows the caller to assert that a requirement has been satisfied.
// This gives us a way to tick the box for a requirement where the usual method would be impractical.
// For example, when batch syncing, forkchoice is only updated at the end of the batch. So the checks that use
// forkchoice, like descends from finalized or parent seen, would necessarily fail. Allowing the caller to
// assert the requirement has been satisfied ensures we have an easy way to audit which piece of code is satisfying
// a requireent outside of this package.
func (bv *ROColumnVerifier) SatisfyRequirement(req Requirement) {
	bv.recordResult(req, nil)
}

func (bv *ROColumnVerifier) recordResult(req Requirement, err *error) {
	if err == nil || *err == nil {
		bv.results.record(req, nil)
		return
	}
	bv.results.record(req, *err)
}

// columnIndexInBounds represents the follow spec verification:
// [REJECT] The sidecar's index is consistent with MAX_columnS_PER_BLOCK -- i.e. column_sidecar.index < MAX_columnS_PER_BLOCK.
func (bv *ROColumnVerifier) ColumnIndexInBounds() (err error) {
	defer bv.recordResult(RequireColumnIndexInBounds, &err)
	if bv.column.Index >= fieldparams.MaxColumnsPerBlock {
		log.WithFields(logging.ColumnFields(bv.column)).Debug("Sidecar index >= MAX_COLUMNS_PER_BLOCK")
		return ErrColumnIndexInvalid
	}
	return nil
}

// NotFromFutureSlot represents the spec verification:
// [IGNORE] The sidecar is not from a future slot (with a MAXIMUM_GOSSIP_CLOCK_DISPARITY allowance)
// -- i.e. validate that block_header.slot <= current_slot
func (bv *ROColumnVerifier) NotFromFutureSlot() (err error) {
	defer bv.recordResult(RequireNotFromFutureSlot, &err)
	if bv.clock.CurrentSlot() == bv.column.Slot() {
		return nil
	}
	// earliestStart represents the time the slot starts, lowered by MAXIMUM_GOSSIP_CLOCK_DISPARITY.
	// We lower the time by MAXIMUM_GOSSIP_CLOCK_DISPARITY in case system time is running slightly behind real time.
	earliestStart := bv.clock.SlotStart(bv.column.Slot()).Add(-1 * params.BeaconConfig().MaximumGossipClockDisparityDuration())
	// If the system time is still before earliestStart, we consider the column from a future slot and return an error.
	if bv.clock.Now().Before(earliestStart) {
		log.WithFields(logging.ColumnFields(bv.column)).Debug("column sidecar slot is too far in the future")
		return ErrFromFutureSlot
	}
	return nil
}

// SlotAboveFinalized represents the spec verification:
// [IGNORE] The sidecar is from a slot greater than the latest finalized slot
// -- i.e. validate that block_header.slot > compute_start_slot_at_epoch(state.finalized_checkpoint.epoch)
func (bv *ROColumnVerifier) SlotAboveFinalized() (err error) {
	defer bv.recordResult(RequireSlotAboveFinalized, &err)
	fcp := bv.fc.FinalizedCheckpoint()
	fSlot, err := slots.EpochStart(fcp.Epoch)
	if err != nil {
		return errors.Wrapf(ErrSlotNotAfterFinalized, "error computing epoch start slot for finalized checkpoint (%d) %s", fcp.Epoch, err.Error())
	}
	if bv.column.Slot() <= fSlot {
		log.WithFields(logging.ColumnFields(bv.column)).Debug("sidecar slot is not after finalized checkpoint")
		return ErrSlotNotAfterFinalized
	}
	return nil
}

// ValidProposerSignature represents the spec verification:
// [REJECT] The proposer signature of column_sidecar.signed_block_header,
// is valid with respect to the block_header.proposer_index pubkey.
func (bv *ROColumnVerifier) ValidProposerSignature(ctx context.Context) (err error) {
	defer bv.recordResult(RequireValidProposerSignature, &err)
	sd := columnToSignatureData(bv.column)
	// First check if there is a cached verification that can be reused.
	seen, err := bv.sc.SignatureVerified(sd)
	if seen {
		columnVerificationProposerSignatureCache.WithLabelValues("hit-valid").Inc()
		if err != nil {
			log.WithFields(logging.ColumnFields(bv.column)).WithError(err).Debug("reusing failed proposer signature validation from cache")
			columnVerificationProposerSignatureCache.WithLabelValues("hit-invalid").Inc()
			return ErrInvalidProposerSignature
		}
		return nil
	}
	columnVerificationProposerSignatureCache.WithLabelValues("miss").Inc()

	// Retrieve the parent state to fallback to full verification.
	parent, err := bv.parentState(ctx)
	if err != nil {
		log.WithFields(logging.ColumnFields(bv.column)).WithError(err).Debug("could not replay parent state for column signature verification")
		return ErrInvalidProposerSignature
	}
	// Full verification, which will subsequently be cached for anything sharing the signature cache.
	if err = bv.sc.VerifySignature(sd, parent); err != nil {
		log.WithFields(logging.ColumnFields(bv.column)).WithError(err).Debug("signature verification failed")
		return ErrInvalidProposerSignature
	}
	return nil
}

// SidecarParentSeen represents the spec verification:
// [IGNORE] The sidecar's block's parent (defined by block_header.parent_root) has been seen
// (via both gossip and non-gossip sources) (a client MAY queue sidecars for processing once the parent block is retrieved).
func (bv *ROColumnVerifier) SidecarParentSeen(parentSeen func([32]byte) bool) (err error) {
	defer bv.recordResult(RequireColumnSidecarParentSeen, &err)
	if parentSeen != nil && parentSeen(bv.column.ParentRoot()) {
		return nil
	}
	if bv.fc.HasNode(bv.column.ParentRoot()) {
		return nil
	}
	log.WithFields(logging.ColumnFields(bv.column)).Debug("parent root has not been seen")
	return ErrSidecarParentNotSeen
}

// SidecarParentValid represents the spec verification:
// [REJECT] The sidecar's block's parent (defined by block_header.parent_root) passes validation.
func (bv *ROColumnVerifier) SidecarParentValid(badParent func([32]byte) bool) (err error) {
	defer bv.recordResult(RequireColumnSidecarParentValid, &err)
	if badParent != nil && badParent(bv.column.ParentRoot()) {
		log.WithFields(logging.ColumnFields(bv.column)).Debug("parent root is invalid")
		return ErrSidecarParentInvalid
	}
	return nil
}

// SidecarParentSlotLower represents the spec verification:
// [REJECT] The sidecar is from a higher slot than the sidecar's block's parent (defined by block_header.parent_root).
func (bv *ROColumnVerifier) SidecarParentSlotLower() (err error) {
	defer bv.recordResult(RequireColumnSidecarParentSlotLower, &err)
	parentSlot, err := bv.fc.Slot(bv.column.ParentRoot())
	if err != nil {
		return errors.Wrap(ErrSlotNotAfterParent, "parent root not in forkchoice")
	}
	if parentSlot >= bv.column.Slot() {
		return ErrSlotNotAfterParent
	}
	return nil
}

// SidecarDescendsFromFinalized represents the spec verification:
// [REJECT] The current finalized_checkpoint is an ancestor of the sidecar's block
// -- i.e. get_checkpoint_block(store, block_header.parent_root, store.finalized_checkpoint.epoch) == store.finalized_checkpoint.root.
func (bv *ROColumnVerifier) SidecarDescendsFromFinalized() (err error) {
	defer bv.recordResult(RequireColumnSidecarDescendsFromFinalized, &err)
	if !bv.fc.HasNode(bv.column.ParentRoot()) {
		log.WithFields(logging.ColumnFields(bv.column)).Debug("parent root not in forkchoice")
		return ErrSidecarNotFinalizedDescendent
	}
	return nil
}

// SidecarInclusionProven represents the spec verification:
// [REJECT] The sidecar's inclusion proof is valid as verified by verify_column_sidecar_inclusion_proof(column_sidecar).
func (bv *ROColumnVerifier) SidecarInclusionProven() (err error) {
	defer bv.recordResult(RequireColumnSidecarInclusionProven, &err)
	if err = blocks.VerifyColumnKZGInclusionProof(bv.column); err != nil {
		log.WithError(err).WithFields(logging.ColumnFields(bv.column)).Debug("column sidecar inclusion proof verification failed")
		return ErrSidecarInclusionProofInvalid
	}
	return nil
}

// SidecarKzgProofVerified represents the spec verification:
// [REJECT] The sidecar's column is valid as verified by
// verify_column_kzg_proof(column_sidecar.column, column_sidecar.kzg_commitment, column_sidecar.kzg_proof).
func (bv *ROColumnVerifier) SidecarKzgProofVerified() (err error) {
	defer bv.recordResult(RequireColumnSidecarKzgProofVerified, &err)
	if err = bv.verifyColumnCommitment(bv.column); err != nil {
		log.WithError(err).WithFields(logging.ColumnFields(bv.column)).Debug("column kzg commitment proof verification failed")
		return ErrSidecarKzgProofInvalid
	}
	return nil
}

// SidecarProposerExpected represents the spec verification:
// [REJECT] The sidecar is proposed by the expected proposer_index for the block's slot
// in the context of the current shuffling (defined by block_header.parent_root/block_header.slot).
// If the proposer_index cannot immediately be verified against the expected shuffling, the sidecar MAY be queued
// for later processing while proposers for the block's branch are calculated -- in such a case do not REJECT, instead IGNORE this message.
func (bv *ROColumnVerifier) SidecarProposerExpected(ctx context.Context) (err error) {
	defer bv.recordResult(RequireColumnSidecarProposerExpected, &err)
	e := slots.ToEpoch(bv.column.Slot())
	if e > 0 {
		e = e - 1
	}
	r, err := bv.fc.TargetRootForEpoch(bv.column.ParentRoot(), e)
	if err != nil {
		return ErrSidecarUnexpectedProposer
	}
	c := &forkchoicetypes.Checkpoint{Root: r, Epoch: e}
	idx, cached := bv.pc.Proposer(c, bv.column.Slot())
	if !cached {
		pst, err := bv.parentState(ctx)
		if err != nil {
			log.WithError(err).WithFields(logging.ColumnFields(bv.column)).Debug("state replay to parent_root failed")
			return ErrSidecarUnexpectedProposer
		}
		idx, err = bv.pc.ComputeProposer(ctx, bv.column.ParentRoot(), bv.column.Slot(), pst)
		if err != nil {
			log.WithError(err).WithFields(logging.ColumnFields(bv.column)).Debug("error computing proposer index from parent state")
			return ErrSidecarUnexpectedProposer
		}
	}
	if idx != bv.column.ProposerIndex() {
		log.WithError(ErrSidecarUnexpectedProposer).
			WithFields(logging.ColumnFields(bv.column)).WithField("expectedProposer", idx).
			Debug("unexpected column proposer")
		return ErrSidecarUnexpectedProposer
	}
	return nil
}

func (bv *ROColumnVerifier) parentState(ctx context.Context) (state.BeaconState, error) {
	if bv.parent != nil {
		return bv.parent, nil
	}
	st, err := bv.sr.StateByRoot(ctx, bv.column.ParentRoot())
	if err != nil {
		return nil, err
	}
	bv.parent = st
	return bv.parent, nil
}

func columnToSignatureData(b blocks.ROColumn) SignatureData {
	return SignatureData{
		Root:      b.BlockRoot(),
		Parent:    b.ParentRoot(),
		Signature: bytesutil.ToBytes96(b.SignedBlockHeader.Signature),
		Proposer:  b.ProposerIndex(),
		Slot:      b.Slot(),
	}
}
