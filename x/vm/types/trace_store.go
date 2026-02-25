package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// TraceStore contains all the methods for tx-trace to interact with the underlying database.
type TraceStore interface {
	// ReadTxTrace retrieve tracing result from underlying database.
	ReadTxTrace(ctx sdk.Context, txHash common.Hash) ([]byte, error)
	// WriteTxTrace write tracing result to underlying database.
	WriteTxTrace(ctx sdk.Context, txHash common.Hash, trace []byte) error
}
