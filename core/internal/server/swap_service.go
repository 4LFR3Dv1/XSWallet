// Package server - Updated SwapService with full implementation
package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/xs-wallet/xscore/internal/config"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/swapkey"
	pb "github.com/xs-wallet/xscore/proto"
)

// SwapService implements pb.SwapServiceServer
type SwapService struct {
	pb.UnimplementedSwapServiceServer
	db         *db.DB
	cfg        *config.Config
	engine     *swap.Engine
	provider   provider.Provider
	submarine  *swap.SubmarineOrchestrator
	seedSource interface {
		Seed() ([]byte, error)
	}
	network string
}

// NewSwapService creates SwapService
func NewSwapService(database *db.DB, cfg *config.Config, engine *swap.Engine, prov provider.Provider) *SwapService {
	network := "regtest"
	if cfg != nil && cfg.Network != "" {
		network = cfg.Network
	}
	return &SwapService{
		db:        database,
		cfg:       cfg,
		engine:    engine,
		provider:  prov,
		submarine: swap.NewSubmarineOrchestrator(engine, database, prov),
		network:   network,
	}
}

// NewSwapServiceWithSecrets creates SwapService and enables deterministic swap key derivation.
func NewSwapServiceWithSecrets(database *db.DB, cfg *config.Config, engine *swap.Engine, prov provider.Provider, seedSource interface {
	Seed() ([]byte, error)
}) *SwapService {
	s := NewSwapService(database, cfg, engine, prov)
	s.seedSource = seedSource
	return s
}

// QuoteSwap requests a quote from the provider
func (s *SwapService) QuoteSwap(ctx context.Context, req *pb.QuoteSwapRequest) (*pb.SwapQuote, error) {
	// Convert proto request to provider request
	provReq := provider.QuoteRequest{
		Kind:      protoKindToProvider(req.Kind),
		FromChain: protoChainToProvider(req.FromChain),
		ToChain:   protoChainToProvider(req.ToChain),
		AmountSat: int64(req.AmountSat),
	}

	// Get submarine params
	if params := req.GetSubmarine(); params != nil {
		provReq.Invoice = params.Invoice
	}
	if params := req.GetReverse(); params != nil {
		provReq.Address = params.PayoutAddress
	}
	if params := req.GetChain(); params != nil {
		provReq.Address = params.PayoutAddress
	}

	// Create quote
	quoteService := swap.NewQuoteService(s.db, s.provider)
	quote, err := quoteService.CreateQuote(ctx, provReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create quote: %v", err)
	}

	// Convert to proto
	return &pb.SwapQuote{
		QuoteId:           quote.QuoteID,
		Kind:              req.Kind,
		FromChain:         req.FromChain,
		ToChain:           req.ToChain,
		AmountSat:         uint64(quote.AmountSat),
		ProviderFeeSat:    uint64(quote.ProviderFeeSat),
		NetworkFeeSat:     uint64(quote.NetworkFeeSat),
		TotalFeeSat:       uint64(quote.TotalFeeSat),
		FeePercentage:     quote.FeePercent,
		LockupAddress:     quote.LockupAddress,
		UserTimeoutBlocks: uint32(quote.UserTimeoutBlocks),
	}, nil
}

// CreateSwap creates a new swap from a quote
func (s *SwapService) CreateSwap(ctx context.Context, req *pb.CreateSwapRequest) (*pb.SwapSnapshot, error) {
	swp, err := s.createSwapFromQuote(ctx, req.QuoteId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create swap: %v", err)
	}

	return swapToProto(swp), nil
}

