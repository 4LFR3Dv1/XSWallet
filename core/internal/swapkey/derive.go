package swapkey

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/xs-wallet/xscore/internal/vault"
)

// Derive returns the swap private/public key pair for a given swap index.
func Derive(seed []byte, index uint32, network string) (*btcec.PrivateKey, *btcec.PublicKey, error) {
	key, err := vault.GetSwapKeyPair(seed, index, toVaultNetwork(network))
	if err != nil {
		return nil, nil, err
	}
	priv, _ := btcec.PrivKeyFromBytes(key.PrivateKey)
	pub, err := btcec.ParsePubKey(key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("parse derived swap pubkey: %w", err)
	}
	return priv, pub, nil
}

func toVaultNetwork(network string) vault.Network {
	switch network {
	case "mainnet":
		return vault.NetworkMainnet
	case "testnet":
		return vault.NetworkTestnet
	default:
		return vault.NetworkRegtest
	}
}
