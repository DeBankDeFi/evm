package pre

import (
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"sync"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/rpc/backend"
	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// HandlerT keeps track of the cpu profiler and trace execution
type HandlerT struct {
	cpuFilename   string
	cpuFile       io.WriteCloser
	mu            sync.Mutex
	traceFilename string
	traceFile     io.WriteCloser
}

// API is the collection of tracing APIs exposed over the private debugging endpoint.
type API struct {
	ctx         *server.Context
	logger      log.Logger
	backend     *backend.Backend
	clientCtx   client.Context
	queryClient *rpctypes.QueryClient
	handler     *HandlerT
}

// NewAPI creates a new API definition for the tracing methods of the Ethereum service.
func NewAPI(
	logger log.Logger,
	backend *backend.Backend,
	clientCtx client.Context,
) *API {
	return &API{
		logger:      logger.With("module", "pre"),
		backend:     backend,
		clientCtx:   clientCtx,
		queryClient: rpctypes.NewQueryClient(clientCtx),
		handler:     new(HandlerT),
	}
}

// CallArgs represents arguments for a single call in a batch.
type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	ChainID  *big.Int        `json:"chainId,omitempty"`
	Tracer   string          `json:"tracer,omitempty"`
}

// TraceMany runs multiple trace calls and returns pre-execution results.
func (a *API) TraceMany(args []CallArgs) ([]evmtypes.PreResult, error) {
	rpcArgs := evmtypes.TransactionArgs{}
	for _, arg := range args {
		realArgs := evmtypes.TransactionArgs{
			From:     arg.From,
			To:       arg.To,
			Gas:      arg.Gas,
			GasPrice: arg.GasPrice,
			Value:    arg.Value,
			Data:     arg.Data,
			Tracer:   arg.Tracer,
			Nonce:    arg.Nonce,
		}
		rpcArgs.Args = append(rpcArgs.Args, realArgs)
	}

	bz, err := json.Marshal(&rpcArgs)
	if err != nil {
		a.logger.Error("json.Marshal failed", "args", args, "err", err)
		return nil, err
	}
	headBlock, _ := a.backend.CurrentHeader()
	blockHash := headBlock.Hash()

	blockRes, err := a.backend.CometBlockByNumber(rpctypes.EthLatestBlockNumber)
	if err != nil {
		return nil, errors.New("header not found")
	}

	// Use chain ID from config or 0 for latest
	tmp := big.NewInt(0)
	if blockRes.Block.ChainID != "" {
		_, _ = tmp.SetString(blockRes.Block.ChainID, 10)
	}
	chainID := sdkmath.NewIntFromBigInt(tmp)

	req := evmtypes.EthCallRequest{
		Args:            bz,
		GasCap:          a.backend.RPCGasCap(),
		ProposerAddress: sdk.ConsAddress(blockRes.Block.ProposerAddress),
		ChainId:         chainID.Int64(),
	}

	ctx := rpctypes.ContextWithHeight(rpctypes.EthLatestBlockNumber.Int64())
	res, err := a.queryClient.EthCall(ctx, &req)
	if err != nil {
		a.logger.Error("EthCall failed", "req", req, "err", err)
		return nil, err
	}
	var preResList []evmtypes.PreResult
	err = json.Unmarshal(res.Ret, &preResList)
	if err != nil {
		a.logger.Error("json.Unmarshal failed", "err", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	for i := range preResList {
		for j := range preResList[i].Logs {
			preResList[i].Logs[j].BlockHash = blockHash
		}
		for j := range preResList[i].Trace {
			preResList[i].Trace[j].BlockHash = blockHash
		}
	}
	return preResList, nil
}
