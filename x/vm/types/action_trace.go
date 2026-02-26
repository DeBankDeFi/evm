package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Diff represents a storage slot change for state diff.
type Diff struct {
	BeforeValue *common.Hash `json:"before"`
	AfterValue  *common.Hash `json:"after"`
}

// AccountDiff is a map of storage slot changes for an account.
type AccountDiff map[common.Hash]Diff

// StateDiff is a map of account address to account storage diffs.
type StateDiff map[common.Address]AccountDiff

// ActionTrace represents single interaction with blockchain (OpenEthereum-style trace).
type ActionTrace struct {
	Subtraces           uint64      `json:"subtraces"`
	TraceAddress        []uint32    `json:"traceAddress"`
	TraceType           string      `json:"type"`
	Action              TAction     `json:"action"`
	Result              *TResult    `json:"result,omitempty"`
	Error               string      `json:"error,omitempty"`
	BlockHash           common.Hash `json:"blockHash,omitempty"`
	BlockNumber         int64       `json:"blockNumber"`
	TransactionHash     common.Hash `json:"transactionHash,omitempty"`
	TransactionPosition uint64      `json:"transactionPosition"`
}

// TAction represents the trace action model (Parity/OpenEthereum style).
type TAction struct {
	CallType      *string         `json:"callType,omitempty"`
	From          *common.Address `json:"from"`
	To            *common.Address `json:"to,omitempty"`
	Value         hexutil.Big     `json:"value"`
	Gas           hexutil.Uint64  `json:"gas"`
	Init          hexutil.Bytes   `json:"init,omitempty"`
	Input         hexutil.Bytes   `json:"input,omitempty"`
	Address       *common.Address `json:"address,omitempty"`
	RefundAddress *common.Address `json:"refundAddress,omitempty"`
	Balance       *hexutil.Big    `json:"balance,omitempty"`
}

// TResult holds information related to result of the processed transaction.
type TResult struct {
	GasUsed   hexutil.Uint64  `json:"gasUsed"`
	Output    *hexutil.Bytes  `json:"output,omitempty" rlp:"nil"`
	Code      hexutil.Bytes   `json:"code,omitempty"`
	Address   *common.Address `json:"address,omitempty" rlp:"nil"`
	RetOffset uint64          `json:"-" rlp:"-"`
	RetSize   uint64          `json:"-" rlp:"-"`
}

// ActionTraces is a slice of ActionTrace for RLP encoding (e.g. trace_transaction response).
type ActionTraces []ActionTrace
