package das

import (
	"context"
	"os"
	"testing"

	dill_das "github.com/PineXLabs/das"
	"github.com/libp2p/go-libp2p"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db/filesystem"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/blocks"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/testing/require"
)

func TestService(t *testing.T) {
	path := "/var/eth/"
	os.RemoveAll(path)
	h, err := libp2p.New()
	require.NoError(t, err)
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/10000")
	require.NoError(t, err)
	err = h.Network().Listen(addr)
	require.NoError(t, err)
	opts := []Option{}
	cs, err := filesystem.NewColumnStorage(filesystem.WithColumnBasePath(path), filesystem.WithColumnRetentionEpochs(4))
	require.NoError(t, err)
	opts = append(opts, WithColumnStorage(cs), WithHost(h), WithBootStrapNodes([]string{"/ip4/127.0.0.1/tcp/9999/p2p/12D3KooWKnspcuJ6cXX4puRU3Bx2ckceP2NpTGXghEx1buEj93Jc"}))
	s, err := NewService(context.Background(), opts...)
	require.NoError(t, err)
	s.Start()
	err = s.Status()
	require.NoError(t, err)

	blob := dill_das.Blob{}
	handle := dill_das.New()
	comm, err := handle.Commit(blob[:])
	require.NoError(t, err)
	comms := [][]byte{dill_das.MarshalCommitment(comm)}
	cols, err := handle.BlobsToColumns([][]byte{blob[:]}, comms)
	require.NoError(t, err)
	root := [32]byte{}
	for i := range cols {
		dataList := make([][]byte, 0)
		for _, data := range cols[i].SegmentDataList {
			dataList = append(dataList, data.Marshal())
		}
		proofs := [][]byte{dill_das.MarshalProof(&cols[i].Proofs[0])}
		sidecar := &ethpb.ColumnSidecar{
			Index:              uint64(i),
			Segments:           dataList,
			BlobKzgCommitments: comms,
			SegmentKzgProofs:   proofs,
			SignedBlockHeader: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					Slot:       555,
					ParentRoot: root[:],
					StateRoot:  root[:],
					BodyRoot:   root[:],
				},
				Signature: make([]byte, 96),
			},
			CommitmentInclusionProofs: []*ethpb.KzgCommitmentInclusionProof{},
			CommitmentsHash:           make([]byte, 32),
		}
		roCol, err := blocks.NewROColumnWithRoot(sidecar, root)
		require.NoError(t, err)
		vroCol := blocks.NewVerifiedROColumn(roCol)
		err = cs.Save(vroCol)
		require.NoError(t, err)
		s.NotifyColumnReceived(root, i)
	}
	err = s.Stop()
	require.NoError(t, err)
}