func (s *SwapService) createSwapFromQuote(ctx context.Context, quoteID string) (*swap.Swap, error) {
	quotes := swap.NewQuoteService(s.db, s.provider)
	quote, err := quotes.GetQuote(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	switch quote.Kind {
	case provider.SwapKindSubmarine:
		return s.submarine.CreateFromQuote(ctx, quoteID)
	case provider.SwapKindReverse, provider.SwapKindChain:
		swapKind := providerKindToSwapKind(quote.Kind)
		swp, err := s.engine.Create(ctx, swapKind, "regtest", 0)
		if err != nil {
			return nil, err
		}

		lockedIntentPayload := swap.LockedIntent{
			Version:       swap.LockedIntentVersion,
			QuoteID:       quote.QuoteID,
			Kind:          string(quote.Kind),
			FromChain:     string(quote.FromChain),
			ToChain:       string(quote.ToChain),
			AmountSat:     quote.AmountSat,
			Invoice:       quote.Invoice,
			PayoutAddress: quote.Address,
			LockupAddress: quote.LockupAddress,
			ClaimAddress:  quote.ClaimAddress,
			TimeoutBlocks: int64(quote.UserTimeoutBlocks),
		}
		lockedIntentBytes, err := lockedIntentPayload.ToJSON()
		if err != nil {
			return nil, fmt.Errorf("marshal locked_intent: %w", err)
		}

		_, err = s.db.ExecContext(ctx, `
			UPDATE swaps
			SET locked_intent = ?,
			    lockup_address = ?,
			    timeout_block_height = ?,
			    invoice = ?
			WHERE id = ?
		`, string(lockedIntentBytes), quote.LockupAddress, quote.UserTimeoutBlocks, quote.Invoice, swp.ID)
		if err != nil {
			return nil, err
		}

		return s.engine.Transition(ctx, swp.ID, swp.Version, swap.StateLocked, "lock", nil)
	default:
		return nil, fmt.Errorf("unsupported quote kind: %s", quote.Kind)
	}
}

// LockSwap locks swap parameters
func (s *SwapService) LockSwap(ctx context.Context, req *pb.LockSwapRequest) (*pb.SwapSnapshot, error) {
	// For now, we need the quote ID - in full implementation this would be stored
	// For MVP, we'll just transition the state
	swp, err := s.engine.Transition(ctx, req.SwapId, int64(req.ExpectedVersion), swap.StateLocked, "lock", nil)
	if err != nil {
		if err == swap.ErrConcurrentModification {
			return nil, status.Error(codes.Aborted, "concurrent modification")
		}
		if errors.Is(err, swap.ErrInvalidTransition) {
			return nil, status.Error(codes.FailedPrecondition, "invalid transition")
		}
		if isSwapNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "swap not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to lock swap: %v", err)
	}
	return swapToProto(swp), nil
}

// CommitSwap broadcasts the funding transaction
func (s *SwapService) CommitSwap(ctx context.Context, req *pb.CommitSwapRequest) (*pb.SwapSnapshot, error) {
	if req.ExpectedVersion == 0 {
		return nil, status.Error(codes.InvalidArgument, "expected_version is required")
	}

	current, err := s.engine.Get(ctx, req.SwapId)
	if err != nil {
		if isSwapNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "swap not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to load swap: %v", err)
	}

	var swp *swap.Swap
	switch current.Kind {
	case swap.KindSubmarine:
		// Submarine commit keeps the current MVP behavior: transition to COMMIT_STARTED.
		swp, err = s.engine.Transition(ctx, req.SwapId, int64(req.ExpectedVersion), swap.StateCommitStarted, "commit", nil)
	case swap.KindReverse, swap.KindChain:
		swp, err = s.commitProviderBackedSwap(ctx, current, int64(req.ExpectedVersion))
	default:
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported swap kind: %s", current.Kind)
	}
	if err != nil {
		if err == swap.ErrConcurrentModification {
			return nil, status.Error(codes.Aborted, "concurrent modification")
		}
		if errors.Is(err, swap.ErrInvalidTransition) {
			return nil, status.Error(codes.FailedPrecondition, "invalid transition")
		}
		if isSwapNotFoundError(err) {
			return nil, status.Error(codes.NotFound, "swap not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to commit swap: %v", err)
	}
	return swapToProto(swp), nil
}

