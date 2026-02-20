package watcher

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/xs-wallet/xscore/internal/boltz"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/swapkey"
)

type integrationSeedVault struct{}

func (integrationSeedVault) EncryptPreimage(b []byte) ([]byte, error) {
	return append([]byte("enc:"), b...), nil
}

func (integrationSeedVault) DecryptPreimage(b []byte) ([]byte, error) {
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}

func (integrationSeedVault) Seed() ([]byte, error) {
	return bytes.Repeat([]byte{0x37}, 32), nil
}

// TestChainExecutionToWaitingProviderBroadcastIntegration creates a real chain swap on Boltz
// testnet and exercises watcher reconciliation until signing path is executed.
//
// Run:
//
//	XS_BOLTZ_CHAIN_EXEC_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/watcher -run TestChainExecutionToWaitingProviderBroadcastIntegration -v
func TestChainExecutionToWaitingProviderBroadcastIntegration(t *testing.T) {
	if os.Getenv("XS_BOLTZ_CHAIN_EXEC_INTEGRATION") != "1" {
		t.Skip("set XS_BOLTZ_CHAIN_EXEC_INTEGRATION=1 to run real chain execution integration")
	}

	baseURL := os.Getenv("XS_BOLTZ_API_URL")
	if baseURL == "" {
		baseURL = "https://api.testnet.boltz.exchange"
	}
	wsURL := os.Getenv("XS_BOLTZ_WS_URL")
	if wsURL == "" {
		wsURL = "wss://api.testnet.boltz.exchange/v2/ws"
	}
	waitTimeout := envInt("XS_BOLTZ_CHAIN_EXEC_WAIT_SECONDS", 900)
	pollSeconds := envInt("XS_BOLTZ_CHAIN_EXEC_POLL_SECONDS", 10)
	simulateTrigger := os.Getenv("XS_BOLTZ_CHAIN_EXEC_SIMULATE_TRIGGER") == "1"
	simulateTerminal := os.Getenv("XS_BOLTZ_CHAIN_EXEC_SIMULATE_TERMINAL")
	fullReal := os.Getenv("XS_BOLTZ_CHAIN_EXEC_FULL_REAL") == "1"
	targetTerminal := expectedTerminalState(os.Getenv("XS_BOLTZ_CHAIN_EXEC_EXPECT_TERMINAL"), swap.StateCompleted)
	if fullReal && (simulateTrigger || simulateTerminal != "") {
		t.Fatalf("FULL_REAL requires no simulation flags: unset XS_BOLTZ_CHAIN_EXEC_SIMULATE_TRIGGER and XS_BOLTZ_CHAIN_EXEC_SIMULATE_TERMINAL")
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
		execProv.forcedStatus = boltz.StatusServerConfirmed
		execProv.simulateExec = true
	}

	client := boltz.NewClient(boltz.ClientConfig{BaseURL: baseURL, Timeout: 30 * time.Second})
	amountSat, err := resolveChainAmountForExecution(ctx, client, "BTC", "L-BTC")
	if err != nil {
		t.Fatalf("resolve chain amount: %v", err)
	}

	s, err := engine.Create(ctx, swap.KindChain, "testnet", 7)
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
		Kind:            provider.SwapKindChain,
		FromChain:       provider.ChainBTC,
		ToChain:         provider.ChainLiquid,
		AmountSat:       amountSat,
		PreimageHash:    s.PreimageHashHex,
		MusigPubkeyAgg:  claimHex,
		ClaimPublicKey:  claimHex,
		RefundPublicKey: refundHex,
	})
	if err != nil {
		t.Fatalf("provider create chain: %v", err)
	}
	if createResp.BoltzID == "" {
		t.Fatalf("provider create returned empty boltz id")
	}

	lockedIntent := map[string]interface{}{
		"quote_id":       "chain-exec-integration",
		"kind":           "chain",
		"from_chain":     "BTC",
		"to_chain":       "L-BTC",
		"amount_sat":     amountSat,
		"boltz_id":       createResp.BoltzID,
		"lockup_address": createResp.LockupAddress,
		"claim_address":  createResp.ClaimAddress,
		"timeout_blocks": createResp.TimeoutBlockHeight,
	}
	if len(createResp.LockupDetails) > 0 {
		lockedIntent["lockup_details"] = json.RawMessage(createResp.LockupDetails)
	}
	if len(createResp.ClaimDetails) > 0 {
		lockedIntent["claim_details"] = json.RawMessage(createResp.ClaimDetails)
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

	t.Logf("Created chain swap boltz_id=%s lockup_address=%s amount_sat=%d", createResp.BoltzID, createResp.LockupAddress, amountSat)
	if simulateTrigger {
		t.Logf("SIMULATE_TRIGGER ativo: status forçado para %s, sem necessidade de funding manual", boltz.StatusServerConfirmed)
	} else {
		t.Logf("Fund the lockup address to reach signing trigger before timeout (%ds)", waitTimeout)
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
			if err := assertChainExecutionEventDetails(ctx, database, waiting.ID); err != nil {
				t.Fatalf("validate swap event details: %v", err)
			}
			if simulateTrigger && simulateTerminal != "" {
				targetState, forcedStatus, err := chainTerminalSimulation(simulateTerminal)
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
						t.Logf("chain execution reached terminal state=%s", terminalSwap.State)
						return
					}
					time.Sleep(time.Duration(pollSeconds) * time.Second)
				}
				finalTerminal, _ := engine.Get(ctx, waiting.ID)
				t.Fatalf("timeout waiting for terminal state=%s; got=%s", targetState, finalTerminal.State)
			}
			if !fullReal {
				t.Logf("chain execution reached state=%s", updated.State)
				return
			}
			t.Logf("chain execution reached signing milestone state=%s; waiting terminal=%s (real mode)", updated.State, targetTerminal)
		}
		if fullReal && reachedWaitingProviderBroadcast {
			if terminalReached(updated.State, targetTerminal) {
				if targetTerminal == swap.StateRefunding {
					if err := assertSwapReachedStateEvent(ctx, database, waiting.ID, "refunding"); err != nil {
						t.Fatalf("validate refunding transition event: %v", err)
					}
				}
				t.Logf("chain execution reached terminal state=%s (real mode)", updated.State)
				return
			}
			if updated.State == swap.StateFailed || updated.State == swap.StateCanceled {
				t.Fatalf("chain execution failed before terminal target=%s: state=%s", targetTerminal, updated.State)
			}
		}

		time.Sleep(time.Duration(pollSeconds) * time.Second)
	}

	finalSwap, _ := engine.Get(ctx, waiting.ID)
	finalStatus, _ := execProv.GetSwapStatus(ctx, createResp.BoltzID)
	if fullReal {
		t.Fatalf("timeout waiting for terminal=%s; final_state=%s boltz_status=%s lockup_address=%s boltz_id=%s",
			targetTerminal, finalSwap.State, finalStatus, createResp.LockupAddress, createResp.BoltzID)
	}
	t.Fatalf("timeout waiting for %s; final_state=%s boltz_status=%s lockup_address=%s boltz_id=%s",
		swap.StateWaitingProviderBroadcast, finalSwap.State, finalStatus, createResp.LockupAddress, createResp.BoltzID)
}

