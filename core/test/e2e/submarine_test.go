package e2e

import (
	"context"
	"testing"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/provider/mock"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/vault"
)

// TestSubmarineE2E tests the complete submarine swap flow
func TestSubmarineE2E(t *testing.T) {
	ctx := context.Background()

	// Setup database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Setup vault
	vaultInstance := vault.NewVault(database)
	mnemonic, err := vaultInstance.Initialize("", "", "test1234")
	if err != nil {
		t.Fatalf("Failed to initialize vault: %v", err)
	}
	t.Logf("Mnemonic: %s", mnemonic)

	sessionID, err := vaultInstance.Unlock("test1234")
	if err != nil {
		t.Fatalf("Failed to unlock vault: %v", err)
	}
	t.Logf("Session ID: %s", sessionID)

	// Setup Bitcoin RPC (requires regtest running)
	btcClient := bitcoin.NewClient("http://127.0.0.1:18443", "rpcuser", "rpcpass")

	// Verify connection
	height, err := btcClient.Height(ctx)
	if err != nil {
		t.Skipf("Skipping E2E test: bitcoind not available: %v", err)
	}
	t.Logf("Bitcoin height: %d", height)

	// Setup provider
	mockProvider := mock.NewMockProvider()

	// Setup swap engine
	engine := swap.NewEngine(database, e2eTestVault{})

	// Setup submarine orchestrator
	submarine := swap.NewSubmarineOrchestrator(engine, database, mockProvider)

	// Step 1: Create quote
	quoteService := swap.NewQuoteService(database, mockProvider)
	quote, err := quoteService.CreateQuote(ctx, provider.QuoteRequest{
		Kind:      provider.SwapKindSubmarine,
		FromChain: provider.ChainBTC,
		ToChain:   provider.ChainLN,
		AmountSat: 100000,
		Invoice:   "lnbc1000000n1...", // Mock invoice
	})
	if err != nil {
		t.Fatalf("Failed to create quote: %v", err)
	}
	t.Logf("Quote ID: %s", quote.QuoteID)

	// Step 2: Create swap
	swp, err := submarine.CreateFromQuote(ctx, quote.QuoteID)
	if err != nil {
		t.Fatalf("Failed to create swap: %v", err)
	}
	t.Logf("Swap ID: %s, State: %s", swp.ID, swp.State)

	if swp.State != swap.StateOpen {
		t.Fatalf("Expected state OPEN, got %s", swp.State)
	}

	// Step 3: Lock swap
	swp, err = submarine.Lock(ctx, swp.ID, quote.QuoteID)
	if err != nil {
		t.Fatalf("Failed to lock swap: %v", err)
	}
	t.Logf("Swap locked, State: %s", swp.State)

	if swp.State != swap.StateLocked {
		t.Fatalf("Expected state LOCKED, got %s", swp.State)
	}

	// Step 4: Commit swap (this will fail without real UTXOs in regtest)
	// For now, we test that the flow is correct
	t.Logf("Commit step would require funded wallet in regtest")

	// Verify swap can be retrieved
	retrieved, err := engine.Get(ctx, swp.ID)
	if err != nil {
		t.Fatalf("Failed to get swap: %v", err)
	}

	if retrieved.ID != swp.ID {
		t.Fatalf("Retrieved swap ID mismatch")
	}

	t.Logf("E2E test passed (partial - requires funded regtest wallet for full test)")
}