func (s *SwapService) commitProviderBackedSwap(ctx context.Context, current *swap.Swap, expectedVersion int64) (*swap.Swap, error) {
	lockedIntentRaw, err := s.getSwapLockedIntent(ctx, current.ID)
	if err != nil {
		return nil, err
	}

	lockedIntent, err := parseLockedIntent(lockedIntentRaw)
	if err != nil {
		return nil, err
	}

	opKey := "commit_provider_create"
	done, _, err := s.engine.CheckIdempotency(ctx, current.ID, opKey)
	if err != nil {
		return nil, err
	}

	if !done {
		claimPub, refundPub, err := s.ensureProviderSwapKeys(ctx, current)
		if err != nil {
			return nil, err
		}
		createReq := provider.CreateRequest{
			QuoteID:         lockedIntent.QuoteID,
			Kind:            provider.SwapKind(lockedIntent.Kind),
			FromChain:       provider.Chain(lockedIntent.FromChain),
			ToChain:         provider.Chain(lockedIntent.ToChain),
			AmountSat:       lockedIntent.AmountSat,
			Invoice:         lockedIntent.Invoice,
			Address:         lockedIntent.PayoutAddress,
			PreimageHash:    current.PreimageHashHex,
			MusigPubkeyAgg:  claimPub,
			ClaimPublicKey:  claimPub,
			RefundPublicKey: refundPub,
		}

		createResp, err := s.provider.Create(ctx, createReq)
		if err != nil {
			return nil, fmt.Errorf("provider create failed: %w", err)
		}
		if err := s.persistProviderCreateResponse(ctx, current.ID, createResp); err != nil {
			return nil, err
		}
		if err := s.engine.RecordOperation(ctx, current.ID, opKey, createResp.BoltzID); err != nil {
			return nil, err
		}
	}

	commitStarted, err := s.engine.Transition(ctx, current.ID, expectedVersion, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		return nil, err
	}

	// Reverse/Chain execution continues by waiting for provider/on-chain progress.
	return s.engine.Transition(ctx, current.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
}

func (s *SwapService) ensureProviderSwapKeys(ctx context.Context, current *swap.Swap) (string, string, error) {
	if strings.TrimSpace(current.ClaimPubkeyHex) != "" && strings.TrimSpace(current.RefundPubkeyHex) != "" {
		return current.ClaimPubkeyHex, current.RefundPubkeyHex, nil
	}
	if s.seedSource == nil {
		return "", "", errors.New("seed source unavailable for provider-backed swap keys")
	}
	seed, err := s.seedSource.Seed()
	if err != nil {
		return "", "", fmt.Errorf("vault locked: %w", err)
	}
	_, pub, err := swapkey.Derive(seed, uint32(current.SwapKeyIndex), s.network)
	if err != nil {
		return "", "", fmt.Errorf("derive swap key: %w", err)
	}
	pubHex := encodeCompressed(pub)

	_, refundPub, err := deriveRefundSibling(seed, uint32(current.SwapKeyIndex), s.network)
	if err != nil {
		return "", "", fmt.Errorf("derive refund key: %w", err)
	}
	refundHex := encodeCompressed(refundPub)

	_, err = s.db.ExecContext(ctx, `
		UPDATE swaps
		SET claim_pubkey_hex = CASE WHEN COALESCE(claim_pubkey_hex, '') = '' THEN ? ELSE claim_pubkey_hex END,
		    refund_pubkey_hex = CASE WHEN COALESCE(refund_pubkey_hex, '') = '' THEN ? ELSE refund_pubkey_hex END,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, pubHex, refundHex, current.ID)
	if err != nil {
		return "", "", err
	}

	claimHex := current.ClaimPubkeyHex
	if strings.TrimSpace(claimHex) == "" {
		claimHex = pubHex
	}
	refHex := current.RefundPubkeyHex
	if strings.TrimSpace(refHex) == "" {
		refHex = refundHex
	}
	return claimHex, refHex, nil
}

func encodeCompressed(pub *btcec.PublicKey) string {
	return fmt.Sprintf("%x", pub.SerializeCompressed())
}

func deriveRefundSibling(seed []byte, index uint32, network string) (*btcec.PrivateKey, *btcec.PublicKey, error) {
	// Use adjacent index as refund sibling for deterministic key separation.
	return swapkey.Derive(seed, index+1, network)
}

func (s *SwapService) getSwapLockedIntent(ctx context.Context, swapID string) (string, error) {
	var lockedIntent sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT locked_intent
		FROM swaps
		WHERE id = ?
	`, swapID).Scan(&lockedIntent); err != nil {
		return "", err
	}
	if !lockedIntent.Valid || strings.TrimSpace(lockedIntent.String) == "" {
		return "", errors.New("locked_intent missing")
	}
	return lockedIntent.String, nil
}

func parseLockedIntent(raw string) (swap.LockedIntent, error) {
	payload, err := swap.ParseLockedIntent(raw)
	if err != nil {
		return swap.LockedIntent{}, err
	}
	if strings.TrimSpace(payload.QuoteID) == "" {
		return swap.LockedIntent{}, errors.New("quote_id missing in locked_intent")
	}
	return payload, nil
}

func (s *SwapService) persistProviderCreateResponse(ctx context.Context, swapID string, resp *provider.CreateResponse) error {
	if err := s.persistProviderCreateDetailsOnLockedIntent(ctx, swapID, resp); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE swaps
		SET boltz_id = ?,
		    lockup_address = CASE WHEN ? != '' THEN ? ELSE lockup_address END,
		    claim_address = CASE WHEN ? != '' THEN ? ELSE claim_address END,
		    timeout_block_height = CASE WHEN ? > 0 THEN ? ELSE timeout_block_height END,
		    boltz_raw = CASE WHEN ? != '' THEN ? ELSE boltz_raw END,
		    boltz_status = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`,
		resp.BoltzID,
		resp.LockupAddress, resp.LockupAddress,
		resp.ClaimAddress, resp.ClaimAddress,
		resp.TimeoutBlockHeight, resp.TimeoutBlockHeight,
		string(resp.BoltzRaw), string(resp.BoltzRaw),
		provider.StatusCreated,
		swapID,
	)
	return err
}

func (s *SwapService) persistProviderCreateDetailsOnLockedIntent(ctx context.Context, swapID string, resp *provider.CreateResponse) error {
	lockedIntentRaw, err := s.getSwapLockedIntent(ctx, swapID)
	if err != nil {
		return err
	}

	payload, err := swap.ParseLockedIntent(lockedIntentRaw)
	if err != nil {
		return err
	}

	if resp.BoltzID != "" {
		payload.BoltzID = resp.BoltzID
	}
	if resp.LockupAddress != "" {
		payload.LockupAddress = resp.LockupAddress
	}
	if resp.ClaimAddress != "" {
		payload.ClaimAddress = resp.ClaimAddress
	}
	if len(resp.ReverseDetails) > 0 && string(resp.ReverseDetails) != "null" {
		payload.ReverseDetails = resp.ReverseDetails
	}
	if len(resp.LockupDetails) > 0 && string(resp.LockupDetails) != "null" {
		payload.LockupDetails = resp.LockupDetails
	}
	if len(resp.ClaimDetails) > 0 && string(resp.ClaimDetails) != "null" {
		payload.ClaimDetails = resp.ClaimDetails
	}
	if resp.TimeoutBlockHeight > 0 {
		payload.TimeoutBlocks = int64(resp.TimeoutBlockHeight)
	}

	updatedPayload, err := payload.ToJSON()
	if err != nil {
		return fmt.Errorf("marshal locked_intent update: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, string(updatedPayload), swapID)
	return err
}

// GetSwap retrieves a swap by ID
func (s *SwapService) GetSwap(ctx context.Context, req *pb.GetSwapRequest) (*pb.SwapSnapshot, error) {
	swp, err := s.engine.Get(ctx, req.SwapId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "swap not found: %v", err)
	}
	return swapToProto(swp), nil
}

// CancelSwap cancels a swap
func (s *SwapService) CancelSwap(ctx context.Context, req *pb.CancelSwapRequest) (*pb.SwapSnapshot, error) {
	swp, err := s.engine.Transition(ctx, req.SwapId, int64(req.ExpectedVersion), swap.StateCanceled, "cancel", map[string]interface{}{"reason": req.Reason})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel swap: %v", err)
	}
	return swapToProto(swp), nil
}

// ListSwaps lists swaps
func (s *SwapService) ListSwaps(ctx context.Context, req *pb.ListSwapsRequest) (*pb.ListSwapsResponse, error) {
	// Query all swaps from database
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, env, version, state, swap_key_index,
		       COALESCE(preimage_hash_hex, ''),
		       COALESCE(lockup_txid, ''), COALESCE(lockup_address, ''),
		       COALESCE(error_message, ''),
		       created_at, updated_at
		FROM swaps
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list swaps: %v", err)
	}
	defer rows.Close()

	var swaps []*pb.SwapSnapshot
	for rows.Next() {
		var id, kind, env, state, preimageHashHex, lockupTxid, lockupAddress, errorMsg, createdAt, updatedAt string
		var version, swapKeyIndex int64

		err := rows.Scan(&id, &kind, &env, &version, &state, &swapKeyIndex,
			&preimageHashHex, &lockupTxid, &lockupAddress, &errorMsg,
			&createdAt, &updatedAt)
		if err != nil {
			continue
		}

		swaps = append(swaps, &pb.SwapSnapshot{
			Id:              id,
			Kind:            kindStringToProto(kind),
			State:           stateStringToProto(state),
			Version:         uint64(version),
			Network:         pb.Network_NETWORK_REGTEST,
			SwapKeyIndex:    uint32(swapKeyIndex),
			PreimageHashHex: preimageHashHex,
			LockupTxid:      lockupTxid,
			ErrorMessage:    errorMsg,
		})
	}

	return &pb.ListSwapsResponse{
		Swaps:      swaps,
		TotalCount: int32(len(swaps)),
	}, nil
}

// GetSwapEvents retrieves events for a swap
func (s *SwapService) GetSwapEvents(ctx context.Context, req *pb.GetSwapEventsRequest) (*pb.GetSwapEventsResponse, error) {
	events, err := s.fetchSwapEvents(ctx, req.GetSwapId(), req.GetAfterSeq(), nil, 500)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get swap events: %v", err)
	}
	return &pb.GetSwapEventsResponse{
		Events: events,
	}, nil
}

// WatchSwap streams swap events
func (s *SwapService) WatchSwap(req *pb.WatchSwapRequest, stream pb.SwapService_WatchSwapServer) error {
	if strings.TrimSpace(req.GetSwapId()) == "" {
		return status.Error(codes.InvalidArgument, "swap_id is required")
	}

	lastSeq := req.GetFromSeq()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		events, err := s.fetchSwapEvents(stream.Context(), req.GetSwapId(), lastSeq, nil, 200)
		if err != nil {
			return status.Errorf(codes.Internal, "watch swap failed: %v", err)
		}
		for _, ev := range events {
			if err := stream.Send(ev); err != nil {
				return err
			}
			if ev.Seq > lastSeq {
				lastSeq = ev.Seq
			}
		}

		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

// WatchAllSwaps streams all swap events
func (s *SwapService) WatchAllSwaps(req *pb.WatchAllSwapsRequest, stream pb.SwapService_WatchAllSwapsServer) error {
	filter := make(map[pb.SwapState]bool, len(req.GetFilterStates()))
	for _, st := range req.GetFilterStates() {
		filter[st] = true
	}
	var filterPtr map[pb.SwapState]bool
	if len(filter) > 0 {
		filterPtr = filter
	}

	lastSeq := req.GetFromSeq()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		events, err := s.fetchSwapEvents(stream.Context(), "", lastSeq, filterPtr, 200)
		if err != nil {
			return status.Errorf(codes.Internal, "watch all swaps failed: %v", err)
		}
		for _, ev := range events {
			if err := stream.Send(ev); err != nil {
				return err
			}
			if ev.Seq > lastSeq {
				lastSeq = ev.Seq
			}
		}

		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Helper: convert swap to proto
func swapToProto(s *swap.Swap) *pb.SwapSnapshot {
	return &pb.SwapSnapshot{
		Id:              s.ID,
		Kind:            kindToProto(s.Kind),
		State:           stateToProto(s.State),
		Version:         uint64(s.Version),
		Network:         pb.Network_NETWORK_REGTEST,
		SwapKeyIndex:    uint32(s.SwapKeyIndex),
		PreimageHashHex: s.PreimageHashHex,
		LockupTxid:      s.LockupTxid,
		ErrorMessage:    s.ErrorMessage,
	}
}

func kindToProto(k swap.Kind) pb.SwapKind {
	switch k {
	case swap.KindSubmarine:
		return pb.SwapKind_SWAP_KIND_SUBMARINE
	case swap.KindReverse:
		return pb.SwapKind_SWAP_KIND_REVERSE
	case swap.KindChain:
		return pb.SwapKind_SWAP_KIND_CHAIN
	default:
		return pb.SwapKind_SWAP_KIND_UNSPECIFIED
	}
}

func kindStringToProto(k string) pb.SwapKind {
	switch k {
	case "submarine":
		return pb.SwapKind_SWAP_KIND_SUBMARINE
	case "reverse":
		return pb.SwapKind_SWAP_KIND_REVERSE
	case "chain":
		return pb.SwapKind_SWAP_KIND_CHAIN
	default:
		return pb.SwapKind_SWAP_KIND_UNSPECIFIED
	}
}

func stateToProto(s swap.State) pb.SwapState {
	switch s {
	case swap.StateOpen:
		return pb.SwapState_SWAP_STATE_OPEN
	case swap.StateLocked:
		return pb.SwapState_SWAP_STATE_LOCKED
	case swap.StateCommitStarted:
		return pb.SwapState_SWAP_STATE_COMMIT_STARTED
	case swap.StateWaiting:
		return pb.SwapState_SWAP_STATE_WAITING
	case swap.StateCompleted:
		return pb.SwapState_SWAP_STATE_COMPLETED
	case swap.StateFailed:
		return pb.SwapState_SWAP_STATE_FAILED
	case swap.StateCanceled:
		return pb.SwapState_SWAP_STATE_CANCELED
	default:
		return pb.SwapState_SWAP_STATE_UNSPECIFIED
	}
}

func stateStringToProto(s string) pb.SwapState {
	switch s {
	case "open":
		return pb.SwapState_SWAP_STATE_OPEN
	case "locked":
		return pb.SwapState_SWAP_STATE_LOCKED
	case "commit_started":
		return pb.SwapState_SWAP_STATE_COMMIT_STARTED
	case "waiting":
		return pb.SwapState_SWAP_STATE_WAITING
	case "waiting_claim_details":
		return pb.SwapState_SWAP_STATE_WAITING_CLAIM_DETAILS
	case "signing_musig2_partial":
		return pb.SwapState_SWAP_STATE_SIGNING_MUSIG2_PARTIAL
	case "sent_partial_to_provider":
		return pb.SwapState_SWAP_STATE_SENT_PARTIAL_TO_PROVIDER
	case "waiting_provider_broadcast":
		return pb.SwapState_SWAP_STATE_WAITING_PROVIDER_BROADCAST
	case "refund_coop_waiting":
		return pb.SwapState_SWAP_STATE_REFUND_COOP_WAITING
	case "fallback_script_ready":
		return pb.SwapState_SWAP_STATE_FALLBACK_SCRIPT_READY
	case "refunding":
		return pb.SwapState_SWAP_STATE_REFUNDING
	case "completed":
		return pb.SwapState_SWAP_STATE_COMPLETED
	case "failed":
		return pb.SwapState_SWAP_STATE_FAILED
	case "canceled":
		return pb.SwapState_SWAP_STATE_CANCELED
	default:
		return pb.SwapState_SWAP_STATE_UNSPECIFIED
	}
}

func (s *SwapService) fetchSwapEvents(ctx context.Context, swapID string, afterSeq uint64, filterStates map[pb.SwapState]bool, limit int) ([]*pb.SwapEvent, error) {
	if limit <= 0 {
		limit = 200
	}

	var (
		rows *sql.Rows
		err  error
	)

	if strings.TrimSpace(swapID) != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT seq, swap_id, COALESCE(from_state, ''), to_state, trigger, COALESCE(details, ''), created_at
			FROM swap_events
			WHERE swap_id = ? AND seq > ?
			ORDER BY seq ASC
			LIMIT ?
		`, swapID, afterSeq, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT seq, swap_id, COALESCE(from_state, ''), to_state, trigger, COALESCE(details, ''), created_at
			FROM swap_events
			WHERE seq > ?
			ORDER BY seq ASC
			LIMIT ?
		`, afterSeq, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]*pb.SwapEvent, 0)
	for rows.Next() {
		var (
			seq       int64
			id        string
			fromState string
			toState   string
			trigger   string
			details   string
			createdAt string
		)
		if err := rows.Scan(&seq, &id, &fromState, &toState, &trigger, &details, &createdAt); err != nil {
			return nil, err
		}

		toProto := stateStringToProto(toState)
		if filterStates != nil && !filterStates[toProto] {
			continue
		}

		var ts *timestamppb.Timestamp
		if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			ts = timestamppb.New(parsed)
		}

		events = append(events, &pb.SwapEvent{
			Seq:         uint64(seq),
			SwapId:      id,
			FromState:   stateStringToProto(fromState),
			ToState:     toProto,
			Trigger:     trigger,
			DetailsJson: details,
			Timestamp:   ts,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// protoKindToProvider converts proto SwapKind to provider SwapKind
func protoKindToProvider(k pb.SwapKind) provider.SwapKind {
	switch k {
	case pb.SwapKind_SWAP_KIND_SUBMARINE:
		return provider.SwapKindSubmarine
	case pb.SwapKind_SWAP_KIND_REVERSE:
		return provider.SwapKindReverse
	case pb.SwapKind_SWAP_KIND_CHAIN:
		return provider.SwapKindChain
	default:
		return provider.SwapKindSubmarine // fallback
	}
}

// protoChainToProvider converts proto Chain to provider Chain
func protoChainToProvider(c pb.Chain) provider.Chain {
	switch c {
	case pb.Chain_CHAIN_BTC:
		return provider.ChainBTC
	case pb.Chain_CHAIN_LN:
		return provider.ChainLN
	case pb.Chain_CHAIN_LIQUID:
		return provider.ChainLiquid
	default:
		return provider.ChainBTC // fallback
	}
}

func providerKindToSwapKind(k provider.SwapKind) swap.Kind {
	switch k {
	case provider.SwapKindSubmarine:
		return swap.KindSubmarine
	case provider.SwapKindReverse:
		return swap.KindReverse
	case provider.SwapKindChain:
		return swap.KindChain
	default:
		return swap.KindSubmarine
	}
}

func isSwapNotFoundError(err error) bool {
	return err != nil && (strings.HasPrefix(err.Error(), "swap not found") || errors.Is(err, sql.ErrNoRows))
}
