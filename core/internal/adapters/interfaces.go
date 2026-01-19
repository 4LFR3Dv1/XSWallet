// Package adapters - Interfaces for blockchain adapters
package adapters

import "context"

// BTCAdapter defines the interface for Bitcoin operations
type BTCAdapter interface {
	// Height returns the current blockchain height
	Height(ctx context.Context) (int64, error)

	// BroadcastTx broadcasts a raw transaction and returns the txid
	BroadcastTx(ctx context.Context, rawHex string) (string, error)

	// GetTx retrieves a transaction by txid
	GetTx(ctx context.Context, txid string) (*Tx, error)

	// EstimateFee estimates the fee rate in sat/vB for confirmation in `blocks` blocks
	EstimateFee(ctx context.Context, blocks int) (float64, error)

	// ListUnspent returns unspent outputs for the given addresses
	ListUnspent(ctx context.Context, minConf, maxConf int, addresses []string) ([]UTXO, error)
}

// LNAdapter defines the interface for Lightning operations
type LNAdapter interface {
	// GetInfo returns node information
	GetInfo(ctx context.Context) (*LNInfo, error)

	// WalletBalance returns the wallet balance in satoshis
	WalletBalance(ctx context.Context) (int64, error)

	// PayInvoice pays a BOLT11 invoice
	PayInvoice(ctx context.Context, bolt11 string) (*PayResult, error)

	// DecodeInvoice decodes a BOLT11 invoice
	DecodeInvoice(ctx context.Context, bolt11 string) (*Invoice, error)
}

// UTXO represents an unspent transaction output
type UTXO struct {
	TxID          string
	Vout          uint32
	Address       string
	AmountSat     int64
	Confirmations int64
	ScriptPubKey  string
}

// Tx represents a transaction
type Tx struct {
	TxID          string
	Confirmations int64
	BlockHash     string
	Hex           string
}

// LNInfo represents Lightning node info
type LNInfo struct {
	PubKey          string
	Alias           string
	NumActiveChannels int
	BlockHeight     int64
	SyncedToChain   bool
}

// PayResult represents the result of a payment
type PayResult struct {
	PaymentHash    string
	PaymentPreimage string
	AmountSat      int64
	FeeSat         int64
	Status         string
}

// Invoice represents a decoded BOLT11 invoice
type Invoice struct {
	PaymentHash string
	AmountSat   int64
	Description string
	Expiry      int64
	Destination string
}
