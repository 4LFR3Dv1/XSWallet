// Package vault - SLIP-0077 blinding key derivation (Liquid confidential)
package vault

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
)

const (
	slip21RootKey  = "Symmetric key seed"
	slip77Label    = "SLIP-0077"
	blindingKeyLen = 32
)

// DeriveSlip77MasterKey derives the master blinding key (32 bytes) from a seed.
// This follows SLIP-0077, which uses SLIP-0021 key derivation with label "SLIP-0077".
func DeriveSlip77MasterKey(seed []byte) ([]byte, error) {
	if len(seed) == 0 {
		return nil, errors.New("seed is empty")
	}

	root := hmacSHA512([]byte(slip21RootKey), seed)
	node := slip21Derive(root, []byte(slip77Label))

	// Per SLIP-0077, use the lower 256-bits of the SLIP-0021 derived node.
	master := node[32:]
	if len(master) != blindingKeyLen {
		return nil, errors.New("invalid master blinding key length")
	}
	return master, nil
}

// DeriveBlindingKey derives the blinding private key for a given scriptPubKey.
func DeriveBlindingKey(seed []byte, scriptPubKey []byte) ([]byte, *btcec.PublicKey, error) {
	master, err := DeriveSlip77MasterKey(seed)
	if err != nil {
		return nil, nil, err
	}

	h := hmac.New(sha256.New, master)
	h.Write(scriptPubKey)
	privBytes := h.Sum(nil)

	if len(privBytes) != blindingKeyLen {
		return nil, nil, errors.New("invalid blinding key length")
	}

	// Validate key range: 1 <= k < N
	n := btcec.S256().N
	k := new(big.Int).SetBytes(privBytes)
	if k.Sign() <= 0 || k.Cmp(n) >= 0 {
		return nil, nil, errors.New("invalid blinding key")
	}

	privKey, pubKey := btcec.PrivKeyFromBytes(privBytes)
	return privKey.Serialize(), pubKey, nil
}

func hmacSHA512(key, msg []byte) []byte {
	h := hmac.New(sha512.New, key)
	h.Write(msg)
	return h.Sum(nil)
}

// slip21Derive implements SLIP-0021 child derivation.
func slip21Derive(parent []byte, label []byte) []byte {
	// parent is 64 bytes: [key||chain_code], use chain_code as HMAC key
	chainCode := parent[32:]
	data := append([]byte{0x00}, label...)
	return hmacSHA512(chainCode, data)
}
