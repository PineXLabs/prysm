package helpers

import (
	"encoding/hex"
	"encoding/json"
	"github.com/prysmaticlabs/prysm/v5/io/file"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"os"
	"strings"
)

// Represents a json object of hex string and uint64 values for
// validators on Ethereum. This file can be generated using the official staking-deposit-cli.
type depositDataJSON struct {
	PubKey                string `json:"pubkey"`
	Amount                uint64 `json:"amount"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	DepositDataRoot       string `json:"deposit_data_root"`
	Signature             string `json:"signature"`
}

func ParseInitialValidators(initialValidatorsPath string) ([][]byte, []*ethpb.Deposit_Data, error) {
	expanded, err := file.ExpandPath(initialValidatorsPath)
	if err != nil {
		return nil, nil, err
	}
	b, err := os.ReadFile(expanded) // #nosec G304
	if err != nil {
		return nil, nil, err
	}
	return depositEntriesFromJSON(b)
}

func depositEntriesFromJSON(enc []byte) ([][]byte, []*ethpb.Deposit_Data, error) {
	var depositJSON []*depositDataJSON
	if err := json.Unmarshal(enc, &depositJSON); err != nil {
		return nil, nil, err
	}
	dds := make([]*ethpb.Deposit_Data, len(depositJSON))
	roots := make([][]byte, len(depositJSON))
	for i, val := range depositJSON {
		root, data, err := depositJSONToDepositData(val)
		if err != nil {
			return nil, nil, err
		}
		dds[i] = data
		roots[i] = root
	}
	return roots, dds, nil
}

func depositJSONToDepositData(input *depositDataJSON) ([]byte, *ethpb.Deposit_Data, error) {
	root, err := hex.DecodeString(strings.TrimPrefix(input.DepositDataRoot, "0x"))
	if err != nil {
		return nil, nil, err
	}
	pk, err := hex.DecodeString(strings.TrimPrefix(input.PubKey, "0x"))
	if err != nil {
		return nil, nil, err
	}
	creds, err := hex.DecodeString(strings.TrimPrefix(input.WithdrawalCredentials, "0x"))
	if err != nil {
		return nil, nil, err
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(input.Signature, "0x"))
	if err != nil {
		return nil, nil, err
	}
	return root, &ethpb.Deposit_Data{
		PublicKey:             pk,
		WithdrawalCredentials: creds,
		Amount:                input.Amount,
		Signature:             sig,
	}, nil
}
