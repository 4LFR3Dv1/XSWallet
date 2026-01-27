package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"domniwallet/wallet-service/internal/address"
	"domniwallet/wallet-service/internal/db"
	"domniwallet/wallet-service/internal/httpapi"
	"domniwallet/wallet-service/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	addr := getenv("WALLET_HTTP_ADDR", ":8081")
	dsn := os.Getenv("WALLET_DB_DSN")
	if dsn == "" {
		log.Fatal("WALLET_DB_DSN is required")
	}

	btcURL := os.Getenv("BTC_RPC_URL")
	btcUser := os.Getenv("BTC_RPC_USER")
	btcPass := os.Getenv("BTC_RPC_PASS")
	btcWallet := os.Getenv("BTC_RPC_WALLET")
	liqURL := os.Getenv("LIQUID_RPC_URL")
	liqUser := os.Getenv("LIQUID_RPC_USER")
	liqPass := os.Getenv("LIQUID_RPC_PASS")
	internalToken := os.Getenv("WALLET_INTERNAL_TOKEN")

	dbConn, err := sql.Open("pgx", dsn)
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
	provider := address.NewProvider(btcURL, btcUser, btcPass, btcWallet, liqURL, liqUser, liqPass)

	server := httpapi.New(st, provider, internalToken)
	httpServer := &http.Server{Addr: addr, Handler: server.Handler()}

	go func() {
		log.Printf("wallet-service listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = httpServer.Shutdown(ctxShutdown)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
