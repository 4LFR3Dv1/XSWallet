// Package boltz - Status normalization para Boltz API v2
// Baseado no lifecycle oficial: api.docs.boltz.exchange/lifecycle
package boltz

import "github.com/xs-wallet/xscore/internal/swap"

// =============================================================================
// STATUS DO LIFECYCLE BOLTZ
// =============================================================================

// Submarine Swap States
const (
	StatusSwapCreated        = "swap.created"
	StatusTxMempool          = "transaction.mempool"
	StatusTxConfirmed        = "transaction.confirmed"
	StatusInvoiceSet         = "invoice.set"
	StatusInvoicePending     = "invoice.pending"
	StatusInvoicePaid        = "invoice.paid"
	StatusInvoiceFailedToPay = "invoice.failedToPay"
	StatusTxClaimPending     = "transaction.claim.pending" // TRIGGER para assinar!
	StatusTxClaimed          = "transaction.claimed"
	StatusSwapExpired        = "swap.expired"
	StatusTxLockupFailed     = "transaction.lockupFailed"

	// Reverse Swap States
	StatusMinerFeePaid   = "minerfee.paid"
	StatusInvoiceSettled = "invoice.settled" // FINAL sucesso reverse!
	StatusInvoiceExpired = "invoice.expired"
	StatusTxFailed       = "transaction.failed"
	StatusTxRefunded     = "transaction.refunded"

	// Chain Swap States
	StatusServerMempool   = "transaction.server.mempool"
	StatusServerConfirmed = "transaction.server.confirmed"
)

// SwapAction indica ação requerida pelo engine
type SwapAction int

const (
	ActionNone   SwapAction = iota
	ActionWait              // Aguardar próximo evento
	ActionSign              // Hora de criar partial signature (MuSig2)
	ActionRefund            // Iniciar processo de refund
)

// StatusMapping contém o mapeamento para cada tipo de swap
type StatusMapping struct {
	State    swap.State
	Action   SwapAction
	Terminal bool
	Trigger  string // Para log de eventos
}

// NormalizeSubmarine normaliza status para Submarine Swap
// Submarine: on-chain BTC/L-BTC → Lightning
func NormalizeSubmarine(boltzStatus string) StatusMapping {
	switch boltzStatus {
	case StatusSwapCreated:
		return StatusMapping{
			State:   swap.StateLocked,
			Action:  ActionWait,
			Trigger: "boltz:swap_created",
		}

	case StatusTxMempool:
		return StatusMapping{
			State:   swap.StateCommitStarted,
			Action:  ActionWait,
			Trigger: "boltz:tx_mempool",
		}

	case StatusTxConfirmed:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:tx_confirmed",
		}

	case StatusInvoiceSet:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:invoice_set",
		}

	case StatusInvoicePending:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:invoice_pending",
		}

	case StatusInvoicePaid:
		// Invoice paga, mas ainda NÃO é hora de assinar!
		// Aguardar transaction.claim.pending
		return StatusMapping{
			State:   swap.StateWaitingClaimDetails,
			Action:  ActionWait,
			Trigger: "boltz:invoice_paid",
		}

	case StatusTxClaimPending:
		// ESTE é o trigger para criar partial signature!
		return StatusMapping{
			State:   swap.StateSigningMusig2Partial,
			Action:  ActionSign,
			Trigger: "boltz:claim_pending",
		}

	case StatusTxClaimed:
		return StatusMapping{
			State:    swap.StateCompleted,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:claimed",
		}

	case StatusInvoiceFailedToPay:
		return StatusMapping{
			State:   swap.StateFallbackScriptReady,
			Action:  ActionRefund,
			Trigger: "boltz:invoice_failed",
		}

	case StatusSwapExpired:
		return StatusMapping{
			State:    swap.StateFailed,
			Action:   ActionRefund,
			Terminal: true,
			Trigger:  "boltz:expired",
		}

	case StatusTxLockupFailed:
		return StatusMapping{
			State:   swap.StateFallbackScriptReady,
			Action:  ActionRefund,
			Trigger: "boltz:lockup_failed",
		}

	default:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:unknown:" + boltzStatus,
		}
	}
}

