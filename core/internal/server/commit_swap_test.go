package server

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider/mock"
	"github.com/xs-wallet/xscore/internal/swap"
	pb "github.com/xs-wallet/xscore/proto"
)

type commitTestVault struct{}

func (commitTestVault) EncryptPreimage(b []byte) ([]byte, error) {
	return append([]byte("enc:"), b...), nil
}

func (commitTestVault) DecryptPreimage(b []byte) ([]byte, error) {
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}

func (commitTestVault) Seed() ([]byte, error) {
	return bytes.Repeat([]byte{0x21}, 32), nil
}

func openServerTestDB(t *testing.T) *db.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
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

func TestCommitSwapExpectedVersionHandling(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	service := NewSwapServiceWithSecrets(database, nil, engine, mock.NewMockProvider(), commitTestVault{})

	created, err := engine.Create(ctx, swap.KindSubmarine, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ? WHERE id = ?`, "{}", created.ID); err != nil {
		t.Fatalf("set locked_intent: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}

	_, err = service.CommitSwap(ctx, &pb.CommitSwapRequest{
		SwapId:          locked.ID,
		ExpectedVersion: 0,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for missing expected_version, got %v (%v)", status.Code(err), err)
	}

	_, err = service.CommitSwap(ctx, &pb.CommitSwapRequest{
		SwapId:          locked.ID,
		ExpectedVersion: uint64(locked.Version + 1),
	})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("expected Aborted for stale version, got %v (%v)", status.Code(err), err)
	}

	resp, err := service.CommitSwap(ctx, &pb.CommitSwapRequest{
		SwapId:          locked.ID,
		ExpectedVersion: uint64(locked.Version),
	})
	if err != nil {
		t.Fatalf("commit with correct version: %v", err)
	}
	if resp.State != pb.SwapState_SWAP_STATE_COMMIT_STARTED {
		t.Fatalf("expected state COMMIT_STARTED, got %v", resp.State)
	}
}
