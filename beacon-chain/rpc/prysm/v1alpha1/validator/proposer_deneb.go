package validator

import (
	"errors"
	"sync"

	"github.com/PineXLabs/das"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/interfaces"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/crypto/hash"
	enginev1 "github.com/prysmaticlabs/prysm/v5/proto/engine/v1"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/runtime/version"
)

var bundleCache = &blobsBundleCache{}

// BlobsBundleCache holds the KZG commitments and other relevant sidecar data for a local beacon block.
type blobsBundleCache struct {
	sync.Mutex
	slot   primitives.Slot
	bundle *enginev1.BlobsBundle
}

// add adds a blobs bundle to the cache.
// same slot overwrites the previous bundle.
func (c *blobsBundleCache) add(slot primitives.Slot, bundle *enginev1.BlobsBundle) {
	c.Lock()
	defer c.Unlock()

	if slot >= c.slot {
		c.bundle = bundle
		c.slot = slot
	}
}

// get gets a blobs bundle from the cache.
func (c *blobsBundleCache) get(slot primitives.Slot) *enginev1.BlobsBundle {
	c.Lock()
	defer c.Unlock()

	if c.slot == slot {
		return c.bundle
	}

	return nil
}

// prune acquires the lock before pruning.
func (c *blobsBundleCache) prune(minSlot primitives.Slot) {
	c.Lock()
	defer c.Unlock()

	if minSlot > c.slot {
		c.slot = 0
		c.bundle = nil
	}
}

// buildBlobSidecars given a block, builds the blob sidecars for the block.
func buildBlobSidecars(blk interfaces.SignedBeaconBlock, blobs [][]byte, kzgProofs [][]byte) ([]*ethpb.BlobSidecar, error) {
	if blk.Version() < version.Deneb {
		return nil, nil // No blobs before deneb.
	}
	denebBlk, err := blk.PbDenebBlock()
	if err != nil {
		return nil, err
	}
	cLen := len(denebBlk.Block.Body.BlobKzgCommitments)
	if cLen != len(blobs) || cLen != len(kzgProofs) {
		return nil, errors.New("blob KZG commitments don't match number of blobs or KZG proofs")
	}
	blobSidecars := make([]*ethpb.BlobSidecar, cLen)
	header, err := blk.Header()
	if err != nil {
		return nil, err
	}
	body := blk.Block().Body()
	for i := range blobSidecars {
		proof, err := blocks.MerkleProofKZGCommitment(body, i)
		if err != nil {
			return nil, err
		}
		blobSidecars[i] = &ethpb.BlobSidecar{
			Index:                    uint64(i),
			Blob:                     blobs[i],
			KzgCommitment:            denebBlk.Block.Body.BlobKzgCommitments[i],
			KzgProof:                 kzgProofs[i],
			SignedBlockHeader:        header,
			CommitmentInclusionProof: proof,
		}
	}
	return blobSidecars, nil
}

// buildColumnSidecars given a block, builds the column sidecars for the block.
func buildColumnSidecars(blk interfaces.SignedBeaconBlock, blobs [][]byte, kzgProofs [][]byte) ([]*ethpb.ColumnSidecar, error) {
	if blk.Version() < version.Deneb {
		return nil, nil // No blobs before deneb.
	}
	denebBlk, err := blk.PbDenebBlock()
	if err != nil {
		return nil, err
	}
	cLen := len(denebBlk.Block.Body.BlobKzgCommitments)
	if cLen != len(blobs) || cLen != len(kzgProofs) {
		return nil, errors.New("blob KZG commitments don't match number of blobs or KZG proofs")
	}
	if cLen <= 0 {
		return nil, nil // No blobs in this block.
	}
	log.Debugf("In buildColumnSidecars, blob count is %d", len(blobs))

	_das := das.New()
	colSidecars, err := _das.BlobsToColumns(blobs, denebBlk.Block.Body.BlobKzgCommitments)
	if err != nil {
		log.Debugf("In buildColumnSidecars, BlobsToColumns failed, error is %s\n", err.Error())
		return nil, err
	}
	/*
		if len(colSidecars) > 0 {
			for i, com := range colSidecars[0].Commitments {
				comStr := fmt.Sprintf("0x%x", das.MarshalCommitment(&com))
				log.Debugf("commitment[%d] is %s", i, comStr)
			}
		}
	*/

	header, err := blk.Header()
	if err != nil {
		return nil, err
	}

	//var merkleProofs [][][]byte
	//for i := range kzgProofs {
	//      proof, err := blocks.MerkleProofKZGCommitment(body, i)
	//      if err != nil {
	//              return nil, err
	//      }
	//      merkleProofs = append(merkleProofs, proof)
	//}

	var commitConcat []byte
	for _, c := range denebBlk.Block.Body.BlobKzgCommitments {
		commitConcat = append(commitConcat, c...)
	}
	commitmentsHash := hash.Hash(commitConcat)

	body := blk.Block().Body()
	commitmentInclusionProofs := make([]*ethpb.KzgCommitmentInclusionProof, 0, cLen)
	//log.Debugf("commitmentInclusionProofs, len is %d, cap is %d", len(commitmentInclusionProofs), cap(commitmentInclusionProofs))
	for i := range denebBlk.Block.Body.BlobKzgCommitments {
		proof, err := blocks.MerkleProofKZGCommitment(body, i) //todo: generate merkle tree once
		//log.Debugf("proof for commitment %d is %v", i, proof)
		if err != nil {
			return nil, err
		}
		kProof := &ethpb.KzgCommitmentInclusionProof{
			CommitmentInclusionProof: proof,
		}
		//log.Debugf("kProof for commitment %d is %v", i, kProof)
		commitmentInclusionProofs = append(commitmentInclusionProofs, kProof)
	}
	//log.Debugf("commitmentInclusionProofs, len is %d, cap is %d", len(commitmentInclusionProofs), cap(commitmentInclusionProofs))

	columnSidecars := make([]*ethpb.ColumnSidecar, len(colSidecars))
	for i := range colSidecars {
		columnSidecars[i] = &ethpb.ColumnSidecar{
			Index:                     uint64(colSidecars[i].ColumnNumber),
			Segments:                  MarshalSegmentDataList(colSidecars[i].SegmentDataList),
			BlobKzgCommitments:        MarshalCommitments(colSidecars[i].Commitments),
			SegmentKzgProofs:          MarshalProofs(colSidecars[i].Proofs),
			SignedBlockHeader:         header,
			CommitmentInclusionProofs: commitmentInclusionProofs,
			CommitmentsHash:           commitmentsHash[:],
		}
	}
	return columnSidecars, nil
}

func MarshalSegmentDataList(segments []das.SegmentData) [][]byte {
	var segmentsBytes [][]byte
	for _, seg := range segments {
		segmentsBytes = append(segmentsBytes, seg.Marshal())
	}
	return segmentsBytes
}

func MarshalCommitments(commitments []das.Commitment) [][]byte {
	var commitmentsBytes [][]byte
	for _, com := range commitments {
		commitmentsBytes = append(commitmentsBytes, das.MarshalCommitment(&com))
	}
	return commitmentsBytes
}

func MarshalProofs(proofs []das.Proof) [][]byte {
	var proofsBytes [][]byte
	for _, proof := range proofs {
		proofsBytes = append(proofsBytes, das.MarshalProof(&proof))
	}
	return proofsBytes
}
