// Package server - Tests for swap service mappings
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/provider/mock"
	"github.com/xs-wallet/xscore/internal/swap"
	pb "github.com/xs-wallet/xscore/proto"
)

type spyCreateProvider struct {
	mu          sync.Mutex
	quotes      map[string]*provider.Quote
	lastQuote   provider.QuoteRequest
	lastCreate  provider.CreateRequest
	createCalls int
	nextCreate  *provider.CreateResponse
}

func newSpyCreateProvider() *spyCreateProvider {
	return &spyCreateProvider{
		quotes: make(map[string]*provider.Quote),
	}
}

func (s *spyCreateProvider) Quote(_ context.Context, req provider.QuoteRequest) (*provider.Quote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastQuote = req
	id := fmt.Sprintf("q-%d", len(s.quotes)+1)
	q := &provider.Quote{
		QuoteID:               id,
		Kind:                  req.Kind,
		FromChain:             req.FromChain,
		ToChain:               req.ToChain,
		AmountSat:             req.AmountSat,
		Invoice:               req.Invoice,
		Address:               req.Address,
		LockupAddress:         "lockup-" + id,
		ClaimAddress:          "claim-" + id,
		UserTimeoutBlocks:     144,
		ProviderTimeoutBlocks: 72,
		ExpiresAt:             time.Now().Add(5 * time.Minute),
	}
	s.quotes[id] = q
	return q, nil
}

func (s *spyCreateProvider) Create(_ context.Context, req provider.CreateRequest) (*provider.CreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCreate = req
	s.createCalls++
	if s.nextCreate != nil {
		resp := *s.nextCreate
		if resp.SwapID == "" {
			resp.SwapID = "boltz-" + req.QuoteID
		}
		if resp.BoltzID == "" {
			resp.BoltzID = "boltz-" + req.QuoteID
		}
		return &resp, nil
	}
	return &provider.CreateResponse{
		SwapID:             "boltz-" + req.QuoteID,
		BoltzID:            "boltz-" + req.QuoteID,
		LockupAddress:      "lockup-" + req.QuoteID,
		ClaimAddress:       "claim-" + req.QuoteID,
		ExpectedAmount:     req.AmountSat,
		TimeoutBlockHeight: 144,
	}, nil
}

func (s *spyCreateProvider) Subscribe(_ context.Context, _ string) (<-chan provider.Update, func(), error) {
	ch := make(chan provider.Update)
	cancel := func() { close(ch) }
	return ch, cancel, nil
}

func (s *spyCreateProvider) GetSwapStatus(_ context.Context, _ string) (string, error) {
	return provider.StatusCreated, nil
}

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
	service := NewSwapServiceWithSecrets(database, nil, engine, mock.NewMockProvider(), commitTestVault{})

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

func TestCreateSwapAutoLocksSubmarine(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	service := NewSwapServiceWithSecrets(database, nil, engine, mock.NewMockProvider(), commitTestVault{})

	quote, err := service.QuoteSwap(ctx, &pb.QuoteSwapRequest{
		Kind:      pb.SwapKind_SWAP_KIND_SUBMARINE,
		FromChain: pb.Chain_CHAIN_BTC,
		ToChain:   pb.Chain_CHAIN_LN,
		AmountSat: 100000,
		Params: &pb.QuoteSwapRequest_Submarine{
			Submarine: &pb.SubmarineQuoteParams{Invoice: "lnbc1mock"},
		},
	})
	if err != nil {
		t.Fatalf("quote swap: %v", err)
	}

	created, err := service.CreateSwap(ctx, &pb.CreateSwapRequest{QuoteId: quote.QuoteId})
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}

	if created.State != pb.SwapState_SWAP_STATE_LOCKED {
		t.Fatalf("expected LOCKED after CreateSwap, got %v", created.State)
	}
}

