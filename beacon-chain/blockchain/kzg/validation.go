package kzg

import (
	"github.com/PineXLabs/das"
	GoKZG "github.com/crate-crypto/go-kzg-4844"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
)

var _das das.DasInterface

func init() {
	_das = das.New()
}

// Verify performs single or batch verification of commitments depending on the number of given BlobSidecars.
func Verify(sidecars ...blocks.ROBlob) error {
	if len(sidecars) == 0 {
		return nil
	}
	if len(sidecars) == 1 {
		return kzgContext.VerifyBlobKZGProof(
			bytesToBlob(sidecars[0].Blob),
			bytesToCommitment(sidecars[0].KzgCommitment),
			bytesToKZGProof(sidecars[0].KzgProof))
	}
	blobs := make([]GoKZG.Blob, len(sidecars))
	cmts := make([]GoKZG.KZGCommitment, len(sidecars))
	proofs := make([]GoKZG.KZGProof, len(sidecars))
	for i, sidecar := range sidecars {
		blobs[i] = bytesToBlob(sidecar.Blob)
		cmts[i] = bytesToCommitment(sidecar.KzgCommitment)
		proofs[i] = bytesToKZGProof(sidecar.KzgProof)
	}
	return kzgContext.VerifyBlobKZGProofBatch(blobs, cmts, proofs)
}

func bytesToBlob(blob []byte) (ret GoKZG.Blob) {
	copy(ret[:], blob)
	return
}

// VerifyColumns performs single or batch verification of commitments depending on the number of given ColumnSidecars.
func VerifyColumns(sidecars ...blocks.ROColumn) error {
	if len(sidecars) == 0 {
		return nil
	}
	for _, sidecar := range sidecars { //todo: batch verify
		err := verifyColumnKZGProof(sidecar)
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyColumnKZGProof(rc blocks.ROColumn) error {
	sampleColumn := das.SampleColumn{
		ColumnNumber: int(rc.ColumnSidecar.Index),
	}
	cs := rc.ColumnSidecar
	for i, seg := range cs.Segments {
		segData := make(das.SegmentData, 0)
		err := segData.Unmarshal(seg)
		if err != nil {
			return err
		}
		commitment, err := das.UnmarshalCommitment(cs.BlobKzgCommitments[i])
		if err != nil {
			return err
		}
		kzgProof, err := das.UnmarshalProof(cs.SegmentKzgProofs[i])
		if err != nil {
			return err
		}
		sampleColumn.SegmentDataList = append(sampleColumn.SegmentDataList, segData)
		sampleColumn.Commitments = append(sampleColumn.Commitments, *commitment)
		sampleColumn.Proofs = append(sampleColumn.Proofs, *kzgProof)
	}

	return _das.VerifyColumn(&sampleColumn)
}

func bytesToCommitment(commitment []byte) (ret GoKZG.KZGCommitment) {
	copy(ret[:], commitment)
	return
}

func bytesToKZGProof(proof []byte) (ret GoKZG.KZGProof) {
	copy(ret[:], proof)
	return
}
