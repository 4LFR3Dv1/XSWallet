//go:build integration

package test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"domniwallet/gateway/internal/db"
	"domniwallet/gateway/internal/httpapi"
	"domniwallet/gateway/internal/mocks"
	"domniwallet/gateway/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestFlowHappyPath(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	h := srv.Handler()
	ts := httptest.NewServer(h)
	defer ts.Close()
	logLine(t, "server=%s", ts.URL)

	// 1) create payment intent
	piResp := postJSON(t, ts.URL+"/payment-intents", map[string]interface{}{
		"user_id":      "user1",
		"amount_cents": int64(200),
	})
	if piResp.status != http.StatusCreated {
		t.Fatalf("payment-intent status=%d body=%s", piResp.status, string(piResp.body))
	}
	var pi struct {
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(piResp.body, &pi); err != nil {
		t.Fatalf("decode payment-intent: %v", err)
	}
	if pi.PaymentID == "" {
		t.Fatalf("empty payment_id")
	}
	logLine(t, "payment_id=%s", pi.PaymentID)

	// 2) balance should be zero before PIX confirmation
	bal := getBalance(t, ts.URL+"/balance/user1")
	if bal.AvailableCents != 0 || bal.LockedCents != 0 {
		t.Fatalf("expected zero balance, got %+v", bal)
	}
	logLine(t, "balance_pre_pix avail=%d locked=%d", bal.AvailableCents, bal.LockedCents)

	// 3) invalid signature should be rejected
	badResp := postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret+"x", map[string]interface{}{
		"payment_id":   pi.PaymentID,
		"status":       "PAID",
		"provider_ref": "prov-1",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	})
	if badResp.status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d", badResp.status)
	}
	logLine(t, "webhook_invalid_sig status=%d", badResp.status)

	// 4) valid PIX webhook
	okResp := postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, map[string]interface{}{
		"payment_id":   pi.PaymentID,
		"status":       "PAID",
		"provider_ref": "prov-1",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	})
	if okResp.status != http.StatusOK {
		t.Fatalf("expected 200 for webhook, got %d body=%s", okResp.status, string(okResp.body))
	}
	logLine(t, "webhook_valid status=%d", okResp.status)

	// balance still zero until on-chain confirm
	bal = getBalance(t, ts.URL+"/balance/user1")
	if bal.AvailableCents != 0 || bal.LockedCents != 0 {
		t.Fatalf("expected zero balance pre-confirm, got %+v", bal)
	}
	logLine(t, "balance_pre_confirm avail=%d locked=%d", bal.AvailableCents, bal.LockedCents)

	// 5) confirm deposit (on-chain)
	depResp := postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": pi.PaymentID,
		"chain_txid": "tx_liquid_1",
	})
	if depResp.status != http.StatusOK {
		t.Fatalf("deposit confirm failed: %d body=%s", depResp.status, string(depResp.body))
	}
	logLine(t, "deposit_confirm status=%d", depResp.status)

	bal = getBalance(t, ts.URL+"/balance/user1")
	if bal.AvailableCents != 200 || bal.LockedCents != 0 {
		t.Fatalf("expected balance 200/0, got %+v", bal)
	}
	logLine(t, "balance_post_confirm avail=%d locked=%d", bal.AvailableCents, bal.LockedCents)

	// 6) register destination
	destResp := postJSON(t, ts.URL+"/destinations", map[string]interface{}{
		"user_id": "user1",
		"network": "btc",
		"address": "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
	})
	if destResp.status != http.StatusOK {
		t.Fatalf("register destination failed: %d", destResp.status)
	}
	logLine(t, "destination_registered status=%d", destResp.status)

	// 7) payout request
	payoutResp := postJSON(t, ts.URL+"/payouts", map[string]interface{}{
		"user_id":      "user1",
		"network":      "btc",
		"address":      "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
		"amount_cents": int64(100),
	})
	if payoutResp.status != http.StatusOK {
		t.Fatalf("payout failed: %d body=%s", payoutResp.status, string(payoutResp.body))
	}
	var payout struct {
		ID     int64  `json:"ID"`
		Status string `json:"Status"`
	}
	json.Unmarshal(payoutResp.body, &payout)
	if payout.ID == 0 {
		t.Fatalf("invalid payout id")
	}
	logLine(t, "payout_created id=%d status=%s", payout.ID, payout.Status)

	bal = getBalance(t, ts.URL+"/balance/user1")
	if bal.AvailableCents != 100 || bal.LockedCents != 100 {
		t.Fatalf("expected balance 100/100, got %+v", bal)
	}
	logLine(t, "balance_post_payout avail=%d locked=%d", bal.AvailableCents, bal.LockedCents)

	// 8) confirm payout
	confirmResp := postJSON(t, ts.URL+"/payouts/confirm", map[string]interface{}{
		"payout_id": payout.ID,
	})
	if confirmResp.status != http.StatusOK {
		t.Fatalf("confirm payout failed: %d body=%s", confirmResp.status, string(confirmResp.body))
	}
	logLine(t, "payout_confirm status=%d", confirmResp.status)

	bal = getBalance(t, ts.URL+"/balance/user1")
	if bal.AvailableCents != 100 || bal.LockedCents != 0 {
		t.Fatalf("expected balance 100/0 after payout confirm, got %+v", bal)
	}
	logLine(t, "balance_final avail=%d locked=%d", bal.AvailableCents, bal.LockedCents)
}

