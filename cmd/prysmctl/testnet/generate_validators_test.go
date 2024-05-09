package testnet

import (
	"context"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/signing"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/container/trie"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls/common"
	"github.com/prysmaticlabs/prysm/v5/io/file"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/runtime/interop"
	"github.com/prysmaticlabs/prysm/v5/testing/assert"
	"github.com/prysmaticlabs/prysm/v5/testing/require"
	"os"
	"testing"
)

func TestGenerateValidators(t *testing.T) {
	sks, pks, err := generateValidators(10)
	assert.NoError(t, err)
	for i, sk := range sks {
		t.Logf("sk %v\n", hexutil.Encode(sk.Marshal()))
		assert.Equal(t, true, pks[i].Equals(sk.PublicKey()))
	}
	dds, _, _ := interop.DepositDataFromKeys(sks, pks)
	var sigs [][]byte
	var msgs [][32]byte

	domain, err := signing.ComputeDomain(params.BeaconConfig().DomainDeposit, nil, nil)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		depositMessage := &ethpb.DepositMessage{
			PublicKey:             pks[i].Marshal(),
			WithdrawalCredentials: dds[i].WithdrawalCredentials,
			Amount:                params.BeaconConfig().MaxEffectiveBalance,
		}
		sr, err := depositMessage.HashTreeRoot()
		root, err := (&ethpb.SigningData{ObjectRoot: sr[:], Domain: domain}).HashTreeRoot()
		assert.NoError(t, err)
		sigs = append(sigs, dds[i].Signature)
		msgs = append(msgs, root)
	}
	verify, err := bls.VerifyMultipleSignatures(sigs, msgs, pks)
	require.NoError(t, err)
	t.Logf("verify %v\n", verify)
}

func TestLoadValidators(t *testing.T) {
	generateGenesisStateFlags.ChainConfigFile = "../config.yml"
	err := setGlobalParams()
	require.NoError(t, err)
	roots, dds, err := getReadValidators("../validators.json")
	require.NoError(t, err)
	var sigs [][]byte
	var msgs [][32]byte
	var pks []common.PublicKey
	domain, err := signing.ComputeDomain(params.BeaconConfig().DomainDeposit, nil, nil)
	require.NoError(t, err)

	for i := 0; i < 1; i++ {
		depositMessage := &ethpb.DepositMessage{
			PublicKey:             dds[i].PublicKey,
			WithdrawalCredentials: dds[i].WithdrawalCredentials,
			Amount:                dds[i].Amount,
		}
		sr, err := signing.ComputeSigningRoot(depositMessage, domain)
		assert.NoError(t, err)
		sigs = append(sigs, dds[i].Signature)
		msgs = append(msgs, sr)
		pubkey, _ := bls.PublicKeyFromBytes(dds[i].PublicKey)
		pks = append(pks, pubkey)
	}
	verify, err := bls.VerifyMultipleSignatures(sigs, msgs, pks)
	require.NoError(t, err)
	assert.Equal(t, verify, true)

	trie, err := trie.GenerateTrieFromItems(roots, params.BeaconConfig().DepositContractTreeDepth)
	require.NoError(t, err)
	deposits, err := interop.GenerateDepositsFromData(dds, trie)
	require.NoError(t, err)
	verify2, err := blocks.BatchVerifyDepositsSignatures(context.Background(), deposits)
	require.NoError(t, err)
	assert.Equal(t, verify2, false)
}

func getReadValidators(depositJsonPath string) ([][]byte, []*ethpb.Deposit_Data, error) {
	expanded, err := file.ExpandPath(depositJsonPath)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("reading deposits from JSON at %s", expanded)
	b, err := os.ReadFile(expanded) // #nosec G304
	if err != nil {
		return nil, nil, err
	}
	roots, dds, err := depositEntriesFromJSON(b)
	if err != nil {
		return nil, nil, err
	}
	return roots, dds, nil
}
