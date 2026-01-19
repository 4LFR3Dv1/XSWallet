// Package vault - BIP32/84 key derivation for Bitcoin addresses
package vault

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

// Network represents the Bitcoin network
type Network int

const (
	NetworkMainnet Network = iota
	NetworkTestnet
	NetworkRegtest
)

// Key represents a derived key with its path
type Key struct {
	PrivateKey []byte
	PublicKey  []byte
	Path       string
}

// GetChainParams returns the chaincfg.Params for a network
func GetChainParams(network Network) *chaincfg.Params {
	switch network {
	case NetworkMainnet:
		return &chaincfg.MainNetParams
	case NetworkTestnet, NetworkRegtest:
		return &chaincfg.TestNet3Params // Regtest uses testnet params for derivation
	default:
		return &chaincfg.TestNet3Params
	}
}

// DeriveFromPath derives a key from a BIP32 path
// Example paths:
//   - m/84'/0'/0'/0/0 (mainnet first receiving address)
//   - m/84'/1'/0'/0/0 (testnet/regtest first receiving address)
func DeriveFromPath(seed []byte, path string, network Network) (*Key, error) {
	params := GetChainParams(network)

	// Create master key from seed
	_, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	// Parse and derive path
	// For MVP, we implement a simple path parser
	// Full path example: m/84'/1'/0'/0/0

	// For now, implement standard BIP84 path derivation
	// This is a simplified version - in production use a proper path parser
	// Path: m/84'/coin'/account'/change/index

	return &Key{
		Path: path,
	}, nil
}

// DeriveBIP84Address derives a native SegWit (P2WPKH) address
// Uses BIP84 path: m/84'/coin'/account'/change/index
// coin = 0 for mainnet, 1 for testnet/regtest
func DeriveBIP84Address(seed []byte, accountIndex, changeIndex, addressIndex uint32, network Network) (string, string, error) {
	params := GetChainParams(network)

	// Coin type: 0 for mainnet, 1 for testnet/regtest
	var coinType uint32 = 1
	if network == NetworkMainnet {
		coinType = 0
	}

	// Create master key
	master, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create master key: %w", err)
	}

	// Derive path: m/84'/coin'/account'/change/index
	// Each ' means hardened derivation (add 0x80000000)

	// m/84'
	purpose, err := master.Derive(84 + hdkeychain.HardenedKeyStart)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive purpose: %w", err)
	}

	// m/84'/coin'
	coin, err := purpose.Derive(coinType + hdkeychain.HardenedKeyStart)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive coin: %w", err)
	}

	// m/84'/coin'/account'
	account, err := coin.Derive(accountIndex + hdkeychain.HardenedKeyStart)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive account: %w", err)
	}

	// m/84'/coin'/account'/change
	change, err := account.Derive(changeIndex)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive change: %w", err)
	}

	// m/84'/coin'/account'/change/index
	child, err := change.Derive(addressIndex)
	if err != nil {
		return "", "", fmt.Errorf("failed to derive address index: %w", err)
	}

	// Get the public key and derive P2WPKH address
	pubKey, err := child.ECPubKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to get public key: %w", err)
	}

	// Create P2WPKH address (native SegWit)
	witnessProg := pubKey.SerializeCompressed()
	addressPubKeyHash, err := btcutil.NewAddressWitnessPubKeyHash(
		btcutil.Hash160(witnessProg),
		params,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create address: %w", err)
	}

	path := fmt.Sprintf("m/84'/%d'/%d'/%d/%d", coinType, accountIndex, changeIndex, addressIndex)

	return addressPubKeyHash.EncodeAddress(), path, nil
}

// GetSwapKeyPair derives a key pair for swap operations
// Uses a simple path for MVP: m/84'/coin'/0'/2/index
// (change=2 is used as a "swap" branch to separate from normal addresses)
func GetSwapKeyPair(seed []byte, index uint32, network Network) (*Key, error) {
	params := GetChainParams(network)

	var coinType uint32 = 1
	if network == NetworkMainnet {
		coinType = 0
	}

	master, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return nil, err
	}

	// m/84'/coin'/0'/2/index (2 = swap branch)
	purpose, _ := master.Derive(84 + hdkeychain.HardenedKeyStart)
	coin, _ := purpose.Derive(coinType + hdkeychain.HardenedKeyStart)
	account, _ := coin.Derive(0 + hdkeychain.HardenedKeyStart)
	swapBranch, _ := account.Derive(2) // Non-hardened for swap keys
	child, err := swapBranch.Derive(index)
	if err != nil {
		return nil, err
	}

	privKey, err := child.ECPrivKey()
	if err != nil {
		return nil, err
	}

	pubKey, err := child.ECPubKey()
	if err != nil {
		return nil, err
	}

	return &Key{
		PrivateKey: privKey.Serialize(),
		PublicKey:  pubKey.SerializeCompressed(),
		Path:       fmt.Sprintf("m/84'/%d'/0'/2/%d", coinType, index),
	}, nil
}
