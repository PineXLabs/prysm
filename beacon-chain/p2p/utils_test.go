package p2p

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/holiman/uint256"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/testing/assert"
	"github.com/prysmaticlabs/prysm/v5/testing/require"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

// Test `verifyConnectivity` function by trying to connect to google.com (successfully)
// and then by connecting to an unreachable IP and ensuring that a log is emitted
func TestVerifyConnectivity(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	hook := logTest.NewGlobal()
	cases := []struct {
		address              string
		port                 uint
		expectedConnectivity bool
		name                 string
	}{
		{"142.250.68.46", 80, true, "Dialing a reachable IP: 142.250.68.46:80"}, // google.com
		{"123.123.123.123", 19000, false, "Dialing an unreachable IP: 123.123.123.123:19000"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf(tc.name),
			func(t *testing.T) {
				verifyConnectivity(tc.address, tc.port, "tcp")
				logMessage := "IP address is not accessible"
				if tc.expectedConnectivity {
					require.LogsDoNotContain(t, hook, logMessage)
				} else {
					require.LogsContain(t, hook, logMessage)
				}
			})
	}
}

func TestSerializeENR(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	t.Run("Ok", func(t *testing.T) {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		db, err := enode.OpenDB("")
		require.NoError(t, err)
		lNode := enode.NewLocalNode(db, key)
		record := lNode.Node().Record()
		s, err := SerializeENR(record)
		require.NoError(t, err)
		assert.NotEqual(t, "", s)
		s = "enr:" + s
		newRec, err := enode.Parse(enode.ValidSchemes, s)
		require.NoError(t, err)
		assert.Equal(t, s, newRec.String())
	})

	t.Run("Nil record", func(t *testing.T) {
		_, err := SerializeENR(nil)
		require.NotNil(t, err)
		assert.ErrorContains(t, "could not serialize nil record", err)
	})
}

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

		selected := selectNearestColumnSubnets(nid, offset, 64, 8)
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
