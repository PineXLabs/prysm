package testnet

import (
	"github.com/ethereum/go-ethereum/common"
	"strconv"
	"testing"
)

func TestDepositInGenesis(t *testing.T) {
	depositCount := strconv.FormatInt(90, 16)
	common.HexToHash(depositCount)
	t.Logf("depositCount %v", common.HexToHash(depositCount))
}
