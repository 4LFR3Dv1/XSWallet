// Package server - Tests for swap service mappings
package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/provider/mock"
	"github.com/xs-wallet/xscore/internal/swap"
	pb "github.com/xs-wallet/xscore/proto"
)

func TestProtoKindToProvider(t *testing.T) {
	cases := []struct {
		in  pb.SwapKind
		out provider.SwapKind
	}{
		{pb.SwapKind_SWAP_KIND_SUBMARINE, provider.SwapKindSubmarine},
		{pb.SwapKind_SWAP_KIND_REVERSE, provider.SwapKindReverse},
		{pb.SwapKind_SWAP_KIND_CHAIN, provider.SwapKindChain},
	}

	for _, c := range cases {
		if got := protoKindToProvider(c.in); got != c.out {
			t.Fatalf("kind %v -> %v, want %v", c.in, got, c.out)
		}
	}
}

func TestProtoChainToProvider(t *testing.T) {
	cases := []struct {
		in  pb.Chain
		out provider.Chain
	}{
		{pb.Chain_CHAIN_BTC, provider.ChainBTC},
		{pb.Chain_CHAIN_LN, provider.ChainLN},
		{pb.Chain_CHAIN_LIQUID, provider.ChainLiquid},
	}

	for _, c := range cases {
		if got := protoChainToProvider(c.in); got != c.out {
			t.Fatalf("chain %v -> %v, want %v", c.in, got, c.out)
		}
	}
}

func TestLockSwapErrorMapping(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	service := NewSwapService(database, nil, engine, mock.NewMockProvider())

	created, err := engine.Create(ctx, swap.KindSubmarine, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ? WHERE id = ?`, "{}", created.ID); err != nil {
		t.Fatalf("set locked_intent: %v", err)
	}

	// Initial lock on a fresh swap must accept expected_version=0.
	lockedFromZero, err := service.LockSwap(ctx, &pb.LockSwapRequest{
		SwapId:          created.ID,
		ExpectedVersion: 0,
	})
	if err != nil {
		t.Fatalf("lock from version 0 should succeed: %v", err)
	}
	if lockedFromZero.State != pb.SwapState_SWAP_STATE_LOCKED {
		t.Fatalf("expected LOCKED state, got %v", lockedFromZero.State)
	}

	// Create another swap for stale-version and invalid-transition checks.
	created2, err := engine.Create(ctx, swap.KindSubmarine, "regtest", 0)
	if err != nil {
		t.Fatalf("create second swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ? WHERE id = ?`, "{}", created2.ID); err != nil {
		t.Fatalf("set locked_intent for second swap: %v", err)
	}

	_, err = service.LockSwap(ctx, &pb.LockSwapRequest{
		SwapId:          created2.ID,
		ExpectedVersion: uint64(created2.Version + 1),
	})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("expected Aborted for stale version, got %v (%v)", status.Code(err), err)
	}

	locked, err := service.LockSwap(ctx, &pb.LockSwapRequest{
		SwapId:          created2.ID,
		ExpectedVersion: uint64(created2.Version),
	})
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}

	_, err = service.LockSwap(ctx, &pb.LockSwapRequest{
		SwapId:          created.ID,
		ExpectedVersion: locked.Version,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for invalid transition, got %v (%v)", status.Code(err), err)
	}

	_, err = service.LockSwap(ctx, &pb.LockSwapRequest{
		SwapId:          "missing-swap-id",
		ExpectedVersion: 1,
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for missing swap, got %v (%v)", status.Code(err), err)
	}
}
