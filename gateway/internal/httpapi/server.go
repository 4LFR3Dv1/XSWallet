package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"domniwallet/gateway/internal/store"
)

type Server struct {
	store   *store.Store
	secret  []byte
	executor store.Executor
}

func NewServer(s *store.Store, secret string, exec store.Executor) *Server {
	return &Server{store: s, secret: []byte(secret), executor: exec}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/payment-intents", s.handlePaymentIntents)
	mux.HandleFunc("/webhooks/pix", s.handlePixWebhook)
	mux.HandleFunc("/deposits/confirm", s.handleDepositConfirm)
	mux.HandleFunc("/destinations", s.handleDestination)
	mux.HandleFunc("/payouts", s.handlePayouts)
	mux.HandleFunc("/payouts/confirm", s.handlePayoutConfirm)
	mux.HandleFunc("/balance/", s.handleBalance)
	return mux
}

type paymentIntentReq struct {
	UserID      string `json:"user_id"`
	AmountCents int64  `json:"amount_cents"`
}

type paymentIntentResp struct {
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
	PixCode   string `json:"pix_code"`
}

func (s *Server) handlePaymentIntents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req paymentIntentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	pi, err := s.store.CreatePaymentIntent(r.Context(), req.UserID, req.AmountCents)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp := paymentIntentResp{PaymentID: pi.PaymentID, Status: pi.Status, PixCode: "pix://" + pi.PaymentID}
	writeJSON(w, http.StatusCreated, resp)
}

type pixWebhookReq struct {
	PaymentID   string `json:"payment_id"`
	Status      string `json:"status"`
	ProviderRef string `json:"provider_ref"`
	AmountCents int64  `json:"amount_cents"`
	Asset       string `json:"asset"`
}

func (s *Server) handlePixWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if err := s.verifySignature(r.Header.Get("X-Signature"), payload); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_signature"})
		return
	}

	var req pixWebhookReq
	if err := json.Unmarshal(payload, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if req.Status != store.StatusPaid {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	if _, err := s.store.MarkPaymentPaid(r.Context(), req.PaymentID, req.ProviderRef, req.AmountCents); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if _, err := s.store.EnsureDepositPending(r.Context(), req.PaymentID, req.Asset, req.AmountCents); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

type depositConfirmReq struct {
	PaymentID string `json:"payment_id"`
	ChainTxID string `json:"chain_txid"`
}

func (s *Server) handleDepositConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req depositConfirmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	dep, err := s.store.ConfirmDeposit(r.Context(), req.PaymentID, req.ChainTxID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, store.ErrPaymentNotPaid) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, dep)
}

type destinationReq struct {
	UserID  string `json:"user_id"`
	Network string `json:"network"`
	Address string `json:"address"`
}

func (s *Server) handleDestination(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req destinationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if err := s.store.RegisterDestination(r.Context(), req.UserID, req.Network, req.Address); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type payoutReq struct {
	UserID      string `json:"user_id"`
	Network     string `json:"network"`
	Address     string `json:"address"`
	AmountCents int64  `json:"amount_cents"`
}

func (s *Server) handlePayouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req payoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	p, err := s.store.CreatePayout(r.Context(), req.UserID, req.Network, req.Address, req.AmountCents, s.executor)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrInsufficientFunds) {
			status = http.StatusConflict
		} else if errors.Is(err, store.ErrDestinationNotFound) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type payoutConfirmReq struct {
	PayoutID int64 `json:"payout_id"`
}

func (s *Server) handlePayoutConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req payoutConfirmReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	p, err := s.store.ConfirmPayout(r.Context(), req.PayoutID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID := r.URL.Path[len("/balance/"):]
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_user"})
		return
	}
	b, err := s.store.GetBalance(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type balanceResp struct {
		UserID         string `json:"user_id"`
		AvailableCents int64  `json:"available_cents"`
		LockedCents    int64  `json:"locked_cents"`
	}
	writeJSON(w, http.StatusOK, balanceResp{
		UserID:         b.UserID,
		AvailableCents: b.AvailableCents,
		LockedCents:    b.LockedCents,
	})
}

func (s *Server) verifySignature(sig string, payload []byte) error {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	expected := mac.Sum(nil)
	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return err
	}
	if !hmac.Equal(sigBytes, expected) {
		return errors.New("signature mismatch")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func sendSignedWebhook(ctx context.Context, url string, secret []byte, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", sig)

	return http.DefaultClient.Do(req)
}

// Unused in prod handler but helpful for tests.
var _ = sendSignedWebhook
