package eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/cosmos/evm/rpc/backend"
	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type multiCallResp struct {
	Results []*callResult   `json:"results"`
	Stats   *multiCallStats `json:"stats"`
}

type callResult struct {
	Code      int           `json:"code"`
	Err       string        `json:"err"`
	FromCache bool          `json:"fromCache"`
	Result    hexutil.Bytes `json:"result"`
	GasUsed   int64         `json:"gasUsed"`
	TimeCost  float64       `json:"timeCost"`
}

type multiCallStats struct {
	BlockNum     int64       `json:"blockNum"`
	BlockHash    common.Hash `json:"blockHash"`
	BlockTime    int64       `json:"blockTime"`
	Success      bool        `json:"success"`
	CacheEnabled bool       `json:"cacheEnabled"`
}

const (
	singleCallTimeout = 5 * time.Second
	multiCallLimit    = 50

	errCodeTxArgs               = -40000
	errNativeMethodNotFound     = -40001
	errNativeMethodInput        = -40002
	errNativeMethodInputAddress = -40003

	errNativeMethodOutput     = -40010
	errNativeMethodStateError = -40011
	errMessageExecuting       = -40012
	errEVMCancelled           = -40013
	errEVMReverted            = -40014
	errEVMFastFailed          = -40015

	errUnderlyingDB = -40020
	errLoadingState = -40021
)

