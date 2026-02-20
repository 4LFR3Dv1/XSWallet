package watcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/xs-wallet/xscore/internal/boltz"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/swapkey"
)

// TestReverseExecutionToWaitingProviderBroadcastIntegration creates a real reverse swap on Boltz
// testnet and exercises watcher reconciliation until reverse signing path is executed.
//
// Run:
//
//	XS_BOLTZ_REVERSE_EXEC_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/watcher -run TestReverseExecutionToWaitingProviderBroadcastIntegration -v
func TestReverseExecutionToWaitingProviderBroadcastIntegration(t *testing.T) {
	if os.Getenv("XS_BOLTZ_REVERSE_EXEC_INTEGRATION") != "1" {
		t.Skip("set XS_BOLTZ_REVERSE_EXEC_INTEGRATION=1 to run real reverse execution integration")
	}

	baseURL := os.Getenv("XS_BOLTZ_API_URL")
	if baseURL == "" {
		baseURL = "https://api.testnet.boltz.exchange"
	}
	wsURL := os.Getenv("XS_BOLTZ_WS_URL")
	if wsURL == "" {
		wsURL = "wss://api.testnet.boltz.exchange/v2/ws"
	}
	waitTimeout := envInt("XS_BOLTZ_REVERSE_EXEC_WAIT_SECONDS", 900)
	pollSeconds := envInt("XS_BOLTZ_REVERSE_EXEC_POLL_SECONDS", 10)
	simulateTrigger := os.Getenv("XS_BOLTZ_REVERSE_EXEC_SIMULATE_TRIGGER") == "1"
	simulateTerminal := os.Getenv("XS_BOLTZ_REVERSE_EXEC_SIMULATE_TERMINAL")
	fullReal := os.Getenv("XS_BOLTZ_REVERSE_EXEC_FULL_REAL") == "1"
	targetTerminal := expectedTerminalState(os.Getenv("XS_BOLTZ_REVERSE_EXEC_EXPECT_TERMINAL"), swap.StateCompleted)
	if fullReal && (simulateTrigger || simulateTerminal != "") {
		t.Fatalf("FULL_REAL requires no simulation flags: unset XS_BOLTZ_REVERSE_EXEC_SIMULATE_TRIGGER and XS_BOLTZ_REVERSE_EXEC_SIMULATE_TERMINAL")
	}

	database := openWatcherIntegrationDB(t)
	vault := integrationSeedVault{}
	engine := swap.NewEngine(database, vault)
	ctx := context.Background()

	prov, err := boltz.NewProvider(boltz.Config{
		BaseURL: baseURL,
		WSURL:   wsURL,
		Network: "testnet",
	})
	if err != nil {
		t.Fatalf("create boltz provider: %v", err)
	}
	defer prov.Close()
	execProv := &integrationProviderOverride{base: prov}
	if simulateTrigger {
		execProv.forcedStatus = boltz.StatusTxConfirmed
		execProv.simulateExec = true
	}

	client := boltz.NewClient(boltz.ClientConfig{BaseURL: baseURL, Timeout: 30 * time.Second})
	amountSat, err := resolveReverseAmountForExecution(ctx, client, "BTC", "BTC")
	if err != nil {
		t.Fatalf("resolve reverse amount: %v", err)
	}

	s, err := engine.Create(ctx, swap.KindReverse, "testnet", 13)
	if err != nil {
		t.Fatalf("create local swap: %v", err)
	}

	seed, err := vault.Seed()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, claimPub, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex), "testnet")
	if err != nil {
		t.Fatalf("derive claim key: %v", err)
	}
	_, refundPub, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex+1), "testnet")
	if err != nil {
		t.Fatalf("derive refund key: %v", err)
	}
	claimHex := fmt.Sprintf("%x", claimPub.SerializeCompressed())
	refundHex := fmt.Sprintf("%x", refundPub.SerializeCompressed())

	createResp, err := execProv.Create(ctx, provider.CreateRequest{
		Kind:            provider.SwapKindReverse,
		FromChain:       provider.ChainLN,
		ToChain:         provider.ChainBTC,
		AmountSat:       amountSat,
		PreimageHash:    s.PreimageHashHex,
		ClaimPublicKey:  claimHex,
		RefundPublicKey: refundHex,
	})
	if err != nil {
		t.Fatalf("provider create reverse: %v", err)
	}
	if createResp.BoltzID == "" {
		t.Fatalf("provider create returned empty boltz id")
	}

	var reverseDetails boltz.ReverseResponse
	if len(createResp.ReverseDetails) > 0 {
		if err := json.Unmarshal(createResp.ReverseDetails, &reverseDetails); err != nil {
			t.Fatalf("unmarshal reverse details: %v", err)
		}
	}

	lockedIntent := map[string]interface{}{
		"version":         swap.LockedIntentVersion,
		"quote_id":        "reverse-exec-integration",
		"kind":            "reverse",
		"from_chain":      "LN",
		"to_chain":        "BTC",
		"amount_sat":      amountSat,
		"boltz_id":        createResp.BoltzID,
		"lockup_address":  createResp.LockupAddress,
		"claim_address":   createResp.ClaimAddress,
		"timeout_blocks":  createResp.TimeoutBlockHeight,
		"reverse_details": json.RawMessage(createResp.ReverseDetails),
	}
	lockedIntentJSON, err := json.Marshal(lockedIntent)
	if err != nil {
		t.Fatalf("marshal locked_intent: %v", err)
	}

	_, err = database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?,
		    claim_pubkey_hex = ?,
		    refund_pubkey_hex = ?,
		    boltz_id = ?,
		    lockup_address = ?,
		    claim_address = ?,
		    timeout_block_height = ?,
		    boltz_status = ?,
		    boltz_raw = CASE WHEN ? != '' THEN ? ELSE boltz_raw END
		WHERE id = ?
	`, string(lockedIntentJSON), claimHex, refundHex, createResp.BoltzID, createResp.LockupAddress,
		createResp.ClaimAddress, createResp.TimeoutBlockHeight, provider.StatusCreated,
		string(createResp.BoltzRaw), string(createResp.BoltzRaw), s.ID)
	if err != nil {
		t.Fatalf("persist swap fields: %v", err)
	}

	locked, err := engine.Transition(ctx, s.ID, s.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("transition locked: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("transition commit_started: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("transition waiting: %v", err)
	}

	w := NewWatcher(database, nil, engine, execProv, vault, "testnet")

	t.Logf("Created reverse swap boltz_id=%s lockup_address=%s amount_sat=%d", createResp.BoltzID, createResp.LockupAddress, amountSat)
	if simulateTrigger {
		t.Logf("SIMULATE_TRIGGER ativo: status forçado para %s, sem necessidade de pagar invoice", boltz.StatusTxConfirmed)
	} else if reverseDetails.Invoice != "" {
		t.Logf("Pay the reverse invoice to trigger provider lockup before timeout (%ds): invoice=%s", waitTimeout, reverseDetails.Invoice)
	} else {
		t.Logf("Pay the reverse invoice to trigger provider lockup before timeout (%ds). invoice is available in boltz_raw.", waitTimeout)
	}

	deadline := time.Now().Add(time.Duration(waitTimeout) * time.Second)
	reachedWaitingProviderBroadcast := false
	for time.Now().Before(deadline) {
		status, err := execProv.GetSwapStatus(ctx, createResp.BoltzID)
		if err != nil {
			t.Logf("GetSwapStatus error: %v", err)
		} else if status != "" {
			_, _ = database.ExecContext(ctx, `UPDATE swaps SET boltz_status = ? WHERE id = ?`, status, waiting.ID)
			t.Logf("boltz status=%s", status)
		}

		if err := w.ReconcileAllActiveSwaps(ctx); err != nil {
			t.Logf("reconcile error: %v", err)
		}
		updated, err := engine.Get(ctx, waiting.ID)
		if err != nil {
			t.Fatalf("get swap: %v", err)
		}
		if updated.State == swap.StateWaitingProviderBroadcast {
			reachedWaitingProviderBroadcast = true
			if err := assertReverseExecutionEventDetails(ctx, database, waiting.ID); err != nil {
				t.Fatalf("validate reverse swap event details: %v", err)
			}
			if simulateTrigger && simulateTerminal != "" {
				targetState, forcedStatus, err := reverseTerminalSimulation(simulateTerminal)
				if err != nil {
					t.Fatalf("invalid terminal simulation: %v", err)
				}
				execProv.forcedStatus = forcedStatus
				t.Logf("SIMULATE_TERMINAL ativo: status forçado para %s; aguardando estado=%s", forcedStatus, targetState)

				terminalDeadline := time.Now().Add(time.Duration(waitTimeout) * time.Second)
				for time.Now().Before(terminalDeadline) {
					_, _ = database.ExecContext(ctx, `UPDATE swaps SET boltz_status = ? WHERE id = ?`, forcedStatus, waiting.ID)
					if err := w.ReconcileAllActiveSwaps(ctx); err != nil {
						t.Logf("reconcile terminal error: %v", err)
					}
					terminalSwap, err := engine.Get(ctx, waiting.ID)
					if err != nil {
						t.Fatalf("get terminal swap: %v", err)
					}
					if terminalReached(terminalSwap.State, targetState) {
						if targetState == swap.StateRefunding {
							if err := assertSwapReachedStateEvent(ctx, database, waiting.ID, "refunding"); err != nil {
								t.Fatalf("validate refunding transition event: %v", err)
							}
						}
						t.Logf("reverse execution reached terminal state=%s", terminalSwap.State)
						return
					}
					time.Sleep(time.Duration(pollSeconds) * time.Second)
				}
				finalTerminal, _ := engine.Get(ctx, waiting.ID)
				t.Fatalf("timeout waiting for terminal state=%s; got=%s", targetState, finalTerminal.State)
			}
			if !fullReal {
				t.Logf("reverse execution reached state=%s", updated.State)
				return
			}
			t.Logf("reverse execution reached signing milestone state=%s; waiting terminal=%s (real mode)", updated.State, targetTerminal)
		}
		if fullReal && reachedWaitingProviderBroadcast {
			if terminalReached(updated.State, targetTerminal) {
				if targetTerminal == swap.StateRefunding {
					if err := assertSwapReachedStateEvent(ctx, database, waiting.ID, "refunding"); err != nil {
						t.Fatalf("validate refunding transition event: %v", err)
					}
				}
				t.Logf("reverse execution reached terminal state=%s (real mode)", updated.State)
				return
			}
			if updated.State == swap.StateFailed || updated.State == swap.StateCanceled {
				t.Fatalf("reverse execution failed before terminal target=%s: state=%s", targetTerminal, updated.State)
			}
		}

		time.Sleep(time.Duration(pollSeconds) * time.Second)
	}

	finalSwap, _ := engine.Get(ctx, waiting.ID)
	finalStatus, _ := execProv.GetSwapStatus(ctx, createResp.BoltzID)
	if fullReal {
		t.Fatalf("timeout waiting for terminal=%s; final_state=%s boltz_status=%s boltz_id=%s",
			targetTerminal, finalSwap.State, finalStatus, createResp.BoltzID)
	}
	t.Fatalf("timeout waiting for %s; final_state=%s boltz_status=%s boltz_id=%s",
		swap.StateWaitingProviderBroadcast, finalSwap.State, finalStatus, createResp.BoltzID)
}

func assertReverseExecutionEventDetails(ctx context.Context, database interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}, swapID string) error {
	var sentDetailsRaw sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT details
		FROM swap_events
		WHERE swap_id = ? AND trigger = 'musig_claim_sent' AND to_state = 'sent_partial_to_provider'
		ORDER BY seq DESC LIMIT 1
	`, swapID).Scan(&sentDetailsRaw); err != nil {
		return fmt.Errorf("sent_partial_to_provider details not found: %w", err)
	}
	if !sentDetailsRaw.Valid || sentDetailsRaw.String == "" {
		return fmt.Errorf("sent_partial_to_provider missing details json")
	}
	var sentDetails map[string]interface{}
	if err := json.Unmarshal([]byte(sentDetailsRaw.String), &sentDetails); err != nil {
		return fmt.Errorf("invalid sent_partial_to_provider details json: %w", err)
	}
	if _, ok := sentDetails["reverse_onchain_amount"]; !ok {
		return fmt.Errorf("missing reverse_onchain_amount in sent_partial_to_provider details")
	}
	if _, ok := sentDetails["has_invoice"]; !ok {
		return fmt.Errorf("missing has_invoice in sent_partial_to_provider details")
	}

	var waitingDetailsRaw sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT details
		FROM swap_events
		WHERE swap_id = ? AND trigger = 'musig_claim_sent' AND to_state = 'waiting_provider_broadcast'
		ORDER BY seq DESC LIMIT 1
	`, swapID).Scan(&waitingDetailsRaw); err != nil {
		return fmt.Errorf("waiting_provider_broadcast details not found: %w", err)
	}
	if !waitingDetailsRaw.Valid || waitingDetailsRaw.String == "" {
		return fmt.Errorf("waiting_provider_broadcast missing details json")
	}
	var waitingDetails map[string]interface{}
	if err := json.Unmarshal([]byte(waitingDetailsRaw.String), &waitingDetails); err != nil {
		return fmt.Errorf("invalid waiting_provider_broadcast details json: %w", err)
	}
	if waitingDetails["await_status"] != "transaction.claim.pending" {
		return fmt.Errorf("unexpected await_status: %v", waitingDetails["await_status"])
	}
	if _, ok := waitingDetails["reverse_lockup_address"]; !ok {
		return fmt.Errorf("missing reverse_lockup_address in waiting_provider_broadcast details")
	}
	return nil
}

func resolveReverseAmountForExecution(ctx context.Context, client *boltz.Client, from, to string) (int64, error) {
	pairs, err := client.GetReversePairs(ctx)
	if err != nil {
		return 0, err
	}
	fromPairs, ok := pairs[from]
	if !ok {
		return 0, fmt.Errorf("pair source not found: %s", from)
	}
	pair, ok := fromPairs[to]
	if !ok {
		return 0, fmt.Errorf("pair target not found: %s -> %s", from, to)
	}
	amount := pair.Limits.Minimal + 10000
	if amount > pair.Limits.Maximal {
		amount = pair.Limits.Minimal
	}
	if amount <= 0 {
		return 0, fmt.Errorf("invalid limits minimal=%d maximal=%d", pair.Limits.Minimal, pair.Limits.Maximal)
	}
	return amount, nil
}

func reverseTerminalSimulation(mode string) (swap.State, string, error) {
	switch mode {
	case "completed":
		return swap.StateCompleted, boltz.StatusInvoiceSettled, nil
	case "refunding":
		return swap.StateRefunding, boltz.StatusSwapExpired, nil
	default:
		return "", "", fmt.Errorf("unsupported mode %q (use completed|refunding)", mode)
	}
}
