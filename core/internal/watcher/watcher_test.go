package watcher

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/swap"
)

type watcherTestVault struct{}

func (watcherTestVault) EncryptPreimage(b []byte) ([]byte, error) {
	return append([]byte("enc:"), b...), nil
}

func (watcherTestVault) DecryptPreimage(b []byte) ([]byte, error) {
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}

func openWatcherTestDB(t *testing.T) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watcher_test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return database
}

func TestBuildReconcileEventKeyAndIdempotency(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})

	created, err := engine.Create(ctx, swap.KindSubmarine, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ?, lockup_txid = ? WHERE id = ?`, "{}", txid, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}

	facts := swap.ComputeFacts(ctx, commitStarted, 100, nil, "")
	action, newState := swap.NextAction(commitStarted, facts)
	if action != swap.ActionTransitionToWaiting || newState != swap.StateWaiting {
		t.Fatalf("unexpected action/state: %v %v", action, newState)
	}

	eventKey := buildReconcileEventKey(commitStarted, facts, action, newState)
	if eventKey == "" {
		t.Fatalf("expected non-empty event key")
	}

	done, _, err := engine.CheckIdempotency(ctx, commitStarted.ID, eventKey)
	if err != nil {
		t.Fatalf("check idempotency (before): %v", err)
	}
	if done {
		t.Fatalf("event should not be marked as processed yet")
	}

	if err := engine.RecordOperation(ctx, commitStarted.ID, eventKey, string(newState)); err != nil {
		t.Fatalf("record operation: %v", err)
	}

	done, result, err := engine.CheckIdempotency(ctx, commitStarted.ID, eventKey)
	if err != nil {
		t.Fatalf("check idempotency (after): %v", err)
	}
	if !done {
		t.Fatalf("event should be marked as processed")
	}
	if result != string(newState) {
		t.Fatalf("unexpected idempotency result: %s", result)
	}
}
