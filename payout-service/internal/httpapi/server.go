package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"domniwallet/payout-service/internal/engine"
	"domniwallet/payout-service/internal/store"
	"domniwallet/reuse/xs_wallet/adapters/bitcoin"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Server struct {
	store     *store.Store
	engine    *engine.Engine
	btcParams *chaincfg.Params
	btcRPC    *bitcoin.Client
}

func New(store *store.Store, engine *engine.Engine, btcParams *chaincfg.Params, btcRPC *bitcoin.Client) *Server {
	return &Server{store: store, engine: engine, btcParams: btcParams, btcRPC: btcRPC}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Post("/v1/payouts", s.handleCreatePayout)
	r.Get("/v1/payouts/{id}", s.handleGetPayout)
	r.Post("/v1/payouts/{id}/retry", s.handleRetryPayout)
	return r
}

type payoutReq struct {
	PaymentID    string                 `json:"payment_id"`
	WithdrawalID *int64                 `json:"withdrawal_id,omitempty"`
	Network      string                 `json:"network"`
	Asset        string                 `json:"asset"`
	Destination  string                 `json:"destination"`
	AmountSats   int64                  `json:"amount_sats"`
	Priority     string                 `json:"priority"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

func (s *Server) handleCreatePayout(w http.ResponseWriter, r *http.Request) {
	var req payoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, apiError{Code: "PAYOUT_INVALID_REQUEST", Message: "invalid_json"})
		return
	}
	if apiErr := s.validatePayoutReq(r.Context(), req); apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var withdrawalID sql.NullInt64
	if req.WithdrawalID != nil && *req.WithdrawalID > 0 {
		withdrawalID = sql.NullInt64{Int64: *req.WithdrawalID, Valid: true}
	}

	payout, err := s.store.CreatePayout(r.Context(), store.CreatePayoutInput{
		PaymentID:    req.PaymentID,
		WithdrawalID: withdrawalID,
		Network:      req.Network,
		Asset:        req.Asset,
		Amount:       req.AmountSats,
		Destination:  req.Destination,
		Priority:     req.Priority,
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeError(w, http.StatusConflict, apiError{Code: "PAYOUT_DUPLICATE", Message: "payment_id already processed"})
			return
		}
		if errors.Is(err, store.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, apiError{Code: "PAYOUT_INVALID_REQUEST", Message: "invalid_input"})
			return
		}
		writeError(w, http.StatusInternalServerError, apiError{Code: "PAYOUT_INTERNAL_ERROR", Message: "internal_error", Retryable: true})
		return
	}

	s.engine.Kick()
	writeJSON(w, http.StatusCreated, payoutToResponse(payout))
}

func (s *Server) handleGetPayout(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, apiError{Code: "PAYOUT_INVALID_ID", Message: "invalid_id"})
		return
	}
	payout, err := s.store.GetPayout(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, apiError{Code: "PAYOUT_NOT_FOUND", Message: "not_found"})
			return
		}
		writeError(w, http.StatusInternalServerError, apiError{Code: "PAYOUT_INTERNAL_ERROR", Message: "internal_error", Retryable: true})
		return
	}
	writeJSON(w, http.StatusOK, payoutToResponse(payout))
}

func (s *Server) handleRetryPayout(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, apiError{Code: "PAYOUT_INVALID_ID", Message: "invalid_id"})
		return
	}
	now := time.Now().UTC()
	if err := s.store.UpdateStatus(r.Context(), id, store.StatusFailedRetry, "", "admin_retry", &now); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, apiError{Code: "PAYOUT_NOT_FOUND", Message: "not_found"})
			return
		}
		writeError(w, http.StatusInternalServerError, apiError{Code: "PAYOUT_INTERNAL_ERROR", Message: "internal_error", Retryable: true})
		return
	}
	s.engine.Kick()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) validatePayoutReq(ctx context.Context, req payoutReq) *apiError {
	if _, err := uuid.Parse(req.PaymentID); err != nil {
		return &apiError{Code: "PAYOUT_INVALID_PAYMENT_ID", Message: "invalid payment_id", Details: map[string]interface{}{"field": "payment_id"}, Retryable: false}
	}
	switch req.Network {
	case "btc", "liquid", "tron":
	default:
		return &apiError{Code: "PAYOUT_INVALID_NETWORK", Message: "invalid network", Details: map[string]interface{}{"field": "network"}}
	}
	if req.AmountSats <= 0 {
		return &apiError{Code: "PAYOUT_INVALID_AMOUNT", Message: "amount must be > 0", Details: map[string]interface{}{"field": "amount_sats"}}
	}
	if req.Destination == "" {
		return &apiError{Code: "PAYOUT_INVALID_ADDRESS", Message: "destination required", Details: map[string]interface{}{"field": "destination"}}
	}
	if hasMixedCaseBech32(req.Destination) {
		return &apiError{Code: "PAYOUT_MIXED_CASE_BECH32", Message: "bech32 must be single case", Details: map[string]interface{}{"field": "destination", "reason": "mixed_case_bech32", "value": req.Destination}}
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if req.Priority != "fast" && req.Priority != "normal" && req.Priority != "slow" {
		return &apiError{Code: "PAYOUT_INVALID_PRIORITY", Message: "invalid priority", Details: map[string]interface{}{"field": "priority"}}
	}
	if req.Network == "btc" && s.btcParams != nil {
		if err := s.validateBTCDestination(ctx, req.Destination); err != nil {
			return err
		}
	}
	if req.Network == "tron" {
		if err := validateTronDestination(req.Destination); err != nil {
			return err
		}
	}
	if req.Network == "liquid" {
		if err := validateLiquidDestination(req.Destination); err != nil {
			return err
		}
	}
	return nil
}

type payoutResp struct {
	ID           int64  `json:"id"`
	PaymentID    string `json:"payment_id"`
	WithdrawalID *int64 `json:"withdrawal_id,omitempty"`
	Network      string `json:"network"`
	Asset        string `json:"asset"`
	Amount       int64  `json:"amount"`
	Destination  string `json:"destination"`
	Status       string `json:"status"`
	TxID         string `json:"txid,omitempty"`
	Attempts     int    `json:"attempts"`
}

func payoutToResponse(p *store.Payout) payoutResp {
	var wid *int64
	if p.WithdrawalID.Valid {
		wid = &p.WithdrawalID.Int64
	}
	var txid string
	if p.TxID.Valid {
		txid = p.TxID.String
	}
	return payoutResp{
		ID:           p.ID,
		PaymentID:    p.PaymentID,
		WithdrawalID: wid,
		Network:      p.Network,
		Asset:        p.Asset,
		Amount:       p.Amount,
		Destination:  p.Destination,
		Status:       p.Status,
		TxID:         txid,
		Attempts:     p.Attempts,
	}
}

type apiError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable"`
}