const (
	nativeAddr = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

var (
	abiUint8, _   = abi.NewType("uint8", "", nil)
	abiUint256, _ = abi.NewType("uint256", "", nil)
	abiString, _  = abi.NewType("string", "", nil)
	abiAddress, _ = abi.NewType("address", "", nil)

	erc20ABI = abi.ABI{
		Methods: map[string]abi.Method{
			"name":        funcName,
			"symbol":      funcSymbol,
			"decimals":    funcDecimals,
			"totalSupply": funcTotalSupply,
			"balanceOf":   funcBalanceOf,
		},
	}

	funcName = abi.NewMethod("name", "name", abi.Function, "", false, false,
		[]abi.Argument{},
		[]abi.Argument{{Name: "", Type: abiString, Indexed: false}},
	)
	funcSymbol = abi.NewMethod("symbol", "symbol", abi.Function, "", false, false,
		[]abi.Argument{},
		[]abi.Argument{{Name: "", Type: abiString, Indexed: false}},
	)
	funcDecimals = abi.NewMethod("decimals", "decimals", abi.Function, "", false, false,
		[]abi.Argument{},
		[]abi.Argument{{Name: "", Type: abiUint8, Indexed: false}},
	)
	funcTotalSupply = abi.NewMethod("totalSupply", "totalSupply", abi.Function, "", false, false,
		[]abi.Argument{},
		[]abi.Argument{{Name: "", Type: abiUint256, Indexed: false}},
	)
	funcBalanceOf = abi.NewMethod("balanceOf", "balanceOf", abi.Function, "", false, false,
		[]abi.Argument{{Name: "", Type: abiAddress, Indexed: false}},
		[]abi.Argument{{Name: "", Type: abiUint256, Indexed: false}},
	)
)

func handleNative(_ context.Context, b backend.EVMBackend, blockNrOrHash rpctypes.BlockNumberOrHash, arg evmtypes.TransactionArgs) ([]byte, int, error) {
	data := arg.GetData()
	method, err := erc20ABI.MethodById(data)
	if err != nil {
		return nil, errNativeMethodNotFound, err
	}
	switch method.Name {
	case "name", "symbol":
		res, err := method.Outputs.Pack("AURA")
		if err != nil {
			return nil, errNativeMethodOutput, err
		}
		return res, 0, nil
	case "decimals":
		res, err := method.Outputs.Pack(uint8(18))
		if err != nil {
			return nil, errNativeMethodOutput, err
		}
		return res, 0, nil
	case "totalSupply":
		res, err := method.Outputs.Pack(big.NewInt(1_000_000_000_000_000_000))
		if err != nil {
			return nil, errNativeMethodOutput, err
		}
		return res, 0, nil
	case "balanceOf":
		inputs, err := method.Inputs.Unpack(data[4:])
		if err != nil || len(inputs) == 0 {
			return nil, errNativeMethodInput, err
		}
		address, ok := inputs[0].(common.Address)
		if !ok {
			return nil, errNativeMethodInputAddress, fmt.Errorf("input address error")
		}
		balanceInt, err := b.GetBalance(address, blockNrOrHash)
		if err != nil {
			return nil, errNativeMethodStateError, err
		}
		balance, err := method.Outputs.Pack(balanceInt.ToInt())
		if err != nil {
			return nil, errNativeMethodOutput, err
		}
		return balance, 0, nil
	default:
		return nil, errNativeMethodNotFound, fmt.Errorf("method not found")
	}
}

func doOneCall(b backend.EVMBackend, blockNrOrHash rpctypes.BlockNumberOrHash, arg evmtypes.TransactionArgs) (*callResult, error) {
	var err error
	result := &callResult{}

	start := time.Now()
	defer func() {
		result.TimeCost = time.Since(start).Seconds()
	}()

	if arg.To != nil && strings.ToLower(arg.To.Hex()) == nativeAddr {
		res, code, nativeErr := handleNative(context.Background(), b, blockNrOrHash, arg)
		if nativeErr != nil {
			result.Code = code
			result.Err = nativeErr.Error()
		}
		result.Result = res
		return result, nativeErr
	}

	blockNum, err := b.BlockNumberFromComet(blockNrOrHash)
	if err != nil {
		result.Code = errUnderlyingDB
		result.Err = err.Error()
		return result, err
	}

	r, err := b.DoCall(arg, blockNum)
	if err != nil {
		result.Code = errMessageExecuting
		result.Err = err.Error()
		return result, err
	}

	result.Result = r.Ret
	result.GasUsed = int64(r.GasUsed)

	return result, nil
}

// MultiCall performs multiple raw contract calls in a single request.
func (e *PublicAPI) MultiCall(args []evmtypes.TransactionArgs, blockNrOrHash rpctypes.BlockNumberOrHash, pfastFail, puseParallel, pdisableCache *bool, _ *rpctypes.StateOverride) (resp *multiCallResp, err error) {
	e.logger.Debug("eth_multiCall", "args", args, "block number or hash", blockNrOrHash)

	if len(args) > multiCallLimit {
		return nil, fmt.Errorf("calls exceed limit, expected: <%v, actual: %v", multiCallLimit, len(args))
	}

	setb := func(p *bool, d bool) bool {
		if p == nil {
			return d
		}
		return *p
	}

	fastFail := setb(pfastFail, true)
	useParallel := setb(puseParallel, true)
	disableCache := setb(pdisableCache, false)

	ret := make([]*callResult, len(args))
	stats := &multiCallStats{
		Success:      true,
		CacheEnabled: !disableCache,
	}

	blockNum, err := e.backend.BlockNumberFromComet(blockNrOrHash)
	if err == nil {
		tmBlock, blockErr := e.backend.CometBlockByNumber(blockNum)
		if blockErr == nil {
			stats.BlockNum = tmBlock.Block.Height
			stats.BlockHash = common.BytesToHash(tmBlock.Block.Hash())
			stats.BlockTime = tmBlock.Block.Time.Unix()
		}
	}

	if useParallel {
		var wg sync.WaitGroup
		for i, arg := range args {
			wg.Add(1)
			go func(i int, arg evmtypes.TransactionArgs) {
				defer wg.Done()
				r, _ := doOneCall(e.backend, blockNrOrHash, arg)
				ret[i] = r
				if r.Err != "" {
					stats.Success = false
				}
			}(i, arg)
		}
		wg.Wait()
		return &multiCallResp{Results: ret, Stats: stats}, nil
	}

	failedOnce := false
	for i, arg := range args {
		if failedOnce {
			ret[i] = &callResult{}
			continue
		}
		r, _ := doOneCall(e.backend, blockNrOrHash, arg)
		ret[i] = r
		if r.Err != "" {
			stats.Success = false
			if fastFail {
				failedOnce = true
			}
		}
	}
	return &multiCallResp{Results: ret, Stats: stats}, nil
}