func TestCreateSwapAutoLocksReverseAndChain(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	service := NewSwapServiceWithSecrets(database, nil, engine, mock.NewMockProvider(), commitTestVault{})

	cases := []struct {
		name      string
		kind      pb.SwapKind
		fromChain pb.Chain
		toChain   pb.Chain
		wantKind  pb.SwapKind
	}{
		{
			name:      "reverse",
			kind:      pb.SwapKind_SWAP_KIND_REVERSE,
			fromChain: pb.Chain_CHAIN_LN,
			toChain:   pb.Chain_CHAIN_BTC,
			wantKind:  pb.SwapKind_SWAP_KIND_REVERSE,
		},
		{
			name:      "chain",
			kind:      pb.SwapKind_SWAP_KIND_CHAIN,
			fromChain: pb.Chain_CHAIN_BTC,
			toChain:   pb.Chain_CHAIN_LIQUID,
			wantKind:  pb.SwapKind_SWAP_KIND_CHAIN,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote, err := service.QuoteSwap(ctx, &pb.QuoteSwapRequest{
				Kind:      tc.kind,
				FromChain: tc.fromChain,
				ToChain:   tc.toChain,
				AmountSat: 100000,
			})
			if err != nil {
				t.Fatalf("quote swap: %v", err)
			}

			created, err := service.CreateSwap(ctx, &pb.CreateSwapRequest{QuoteId: quote.QuoteId})
			if err != nil {
				t.Fatalf("create swap: %v", err)
			}

			if created.State != pb.SwapState_SWAP_STATE_LOCKED {
				t.Fatalf("expected LOCKED after CreateSwap, got %v", created.State)
			}
			if created.Kind != tc.wantKind {
				t.Fatalf("expected kind %v, got %v", tc.wantKind, created.Kind)
			}
		})
	}
}

func TestCommitSwapReverseAndChainBranchByKind(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	service := NewSwapServiceWithSecrets(database, nil, engine, mock.NewMockProvider(), commitTestVault{})

	cases := []struct {
		name      string
		kind      pb.SwapKind
		fromChain pb.Chain
		toChain   pb.Chain
	}{
		{
			name:      "reverse_commit_goes_to_waiting",
			kind:      pb.SwapKind_SWAP_KIND_REVERSE,
			fromChain: pb.Chain_CHAIN_LN,
			toChain:   pb.Chain_CHAIN_BTC,
		},
		{
			name:      "chain_commit_goes_to_waiting",
			kind:      pb.SwapKind_SWAP_KIND_CHAIN,
			fromChain: pb.Chain_CHAIN_BTC,
			toChain:   pb.Chain_CHAIN_LIQUID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quote, err := service.QuoteSwap(ctx, &pb.QuoteSwapRequest{
				Kind:      tc.kind,
				FromChain: tc.fromChain,
				ToChain:   tc.toChain,
				AmountSat: 100000,
			})
			if err != nil {
				t.Fatalf("quote swap: %v", err)
			}

			created, err := service.CreateSwap(ctx, &pb.CreateSwapRequest{QuoteId: quote.QuoteId})
			if err != nil {
				t.Fatalf("create swap: %v", err)
			}
			if created.State != pb.SwapState_SWAP_STATE_LOCKED {
				t.Fatalf("expected LOCKED before commit, got %v", created.State)
			}

			committed, err := service.CommitSwap(ctx, &pb.CommitSwapRequest{
				SwapId:          created.Id,
				ExpectedVersion: created.Version,
			})
			if err != nil {
				t.Fatalf("commit swap: %v", err)
			}
			if committed.State != pb.SwapState_SWAP_STATE_WAITING {
				t.Fatalf("expected WAITING after commit for %s, got %v", tc.name, committed.State)
			}

			var boltzID sql.NullString
			if err := database.QueryRowContext(ctx, `SELECT boltz_id FROM swaps WHERE id = ?`, created.Id).Scan(&boltzID); err != nil {
				t.Fatalf("query boltz_id: %v", err)
			}
			if !boltzID.Valid || boltzID.String == "" {
				t.Fatalf("expected boltz_id to be persisted after commit")
			}
		})
	}
}

