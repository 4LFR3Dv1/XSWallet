// Package main - XS Wallet Core Daemon (Updated)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/adapters/liquid"
	"github.com/xs-wallet/xscore/internal/adapters/lnd"
	"github.com/xs-wallet/xscore/internal/boltz"
	"github.com/xs-wallet/xscore/internal/config"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/provider/mock"
	"github.com/xs-wallet/xscore/internal/server"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/vault"
	"github.com/xs-wallet/xscore/internal/watcher"
	pb "github.com/xs-wallet/xscore/proto"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

func main() {
	// Flags
	configPath := flag.String("config", "", "Path to config file")
	dataDir := flag.String("datadir", "", "Data directory")
	network := flag.String("network", "regtest", "Network: mainnet, testnet, regtest")
	port := flag.Int("port", 9735, "gRPC server port")
	flag.Parse()

	// Banner
	fmt.Printf("XS Wallet Core v%s (%s)\n", version, commit)
	fmt.Printf("Network: %s\n", *network)

	// Load config
	cfg, err := config.Load(*configPath, *dataDir, *network)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize vault
	vaultInstance := vault.NewVault(database)

	// Initialize adapters
	btcRPCURL := fmt.Sprintf("http://%s:%d", cfg.Bitcoind.Host, cfg.Bitcoind.Port)
	btcClient := bitcoin.NewClient(btcRPCURL, cfg.Bitcoind.User, cfg.Bitcoind.Password)

	// Initialize LND adapter (optional - used for Lightning operations)
	var lndClient *lnd.Client
	if cfg.LND.Enabled && cfg.LND.TLSCert != "" && cfg.LND.Macaroon != "" {
		lndCfg := lnd.Config{
			Host:         fmt.Sprintf("%s:%d", cfg.LND.Host, cfg.LND.Port),
			TLSCertPath:  cfg.LND.TLSCert,
			MacaroonPath: cfg.LND.Macaroon,
			Network:      cfg.Network,
		}
		var err error
		lndClient, err = lnd.NewClient(lndCfg)
		if err != nil {
			log.Printf("Warning: LND adapter unavailable: %v", err)
		} else {
			log.Printf("LND adapter connected to %s:%d", cfg.LND.Host, cfg.LND.Port)
			defer lndClient.Close()
		}
	} else {
		log.Printf("LND adapter not configured (set LND.TLSCert and LND.Macaroon in config)")
	}

	// Initialize Liquid/Elements adapter (optional - used for Liquid operations)
	var liquidClient *liquid.Client
	if cfg.Elementsd.Enabled {
		liquidCfg := liquid.Config{
			Host:     fmt.Sprintf("%s:%d", cfg.Elementsd.Host, cfg.Elementsd.Port),
			User:     cfg.Elementsd.User,
			Password: cfg.Elementsd.Password,
		}
		liquidClient = liquid.NewClient(liquidCfg)
		log.Printf("Liquid adapter configured for %s:%d", cfg.Elementsd.Host, cfg.Elementsd.Port)

		// Test connection
		ctx := context.Background()
		if info, err := liquidClient.GetBlockchainInfo(ctx); err != nil {
			log.Printf("Warning: Liquid connection test failed: %v", err)
		} else {
			log.Printf("Liquid connected: chain=%s, blocks=%d", info.Chain, info.Blocks)
		}
	} else {
		log.Printf("Liquid adapter not configured")
	}

	// Log adapter status summary
	_ = lndClient    // Silence unused variable warning (will be used in future)
	_ = liquidClient // Silence unused variable warning (will be used in future)

	// Initialize provider (Boltz for mainnet/testnet, Mock for regtest fallback)
	var provider provider.Provider

	if cfg.Network == "regtest" && cfg.Boltz.APIURL == "http://127.0.0.1:9001" {
		// Try Boltz first, fallback to mock if unavailable
		boltzCfg := boltz.Config{
			BaseURL: cfg.Boltz.APIURL,
			WSURL:   cfg.Boltz.WSURL,
			Network: cfg.Network,
		}
		boltzProvider, err := boltz.NewProvider(boltzCfg)
		if err != nil {
			log.Printf("Warning: Boltz provider unavailable, using mock: %v", err)
			provider = mock.NewMockProvider()
		} else {
			log.Printf("Using Boltz provider at %s", cfg.Boltz.APIURL)
			provider = boltzProvider
			defer boltzProvider.Close()
		}
	} else {
		// Mainnet/testnet: always use Boltz
		boltzCfg := boltz.Config{
			BaseURL: cfg.Boltz.APIURL,
			WSURL:   cfg.Boltz.WSURL,
			Network: cfg.Network,
		}
		boltzProvider, err := boltz.NewProvider(boltzCfg)
		if err != nil {
			log.Fatalf("Failed to create Boltz provider: %v", err)
		}
		log.Printf("Using Boltz provider at %s", cfg.Boltz.APIURL)
		provider = boltzProvider
		defer boltzProvider.Close()
	}

	// Initialize swap engine
	swapEngine := swap.NewEngine(database, vaultInstance)

	// Initialize watcher
	watcherInstance := watcher.NewWatcher(database, btcClient, swapEngine)

	// Start watcher (includes ReconcileAllActiveSwaps on boot)
	ctx := context.Background()
	if err := watcherInstance.Start(ctx); err != nil {
		log.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcherInstance.Stop()

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			server.LoggingInterceptor,
			server.NewAuthInterceptor(vaultInstance),
		),
		grpc.ChainStreamInterceptor(
			server.StreamLoggingInterceptor,
			server.NewStreamAuthInterceptor(vaultInstance),
		),
	)

	// Register services
	swapService := server.NewSwapService(database, cfg, swapEngine, provider)
	walletService := server.NewWalletService(database, cfg, vaultInstance, btcClient, liquidClient)
	nodeService := server.NewNodeService(cfg)

	pb.RegisterSwapServiceServer(grpcServer, swapService)
	pb.RegisterWalletServiceServer(grpcServer, walletService)
	pb.RegisterNodeServiceServer(grpcServer, nodeService)

	// Enable reflection for debugging
	reflection.Register(grpcServer)

	// Start listener
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		grpcServer.GracefulStop()
		cancel()
	}()

	// Start server
	log.Printf("gRPC server listening on %s", addr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	<-shutdownCtx.Done()
	log.Println("Shutdown complete")
}
