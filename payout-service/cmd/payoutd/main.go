package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"domniwallet/payout-service/internal/config"
	"domniwallet/payout-service/internal/db"
	"domniwallet/payout-service/internal/engine"
	"domniwallet/payout-service/internal/executors/btc"
	"domniwallet/payout-service/internal/executors/liquid"
	"domniwallet/payout-service/internal/executors/tron"
	"domniwallet/payout-service/internal/httpapi"
	"domniwallet/payout-service/internal/store"
	"domniwallet/payout-service/internal/wallet"
	"domniwallet/reuse/xs_wallet/adapters/bitcoin"
	"github.com/btcsuite/btcd/chaincfg"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.Load()
	if cfg.DBDSN == "" {
		log.Fatal("PAYOUT_DB_DSN is required")
	}

	dbConn, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		log.Fatalf("db open failed: %v", err)
	}
	defer dbConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := dbConn.PingContext(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	if err := db.Migrate(ctx, dbConn); err != nil {
		log.Fatalf("db migrate failed: %v", err)
	}

	st := store.New(dbConn)
	walletClient := wallet.New(cfg.WalletServiceURL, cfg.WalletInternalToken)

	executors := map[string]engine.Executor{}
	var btcParams *chaincfg.Params
	var btcRPC *bitcoin.Client
	if cfg.BTC.RPCURL != "" {
		btcExec, err := btc.New(cfg.BTC.RPCURL, cfg.BTC.RPCUser, cfg.BTC.RPCPass, cfg.BTC.RPCWallet, cfg.BTC.Network, cfg.BTC.Confirmations, cfg.BTC.FeeFallbackSatVb)
		if err != nil {
			log.Fatalf("btc executor init failed: %v", err)
		}
		executors["btc"] = btcExec
		btcParams = btcExec.Params()
		baseURL := cfg.BTC.RPCURL
		if idx := strings.Index(cfg.BTC.RPCURL, "/wallet/"); idx != -1 {
			baseURL = cfg.BTC.RPCURL[:idx]
		}
		btcRPC = bitcoin.NewClient(baseURL, cfg.BTC.RPCUser, cfg.BTC.RPCPass)
	}
	executors["liquid"] = liquid.New()
	executors["tron"] = tron.New()

	eng := engine.New(st, walletClient, executors, cfg.MaxAttempts, cfg.WorkerInterval, cfg.CircuitFailureThreshold, cfg.CircuitOpenDuration)

	server := httpapi.New(st, eng, btcParams, btcRPC)
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: server.Handler()}

	ctxEngine, cancelEngine := context.WithCancel(context.Background())
	go eng.Run(ctxEngine)

	go func() {
		log.Printf("payout-service listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	cancelEngine()
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = httpServer.Shutdown(ctxShutdown)
}