func TestCommitSwapUsesLockedIntentPayloadInProviderCreate(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	spy := newSpyCreateProvider()
	service := NewSwapServiceWithSecrets(database, nil, engine, spy, commitTestVault{})

	quote, err := service.QuoteSwap(ctx, &pb.QuoteSwapRequest{
		Kind:      pb.SwapKind_SWAP_KIND_REVERSE,
		FromChain: pb.Chain_CHAIN_LN,
		ToChain:   pb.Chain_CHAIN_BTC,
		AmountSat: 210000,
		Params: &pb.QuoteSwapRequest_Reverse{
			Reverse: &pb.ReverseQuoteParams{PayoutAddress: "bcrt1qtestdest"},
		},
	})
	if err != nil {
		t.Fatalf("quote swap: %v", err)
	}

	created, err := service.CreateSwap(ctx, &pb.CreateSwapRequest{QuoteId: quote.QuoteId})
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if created.State != pb.SwapState_SWAP_STATE_LOCKED {
		t.Fatalf("expected LOCKED before commit, got %v", created.State)
	}

	_, err = service.CommitSwap(ctx, &pb.CommitSwapRequest{
		SwapId:          created.Id,
		ExpectedVersion: created.Version,
	})
	if err != nil {
		t.Fatalf("commit swap: %v", err)
	}

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if spy.createCalls != 1 {
		t.Fatalf("expected one provider.Create call, got %d", spy.createCalls)
	}
	if spy.lastCreate.QuoteID != quote.QuoteId {
		t.Fatalf("expected quote_id %s, got %s", quote.QuoteId, spy.lastCreate.QuoteID)
	}
	if spy.lastCreate.Address != "bcrt1qtestdest" {
		t.Fatalf("expected payout address from locked_intent, got %s", spy.lastCreate.Address)
	}
	if spy.lastCreate.Kind != provider.SwapKindReverse {
		t.Fatalf("expected reverse kind, got %s", spy.lastCreate.Kind)
	}
	if spy.lastCreate.AmountSat != 210000 {
		t.Fatalf("expected amount 210000, got %d", spy.lastCreate.AmountSat)
	}
	if len(spy.lastCreate.PreimageHash) != 64 {
		t.Fatalf("expected preimage hash hex length 64, got %d", len(spy.lastCreate.PreimageHash))
	}
}

