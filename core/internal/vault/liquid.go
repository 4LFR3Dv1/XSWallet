// Package vault - Liquid network parameters helpers
package vault

import "github.com/btcsuite/btcd/chaincfg"

const (
	// LiquidCoinTypeMainnet is SLIP-0044 coin type for Liquid mainnet.
	LiquidCoinTypeMainnet uint32 = 1776
)

// LiquidCoinType returns the coin type for Liquid.
// Mainnet uses 1776; testnet/regtest fallback to 1 (testnet).
func LiquidCoinType(network Network) uint32 {
	if network == NetworkMainnet {
		return LiquidCoinTypeMainnet
	}
	return 1
}

// LiquidParams returns chain params for Liquid address encoding.
// Bech32 HRP values are aligned with Liquid network conventions.
func LiquidParams(network Network) *chaincfg.Params {
	var base *chaincfg.Params
	switch network {
	case NetworkMainnet:
		base = &chaincfg.MainNetParams
	case NetworkTestnet, NetworkRegtest:
		base = &chaincfg.TestNet3Params
	default:
		base = &chaincfg.TestNet3Params
	}

	params := *base
	switch network {
	case NetworkMainnet:
		params.Name = "liquid-mainnet"
		params.Bech32HRPSegwit = "ex" // unconfidential Liquid bech32 prefix
		params.PrivateKeyID = 0x80
	case NetworkTestnet:
		params.Name = "liquid-testnet"
		params.Bech32HRPSegwit = "tex"
		params.PrivateKeyID = 0xEF
	case NetworkRegtest:
		params.Name = "liquid-regtest"
		params.Bech32HRPSegwit = "ert"
		params.PrivateKeyID = 0xEF
	default:
		params.Name = "liquid-testnet"
		params.Bech32HRPSegwit = "tex"
		params.PrivateKeyID = 0xEF
	}

	return &params
}