func openWatcherIntegrationDB(t *testing.T) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watcher_chain_exec_integration.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return database
}

func resolveChainAmountForExecution(ctx context.Context, client *boltz.Client, from, to string) (int64, error) {
	pairs, err := client.GetChainPairs(ctx)
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

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func chainTerminalSimulation(mode string) (swap.State, string, error) {
	switch mode {
	case "completed":
		return swap.StateCompleted, boltz.StatusTxClaimed, nil
	case "refunding":
		return swap.StateRefunding, boltz.StatusSwapExpired, nil
	default:
		return "", "", fmt.Errorf("unsupported mode %q (use completed|refunding)", mode)
	}
}

func terminalReached(current, target swap.State) bool {
	if current == target {
		return true
	}
	return target == swap.StateRefunding && current == swap.StateCompleted
}

func expectedTerminalState(mode string, fallback swap.State) swap.State {
	switch mode {
	case "", "completed":
		return swap.StateCompleted
	case "refunding":
		return swap.StateRefunding
	default:
		return fallback
	}
}

func assertSwapReachedStateEvent(ctx context.Context, database *db.DB, swapID, toState string) error {
	var count int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM swap_events
		WHERE swap_id = ? AND to_state = ?
	`, swapID, toState).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("state transition to %s was not recorded", toState)
	}
	return nil
}

func assertChainExecutionEventDetails(ctx context.Context, database *db.DB, swapID string) error {
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
	if _, ok := sentDetails["claim_lockup_address"]; !ok {
		return fmt.Errorf("missing claim_lockup_address in sent_partial_to_provider details")
	}
	if _, ok := sentDetails["has_blinding_key"]; !ok {
		return fmt.Errorf("missing has_blinding_key in sent_partial_to_provider details")
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
	if _, ok := waitingDetails["claim_lockup_address"]; !ok {
		return fmt.Errorf("missing claim_lockup_address in waiting_provider_broadcast details")
	}
	return nil
}