func TestCommitSwapChainPersistsCreateDetailsInLockedIntentAndBoltzRaw(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	spy := newSpyCreateProvider()
	spy.nextCreate = &provider.CreateResponse{
		SwapID:             "boltz-chain-q",
		BoltzID:            "boltz-chain-q",
		LockupAddress:      "tb1qchainlockup",
		ClaimAddress:       "tlq1chainclaim",
		TimeoutBlockHeight: 123456,
		BoltzRaw:           json.RawMessage(`{"id":"boltz-chain-q","lockupDetails":{"lockupAddress":"tb1qchainlockup"},"claimDetails":{"lockupAddress":"tlq1chainclaim"}}`),
		LockupDetails:      json.RawMessage(`{"lockupAddress":"tb1qchainlockup","timeoutBlockHeight":123456}`),
		ClaimDetails:       json.RawMessage(`{"lockupAddress":"tlq1chainclaim","timeoutBlockHeight":654321,"blindingKey":"ab"}`),
	}
	service := NewSwapServiceWithSecrets(database, nil, engine, spy, commitTestVault{})

	quote, err := service.QuoteSwap(ctx, &pb.QuoteSwapRequest{
		Kind:      pb.SwapKind_SWAP_KIND_CHAIN,
		FromChain: pb.Chain_CHAIN_BTC,
		ToChain:   pb.Chain_CHAIN_LIQUID,
		AmountSat: 3000000,
		Params: &pb.QuoteSwapRequest_Chain{
			Chain: &pb.ChainQuoteParams{PayoutAddress: "tlq1dest"},
		},
	})
	if err != nil {
		t.Fatalf("quote swap: %v", err)
	}
	created, err := service.CreateSwap(ctx, &pb.CreateSwapRequest{QuoteId: quote.QuoteId})
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}

	_, err = service.CommitSwap(ctx, &pb.CommitSwapRequest{
		SwapId:          created.Id,
		ExpectedVersion: created.Version,
	})
	if err != nil {
		t.Fatalf("commit swap: %v", err)
	}

	var lockedIntent string
	var boltzRaw sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT locked_intent, boltz_raw FROM swaps WHERE id = ?`, created.Id).Scan(&lockedIntent, &boltzRaw); err != nil {
		t.Fatalf("query swap: %v", err)
	}
	if !boltzRaw.Valid || boltzRaw.String == "" {
		t.Fatalf("expected boltz_raw to be persisted")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(lockedIntent), &payload); err != nil {
		t.Fatalf("unmarshal locked_intent: %v", err)
	}
	if payload["boltz_id"] != "boltz-chain-q" {
		t.Fatalf("expected boltz_id in locked_intent, got %v", payload["boltz_id"])
	}
	if payload["version"] != float64(swap.LockedIntentVersion) {
		t.Fatalf("expected locked_intent version %d, got %v", swap.LockedIntentVersion, payload["version"])
	}
	if _, ok := payload["lockup_details"]; !ok {
		t.Fatalf("expected lockup_details in locked_intent")
	}
	if _, ok := payload["claim_details"]; !ok {
		t.Fatalf("expected claim_details in locked_intent")
	}
}

func TestCommitSwapReversePersistsCreateDetailsInLockedIntentAndBoltzRaw(t *testing.T) {
	ctx := context.Background()
	database := openServerTestDB(t)
	engine := swap.NewEngine(database, commitTestVault{})
	spy := newSpyCreateProvider()
	spy.nextCreate = &provider.CreateResponse{
		SwapID:             "boltz-reverse-q",
		BoltzID:            "boltz-reverse-q",
		LockupAddress:      "tb1qreverselockup",
		ClaimAddress:       "bcrt1qpayoutdest",
		ExpectedAmount:     120000,
		TimeoutBlockHeight: 789012,
		BoltzRaw:           json.RawMessage(`{"id":"boltz-reverse-q","invoice":"lntb1...","lockupAddress":"tb1qreverselockup"}`),
		ReverseDetails:     json.RawMessage(`{"id":"boltz-reverse-q","invoice":"lntb1...","lockupAddress":"tb1qreverselockup","swapTree":{"claimLeaf":{"version":192,"output":"51"}}}`),
	}
	service := NewSwapServiceWithSecrets(database, nil, engine, spy, commitTestVault{})

	quote, err := service.QuoteSwap(ctx, &pb.QuoteSwapRequest{
		Kind:      pb.SwapKind_SWAP_KIND_REVERSE,
		FromChain: pb.Chain_CHAIN_LN,
		ToChain:   pb.Chain_CHAIN_BTC,
		AmountSat: 120000,
		Params: &pb.QuoteSwapRequest_Reverse{
			Reverse: &pb.ReverseQuoteParams{PayoutAddress: "bcrt1qpayoutdest"},
		},
	})
	if err != nil {
		t.Fatalf("quote swap: %v", err)
	}
	created, err := service.CreateSwap(ctx, &pb.CreateSwapRequest{QuoteId: quote.QuoteId})
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}

	_, err = service.CommitSwap(ctx, &pb.CommitSwapRequest{
		SwapId:          created.Id,
		ExpectedVersion: created.Version,
	})
	if err != nil {
		t.Fatalf("commit swap: %v", err)
	}

	var lockedIntent string
	var boltzRaw sql.NullString
	if err := database.QueryRowContext(ctx, `SELECT locked_intent, boltz_raw FROM swaps WHERE id = ?`, created.Id).Scan(&lockedIntent, &boltzRaw); err != nil {
		t.Fatalf("query swap: %v", err)
	}
	if !boltzRaw.Valid || boltzRaw.String == "" {
		t.Fatalf("expected boltz_raw to be persisted")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(lockedIntent), &payload); err != nil {
		t.Fatalf("unmarshal locked_intent: %v", err)
	}
	if payload["boltz_id"] != "boltz-reverse-q" {
		t.Fatalf("expected boltz_id in locked_intent, got %v", payload["boltz_id"])
	}
	if payload["version"] != float64(swap.LockedIntentVersion) {
		t.Fatalf("expected locked_intent version %d, got %v", swap.LockedIntentVersion, payload["version"])
	}
	if _, ok := payload["reverse_details"]; !ok {
		t.Fatalf("expected reverse_details in locked_intent")
	}
}

func TestGetSwapEventsReturnsRecordedTransitions(t *testing.T) {
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
	_, err = engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit transition: %v", err)
	}

	resp, err := service.GetSwapEvents(ctx, &pb.GetSwapEventsRequest{
		SwapId:   created.ID,
		AfterSeq: 0,
	})
	if err != nil {
		t.Fatalf("GetSwapEvents: %v", err)
	}
	if len(resp.Events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(resp.Events))
	}

	respAfter, err := service.GetSwapEvents(ctx, &pb.GetSwapEventsRequest{
		SwapId:   created.ID,
		AfterSeq: resp.Events[0].Seq,
	})
	if err != nil {
		t.Fatalf("GetSwapEvents after seq: %v", err)
	}
	if len(respAfter.Events) == 0 {
		t.Fatalf("expected events after seq=%d", resp.Events[0].Seq)
	}
}
