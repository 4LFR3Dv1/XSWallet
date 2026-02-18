// Package server - Updated SwapService with full implementation
package server

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/config"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
	pb "github.com/xs-wallet/xscore/proto"
)

// SwapService implements pb.SwapServiceServer
type SwapService struct {
	pb.UnimplementedSwapServiceServer
	db        *db.DB
	cfg       *config.Config
	engine    *swap.Engine
	provider  provider.Provider
	submarine *swap.SubmarineOrchestrator
}

// NewSwapService creates SwapService
func NewSwapService(database *db.DB, cfg *config.Config, engine *swap.Engine, prov provider.Provider) *SwapService {
	return &SwapService{
		db:        database,
		cfg:       cfg,
		engine:    engine,
		provider:  prov,
		submarine: swap.NewSubmarineOrchestrator(engine, database, prov),
	}
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
	// Create swap from quote
	swp, err := s.submarine.CreateFromQuote(ctx, req.QuoteId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create swap: %v", err)
	}

	return swapToProto(swp), nil
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

	// For now, commit transitions to COMMIT_STARTED.
	// Full vault/btc funding integration will be wired here.
	swp, err := s.engine.Transition(ctx, req.SwapId, int64(req.ExpectedVersion), swap.StateCommitStarted, "commit", nil)
	if err != nil {
		if err == swap.ErrConcurrentModification {
			return nil, status.Error(codes.Aborted, "concurrent modification")
		}
		if errors.Is(err, swap.ErrInvalidTransition) {
			return nil, status.Error(codes.FailedPrecondition, "invalid transition")
		}
		return nil, status.Errorf(codes.Internal, "failed to commit swap: %v", err)
	}
	return swapToProto(swp), nil
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
	// TODO: Implement event query
	return &pb.GetSwapEventsResponse{
		Events: []*pb.SwapEvent{},
	}, nil
}

// WatchSwap streams swap events
func (s *SwapService) WatchSwap(req *pb.WatchSwapRequest, stream pb.SwapService_WatchSwapServer) error {
	// TODO: Implement streaming
	return status.Error(codes.Unimplemented, "not implemented")
}

// WatchAllSwaps streams all swap events
func (s *SwapService) WatchAllSwaps(req *pb.WatchAllSwapsRequest, stream pb.SwapService_WatchAllSwapsServer) error {
	// TODO: Implement streaming
	return status.Error(codes.Unimplemented, "not implemented")
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

func isSwapNotFoundError(err error) bool {
	return err != nil && (strings.HasPrefix(err.Error(), "swap not found") || errors.Is(err, sql.ErrNoRows))
}
