package helpers

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/holiman/uint256"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v5/crypto/hash"
	"github.com/prysmaticlabs/prysm/v5/encoding/bytesutil"
)

type distIdx struct {
	idx  int
	dist *uint256.Int
}

// pick the <colRequired> nearest subnets to listen, by "nearest" we mean the xor distance
// the subnet id are distributed evenly in the uint256 id space
func selectNearestColumnSubnets(nodeID enode.ID, subnetCount int, colRequired int) []int {
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

func columnId(subnetCount int, columnIndex int) *uint256.Int {
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

func shuffledColumnIdsByNodeOffset(nodeOffset uint64, epoch primitives.Epoch) []primitives.ValidatorIndex {
	seed := hash.Hash(bytesutil.Bytes8((uint64(epoch)/params.BeaconConfig().EpochsPerColumnSubnetSubscription + nodeOffset)))
	subnetNumber := params.BeaconConfig().ColumnSubnetCount

	subnets := make([]primitives.ValidatorIndex, subnetNumber)
	for i := range subnets {
		subnets[i] = primitives.ValidatorIndex(i)
	}
	ShuffleList(subnets, seed)
	return subnets
}

func computePrefixForColumnSubnets(nodeID enode.ID) uint64 {
	num := uint256.NewInt(0).SetBytes(nodeID.Bytes())
	remBits := params.BeaconConfig().NodeIdBits - params.BeaconConfig().ColumnSubnetPrefixBits
	// Number of bits left will be representable by a uint64 value.
	nodeIdPrefix := num.Rsh(num, uint(remBits)).Uint64()
	return nodeIdPrefix
}

func computeOffsetForColumnSubnets(nodeID enode.ID) uint64 {
	num := uint256.NewInt(0).SetBytes(nodeID.Bytes())
	nodeOffset := num.Mod(num, uint256.NewInt(params.BeaconConfig().EpochsPerColumnSubnetSubscription)).Uint64()
	return nodeOffset
}

func ComputeColumnSubnetSubscriptionExpirationTime(nodeID enode.ID, epoch primitives.Epoch) time.Duration {
	prefix := computePrefixForColumnSubnets(nodeID)
	pastEpochs := (prefix + uint64(epoch)) % params.BeaconConfig().EpochsPerColumnSubnetSubscription
	remEpochs := params.BeaconConfig().EpochsPerColumnSubnetSubscription - pastEpochs
	epochDuration := time.Duration(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	epochTime := time.Duration(remEpochs) * epochDuration
	return epochTime * time.Second
}

func ComputeSubscribedColumnSubnets(nodeID enode.ID, epoch primitives.Epoch, extraRequired int) ([]uint64, error) {
	subs := []uint64{}
	subnetCount := params.BeaconConfig().ColumnSubnetCount
	nodeIDPrefix := computePrefixForColumnSubnets(nodeID)
	subnets := shuffledColumnIdsByNodeOffset(nodeIDPrefix, epoch)
	subnetRequired := int(params.BeaconConfig().BeaconColumnSubnetCustodyRequired)
	subnetRequired += extraRequired
	colIdxs := selectNearestColumnSubnets(nodeID, int(subnetCount), subnetRequired)
	for _, i := range colIdxs {
		subs = append(subs, uint64(subnets[i]))
	}
	return subs, nil
}

func ComputeColumnIds(columnIndex int, subnetCount int, epoch primitives.Epoch) []enode.ID {
	num := params.BeaconConfig().EpochsPerColumnSubnetSubscription
	columnIds := make([]enode.ID, 0, num)
	for i := range num {
		subnets := shuffledColumnIdsByNodeOffset(i, epoch)
		for idx, sub := range subnets {
			if columnIndex == int(sub) {
				id := columnId(subnetCount, idx)
				columnIds = append(columnIds, id.Bytes32())
			}
		}
	}
	return columnIds
}
