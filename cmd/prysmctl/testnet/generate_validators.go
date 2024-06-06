package testnet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
	"github.com/prysmaticlabs/prysm/v5/runtime/interop"
	"github.com/urfave/cli/v2"
	"os"
	"strconv"
)

var (
	generateValidatorsFlags = struct {
		NumValidators        uint64
		OutputValidatorsJSON string
		OutputValidatorsKeys string
	}{}

	generateValidatorsCmd = &cli.Command{
		Name:  "generate-validators",
		Usage: "Generate a set of validators",
		Action: func(cliCtx *cli.Context) error {
			if err := cliActionGenerateValidators(); err != nil {
				log.WithError(err).Fatal("Could not generate validators")
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "chain-config-file",
				Destination: &generateGenesisStateFlags.ChainConfigFile,
				Usage:       "The path to a YAML file with chain config values",
			},
			&cli.Uint64Flag{
				Name:        "num-validators",
				Usage:       "Number of validators to deterministically generate in the genesis state",
				Destination: &generateValidatorsFlags.NumValidators,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "output-validators-json",
				Destination: &generateValidatorsFlags.OutputValidatorsJSON,
				Usage:       "The path to persist validators data",
			},
			&cli.StringFlag{
				Name:        "output-validators-keys",
				Destination: &generateValidatorsFlags.OutputValidatorsKeys,
				Usage:       "The path to keep validators private keys",
			},
		},
	}
)

func cliActionGenerateValidators() error {
	if err := setGlobalParams(); err != nil {
		return fmt.Errorf("could not set config params: %v", err)
	}

	numValidators := generateValidatorsFlags.NumValidators
	outputValidatorsJsonFlag := generateValidatorsFlags.OutputValidatorsJSON
	outputValidatorsKeysFlag := generateValidatorsFlags.OutputValidatorsKeys
	if outputValidatorsKeysFlag == "" || outputValidatorsJsonFlag == "" {
		return fmt.Errorf("either missing output-validators-json or output-validators-keys flag")
	}
	sks, pks, err := generateValidators(int(numValidators))
	if err != nil {
		return err
	}

	for index, sk := range sks {
		var buffer bytes.Buffer
		buffer.WriteString(outputValidatorsKeysFlag)
		buffer.WriteString("validator-privateKey")
		buffer.WriteString(strconv.Itoa(index))
		buffer.WriteString(".txt")

		err = os.WriteFile(buffer.String(), []byte(hexutil.Encode(sk.Marshal())), os.ModeTemporary)
		if err != nil {
			return errors.Wrap(err, "could not persist validators' private key to local disk")
		}

	}

	dds, _, err := interop.DepositDataFromKeys(sks, pks)
	if err != nil {
		return errors.Wrap(err, "could not generate deposit data from keys")
	}

	jsonData := make([]*depositDataJSON, numValidators)
	for i := 0; i < int(numValidators); i++ {
		dataRoot, err := dds[i].HashTreeRoot()

		if err != nil {
			return errors.Wrap(err, "could not generate deposits from the deposit data provided")
		}

		jsonData[i] = &depositDataJSON{
			PubKey:                fmt.Sprintf("%#x", dds[i].PublicKey),
			Amount:                dds[i].Amount,
			WithdrawalCredentials: fmt.Sprintf("%#x", dds[i].WithdrawalCredentials),
			DepositDataRoot:       fmt.Sprintf("%#x", dataRoot),
			Signature:             fmt.Sprintf("%#x", dds[i].Signature),
		}
	}
	depositsDataJson, err := json.MarshalIndent(jsonData, "", "\t")
	if err != nil {
		return errors.Wrap(err, "could not marshal deposit json data")
	}

	err = os.WriteFile(outputValidatorsJsonFlag, depositsDataJson, os.ModeTemporary)
	if err != nil {
		return errors.Wrap(err, "could not persist deposit json to local disk")
	}
	return nil
}

func generateValidators(numValidators int) ([]bls.SecretKey, []bls.PublicKey, error) {
	privKeys := make([]bls.SecretKey, numValidators)
	pubKeys := make([]bls.PublicKey, numValidators)

	for i := 0; i < numValidators; i++ {
		sk, err := bls.RandKey()
		if err != nil {
			return nil, nil, err
		}
		privKeys[i] = sk
		pubKeys[i] = sk.PublicKey()
	}

	return privKeys, pubKeys, nil
}
