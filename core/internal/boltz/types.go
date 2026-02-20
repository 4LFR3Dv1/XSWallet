// Package boltz - Types para Boltz API v2
// Alinhados com OpenAPI spec oficial
package boltz

import (
	"encoding/json"
	"time"
)

// =============================================================================
// PAIR INFO (GET /v2/swap/submarine, /v2/swap/reverse)
// =============================================================================

type PairInfo struct {
	Hash   string     `json:"hash"`
	Rate   float64    `json:"rate"`
	Limits PairLimits `json:"limits"`
	Fees   PairFees   `json:"fees"`
}

type PairLimits struct {
	Minimal         int64 `json:"minimal"`
	Maximal         int64 `json:"maximal"`
	MaximalZeroConf int64 `json:"maximalZeroConf,omitempty"`
}

type PairFees struct {
	Percentage float64      `json:"percentage"`
	MinerFees  MinerFeesAny `json:"minerFees"`
}

// MinerFeesAny lida com campo polimórfico (number OU object)
type MinerFeesAny struct {
	Flat   *int64     // Quando é número (submarine)
	Detail *MinerFees // Quando é objeto (reverse/chain)
}

type MinerFees struct {
	Lockup int64 `json:"lockup"`
	Claim  int64 `json:"claim"`
}

func (m *MinerFeesAny) UnmarshalJSON(b []byte) error {
	// Tenta como número primeiro
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		m.Flat = &n
		return nil
	}
	// Senão, tenta como objeto
	var obj MinerFees
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	m.Detail = &obj
	return nil
}

func (m MinerFeesAny) MarshalJSON() ([]byte, error) {
	if m.Flat != nil {
		return json.Marshal(*m.Flat)
	}
	if m.Detail != nil {
		return json.Marshal(m.Detail)
	}
	return []byte("0"), nil
}

// Total retorna o total de miner fees
func (m MinerFeesAny) Total() int64 {
	if m.Flat != nil {
		return *m.Flat
	}
	if m.Detail != nil {
		return m.Detail.Lockup + m.Detail.Claim
	}
	return 0
}

// =============================================================================
// SUBMARINE SWAP (POST /v2/swap/submarine)
// =============================================================================

type SubmarineRequest struct {
	From            string `json:"from"`                      // "BTC" ou "L-BTC"
	To              string `json:"to"`                        // "BTC" (LN)
	Invoice         string `json:"invoice,omitempty"`         // BOLT11 invoice
	PreimageHash    string `json:"preimageHash,omitempty"`    // Se invoice não for fornecida
	RefundPublicKey string `json:"refundPublicKey,omitempty"` // Pubkey para refund
	PairHash        string `json:"pairHash,omitempty"`        // Verificação de fee drift
	ReferralID      string `json:"referralId,omitempty"`      // Tracking
}

type SubmarineResponse struct {
	ID                 string   `json:"id"`
	BIP21              string   `json:"bip21"`
	Address            string   `json:"address"`
	SwapTree           SwapTree `json:"swapTree"`
	ClaimPublicKey     string   `json:"claimPublicKey"`
	TimeoutBlockHeight int64    `json:"timeoutBlockHeight"`
	AcceptZeroConf     bool     `json:"acceptZeroConf"`
	ExpectedAmount     int64    `json:"expectedAmount"`
	BlindingKey        string   `json:"blindingKey,omitempty"` // Liquid only
	ReferralID         string   `json:"referralId,omitempty"`
}

type SwapTree struct {
	ClaimLeaf  TreeLeaf `json:"claimLeaf"`
	RefundLeaf TreeLeaf `json:"refundLeaf"`
}

type TreeLeaf struct {
	Version int    `json:"version"`
	Output  string `json:"output"` // Script hex
}

// =============================================================================
// REVERSE SWAP (POST /v2/swap/reverse)
// =============================================================================

type ReverseRequest struct {
	From           string `json:"from"`                     // "BTC" (LN)
	To             string `json:"to"`                       // "BTC" ou "L-BTC"
	PreimageHash   string `json:"preimageHash,omitempty"`   // SHA256(R), client-generated
	ClaimPublicKey string `json:"claimPublicKey,omitempty"` // Para claimar on-chain
	InvoiceAmount  int64  `json:"invoiceAmount,omitempty"`  // Valor da invoice
	OnchainAmount  int64  `json:"onchainAmount,omitempty"`  // Valor on-chain
	PairHash       string `json:"pairHash,omitempty"`
	ReferralID     string `json:"referralId,omitempty"`
	Address        string `json:"address,omitempty"` // BIP-21 direct payment
	ClaimCovenant  bool   `json:"claimCovenant,omitempty"`
}

type ReverseResponse struct {
	ID                 string   `json:"id"`
	Invoice            string   `json:"invoice"`
	SwapTree           SwapTree `json:"swapTree"`
	RefundPublicKey    string   `json:"refundPublicKey"`
	LockupAddress      string   `json:"lockupAddress"`
	TimeoutBlockHeight int64    `json:"timeoutBlockHeight"`
	OnchainAmount      int64    `json:"onchainAmount"`
	BlindingKey        string   `json:"blindingKey,omitempty"`
	ReferralID         string   `json:"referralId,omitempty"`
}

// =============================================================================
// CHAIN SWAP (POST /v2/swap/chain)
// =============================================================================

