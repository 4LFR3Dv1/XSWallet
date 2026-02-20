package boltz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

// TestCreateReverseIntegration exercises the real boltz-backend /v2/swap/reverse contract.
// Run only when explicitly enabled:
//
//	XS_BOLTZ_REVERSE_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateReverseIntegration -v
func TestCreateReverseIntegration(t *testing.T) {
	if os.Getenv("XS_BOLTZ_REVERSE_INTEGRATION") != "1" {
		t.Skip("set XS_BOLTZ_REVERSE_INTEGRATION=1 to run real /v2/swap/reverse integration test")
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

	from, to, amountSat, err := resolveReversePairAndAmount(context.Background(), client)
	if err != nil {
		t.Fatalf("failed to resolve reverse limits: %v", err)
	}
	preimageHash, err := newUniquePreimageHashReverse()
	if err != nil {
		t.Fatalf("failed to generate unique preimage hash: %v", err)
	}
	claimPriv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("failed to generate claim key: %v", err)
	}

	req := ReverseRequest{
		From:           from,
		To:             to,
		PreimageHash:   preimageHash,
		ClaimPublicKey: hex.EncodeToString(claimPriv.PubKey().SerializeCompressed()),
		InvoiceAmount:  amountSat,
	}

	resp, err := client.CreateReverse(context.Background(), req)
	if err != nil {
		reqJSON, _ := json.Marshal(req)
		t.Fatalf("CreateReverse failed (schema may differ from current client model): %v; request=%s", err, string(reqJSON))
	}
	respJSON, _ := json.Marshal(resp)

	if resp.ID == "" {
		t.Fatalf("CreateReverse returned empty id; response=%s", string(respJSON))
	}
	if resp.Invoice == "" {
		t.Fatalf("CreateReverse returned empty invoice; response=%s", string(respJSON))
	}
	if resp.TimeoutBlockHeight <= 0 {
		t.Fatalf("CreateReverse returned invalid timeoutBlockHeight=%d; response=%s", resp.TimeoutBlockHeight, string(respJSON))
	}
	if resp.RefundPublicKey == "" {
		t.Fatalf("CreateReverse returned empty refundPublicKey; response=%s", string(respJSON))
	}

	t.Logf("CreateReverse OK: id=%s timeout=%d response=%s", resp.ID, resp.TimeoutBlockHeight, string(respJSON))
}

func resolveReversePairAndAmount(ctx context.Context, client *Client) (string, string, int64, error) {
	pairs, err := client.GetReversePairs(ctx)
	if err != nil {
		return "", "", 0, err
	}
	for from, toMap := range pairs {
		for to, pair := range toMap {
			if pair.Limits.Minimal <= 0 {
				continue
			}
			amount := pair.Limits.Minimal + 10000
			if amount > pair.Limits.Maximal {
				amount = pair.Limits.Minimal
			}
			if amount > 0 {
				return from, to, amount, nil
			}
		}
	}
	return "", "", 0, ErrAmountOutOfBounds
}

func newUniquePreimageHashReverse() (string, error) {
	var preimage [32]byte
	if _, err := rand.Read(preimage[:]); err != nil {
		return "", err
	}
	sum := sha256.Sum256(preimage[:])
	return hex.EncodeToString(sum[:]), nil
}