func TestDepositBlockedWithoutPIX(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	logLine(t, "server=%s", ts.URL)

	piResp := postJSON(t, ts.URL+"/payment-intents", map[string]interface{}{
		"user_id":      "user2",
		"amount_cents": int64(200),
	})
	if piResp.status != http.StatusCreated {
		t.Fatalf("payment-intent status=%d", piResp.status)
	}
	var pi struct {
		PaymentID string `json:"payment_id"`
	}
	json.Unmarshal(piResp.body, &pi)

	depResp := postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": pi.PaymentID,
		"chain_txid": "tx_liquid_2",
	})
	if depResp.status != http.StatusConflict {
		t.Fatalf("expected conflict when confirming without PIX, got %d", depResp.status)
	}
	logLine(t, "deposit_blocked_without_pix status=%d", depResp.status)
}

func TestWebhookAmountMismatch(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	logLine(t, "server=%s", ts.URL)

	paymentID := createPaymentIntent(t, ts.URL, "user3", 200)

	resp := postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, map[string]interface{}{
		"payment_id":   paymentID,
		"status":       "PAID",
		"provider_ref": "prov-x",
		"amount_cents": int64(300), // mismatch
		"asset":        "lbrl",
	})
	if resp.status != http.StatusBadRequest {
		t.Fatalf("expected 400 for amount mismatch, got %d body=%s", resp.status, string(resp.body))
	}

	// deposit confirm should be blocked
	depResp := postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": paymentID,
		"chain_txid": "tx_liquid_x",
	})
	if depResp.status != http.StatusConflict {
		t.Fatalf("expected conflict when confirming without PIX paid, got %d", depResp.status)
	}
}

func TestWebhookIdempotent(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	paymentID := createPaymentIntent(t, ts.URL, "user4", 200)

	payload := map[string]interface{}{
		"payment_id":   paymentID,
		"status":       "PAID",
		"provider_ref": "prov-dup",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	}

	resp1 := postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, payload)
	if resp1.status != http.StatusOK {
		t.Fatalf("webhook #1 failed: %d body=%s", resp1.status, string(resp1.body))
	}

	resp2 := postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, payload)
	if resp2.status != http.StatusOK {
		t.Fatalf("webhook #2 failed: %d body=%s", resp2.status, string(resp2.body))
	}

	depResp := postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": paymentID,
		"chain_txid": "tx_liquid_dup",
	})
	if depResp.status != http.StatusOK {
		t.Fatalf("deposit confirm failed: %d body=%s", depResp.status, string(depResp.body))
	}

	bal := getBalance(t, ts.URL+"/balance/user4")
	if bal.AvailableCents != 200 || bal.LockedCents != 0 {
		t.Fatalf("expected balance 200/0, got %+v", bal)
	}
}

func TestDepositConfirmIdempotent(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	paymentID := createPaymentIntent(t, ts.URL, "user5", 200)

	okResp := postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, map[string]interface{}{
		"payment_id":   paymentID,
		"status":       "PAID",
		"provider_ref": "prov-1",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	})
	if okResp.status != http.StatusOK {
		t.Fatalf("webhook failed: %d", okResp.status)
	}

	for i := 0; i < 2; i++ {
		depResp := postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
			"payment_id": paymentID,
			"chain_txid": "tx_liquid_idem",
		})
		if depResp.status != http.StatusOK {
			t.Fatalf("deposit confirm #%d failed: %d body=%s", i+1, depResp.status, string(depResp.body))
		}
	}

	bal := getBalance(t, ts.URL+"/balance/user5")
	if bal.AvailableCents != 200 || bal.LockedCents != 0 {
		t.Fatalf("expected balance 200/0, got %+v", bal)
	}
}

func TestPayoutRequiresDestination(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	paymentID := createPaymentIntent(t, ts.URL, "user6", 200)
	postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, map[string]interface{}{
		"payment_id":   paymentID,
		"status":       "PAID",
		"provider_ref": "prov-1",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	})
	postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": paymentID,
		"chain_txid": "tx_liquid_ok",
	})

	payoutResp := postJSON(t, ts.URL+"/payouts", map[string]interface{}{
		"user_id":      "user6",
		"network":      "btc",
		"address":      "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
		"amount_cents": int64(100),
	})
	if payoutResp.status != http.StatusForbidden {
		t.Fatalf("expected 403 when destination not registered, got %d", payoutResp.status)
	}
}

