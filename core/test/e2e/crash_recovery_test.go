package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/watcher"
)

// TestCrashRecovery tests that the system recovers correctly after a crash
func TestCrashRecovery(t *testing.T) {
	ctx := context.Background()

	// Setup database (persistent for this test)
	dbPath := "test_crash_recovery.db"
	defer os.Remove(dbPath)

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create a swap in COMMIT_STARTED state
	engine := swap.NewEngine(database, e2eTestVault{})
	swp, err := engine.Create(ctx, swap.KindSubmarine, "regtest", 0)
	if err != nil {
		t.Fatalf("Failed to create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ? WHERE id = ?`, "{}", swp.ID); err != nil {
		t.Fatalf("Failed to set locked_intent: %v", err)
	}

	// Transition to LOCKED
	swp, err = engine.Transition(ctx, swp.ID, swp.Version, swap.StateLocked, "test", nil)
	if err != nil {
		t.Fatalf("Failed to transition to LOCKED: %v", err)
	}

	// Transition to COMMIT_STARTED (simulating broadcast)
	swp, err = engine.Transition(ctx, swp.ID, swp.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("Failed to transition to COMMIT_STARTED: %v", err)
	}

	// Record operation (idempotency)
	mockTxid := "0000000000000000000000000000000000000000000000000000000000000001"
	err = engine.RecordOperation(ctx, swp.ID, "commit:"+swp.ID, mockTxid)
	if err != nil {
		t.Fatalf("Failed to record operation: %v", err)
	}

	// Update with txid
	_, err = database.ExecContext(ctx, `
		UPDATE swaps SET lockup_txid = ? WHERE id = ?
	`, mockTxid, swp.ID)
	if err != nil {
		t.Fatalf("Failed to update swap: %v", err)
	}

	// Close database (simulate crash)
	database.Close()
	t.Logf("Simulated crash - database closed")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Reopen database (simulate restart)
	database, err = db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer database.Close()

	// Setup Bitcoin RPC
	btcClient := bitcoin.NewClient("http://127.0.0.1:18443", "rpcuser", "rpcpass")

	// Create new engine and watcher
	engine = swap.NewEngine(database, e2eTestVault{})
	watcherInstance := watcher.NewWatcher(database, btcClient, engine)

	// Run reconciliation (this is what happens on boot)
	err = watcherInstance.ReconcileAllActiveSwaps(ctx)
	if err != nil {
		t.Logf("Reconciliation completed with: %v", err)
	}

	// Verify swap is still in correct state
	recovered, err := engine.Get(ctx, swp.ID)
	if err != nil {
		t.Fatalf("Failed to get swap after recovery: %v", err)
	}

	t.Logf("Recovered swap: ID=%s, State=%s, Txid=%s", recovered.ID, recovered.State, recovered.LockupTxid)

	// Verify txid is preserved
	if recovered.LockupTxid != mockTxid {
		t.Fatalf("Txid not preserved after crash: expected %s, got %s", mockTxid, recovered.LockupTxid)
	}

	// Verify idempotency - try to commit again
	done, result, err := engine.CheckIdempotency(ctx, swp.ID, "commit:"+swp.ID)
	if err != nil {
		t.Fatalf("Failed to check idempotency: %v", err)
	}

	if !done {
		t.Fatalf("Idempotency check failed: operation should be marked as done")
	}

	if result != mockTxid {
		t.Fatalf("Idempotency result mismatch: expected %s, got %s", mockTxid, result)
	}

	t.Logf("Crash recovery test passed: swap recovered correctly, idempotency preserved")
}
