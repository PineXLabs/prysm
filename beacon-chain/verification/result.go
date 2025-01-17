package verification

// Requirement represents a validation check that needs to pass in order for a Verified form a consensus type to be issued.
type Requirement int

var unknownRequirementName = "unknown"

func (r Requirement) String() string {
	switch r {
	case RequireBlobIndexInBounds:
		return "RequireBlobIndexInBounds"
	case RequireNotFromFutureSlot:
		return "RequireNotFromFutureSlot"
	case RequireSlotAboveFinalized:
		return "RequireSlotAboveFinalized"
	case RequireValidProposerSignature:
		return "RequireValidProposerSignature"
	case RequireSidecarParentSeen:
		return "RequireSidecarParentSeen"
	case RequireSidecarParentValid:
		return "RequireSidecarParentValid"
	case RequireSidecarParentSlotLower:
		return "RequireSidecarParentSlotLower"
	case RequireSidecarDescendsFromFinalized:
		return "RequireSidecarDescendsFromFinalized"
	case RequireSidecarInclusionProven:
		return "RequireSidecarInclusionProven"
	case RequireSidecarKzgProofVerified:
		return "RequireSidecarKzgProofVerified"
	case RequireSidecarProposerExpected:
		return "RequireSidecarProposerExpected"
	case RequireColumnIndexInBounds:
		return "RequireColumnIndexInBounds"
	case RequireColumnSidecarParentSeen:
		return "RequireColumnSidecarParentSeen"
	case RequireColumnSidecarParentValid:
		return "RequireColumnSidecarParentValid"
	case RequireColumnSidecarParentSlotLower:
		return "RequireColumnSidecarParentSlotLower"
	case RequireColumnSidecarDescendsFromFinalized:
		return "RequireColumnSidecarDescendsFromFinalized"
	case RequireColumnSidecarInclusionProven:
		return "RequireColumnSidecarInclusionProven"
	case RequireColumnSidecarKzgProofVerified:
		return "RequireColumnSidecarInclusionProven"
	case RequireColumnSidecarProposerExpected:
		return "RequireColumnSidecarProposerExpected"
	default:
		return unknownRequirementName
	}
}

type requirementList []Requirement

func (rl requirementList) excluding(minus ...Requirement) []Requirement {
	rm := make(map[Requirement]struct{})
	nl := make([]Requirement, 0, len(rl)-len(minus))
	for i := range minus {
		rm[minus[i]] = struct{}{}
	}
	for i := range rl {
		if _, excluded := rm[rl[i]]; excluded {
			continue
		}
		nl = append(nl, rl[i])
	}
	return nl
}

// results collects positive verification results.
// This bitmap can be used to test which verifications have been successfully completed in order to
// decide whether it is safe to issue a "Verified" type variant.
type results struct {
	done map[Requirement]error
	reqs []Requirement
}

func newResults(reqs ...Requirement) *results {
	return &results{done: make(map[Requirement]error, len(reqs)), reqs: reqs}
}

func (r *results) record(req Requirement, err error) {
	r.done[req] = err
}

// allSatisfied returns true if there is a nil error result for every Requirement.
func (r *results) allSatisfied() bool {
	if len(r.done) != len(r.reqs) {
		return false
	}
	for i := range r.reqs {
		err, ok := r.done[r.reqs[i]]
		if !ok || err != nil {
			return false
		}
	}
	return true
}

func (r *results) executed(req Requirement) bool {
	_, ok := r.done[req]
	return ok
}

func (r *results) result(req Requirement) error {
	return r.done[req]
}

func (r *results) errors(err error) error {
	return newVerificationMultiError(r, err)
}

func (r *results) failures() map[Requirement]error {
	fail := make(map[Requirement]error, len(r.done))
	for i := range r.reqs {
		req := r.reqs[i]
		err, ok := r.done[req]
		if !ok {
			fail[req] = ErrMissingVerification
			continue
		}
		if err != nil {
			fail[req] = err
		}
	}
	return fail
}
