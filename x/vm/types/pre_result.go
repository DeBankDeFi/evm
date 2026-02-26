package types

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	PreErrorUnKnown            = 1000
	PreErrorInsufficientBalane = 1001
	PreErrorReverted           = 1002
)

type RpcLog struct {
	Address     string         `json:"address" gencodec:"required"`
	Topics      []string       `json:"topics" gencodec:"required"`
	Data        hexutil.Bytes  `json:"data" gencodec:"required"`
	BlockNumber hexutil.Uint64 `json:"blockNumber"`
	TxHash      string         `json:"transactionHash" gencodec:"required"`
	TxIndex     hexutil.Uint   `json:"transactionIndex"`
	BlockHash   common.Hash    `json:"blockHash"`
	Index       hexutil.Uint   `json:"logIndex"`
	Removed     bool           `json:"removed"`
}

type PreError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type PreResult struct {
	Trace     json.RawMessage `json:"trace"`
	Logs      []RpcLog        `json:"logs"`
	StateDiff StateDiff       `json:"stateDiff"`
	Error     PreError        `json:"error,omitempty"`
	GasUsed   uint64          `json:"gasUsed"`
}
