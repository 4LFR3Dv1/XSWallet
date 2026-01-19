// Package boltz - Testes para status.go
package boltz

import (
	"testing"

	"github.com/xs-wallet/xscore/internal/swap"
)

func TestNormalizeSubmarine_ClaimPending(t *testing.T) {
	// O trigger correto para assinar partial signature
	mapping := NormalizeSubmarine(StatusTxClaimPending)

	if mapping.State != swap.StateSigningMusig2Partial {
		t.Errorf("expected StateSigningMusig2Partial, got %s", mapping.State)
	}

	if mapping.Action != ActionSign {
		t.Errorf("expected ActionSign, got %d", mapping.Action)
	}

	if mapping.Terminal {
		t.Error("expected non-terminal")
	}
}

func TestNormalizeSubmarine_InvoicePaid_NotSignYet(t *testing.T) {
	// invoice.paid NÃO é o trigger para assinar - deve esperar transaction.claim.pending
	mapping := NormalizeSubmarine(StatusInvoicePaid)

	if mapping.State != swap.StateWaitingClaimDetails {
		t.Errorf("expected StateWaitingClaimDetails, got %s", mapping.State)
	}

	if mapping.Action != ActionWait {
		t.Errorf("expected ActionWait, got %d", mapping.Action)
	}
}

func TestNormalizeSubmarine_Claimed(t *testing.T) {
	mapping := NormalizeSubmarine(StatusTxClaimed)

	if mapping.State != swap.StateCompleted {
		t.Errorf("expected StateCompleted, got %s", mapping.State)
	}

	if !mapping.Terminal {
		t.Error("expected terminal")
	}
}

func TestNormalizeSubmarine_FailedToPay(t *testing.T) {
	mapping := NormalizeSubmarine(StatusInvoiceFailedToPay)

	if mapping.State != swap.StateFallbackScriptReady {
		t.Errorf("expected StateFallbackScriptReady, got %s", mapping.State)
	}

	if mapping.Action != ActionRefund {
		t.Errorf("expected ActionRefund, got %d", mapping.Action)
	}
}

func TestNormalizeReverse_InvoiceSettled(t *testing.T) {
	// invoice.settled é o status final de sucesso para reverse
	mapping := NormalizeReverse(StatusInvoiceSettled)

	if mapping.State != swap.StateCompleted {
		t.Errorf("expected StateCompleted, got %s", mapping.State)
	}

	if !mapping.Terminal {
		t.Error("expected terminal")
	}
}

func TestNormalizeReverse_TxMempool_Sign(t *testing.T) {
	// Quando Boltz lockup está no mempool, podemos claimar
	mapping := NormalizeReverse(StatusTxMempool)

	if mapping.State != swap.StateSigningMusig2Partial {
		t.Errorf("expected StateSigningMusig2Partial, got %s", mapping.State)
	}

	if mapping.Action != ActionSign {
		t.Errorf("expected ActionSign, got %d", mapping.Action)
	}
}

func TestNormalizeReverse_TxRefunded(t *testing.T) {
	// Boltz refundou seus próprios fundos
	mapping := NormalizeReverse(StatusTxRefunded)

	if mapping.State != swap.StateFailed {
		t.Errorf("expected StateFailed, got %s", mapping.State)
	}

	if !mapping.Terminal {
		t.Error("expected terminal")
	}
}

func TestNormalizeChain_ServerMempool(t *testing.T) {
	mapping := NormalizeChain(StatusServerMempool)

	if mapping.State != swap.StateSigningMusig2Partial {
		t.Errorf("expected StateSigningMusig2Partial, got %s", mapping.State)
	}

	if mapping.Action != ActionSign {
		t.Errorf("expected ActionSign, got %d", mapping.Action)
	}
}

func TestNormalize_Generic(t *testing.T) {
	// Testa função genérica
	subMapping := Normalize(StatusTxClaimPending, swap.KindSubmarine)
	if subMapping.Action != ActionSign {
		t.Errorf("submarine: expected ActionSign")
	}

	revMapping := Normalize(StatusInvoiceSettled, swap.KindReverse)
	if revMapping.State != swap.StateCompleted {
		t.Errorf("reverse: expected StateCompleted")
	}

	chainMapping := Normalize(StatusServerConfirmed, swap.KindChain)
	if chainMapping.Action != ActionSign {
		t.Errorf("chain: expected ActionSign")
	}
}

func TestIsActionRequired(t *testing.T) {
	if !IsActionRequired(ActionSign) {
		t.Error("ActionSign should require action")
	}

	if !IsActionRequired(ActionRefund) {
		t.Error("ActionRefund should require action")
	}

	if IsActionRequired(ActionWait) {
		t.Error("ActionWait should not require action")
	}

	if IsActionRequired(ActionNone) {
		t.Error("ActionNone should not require action")
	}
}

// Testes de todos os status do lifecycle
func TestAllSubmarineStatuses(t *testing.T) {
	testCases := []struct {
		status   string
		expected swap.State
		terminal bool
	}{
		{StatusSwapCreated, swap.StateLocked, false},
		{StatusTxMempool, swap.StateCommitStarted, false},
		{StatusTxConfirmed, swap.StateWaiting, false},
		{StatusInvoiceSet, swap.StateWaiting, false},
		{StatusInvoicePending, swap.StateWaiting, false},
		{StatusInvoicePaid, swap.StateWaitingClaimDetails, false},
		{StatusTxClaimPending, swap.StateSigningMusig2Partial, false},
		{StatusTxClaimed, swap.StateCompleted, true},
		{StatusInvoiceFailedToPay, swap.StateFallbackScriptReady, false},
		{StatusSwapExpired, swap.StateFailed, true},
		{StatusTxLockupFailed, swap.StateFallbackScriptReady, false},
	}

	for _, tc := range testCases {
		t.Run(tc.status, func(t *testing.T) {
			mapping := NormalizeSubmarine(tc.status)
			if mapping.State != tc.expected {
				t.Errorf("status %s: expected %s, got %s", tc.status, tc.expected, mapping.State)
			}
			if mapping.Terminal != tc.terminal {
				t.Errorf("status %s: expected terminal=%v, got %v", tc.status, tc.terminal, mapping.Terminal)
			}
		})
	}
}
