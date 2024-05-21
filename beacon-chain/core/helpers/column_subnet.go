package helpers

import (
	"math/big"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/holiman/uint256"
	"github.com/prysmaticlabs/prysm/v5/config/params"
)

type distIdx struct {
	idx  int
	dist *uint256.Int
}

// pick the <colRequired> nearest subnets to listen, by "nearest" we mean the xor distance
// the subnet id are distributed evenly in the uint256 id space
func SelectNearestColumnSubnets(nodeID enode.ID, subnetCount int, colRequired int) []int {
	nodeIDUint256 := uint256.NewInt(0).SetBytes(nodeID.Bytes())
	log2ColSubnetCount := params.BeaconConfig().ColumnSubnetCountLog2
	prefixBits := params.BeaconConfig().ColumnSubnetPrefixBits

	// make sure that none of the sunbet id is zero
	minimumSubnetIdDistance := big.NewInt(1)
	minimumSubnetIdDistance.Lsh(minimumSubnetIdDistance, 256-uint(log2ColSubnetCount+prefixBits)-1)
	subnetIdDistance := uint256.NewInt(0)
	subnetIdDistance.SetFromBig(minimumSubnetIdDistance)

	cols := make([]distIdx, subnetCount)
	for i := range cols {
		colId := uint256.NewInt(uint64(i + 1))
		// TODO: pre-calculate col ids
		colId.Mul(colId, subnetIdDistance)
		distance := uint256.NewInt(0).Xor(colId, nodeIDUint256)
		cols[i].idx = i
		cols[i].dist = distance
	}
	quickselect(cols, 0, len(cols)-1, colRequired)
	res := make([]int, colRequired)
	for i := range res {
		res[i] = cols[i].idx
	}
	return res
}

func ColumnId(subnetCount int, columnIndex int) *uint256.Int {
	log2ColSubnetCount := params.BeaconConfig().ColumnSubnetCountLog2
	prefixBits := params.BeaconConfig().ColumnSubnetPrefixBits
	minimumSubnetIdDistance := big.NewInt(1)
	minimumSubnetIdDistance.Lsh(minimumSubnetIdDistance, 256-uint(log2ColSubnetCount+prefixBits)-1)
	subnetIdDistance := uint256.NewInt(0)
	subnetIdDistance.SetFromBig(minimumSubnetIdDistance)
	colId := uint256.NewInt(uint64(columnIndex) + 1)
	colId.Mul(colId, subnetIdDistance)
	return colId
}

func quickselect(arr []distIdx, left, right, k int) {
	if left < right {
		pivotIndex := partition(arr, left, right)

		if pivotIndex == k {
			return
		} else if pivotIndex < k {
			quickselect(arr, pivotIndex+1, right, k)
		} else {
			quickselect(arr, left, pivotIndex-1, k)
		}
	}
}

func partition(arr []distIdx, left, right int) int {
	pivot := arr[right]
	i := left

	for j := left; j < right; j++ {
		if arr[j].dist.Lt(pivot.dist) {
			arr[i], arr[j] = arr[j], arr[i]
			i++
		}
	}

	arr[i], arr[right] = arr[right], arr[i]
	return i
}