type ChainRequest struct {
	From            string `json:"from"`                       // "BTC" ou "L-BTC"
	To              string `json:"to"`                         // "BTC" ou "L-BTC"
	PreimageHash    string `json:"preimageHash,omitempty"`     // SHA256(R)
	MusigPubkeyAgg  string `json:"musig_pubkey_agg,omitempty"` // Legacy/alternate backend field name
	ClaimPublicKey  string `json:"claimPublicKey,omitempty"`   // claim key path
	RefundPublicKey string `json:"refundPublicKey,omitempty"`  // refund key path
	FromAmount      int64  `json:"fromAmount,omitempty"`       // legacy/alternate field name
	UserLockAmount  int64  `json:"userLockAmount,omitempty"`   // current docs/examples
	ToAmount        int64  `json:"toAmount,omitempty"`
	Address         string `json:"address,omitempty"`      // optional payout address
	ClaimAddress    string `json:"claimAddress,omitempty"` // docs/examples use this key
	PairHash        string `json:"pairHash,omitempty"`
	ReferralID      string `json:"referralId,omitempty"`
}

type ChainResponse struct {
	ID                 string            `json:"id"`
	LockupAddress      string            `json:"lockupAddress"`
	ClaimAddress       string            `json:"claimAddress,omitempty"`
	TimeoutBlockHeight int64             `json:"timeoutBlockHeight"`
	ExpectedAmount     int64             `json:"expectedAmount,omitempty"`
	BlindingKey        string            `json:"blindingKey,omitempty"`
	ReferralID         string            `json:"referralId,omitempty"`
	LockupDetails      *ChainSwapDetails `json:"lockupDetails,omitempty"`
	ClaimDetails       *ChainSwapDetails `json:"claimDetails,omitempty"`
}

type ChainSwapDetails struct {
	LockupAddress      string   `json:"lockupAddress,omitempty"`
	TimeoutBlockHeight int64    `json:"timeoutBlockHeight,omitempty"`
	ExpectedAmount     int64    `json:"expectedAmount,omitempty"`
	BlindingKey        string   `json:"blindingKey,omitempty"`
	ServerPublicKey    string   `json:"serverPublicKey,omitempty"`
	SwapTree           SwapTree `json:"swapTree,omitempty"`
}

func (r ChainResponse) EffectiveLockupAddress() string {
	if r.LockupAddress != "" {
		return r.LockupAddress
	}
	if r.LockupDetails != nil {
		return r.LockupDetails.LockupAddress
	}
	return ""
}

func (r ChainResponse) EffectiveTimeoutBlockHeight() int64 {
	if r.TimeoutBlockHeight > 0 {
		return r.TimeoutBlockHeight
	}
	if r.LockupDetails != nil && r.LockupDetails.TimeoutBlockHeight > 0 {
		return r.LockupDetails.TimeoutBlockHeight
	}
	return 0
}

func (r ChainResponse) EffectiveExpectedAmount() int64 {
	if r.ExpectedAmount > 0 {
		return r.ExpectedAmount
	}
	if r.LockupDetails != nil && r.LockupDetails.ExpectedAmount > 0 {
		return r.LockupDetails.ExpectedAmount
	}
	return 0
}

// =============================================================================
// STATUS (GET /v2/swap/{id}, WebSocket)
// =============================================================================

type SwapStatus struct {
	Status           string  `json:"status"`
	Transaction      *TxInfo `json:"transaction,omitempty"`
	FailureReason    string  `json:"failureReason,omitempty"`
	ZeroConfRejected bool    `json:"zeroConfRejected,omitempty"`
}

type TxInfo struct {
	ID  string `json:"id"`
	Hex string `json:"hex,omitempty"`
}

// =============================================================================
// CLAIM DETAILS
// =============================================================================

// SubmarineClaimDetails para GET /v2/swap/submarine/{id}/claim
type SubmarineClaimDetails struct {
	Preimage        string `json:"preimage"`
	PubNonce        string `json:"pubNonce"`
	PublicKey       string `json:"publicKey"`
	TransactionHash string `json:"transactionHash"`
}

// PartialSignature para POST claim
type PartialSignature struct {
	PubNonce         string `json:"pubNonce"`
	PartialSignature string `json:"partialSignature"`
}

// ReverseClaimRequest para POST /v2/swap/reverse/{id}/claim
type ReverseClaimRequest struct {
	Preimage    string `json:"preimage"`
	PubNonce    string `json:"pubNonce"`
	Transaction string `json:"transaction,omitempty"`
	Index       int    `json:"index,omitempty"`
}

// ReverseClaimResponse resposta do claim reverse
type ReverseClaimResponse struct {
	PubNonce         string `json:"pubNonce"`
	PartialSignature string `json:"partialSignature"`
}

// ChainClaimDetails para GET /v2/swap/chain/{id}/claim
type ChainClaimDetails struct {
	PubNonce        string `json:"pubNonce"`
	PublicKey       string `json:"publicKey"`
	TransactionHash string `json:"transactionHash"`
}

// =============================================================================
// WEBSOCKET
// =============================================================================

type WSSubscribeRequest struct {
	Op      string   `json:"op"`      // "subscribe"
	Channel string   `json:"channel"` // "swap.update"
	Args    []string `json:"args"`    // swap IDs
}

type WSMessage struct {
	Event   string        `json:"event"`   // "update"
	Channel string        `json:"channel"` // "swap.update"
	Args    []WSUpdateArg `json:"args"`
}

type WSUpdateArg struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	FailureReason string  `json:"failureReason,omitempty"`
	Transaction   *TxInfo `json:"transaction,omitempty"`
}

// =============================================================================
// INTERNAL TRACKING
// =============================================================================

type TrackedSwap struct {
	BoltzID    string
	LocalID    string
	LastStatus string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