func TestPayoutInsufficientFunds(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	paymentID := createPaymentIntent(t, ts.URL, "user7", 200)
	postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, map[string]interface{}{
		"payment_id":   paymentID,
		"status":       "PAID",
		"provider_ref": "prov-1",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	})
	postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": paymentID,
		"chain_txid": "tx_liquid_ok",
	})
	postJSON(t, ts.URL+"/destinations", map[string]interface{}{
		"user_id": "user7",
		"network": "btc",
		"address": "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
	})

	payoutResp := postJSON(t, ts.URL+"/payouts", map[string]interface{}{
		"user_id":      "user7",
		"network":      "btc",
		"address":      "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
		"amount_cents": int64(300),
	})
	if payoutResp.status != http.StatusConflict {
		t.Fatalf("expected 409 for insufficient funds, got %d", payoutResp.status)
	}
}

func TestPayoutConfirmIdempotent(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("set PG_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConn, cleanup := setupDB(t, ctx, dsn)
	defer cleanup()

	st := store.New(dbConn)
	exec := &mocks.Executor{}
	secret := "testsecret"

	srv := httpapi.NewServer(st, secret, exec)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	paymentID := createPaymentIntent(t, ts.URL, "user8", 200)
	postJSONWithSignature(t, ts.URL+"/webhooks/pix", secret, map[string]interface{}{
		"payment_id":   paymentID,
		"status":       "PAID",
		"provider_ref": "prov-1",
		"amount_cents": int64(200),
		"asset":        "lbrl",
	})
	postJSON(t, ts.URL+"/deposits/confirm", map[string]interface{}{
		"payment_id": paymentID,
		"chain_txid": "tx_liquid_ok",
	})
	postJSON(t, ts.URL+"/destinations", map[string]interface{}{
		"user_id": "user8",
		"network": "btc",
		"address": "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
	})

	payoutResp := postJSON(t, ts.URL+"/payouts", map[string]interface{}{
		"user_id":      "user8",
		"network":      "btc",
		"address":      "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
		"amount_cents": int64(100),
	})
	if payoutResp.status != http.StatusOK {
		t.Fatalf("payout failed: %d", payoutResp.status)
	}
	var payout struct {
		ID int64 `json:"ID"`
	}
	_ = json.Unmarshal(payoutResp.body, &payout)
	if payout.ID == 0 {
		t.Fatalf("invalid payout id")
	}

	for i := 0; i < 2; i++ {
		confirmResp := postJSON(t, ts.URL+"/payouts/confirm", map[string]interface{}{
			"payout_id": payout.ID,
		})
		if confirmResp.status != http.StatusOK {
			t.Fatalf("confirm payout #%d failed: %d", i+1, confirmResp.status)
		}
	}

	bal := getBalance(t, ts.URL+"/balance/user8")
	if bal.AvailableCents != 100 || bal.LockedCents != 0 {
		t.Fatalf("expected balance 100/0 after payout confirm, got %+v", bal)
	}
}

type httpResp struct {
	status int
	body   []byte
}

type balance struct {
	UserID         string `json:"user_id"`
	AvailableCents int64  `json:"available_cents"`
	LockedCents    int64  `json:"locked_cents"`
}

func setupDB(t *testing.T, ctx context.Context, dsn string) (*sql.DB, func()) {
	dbConn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := dbConn.PingContext(ctx); err != nil {
		_ = dbConn.Close()
		t.Fatalf("ping db: %v", err)
	}
	if err := db.Migrate(ctx, dbConn); err != nil {
		_ = dbConn.Close()
		t.Fatalf("migrate: %v", err)
	}
	_, err = dbConn.ExecContext(ctx, `TRUNCATE payment_intents, deposits, balances, destinations, payouts RESTART IDENTITY`) // clean
	if err != nil {
		_ = dbConn.Close()
		t.Fatalf("truncate: %v", err)
	}
	return dbConn, func() { _ = dbConn.Close() }
}

func postJSON(t *testing.T, url string, payload interface{}) httpResp {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http post: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return httpResp{status: resp.StatusCode, body: respBody}
}

func postJSONWithSignature(t *testing.T, url, secret string, payload interface{}) httpResp {
	body, _ := json.Marshal(payload)
	sig := hmacSha256Hex([]byte(secret), body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return httpResp{status: resp.StatusCode, body: respBody}
}

func getBalance(t *testing.T, url string) balance {
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var b balance
	if err := json.Unmarshal(body, &b); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	return b
}

func hmacSha256Hex(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func logLine(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	t.Logf(format, args...)
}

func createPaymentIntent(t *testing.T, baseURL, userID string, amount int64) string {
	t.Helper()
	resp := postJSON(t, baseURL+"/payment-intents", map[string]interface{}{
		"user_id":      userID,
		"amount_cents": amount,
	})
	if resp.status != http.StatusCreated {
		t.Fatalf("payment-intent status=%d body=%s", resp.status, string(resp.body))
	}
	var pi struct {
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(resp.body, &pi); err != nil {
		t.Fatalf("decode payment-intent: %v", err)
	}
	if pi.PaymentID == "" {
		t.Fatalf("empty payment_id")
	}
	return pi.PaymentID
}
