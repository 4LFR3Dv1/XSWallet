package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"domniwallet/wallet-service/internal/address"
	"domniwallet/wallet-service/internal/store"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	store         *store.Store
	addrProvider  *address.Provider
	internalToken string
}

func New(store *store.Store, provider *address.Provider, internalToken string) *Server {
	return &Server{store: store, addrProvider: provider, internalToken: internalToken}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	// Public API
	r.Post("/v1/accounts", s.handleCreateAccount)
	r.Post("/v1/accounts/{uuid}/addresses", s.handleCreateAddress)
	r.Get("/v1/accounts/{uuid}/balances", s.handleBalances)
	r.Get("/v1/accounts/{uuid}/transactions", s.handleTransactions)
	r.Post("/v1/withdrawals", s.handleCreateWithdrawal)
	r.Get("/v1/withdrawals/{id}", s.handleGetWithdrawal)

	// Internal (watcher) endpoints
	r.Route("/v1/internal", func(ir chi.Router) {
		ir.Use(s.requireInternalToken)
		ir.Post("/utxos", s.handleUpsertUTXO)
		ir.Post("/accounts", s.handleUpsertAccount)
		ir.Post("/withdrawals/status", s.handleUpdateWithdrawalStatus)
	})

	return r
}

type createAccountReq struct {
	UUID     string `json:"uuid"`
	UserXpub string `json:"user_xpub"`
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	acc, err := s.store.CreateAccount(r.Context(), req.UUID, req.UserXpub)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

type createAddressReq struct {
	Network string `json:"network"`
	Asset   string `json:"asset"`
}

type createAddressResp struct {
	Address        string `json:"address"`
	DerivationPath string `json:"derivation_path"`
}

func (s *Server) handleCreateAddress(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req createAddressReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	addr, derivation, err := s.addrProvider.NextAddress(r.Context(), req.Network, req.Asset)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_, err = s.store.CreateAddress(r.Context(), uuid, req.Network, req.Asset, addr, derivation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, createAddressResp{Address: addr, DerivationPath: derivation})
}

func (s *Server) handleBalances(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	balances, err := s.store.GetBalances(r.Context(), uuid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{ "balances": balances })
}

func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil {
			limit = v
		}
	}
	txs, err := s.store.ListTransactions(r.Context(), uuid, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{ "transactions": txs })
}

type createWithdrawalReq struct {
	AccountUUID string `json:"account_uuid"`
	Network     string `json:"network"`
	Asset       string `json:"asset"`
	Amount      int64  `json:"amount"`
	Destination string `json:"destination"`
}

func (s *Server) handleCreateWithdrawal(w http.ResponseWriter, r *http.Request) {
	var req createWithdrawalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	wdr, err := s.store.CreateWithdrawal(r.Context(), req.AccountUUID, req.Network, req.Asset, req.Destination, req.Amount)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrInsufficient) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, wdr)
}

func (s *Server) handleGetWithdrawal(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
		return
	}
	wdr, err := s.store.GetWithdrawal(r.Context(), id)
	if err != nil {
		status := http.StatusNotFound
		if !errors.Is(err, store.ErrNotFound) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wdr)
}

type upsertUtxoReq struct {
	Network       string `json:"network"`
	Asset         string `json:"asset"`
	TxID          string `json:"txid"`
	Vout          int64  `json:"vout"`
	Address       string `json:"address"`
	Amount        int64  `json:"amount"`
	Confirmations int    `json:"confirmations"`
	ConfirmedAt   string `json:"confirmed_at"`
	BlockHash     string `json:"block_hash"`
	BlockHeight   int64  `json:"block_height"`
}

func (s *Server) handleUpsertUTXO(w http.ResponseWriter, r *http.Request) {
	var req upsertUtxoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	confirmedAt := time.Now().UTC()
	if req.ConfirmedAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ConfirmedAt); err == nil {
			confirmedAt = t
		}
	}
	if err := s.store.UpsertUTXO(r.Context(), req.Network, req.Asset, req.TxID, req.Vout, req.Address, req.Amount, req.Confirmations, confirmedAt, req.BlockHash, req.BlockHeight); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type upsertAccountReq struct {
	Network   string `json:"network"`
	Asset     string `json:"asset"`
	Address   string `json:"address"`
	Balance   int64  `json:"balance"`
	LastBlock int64  `json:"last_block"`
	LastTxID  string `json:"last_txid"`
}

func (s *Server) handleUpsertAccount(w http.ResponseWriter, r *http.Request) {
	var req upsertAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if err := s.store.UpsertAccountBalance(r.Context(), req.Network, req.Asset, req.Address, req.Balance, req.LastBlock, req.LastTxID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type updateWithdrawalReq struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	TxID   string `json:"txid"`
}

func (s *Server) handleUpdateWithdrawalStatus(w http.ResponseWriter, r *http.Request) {
	var req updateWithdrawalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if req.ID == 0 || req.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	wdr, err := s.store.UpdateWithdrawalStatus(r.Context(), req.ID, req.Status, req.TxID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wdr)
}

func (s *Server) requireInternalToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.internalToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(r.Header.Get("X-Internal-Token"))
		if token != s.internalToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
