package structs

type ColumnSidecarsResponse struct {
	Data []*ColumnSidecar `json:"data"`
}

type ColumnSidecar struct {
	Index                     string                         `json:"index"`
	Segments                  []string                       `json:"segments"`
	BlobKzgCommitments        []string                       `json:"blob_kzg_commitments"`
	SegmentKzgProofs          []string                       `json:"segment_kzg_proofs"`
	SignedBlockHeader         *SignedBeaconBlockHeader       `json:"signed_block_header"`
	CommitmentInclusionProofs []*KzgCommitmentInclusionProof `json:"commitment_inclusion_proofs"`
	CommitmentsHash           string                         `json:"commitments_hash"`
}

type KzgCommitmentInclusionProof struct {
	CommitmentInclusionProof []string `json:"commitment_inclusion_proof"`
}
