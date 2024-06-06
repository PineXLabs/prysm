package accounts

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/cmd/validator/flags"
	"github.com/prysmaticlabs/prysm/v5/contracts/deposit"
	"github.com/prysmaticlabs/prysm/v5/io/file"
	"github.com/urfave/cli/v2"
	"math/big"
	"os"
	"strings"
	"time"
)

func Deposit(c *cli.Context) (string, error) {
	if !c.IsSet(flags.ExecutionRPCProviderFlag.Name) || !c.IsSet(flags.DepositSignerPrivateKey.Name) || !c.IsSet(flags.DepositFileDirFlag.Name) {
		return "", errors.Errorf("Not enough deposit operation args found, please provide exeuction node rpc url via flag --%s "+
			"and a signer private key file via flag --%s and the new validator's deposit info via flag --%s ",
			flags.ExecutionRPCProviderFlag.Name,
			flags.DepositSignerPrivateKey.Name,
			flags.DepositFileDirFlag.Name,
		)

	}
	executionRPCProvider := c.String(flags.ExecutionRPCProviderFlag.Name)
	signerPriKey := c.String(flags.DepositSignerPrivateKey.Name)
	depositPath := c.String(flags.DepositFileDirFlag.Name)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(signerPriKey, "0x"))
	if err != nil {
		return "", errors.Wrap(err, "could not parse signer private key")
	}

	executionClient, err := ethclient.Dial(executionRPCProvider)
	if err != nil {
		return "", errors.Wrap(err, "could not init a execution client")
	}
	chainID, err := executionClient.ChainID(context.Background())
	if err != nil {
		return "", errors.Wrap(err, "could not query chainID of execution layer")
	}

	depositSigner, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(0).SetUint64(chainID.Uint64()))

	depositContract, err := deposit.NewDepositContract(common.HexToAddress("0x4242424242424242424242424242424242424242"), executionClient)
	if err != nil {
		return "", errors.Wrap(err, "could not build a deposit contract instance")
	}
	ds, err := parseValidatorInfo(depositPath)
	if err != nil {
		return "", errors.Wrap(err, "could not parse deposit data")
	}

	pk, err := hex.DecodeString(strings.TrimPrefix(ds.PubKey, "0x"))
	wc, err := hex.DecodeString(strings.TrimPrefix(ds.WithdrawalCredentials, "0x"))
	sig, err := hex.DecodeString(strings.TrimPrefix(ds.Signature, "0x"))
	depositDataRoot, err := hex.DecodeString(strings.TrimPrefix(ds.DepositDataRoot, "0x"))
	depositSigner.Value = new(big.Int)
	depositSigner.Value.SetString("32000000000000000000", 10)

	if err != nil {
		return "", errors.Wrap(err, "could not decode deposit data")
	}

	tx, err := depositContract.Deposit(depositSigner, pk, wc, sig, [32]byte(depositDataRoot))
	if err != nil {
		return "", errors.Wrap(err, "could not send a deposit tx")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, executionClient, tx)
	if err != nil {
		return "", err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return "", fmt.Errorf("deposit transaction has failed,  receipt: %+v. tx: %+v, gas: %v", receipt, tx, tx.Gas())
	}

	return tx.Hash().String(), nil
}

type depositDataJSON struct {
	PubKey                string `json:"pubkey"`
	Amount                uint64 `json:"amount"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	DepositDataRoot       string `json:"deposit_data_root"`
	Signature             string `json:"signature"`
}

func parseValidatorInfo(valFilePath string) (*depositDataJSON, error) {

	expanded, err := file.ExpandPath(valFilePath)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(expanded) // #nosec G304
	if err != nil {
		return nil, err
	}
	var ds depositDataJSON
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, err
	}
	return &ds, nil
}
