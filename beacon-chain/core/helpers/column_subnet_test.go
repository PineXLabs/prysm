package helpers

import (
	"math"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/holiman/uint256"
	"github.com/prysmaticlabs/prysm/v5/testing/require"
)

type distIdxSlice []distIdx

func (s distIdxSlice) Len() int {
	return len(s)
}
func (s distIdxSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s distIdxSlice) Less(i, j int) bool {
	return s[i].dist.Lt(s[j].dist)
}

func TestSelectNearestColumnSunbet(t *testing.T) {
	t.Run("random nodeID", func(t *testing.T) {
		idBytes := make([]byte, 32)
		rand.Read(idBytes)
		nid := enode.ID([32]byte(idBytes))
		nodeIDUint256 := uint256.NewInt(0).SetBytes(nid.Bytes())

		offset := uint256.NewInt(0).SetBytes(nid[:])

		subnetNumber := 64
		log2ColSubnetNum := math.Log2(float64(subnetNumber))
		distance := big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256-int64(log2ColSubnetNum)+1), nil)
		subnetIdDist := uint256.NewInt(0)
		subnetIdDist.SetFromBig(distance)
		halfDistance := uint256.NewInt(0).Div(subnetIdDist, uint256.NewInt(2))
		offset.Mod(offset, halfDistance)

		selected := SelectNearestColumnSubnets(nid, offset, 64, 8)
		require.Equal(t, 8, len(selected))
		smap := make(map[int]struct{})
		for _, s := range selected {
			smap[s] = struct{}{}
		}
		cols := make(distIdxSlice, subnetNumber)
		for i := range cols {
			colId := uint256.NewInt(uint64(i + 1))
			colId.Mul(colId, subnetIdDist).Add(colId, offset)
			distance := uint256.NewInt(0).Xor(colId, nodeIDUint256)
			cols[i].idx = i
			cols[i].dist = distance
		}
		sort.Sort(distIdxSlice(cols))
		cols = cols[:8]
		for _, col := range cols {
			_, ok := smap[col.idx]
			require.Equal(t, true, ok)
		}
	})

}
