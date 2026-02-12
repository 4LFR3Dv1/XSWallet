package e2e

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/adapters/liquid"
	"github.com/xs-wallet/xscore/internal/config"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/server"
	"github.com/xs-wallet/xscore/internal/vault"
	pb "github.com/xs-wallet/xscore/proto"
)

func TestWalletOnchainBTC(t *testing.T) {
	ctx := context.Background()
	cfg, database, btcClient, liquidClient := setupWalletE2E(t)
	defer database.Close()

	waitForBitcoind(t, ctx, btcClient)

	walletSvc := server.NewWalletService(database, cfg, vault.NewVault(database), btcClient, liquidClient)

	initResp, err := walletSvc.InitializeVault(ctx, &pb.InitializeVaultRequest{
		SeedSource: &pb.InitializeVaultRequest_Generate{Generate: &pb.GenerateNewSeed{WordCount: 24}},
		Pin:        "test1234",
	})
	if err != nil || !initResp.Success {
		t.Fatalf("InitializeVault failed: %v", err)
	}

	addrResp, err := walletSvc.GetNewAddress(ctx, &pb.GetNewAddressRequest{Chain: pb.Chain_CHAIN_BTC, Label: "funding"})
	if err != nil {
		t.Fatalf("GetNewAddress BTC failed: %v", err)
	}

	// Fund address by mining blocks to it (coinbase maturity = 100 blocks)
	if _, err := btcClient.GenerateToAddress(ctx, 101, addrResp.Address); err != nil {
		t.Fatalf("GenerateToAddress BTC failed: %v", err)
	}

	utxos, err := walletSvc.ListUtxos(ctx, &pb.ListUtxosRequest{Chain: pb.Chain_CHAIN_BTC, IncludeReserved: true})
	if err != nil {
		t.Fatalf("ListUtxos BTC failed: %v", err)
	}
	if len(utxos.Utxos) == 0 {
		t.Fatalf("expected BTC UTXOs after funding")
	}

	balance, err := walletSvc.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_BTC})
	if err != nil {
		t.Fatalf("GetBalance BTC failed: %v", err)
	}
	if balance.ConfirmedSat == 0 {
		t.Fatalf("expected confirmed BTC balance > 0")
	}

	destResp, err := walletSvc.GetNewAddress(ctx, &pb.GetNewAddressRequest{Chain: pb.Chain_CHAIN_BTC, Label: "dest"})
	if err != nil {
		t.Fatalf("GetNewAddress BTC dest failed: %v", err)
	}

	sendResp, err := walletSvc.SendOnchain(ctx, &pb.SendOnchainRequest{
		Chain:        pb.Chain_CHAIN_BTC,
		Address:      destResp.Address,
		AmountSat:    10000,
		FeeRateSatVb: 1,
		Label:        "e2e-send",
	})
	if err != nil || !sendResp.Success || sendResp.Txid == "" {
		t.Fatalf("SendOnchain BTC failed: %v", err)
	}

	// Mine a block to confirm
	if _, err := btcClient.GenerateToAddress(ctx, 1, addrResp.Address); err != nil {
		t.Fatalf("GenerateToAddress BTC confirm failed: %v", err)
	}

	utxosAfter, err := walletSvc.ListUtxos(ctx, &pb.ListUtxosRequest{Chain: pb.Chain_CHAIN_BTC, IncludeReserved: true})
	if err != nil {
		t.Fatalf("ListUtxos BTC after send failed: %v", err)
	}
	if len(utxosAfter.Utxos) == 0 {
		t.Fatalf("expected BTC UTXOs after send")
	}
	foundDest := false
	for _, u := range utxosAfter.Utxos {
		if u.Address == destResp.Address && u.AmountSat == 10000 {
			foundDest = true
			break
		}
	}
	if !foundDest {
		t.Fatalf("expected BTC output to destination address")
	}

	txs, err := walletSvc.ListTransactions(ctx, &pb.ListTransactionsRequest{Chain: pb.Chain_CHAIN_BTC, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListTransactions BTC failed: %v", err)
	}
	if txs.TotalCount == 0 {
		t.Fatalf("expected BTC transactions recorded")
	}
}

