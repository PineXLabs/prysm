package p2p

import (
	"crypto/rand"
	"fmt"
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

func TestDistance(t *testing.T) {
	idBytes := make([]byte, 32)
	rand.Read(idBytes)
	nid1 := enode.ID([32]byte(idBytes))

	idBytes2 := make([]byte, 32)
	rand.Read(idBytes2)
	nid2 := enode.ID([32]byte(idBytes2))

	idBytes3 := make([]byte, 32)
	rand.Read(idBytes2)
	nid3 := enode.ID([32]byte(idBytes3))

	nid1Uint256 := uint256.NewInt(0).SetBytes(nid1.Bytes())
	nid2Uint256 := uint256.NewInt(0).SetBytes(nid2.Bytes())
	nid3Uint256 := uint256.NewInt(0).SetBytes(nid3.Bytes())
	dist12 := uint256.NewInt(0).Xor(nid1Uint256, nid2Uint256)
	dist13 := uint256.NewInt(0).Xor(nid1Uint256, nid3Uint256)
	ge := dist12.Gt(dist13)
	ge2 := enode.DistCmp(nid1, nid2, nid3) > 0
	require.Equal(t, ge, ge2)
}
