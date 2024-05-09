package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

import (
	"crypto/sha256"
	"errors"
)

func localsha256(data []byte) ([]byte, error) {
	s256 := sha256.New()
	sn, err := s256.Write(data)
	if err != nil {
		return nil, err
	}
	if sn != len(data) {
		return nil, errors.New("error data size")
	}
	return s256.Sum(nil), nil
}

func Sha256(x []byte) [32]byte {
	d, err := localsha256(x)
	if err != nil {
		panic(err)
	}
	return [32]byte(d)
}

func Hash(left [32]byte, right [32]byte) [32]byte {
	return [32]byte(Sha256(append(left[:], right[:]...)))
}

func Zero32() [32]byte {
	return [32]byte(bytes.Repeat([]byte{0x00}, 32))
}

type MerkleTree struct {
	DEPTH        int
	ZeroHashes   [][32]byte
	branch       [][32]byte
	depositCount int
}

func NewMerkleTree(depth int) *MerkleTree {
	m := &MerkleTree{depth, make([][32]byte, depth), make([][32]byte, depth), 0}
	m.InitMerkle()
	return m
}

func (m *MerkleTree) InitMerkle() {
	m.ZeroHashes[0] = Zero32()
	for i := 1; i < m.DEPTH; i++ {
		last := m.ZeroHashes[i-1]
		m.ZeroHashes[i] = Hash(last, last)
	}
	for i := 0; i < m.DEPTH; i++ {
		m.branch[i] = Zero32()
	}
}

func (m *MerkleTree) PrintZeroHashes() {
	for i := 0; i < m.DEPTH; i++ {
		fmt.Println(i, hex.EncodeToString(m.ZeroHashes[i][:]))
	}
}

func (m *MerkleTree) AddValue(index int, value [32]byte) {
	i := 0
	for (index+1)%(1<<(i+1)) == 0 {
		i++
	}
	for j := 0; j < i; j++ {
		value = Hash(m.branch[j], value)
	}
	m.branch[i] = value
	m.depositCount++
}

func (m *MerkleTree) BranchByBranch(values [][32]byte) [32]byte {
	copy(m.branch, m.ZeroHashes)
	for index, value := range values {
		m.AddValue(index, value)
	}
	return m.GetDepositDataRootOfIndex(len(values))
}

// 1、2、3 通过传入的merkleRoot&&count获得depositRoot
func (m *MerkleTree) GetDepositRoot(r [32]byte, ll uint64) ([]byte, [32]byte) {
	buf := new(bytes.Buffer)
	_, err := buf.Write(r[:])
	if err != nil {
		panic(err)
	}
	count := make([]byte, 8)
	binary.LittleEndian.PutUint64(count, ll)
	_, err = buf.Write(count)
	if err != nil {
		panic(err)
	}
	_, err = buf.Write(bytes.Repeat([]byte{0x00}, 24))
	if err != nil {
		panic(err)
	}
	return count, Sha256(buf.Bytes())
}

func (m *MerkleTree) GetDepositDataRootOfIndex(size int) [32]byte {
	r := Zero32()
	for h := 0; h < m.DEPTH; h++ {
		if (size>>h)%2 == 1 {
			r = Hash(m.branch[h], r)
		} else {
			r = Hash(r, m.ZeroHashes[h])
		}
	}
	return r
}

func (m *MerkleTree) GetDepositDataRoot() [32]byte {
	r := Zero32()
	for h := 0; h < m.DEPTH; h++ {
		if (m.depositCount>>h)%2 == 1 {
			r = Hash(m.branch[h], r)
		} else {
			r = Hash(r, m.ZeroHashes[h])
		}
	}
	return r
}

func (m *MerkleTree) VerifyProof(root, item [32]byte, merkleIndex uint64, proof [][32]byte) bool {
	node := item
	for h := uint64(0); h <= uint64(m.DEPTH); h++ {
		if ((merkleIndex / (1 << h)) % 2) != 0 {
			node = Hash(proof[h], node)
		} else {
			node = Hash(node, proof[h])
		}
	}
	return bytes.Equal(root[:], node[:])
}

func (m *MerkleTree) GetBranches() [][32]byte {
	return m.branch
}
