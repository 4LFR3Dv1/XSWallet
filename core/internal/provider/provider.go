// Package provider - Provider interface and types for swap providers
package provider

import (
	"context"
	"encoding/json"
	"time"
)

// Provider defines the interface for swap providers (Boltz, Mock, etc.)
type Provider interface {
	// Quote requests a quote for a swap
	Quote(ctx context.Context, req QuoteRequest) (*Quote, error)

	// Create creates a swap from a prepared intent
	Create(ctx context.Context, req CreateRequest) (*CreateResponse, error)

	// Subscribe returns a channel of updates for a swap
	Subscribe(ctx context.Context, swapID string) (<-chan Update, func(), error)

	// GetSwapStatus returns the current status of a swap
	GetSwapStatus(ctx context.Context, swapID string) (string, error)
}

// SwapKind represents the type of swap
type SwapKind string

const (
	SwapKindSubmarine SwapKind = "submarine"
	SwapKindReverse   SwapKind = "reverse"
	SwapKindChain     SwapKind = "chain"
)

// Chain represents a blockchain
type Chain string

const (
	ChainBTC    Chain = "BTC"
	ChainLiquid Chain = "L-BTC"
	ChainLN     Chain = "LN"
)

// QuoteRequest is a request for a swap quote
type QuoteRequest struct {
	Kind      SwapKind
	FromChain Chain
	ToChain   Chain
	AmountSat int64
	Invoice   string // For submarine swaps
	Address   string // For reverse/chain swaps
}

// Quote represents a provider quote
type Quote struct {
	QuoteID   string
	Kind      SwapKind
	FromChain Chain
	ToChain   Chain
	AmountSat int64
	Address   string // Optional payout address from quote request

	// Fees
	ProviderFeeSat int64
	NetworkFeeSat  int64
	TotalFeeSat    int64
	FeePercent     float64

	// Provider data
	LockupAddress string
	ClaimAddress  string
	Invoice       string

	// Timeouts
	UserTimeoutBlocks     int
	ProviderTimeoutBlocks int

	// Expiry
	ExpiresAt time.Time
}

// CreateRequest is a normalized swap creation payload used across providers.
type CreateRequest struct {
	QuoteID         string
	Kind            SwapKind
	FromChain       Chain
	ToChain         Chain
	AmountSat       int64
	Invoice         string
	Address         string
	PreimageHash    string
	MusigPubkeyAgg  string
	ClaimPublicKey  string
	RefundPublicKey string
	PairHash        string
	ReferralID      string
}

// CreateResponse is the response from creating a swap
type CreateResponse struct {
	SwapID             string
	BoltzID            string
	LockupAddress      string
	ClaimAddress       string
	ExpectedAmount     int64
	TimeoutBlockHeight int
	RedeemScript       string
	ClaimPublicKey     string
	RefundPublicKey    string
	BoltzRaw           json.RawMessage
	ReverseDetails     json.RawMessage
	LockupDetails      json.RawMessage
	ClaimDetails       json.RawMessage
}

// Update represents a swap status update
type Update struct {
	SwapID    string
	Status    string
	TxID      string
	Confirmed bool
	Error     string
	Timestamp time.Time
}

// Status constants
const (
	StatusCreated              = "swap.created"
	StatusTransactionMempool   = "transaction.mempool"
	StatusTransactionConfirmed = "transaction.confirmed"
	StatusInvoiceSet           = "invoice.set"
	StatusInvoicePending       = "invoice.pending"
	StatusInvoicePaid          = "invoice.paid"
	StatusCompleted            = "swap.completed"
	StatusExpired              = "swap.expired"
	StatusFailed               = "swap.failed"
)
