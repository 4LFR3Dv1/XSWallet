// Package swap - Reconciliation logic for crash recovery
// ComputeFacts and NextAction determine state transitions based on objective facts
package swap

import (
	"context"
)

// Facts represents the objective state collected from external sources
type Facts struct {
	// Block height
	CurrentHeight int64

	// Transaction status
	LockupSeen      bool
	LockupConfirmed bool
	LockupConfs     int64

	// Provider status (from WebSocket updates)
	ProviderStatus string // "pending", "paid", "completed", "expired", "failed"

	// Timeouts
	TimeoutBlock        int64
	TimeoutApproaching  bool // Less than 10 blocks
	TimeoutReached      bool

	// Claim/refund status
	ClaimSeen   bool
	RefundSeen  bool
}

// Action represents the next action to take
type Action int

const (
	ActionNone Action = iota
	ActionWaitForLockup
	ActionTransitionToWaiting
	ActionTransitionToCompleted
	ActionTransitionToFailed
	ActionPrepareRefund
	ActionBroadcastRefund
)

// ComputeFacts gathers objective facts about a swap's current state
func ComputeFacts(ctx context.Context, swap *Swap, height int64, lockupTx interface{}, providerStatus string) Facts {
	facts := Facts{
		CurrentHeight:  height,
		ProviderStatus: providerStatus,
	}

	// Check lockup transaction status
	if swap.LockupTxid != "" {
		facts.LockupSeen = true
		// In a real implementation, we'd check confirmations from the adapter
		// For now, we assume if txid exists it's at least in mempool
	}

	// Check timeout
	if swap.TimeoutBlock > 0 {
		facts.TimeoutBlock = swap.TimeoutBlock
		blocksRemaining := swap.TimeoutBlock - height
		if blocksRemaining <= 10 {
			facts.TimeoutApproaching = true
		}
		if blocksRemaining <= 0 {
			facts.TimeoutReached = true
		}
	}

	// Map provider status
	switch providerStatus {
	case "completed", "swap.completed":
		facts.ClaimSeen = true
	case "failed", "swap.failed", "expired", "swap.expired":
		facts.RefundSeen = true
	}

	return facts
}

// NextAction determines what action to take based on current state and facts
// This is the core of the deterministic reconciliation logic
func NextAction(swap *Swap, facts Facts) (Action, State) {
	switch swap.State {
	case StateOpen:
		// Can only lock or cancel from open
		return ActionNone, StateOpen

	case StateLocked:
		// Ready to commit, waiting for user action
		return ActionNone, StateLocked

	case StateCommitStarted:
		// Funding tx broadcast, waiting for it to appear
		if facts.LockupSeen {
			return ActionTransitionToWaiting, StateWaiting
		}
		// If timeout reached without funding, fail
		if facts.TimeoutReached {
			return ActionTransitionToFailed, StateFailed
		}
		return ActionWaitForLockup, StateCommitStarted

	case StateWaiting:
		// Waiting for provider to complete
		if facts.ProviderStatus == "completed" || facts.ProviderStatus == "swap.completed" {
			return ActionTransitionToCompleted, StateCompleted
		}
		// If timeout approaching and not completed, prepare refund
		if facts.TimeoutApproaching {
			return ActionPrepareRefund, StateRefundCoopWaiting
		}
		return ActionNone, StateWaiting

	case StateWaitingClaimDetails, StateSigningMusig2Partial, StateSentPartialToProvider, StateWaitingProviderBroadcast:
		// MuSig2 flow states - for MVP, simplified to waiting for completion
		if facts.ProviderStatus == "completed" || facts.ClaimSeen {
			return ActionTransitionToCompleted, StateCompleted
		}
		if facts.TimeoutApproaching {
			return ActionPrepareRefund, StateFallbackScriptReady
		}
		return ActionNone, swap.State

	case StateRefundCoopWaiting:
		// Trying cooperative refund
		if facts.ProviderStatus == "completed" {
			return ActionTransitionToCompleted, StateCompleted
		}
		// If no response, go to fallback
		return ActionPrepareRefund, StateFallbackScriptReady

	case StateFallbackScriptReady:
		// Script-path refund ready
		if facts.TimeoutReached {
			return ActionBroadcastRefund, StateRefunding
		}
		return ActionNone, StateFallbackScriptReady

	case StateRefunding:
		// Refund tx broadcast
		if facts.RefundSeen {
			// Refund confirmed = completed (with refund)
			return ActionTransitionToCompleted, StateCompleted
		}
		return ActionNone, StateRefunding

	case StateCompleted, StateFailed, StateCanceled:
		// Terminal states - no action
		return ActionNone, swap.State

	default:
		return ActionNone, swap.State
	}
}

// ShouldReconcile returns true if the swap is in a non-terminal state
func ShouldReconcile(state State) bool {
	switch state {
	case StateCompleted, StateFailed, StateCanceled:
		return false
	default:
		return true
	}
}
