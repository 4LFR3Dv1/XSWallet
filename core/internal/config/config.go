// Package config - Configuração do XS Core
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config representa a configuração do core
type Config struct {
	// Paths
	DataDir string `json:"data_dir"`
	DBPath  string `json:"db_path"`

	// Network
	Network string `json:"network"` // mainnet, testnet, regtest

	// Nodes
	Bitcoind  NodeConfig `json:"bitcoind"`
	Elementsd NodeConfig `json:"elementsd"`
	LND       NodeConfig `json:"lnd"`

	// Boltz
	Boltz BoltzConfig `json:"boltz"`

	// Security
	Security SecurityConfig `json:"security"`
}

// NodeConfig configuração de um node
type NodeConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DataDir  string `json:"data_dir"`
}

// BoltzConfig configuração do Boltz
type BoltzConfig struct {
	APIURL string `json:"api_url"`
	WSURL  string `json:"ws_url"`
}

// SecurityConfig configuração de segurança
type SecurityConfig struct {
	// Argon2id parameters
	Argon2Memory      uint32 `json:"argon2_memory"`      // 64MB
	Argon2Iterations  uint32 `json:"argon2_iterations"`  // 3
	Argon2Parallelism uint8  `json:"argon2_parallelism"` // 1

	// Rate limiting
	MaxPINAttempts   int `json:"max_pin_attempts"`   // 10
	LockoutMinutes   int `json:"lockout_minutes"`    // 30
	BackoffMultipler int `json:"backoff_multiplier"` // 2
}

// Load carrega ou cria configuração
func Load(configPath, dataDir, network string) (*Config, error) {
	// Defaults
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".xs-wallet")
	}

	cfg := &Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "xs-wallet.db"),
		Network: network,
		Bitcoind: NodeConfig{
			Enabled:  true,
			Host:     "127.0.0.1",
			Port:     bitcoindPort(network),
			User:     "rpcuser",
			Password: "rpcpass",
			DataDir:  filepath.Join(dataDir, "nodes", "btc", network),
		},
		Elementsd: NodeConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    7041,
			DataDir: filepath.Join(dataDir, "nodes", "liquid", network),
		},
		LND: NodeConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    10009,
			DataDir: filepath.Join(dataDir, "nodes", "lnd", network),
		},
		Boltz: BoltzConfig{
			APIURL: boltzAPIURL(network),
			WSURL:  boltzWSURL(network),
		},
		Security: SecurityConfig{
			Argon2Memory:      64 * 1024, // 64MB
			Argon2Iterations:  3,
			Argon2Parallelism: 1,
			MaxPINAttempts:    10,
			LockoutMinutes:    30,
			BackoffMultipler:  2,
		},
	}

	// Ensure directories exist
	dirs := []string{
		cfg.DataDir,
		cfg.Bitcoind.DataDir,
		cfg.Elementsd.DataDir,
		cfg.LND.DataDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}

	// Load from file if exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	return cfg, nil
}

func boltzAPIURL(network string) string {
	switch network {
	case "mainnet":
		return "https://api.boltz.exchange"
	case "testnet":
		return "https://api.testnet.boltz.exchange"
	default:
		return "http://127.0.0.1:9001"
	}
}

func boltzWSURL(network string) string {
	switch network {
	case "mainnet":
		return "wss://api.boltz.exchange/v2/ws"
	case "testnet":
		return "wss://api.testnet.boltz.exchange/v2/ws"
	default:
		return "ws://127.0.0.1:9001/v2/ws"
	}
}

func bitcoindPort(network string) int {
	switch network {
	case "mainnet":
		return 8332
	case "testnet":
		return 18332
	default: // regtest
		return 18443
	}
}
