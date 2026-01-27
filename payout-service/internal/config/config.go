package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr string
	DBDSN    string

	WalletServiceURL    string
	WalletInternalToken string

	BTC struct {
		RPCURL           string
		RPCUser          string
		RPCPass          string
		RPCWallet        string
		Network          string
		Confirmations    int64
		FeeFallbackSatVb float64
	}

	MaxAttempts             int
	WorkerInterval          time.Duration
	CircuitFailureThreshold int
	CircuitOpenDuration     time.Duration
}

func Load() Config {
	cfg := Config{
		HTTPAddr: ":8090",
	}

	cfg.DBDSN = os.Getenv("PAYOUT_DB_DSN")
	if v := os.Getenv("PAYOUT_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	cfg.WalletServiceURL = os.Getenv("WALLET_SERVICE_URL")
	cfg.WalletInternalToken = os.Getenv("WALLET_INTERNAL_TOKEN")

	cfg.BTC.RPCURL = os.Getenv("BTC_RPC_URL")
	cfg.BTC.RPCUser = os.Getenv("BTC_RPC_USER")
	cfg.BTC.RPCPass = os.Getenv("BTC_RPC_PASS")
	cfg.BTC.RPCWallet = os.Getenv("BTC_RPC_WALLET")
	cfg.BTC.Network = os.Getenv("BTC_NETWORK")
	cfg.BTC.Confirmations = int64(getInt("PAYOUT_BTC_CONFIRMATIONS", 1))
	cfg.BTC.FeeFallbackSatVb = getFloat("PAYOUT_FEE_FALLBACK_SATVB", 10)

	cfg.MaxAttempts = getInt("PAYOUT_MAX_ATTEMPTS", 3)
	cfg.WorkerInterval = getDuration("PAYOUT_WORKER_INTERVAL", 2*time.Second)
	cfg.CircuitFailureThreshold = getInt("PAYOUT_CIRCUIT_FAILURE_THRESHOLD", 5)
	cfg.CircuitOpenDuration = getDuration("PAYOUT_CIRCUIT_OPEN_DURATION", 60*time.Second)

	return cfg
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