func TestWalletOnchainLiquid(t *testing.T) {
	ctx := context.Background()
	cfg, database, btcClient, liquidClient := setupWalletE2E(t)
	defer database.Close()

	waitForElements(t, ctx, liquidClient)

	walletSvc := server.NewWalletService(database, cfg, vault.NewVault(database), btcClient, liquidClient)

	initResp, err := walletSvc.InitializeVault(ctx, &pb.InitializeVaultRequest{
		SeedSource: &pb.InitializeVaultRequest_Generate{Generate: &pb.GenerateNewSeed{WordCount: 24}},
		Pin:        "test1234",
	})
	if err != nil || !initResp.Success {
		t.Fatalf("InitializeVault failed: %v", err)
	}

	addrResp, err := walletSvc.GetNewAddress(ctx, &pb.GetNewAddressRequest{Chain: pb.Chain_CHAIN_LIQUID, Label: "funding"})
	if err != nil {
		t.Fatalf("GetNewAddress Liquid failed: %v", err)
	}

	plainAddr := lookupLiquidPlainAddress(t, database)

	// Fund by mining blocks to unconfidential address (matures after 100 blocks)
	if _, err := liquidClient.GenerateToAddress(ctx, 101, plainAddr); err != nil {
		t.Fatalf("GenerateToAddress Liquid failed: %v", err)
	}

	utxos, err := walletSvc.ListUtxos(ctx, &pb.ListUtxosRequest{Chain: pb.Chain_CHAIN_LIQUID, IncludeReserved: true})
	if err != nil {
		t.Fatalf("ListUtxos Liquid failed: %v", err)
	}
	if len(utxos.Utxos) == 0 {
		t.Fatalf("expected Liquid UTXOs after funding")
	}

	balance, err := walletSvc.GetBalance(ctx, &pb.GetBalanceRequest{Chain: pb.Chain_CHAIN_LIQUID})
	if err != nil {
		t.Fatalf("GetBalance Liquid failed: %v", err)
	}
	if balance.ConfirmedSat == 0 {
		t.Fatalf("expected confirmed Liquid balance > 0")
	}

	destResp, err := walletSvc.GetNewAddress(ctx, &pb.GetNewAddressRequest{Chain: pb.Chain_CHAIN_LIQUID, Label: "dest"})
	if err != nil {
		t.Fatalf("GetNewAddress Liquid dest failed: %v", err)
	}

	sendResp, err := walletSvc.SendOnchain(ctx, &pb.SendOnchainRequest{
		Chain:        pb.Chain_CHAIN_LIQUID,
		Address:      destResp.Address,
		AmountSat:    10000,
		FeeRateSatVb: 1,
		Label:        "e2e-send",
	})
	if err != nil || !sendResp.Success || sendResp.Txid == "" {
		t.Fatalf("SendOnchain Liquid failed: %v", err)
	}

	if _, err := liquidClient.GenerateToAddress(ctx, 1, plainAddr); err != nil {
		t.Fatalf("GenerateToAddress Liquid confirm failed: %v", err)
	}

	utxosAfter, err := walletSvc.ListUtxos(ctx, &pb.ListUtxosRequest{Chain: pb.Chain_CHAIN_LIQUID, IncludeReserved: true})
	if err != nil {
		t.Fatalf("ListUtxos Liquid after send failed: %v", err)
	}
	if len(utxosAfter.Utxos) == 0 {
		t.Fatalf("expected Liquid UTXOs after send")
	}

	txs, err := walletSvc.ListTransactions(ctx, &pb.ListTransactionsRequest{Chain: pb.Chain_CHAIN_LIQUID, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListTransactions Liquid failed: %v", err)
	}
	if txs.TotalCount == 0 {
		t.Fatalf("expected Liquid transactions recorded")
	}
}

func setupWalletE2E(t *testing.T) (*config.Config, *db.DB, *bitcoin.Client, *liquid.Client) {
	t.Helper()

	dataDir := t.TempDir()
	cfg, err := config.Load("", dataDir, "regtest")
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	cfg.Bitcoind.Host = getEnv("XS_E2E_BTC_HOST", "127.0.0.1")
	cfg.Bitcoind.Port = getEnvInt("XS_E2E_BTC_PORT", 18443)
	cfg.Bitcoind.User = getEnv("XS_E2E_BTC_USER", "xswallet")
	cfg.Bitcoind.Password = getEnv("XS_E2E_BTC_PASS", "xswallet_dev_pass_2026")

	cfg.Elementsd.Enabled = true
	cfg.Elementsd.Host = getEnv("XS_E2E_ELEMENTS_HOST", "127.0.0.1")
	cfg.Elementsd.Port = getEnvInt("XS_E2E_ELEMENTS_PORT", 18884)
	cfg.Elementsd.User = getEnv("XS_E2E_ELEMENTS_USER", "elements")
	cfg.Elementsd.Password = getEnv("XS_E2E_ELEMENTS_PASS", "elements_dev_pass_2026")

	dbPath := filepath.Join(dataDir, "xs-wallet.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("db.Migrate failed: %v", err)
	}

	btcURL := "http://" + cfg.Bitcoind.Host + ":" + strconv.Itoa(cfg.Bitcoind.Port)
	btcClient := bitcoin.NewClient(btcURL, cfg.Bitcoind.User, cfg.Bitcoind.Password)

	liquidClient := liquid.NewClient(liquid.Config{
		Host:     cfg.Elementsd.Host + ":" + strconv.Itoa(cfg.Elementsd.Port),
		User:     cfg.Elementsd.User,
		Password: cfg.Elementsd.Password,
	})

	return cfg, database, btcClient, liquidClient
}

func waitForBitcoind(t *testing.T, ctx context.Context, client *bitcoin.Client) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.Height(ctx); err == nil {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("bitcoind not reachable - start docker compose stack")
}

func waitForElements(t *testing.T, ctx context.Context, client *liquid.Client) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.GetBlockchainInfo(ctx); err == nil {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("elementsd not reachable - start docker compose stack")
}

func lookupLiquidPlainAddress(t *testing.T, database *db.DB) string {
	t.Helper()
	var plain sql.NullString
	err := database.QueryRow(`SELECT address_plain FROM wallet_addresses WHERE chain = 'liquid' ORDER BY id DESC LIMIT 1`).Scan(&plain)
	if err != nil {
		t.Fatalf("failed to query liquid address: %v", err)
	}
	if !plain.Valid || plain.String == "" {
		t.Fatalf("missing liquid unconfidential address")
	}
	return plain.String
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