// NormalizeReverse normaliza status para Reverse Swap
// Reverse: Lightning → on-chain BTC/L-BTC
func NormalizeReverse(boltzStatus string) StatusMapping {
	switch boltzStatus {
	case StatusSwapCreated:
		return StatusMapping{
			State:   swap.StateLocked,
			Action:  ActionWait,
			Trigger: "boltz:swap_created",
		}

	case StatusMinerFeePaid:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:minerfee_paid",
		}

	case StatusTxMempool:
		// Boltz lockup no mempool - podemos claimar!
		return StatusMapping{
			State:   swap.StateSigningMusig2Partial,
			Action:  ActionSign,
			Trigger: "boltz:tx_mempool",
		}

	case StatusTxConfirmed:
		// Boltz lockup confirmado - definitivamente claimar
		return StatusMapping{
			State:   swap.StateSigningMusig2Partial,
			Action:  ActionSign,
			Trigger: "boltz:tx_confirmed",
		}

	case StatusInvoiceSettled:
		// FINAL SUCESSO para Reverse!
		return StatusMapping{
			State:    swap.StateCompleted,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:invoice_settled",
		}

	case StatusInvoiceExpired:
		return StatusMapping{
			State:    swap.StateFailed,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:invoice_expired",
		}

	case StatusTxFailed:
		return StatusMapping{
			State:    swap.StateFailed,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:tx_failed",
		}

	case StatusTxRefunded:
		// Boltz refundou seus próprios fundos (usuário não clamou a tempo)
		return StatusMapping{
			State:    swap.StateFailed,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:tx_refunded",
		}

	case StatusSwapExpired:
		return StatusMapping{
			State:    swap.StateFailed,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:expired",
		}

	default:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:unknown:" + boltzStatus,
		}
	}
}

// NormalizeChain normaliza status para Chain Swap
// Chain: BTC ↔ L-BTC
func NormalizeChain(boltzStatus string) StatusMapping {
	switch boltzStatus {
	case StatusSwapCreated:
		return StatusMapping{
			State:   swap.StateLocked,
			Action:  ActionWait,
			Trigger: "boltz:swap_created",
		}

	case StatusTxMempool:
		return StatusMapping{
			State:   swap.StateCommitStarted,
			Action:  ActionWait,
			Trigger: "boltz:tx_mempool",
		}

	case StatusTxConfirmed:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:tx_confirmed",
		}

	case StatusServerMempool:
		return StatusMapping{
			State:   swap.StateSigningMusig2Partial,
			Action:  ActionSign,
			Trigger: "boltz:server_mempool",
		}

	case StatusServerConfirmed:
		return StatusMapping{
			State:   swap.StateSigningMusig2Partial,
			Action:  ActionSign,
			Trigger: "boltz:server_confirmed",
		}

	case StatusTxClaimed:
		return StatusMapping{
			State:    swap.StateCompleted,
			Action:   ActionNone,
			Terminal: true,
			Trigger:  "boltz:claimed",
		}

	case StatusSwapExpired:
		return StatusMapping{
			State:    swap.StateFailed,
			Action:   ActionRefund,
			Terminal: true,
			Trigger:  "boltz:expired",
		}

	case StatusTxLockupFailed:
		return StatusMapping{
			State:   swap.StateFallbackScriptReady,
			Action:  ActionRefund,
			Trigger: "boltz:lockup_failed",
		}

	default:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "boltz:unknown:" + boltzStatus,
		}
	}
}

// Normalize é a função genérica que escolhe o normalizer correto
func Normalize(boltzStatus string, kind swap.Kind) StatusMapping {
	switch kind {
	case swap.KindSubmarine:
		return NormalizeSubmarine(boltzStatus)
	case swap.KindReverse:
		return NormalizeReverse(boltzStatus)
	case swap.KindChain:
		return NormalizeChain(boltzStatus)
	default:
		return StatusMapping{
			State:   swap.StateWaiting,
			Action:  ActionWait,
			Trigger: "unknown_kind",
		}
	}
}

// IsActionRequired retorna true se estado requer ação imediata
func IsActionRequired(action SwapAction) bool {
	return action == ActionSign || action == ActionRefund
}
