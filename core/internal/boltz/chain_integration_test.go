package boltz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

// TestCreateChainIntegration exercises the real boltz-backend /v2/swap/chain contract.
// Run only when explicitly enabled:
//
//	XS_BOLTZ_CHAIN_INTEGRATION=1 XS_BOLTZ_API_URL=http://127.0.0.1:9001 go test ./internal/boltz -run TestCreateChainIntegration -v
func TestCreateChainIntegration(t *testing.T) {
	if os.Getenv("XS_BOLTZ_CHAIN_INTEGRATION") != "1" {
		t.Skip("set XS_BOLTZ_CHAIN_INTEGRATION=1 to run real /v2/swap/chain integration test")
	}

	baseURL := os.Getenv("XS_BOLTZ_API_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9001"
	}

	if err := requireBoltzHealthy(baseURL); err != nil {
		t.Fatalf("boltz backend not reachable at %s: %v", baseURL, err)
	}

	client := NewClient(ClientConfig{
		BaseURL: baseURL,
		Timeout: 20 * time.Second,
	})
	amountSat, err := resolveChainAmount(context.Background(), client, "BTC", "L-BTC")
	if err != nil {
		t.Fatalf("failed to resolve chain limits: %v", err)
	}

	preimageHash, err := newUniquePreimageHash()
	if err != nil {
		t.Fatalf("failed to generate unique preimage hash: %v", err)
	}
	claimPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("failed to generate claim key: %v", err)
	}
	refundPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("failed to generate refund key: %v", err)
	}

	req := ChainRequest{
		From:            "BTC",
		To:              "L-BTC",
		PreimageHash:    preimageHash,
		MusigPubkeyAgg:  hex.EncodeToString(claimPriv.PubKey().SerializeCompressed()),
		ClaimPublicKey:  hex.EncodeToString(claimPriv.PubKey().SerializeCompressed()),
		RefundPublicKey: hex.EncodeToString(refundPriv.PubKey().SerializeCompressed()),
		FromAmount:      amountSat,
		UserLockAmount:  amountSat,
	}

	resp, err := client.CreateChain(context.Background(), req)
	if err != nil {
		reqJSON, _ := json.Marshal(req)
		t.Fatalf("CreateChain failed (schema may differ from current client model): %v; request=%s", err, string(reqJSON))
	}
	respJSON, _ := json.Marshal(resp)

	if resp.ID == "" {
		t.Fatalf("CreateChain returned empty id")
	}
	lockupAddress := resp.EffectiveLockupAddress()
	timeout := resp.EffectiveTimeoutBlockHeight()
	if lockupAddress == "" {
		t.Fatalf("CreateChain returned empty effective lockupAddress; response=%s", string(respJSON))
	}
	if timeout <= 0 {
		t.Fatalf("CreateChain returned invalid effective timeoutBlockHeight=%d; response=%s", timeout, string(respJSON))
	}

	t.Logf("CreateChain OK: id=%s lockup=%s timeout=%d response=%s", resp.ID, lockupAddress, timeout, string(respJSON))
}

func newUniquePreimageHash() (string, error) {
	// 32 bytes random preimage, then SHA256 to produce a valid unique preimageHash.
	var preimage [32]byte
	if _, err := rand.Read(preimage[:]); err != nil {
		return "", err
	}
	sum := sha256.Sum256(preimage[:])
	return hex.EncodeToString(sum[:]), nil
}

func resolveChainAmount(ctx context.Context, client *Client, from, to string) (int64, error) {
	pairs, err := client.GetChainPairs(ctx)
	if err != nil {
		return 0, err
	}
	fromPairs, ok := pairs[from]
	if !ok {
		return 0, fmt.Errorf("pair source not found: %s", from)
	}
	pair, ok := fromPairs[to]
	if !ok {
		return 0, fmt.Errorf("pair target not found: %s -> %s", from, to)
	}
	amount := pair.Limits.Minimal + 10000
	if amount > pair.Limits.Maximal {
		amount = pair.Limits.Minimal
	}
	if amount <= 0 {
		return 0, fmt.Errorf("invalid chain pair limits: minimal=%d maximal=%d", pair.Limits.Minimal, pair.Limits.Maximal)
	}
	return amount, nil
}

func requireBoltzHealthy(baseURL string) error {
	resp, err := http.Get(baseURL + "/version")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET /version returned %d: %s", resp.StatusCode, string(body))
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("invalid /version payload: %w", err)
	}
	return nil
}