func writeAPIError(w http.ResponseWriter, err *apiError) {
	if err == nil {
		return
	}
	status := http.StatusBadRequest
	retryAfter := 30
	switch err.Code {
	case "PAYOUT_DUPLICATE":
		status = http.StatusConflict
	case "PAYOUT_LIMIT_EXCEEDED", "PAYOUT_DESTINATION_COOLDOWN":
		status = http.StatusTooManyRequests
		retryAfter = 60
	case "PAYOUT_CIRCUIT_OPEN":
		status = http.StatusServiceUnavailable
		err.Retryable = true
		retryAfter = 60
	case "PAYOUT_INSUFFICIENT_BALANCE":
		status = http.StatusServiceUnavailable
		err.Retryable = true
		retryAfter = 300
	case "PAYOUT_NETWORK_UNAVAILABLE":
		status = http.StatusServiceUnavailable
		err.Retryable = true
	}
	writeError(w, status, *err, retryAfter)
}

func writeError(w http.ResponseWriter, status int, err apiError, retryAfterSec ...int) {
	if err.Retryable {
		w.Header().Set("X-Retryable", "true")
		retry := 30
		if len(retryAfterSec) > 0 && retryAfterSec[0] > 0 {
			retry = retryAfterSec[0]
		}
		w.Header().Set("X-Retry-After", strconv.Itoa(retry))
	}
	writeJSON(w, status, map[string]interface{}{
		"error": err,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func parseID(r *http.Request, key string) (int64, error) {
	idStr := chi.URLParam(r, key)
	var id int64
	for _, c := range idStr {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid_id")
		}
		id = id*10 + int64(c-'0')
	}
	return id, nil
}

func (s *Server) validateBTCDestination(ctx context.Context, address string) *apiError {
	if len(address) < 26 || len(address) > 90 {
		return &apiError{
			Code:    "PAYOUT_INVALID_ADDRESS",
			Message: "invalid address length",
			Details: map[string]interface{}{"field": "destination", "reason": "invalid_length", "value": address},
		}
	}
	if s.btcRPC != nil {
		info, err := s.btcRPC.GetAddressInfo(ctx, address)
		if err != nil || info == nil || !info.IsValid {
			return &apiError{
				Code:    "PAYOUT_INVALID_ADDRESS",
				Message: "rpc validation failed",
				Details: map[string]interface{}{"field": "destination", "reason": "rpc_validation_failed", "value": address},
			}
		}
		if !info.IsWitness || info.WitnessVersion != 1 {
			return &apiError{
				Code:    "PAYOUT_INVALID_ADDRESS",
				Message: "not taproot",
				Details: map[string]interface{}{"field": "destination", "reason": "not_taproot", "value": address},
			}
		}
		return nil
	}
	if s.btcParams != nil && !bitcoin.IsTaprootAddress(address, s.btcParams) {
		return &apiError{
			Code:    "PAYOUT_INVALID_ADDRESS",
			Message: "not taproot",
			Details: map[string]interface{}{"field": "destination", "reason": "not_taproot", "value": address},
		}
	}
	return nil
}

func validateTronDestination(address string) *apiError {
	if len(address) < 34 || !strings.HasPrefix(address, "T") {
		return &apiError{
			Code:    "PAYOUT_INVALID_ADDRESS",
			Message: "invalid tron address",
			Details: map[string]interface{}{"field": "destination", "reason": "invalid_format", "value": address},
		}
	}
	return nil
}

func validateLiquidDestination(address string) *apiError {
	if len(address) < 26 || len(address) > 120 {
		return &apiError{
			Code:    "PAYOUT_INVALID_ADDRESS",
			Message: "invalid liquid address",
			Details: map[string]interface{}{"field": "destination", "reason": "invalid_format", "value": address},
		}
	}
	return nil
}

func hasMixedCaseBech32(address string) bool {
	lower := strings.ToLower(address)
	upper := strings.ToUpper(address)
	if lower == address || upper == address {
		return false
	}
	prefixes := []string{"bc1", "tb1", "bcrt1", "lq1", "el1"}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}
