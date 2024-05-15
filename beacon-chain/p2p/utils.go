package p2p

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"path"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/holiman/uint256"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/wrapper"
	ecdsaprysm "github.com/prysmaticlabs/prysm/v5/crypto/ecdsa"
	"github.com/prysmaticlabs/prysm/v5/io/file"
	pb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1/metadata"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const keyPath = "network-keys"
const metaDataPath = "metaData"

const dialTimeout = 1 * time.Second

// SerializeENR takes the enr record in its key-value form and serializes it.
func SerializeENR(record *enr.Record) (string, error) {
	if record == nil {
		return "", errors.New("could not serialize nil record")
	}
	buf := bytes.NewBuffer([]byte{})
	if err := record.EncodeRLP(buf); err != nil {
		return "", errors.Wrap(err, "could not encode ENR record to bytes")
	}
	enrString := base64.RawURLEncoding.EncodeToString(buf.Bytes())
	return enrString, nil
}

// Determines a private key for p2p networking from the p2p service's
// configuration struct. If no key is found, it generates a new one.
func privKey(cfg *Config) (*ecdsa.PrivateKey, error) {
	defaultKeyPath := path.Join(cfg.DataDir, keyPath)
	privateKeyPath := cfg.PrivateKey

	// PrivateKey cli flag takes highest precedence.
	if privateKeyPath != "" {
		return privKeyFromFile(cfg.PrivateKey)
	}

	_, err := os.Stat(defaultKeyPath)
	defaultKeysExist := !os.IsNotExist(err)
	if err != nil && defaultKeysExist {
		return nil, err
	}
	// Default keys have the next highest precedence, if they exist.
	if defaultKeysExist {
		return privKeyFromFile(defaultKeyPath)
	}
	// There are no keys on the filesystem, so we need to generate one.
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	// If the StaticPeerID flag is set, save the generated key as the default
	// key, so that it will be used by default on the next node start.
	if cfg.StaticPeerID {
		rawbytes, err := priv.Raw()
		if err != nil {
			return nil, err
		}
		dst := make([]byte, hex.EncodedLen(len(rawbytes)))
		hex.Encode(dst, rawbytes)
		if err := file.WriteFile(defaultKeyPath, dst); err != nil {
			return nil, err
		}
		log.Infof("Wrote network key to file")
		// Read the key from the defaultKeyPath file just written
		// for the strongest guarantee that the next start will be the same as this one.
		return privKeyFromFile(defaultKeyPath)
	}
	return ecdsaprysm.ConvertFromInterfacePrivKey(priv)
}

// Retrieves a p2p networking private key from a file path.
func privKeyFromFile(path string) (*ecdsa.PrivateKey, error) {
	src, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		log.WithError(err).Error("Error reading private key from file")
		return nil, err
	}
	dst := make([]byte, hex.DecodedLen(len(src)))
	_, err = hex.Decode(dst, src)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode hex string")
	}
	unmarshalledKey, err := crypto.UnmarshalSecp256k1PrivateKey(dst)
	if err != nil {
		return nil, err
	}
	return ecdsaprysm.ConvertFromInterfacePrivKey(unmarshalledKey)
}

// Retrieves node p2p metadata from a set of configuration values
// from the p2p service.
// TODO: Figure out how to do a v1/v2 check.
func metaDataFromConfig(cfg *Config) (metadata.Metadata, error) {
	defaultKeyPath := path.Join(cfg.DataDir, metaDataPath)
	metaDataPath := cfg.MetaDataDir

	_, err := os.Stat(defaultKeyPath)
	defaultMetadataExist := !os.IsNotExist(err)
	if err != nil && defaultMetadataExist {
		return nil, err
	}
	if metaDataPath == "" && !defaultMetadataExist {
		metaData := &pb.MetaDataV0{
			SeqNumber: 0,
			Attnets:   bitfield.NewBitvector64(),
		}
		dst, err := proto.Marshal(metaData)
		if err != nil {
			return nil, err
		}
		if err := file.WriteFile(defaultKeyPath, dst); err != nil {
			return nil, err
		}
		return wrapper.WrappedMetadataV0(metaData), nil
	}
	if defaultMetadataExist && metaDataPath == "" {
		metaDataPath = defaultKeyPath
	}
	src, err := os.ReadFile(metaDataPath) // #nosec G304
	if err != nil {
		log.WithError(err).Error("Error reading metadata from file")
		return nil, err
	}
	metaData := &pb.MetaDataV0{}
	if err := proto.Unmarshal(src, metaData); err != nil {
		return nil, err
	}
	return wrapper.WrappedMetadataV0(metaData), nil
}

// Attempt to dial an address to verify its connectivity
func verifyConnectivity(addr string, port uint, protocol string) {
	if addr != "" {
		a := net.JoinHostPort(addr, fmt.Sprintf("%d", port))
		fields := logrus.Fields{
			"protocol": protocol,
			"address":  a,
		}
		conn, err := net.DialTimeout(protocol, a, dialTimeout)
		if err != nil {
			log.WithError(err).WithFields(fields).Warn("IP address is not accessible")
			return
		}
		if err := conn.Close(); err != nil {
			log.WithError(err).Debug("Could not close connection")
		}
	}
}

type distIdx struct {
	idx  int
	dist *uint256.Int
}

// pick the <colRequired> nearest subnets to listen, by "nearest" we mean the xor distance
// the subnet id are distributed evenly in the uint256 id space, but with an offset added
// so that a small portion of nodes are not able to controll a subnet id easily
func selectNearestColumnSubnets(nodeID enode.ID, offset *uint256.Int, subnetNum int, colRequired int) []int {
	nodeIDUint256 := uint256.NewInt(0).SetBytes(nodeID.Bytes())
	log2ColSubnetNum := math.Log2(float64(subnetNum))
	// we need to make sure that none of the sunbet id is zero or max(uint256)
	minimumSubnetIdDistance := big.NewInt(0).Exp(big.NewInt(2), big.NewInt(256-int64(log2ColSubnetNum)+1), nil)
	subnetIdDistance := uint256.NewInt(0)
	subnetIdDistance.SetFromBig(minimumSubnetIdDistance)
	halfDistance := uint256.NewInt(0).Div(subnetIdDistance, uint256.NewInt(2))
	// offset is not allowed to be greater than half of the smallest subnetid
	offset.Mod(offset, halfDistance)

	cols := make([]distIdx, subnetNum)
	for i := range cols {
		colId := uint256.NewInt(uint64(i + 1))
		// TODO: pre-calculate col ids
		colId.Mul(colId, subnetIdDistance).Add(colId, offset)
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
