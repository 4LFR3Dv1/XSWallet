// Package vault - BIP39 mnemonic generation and validation
// Uses go-bip39 for entropy generation and word list
package vault

import (
	"crypto/rand"
	"errors"
	"strings"

	"github.com/tyler-smith/go-bip39"
)

var (
	// ErrInvalidWordCount is returned when word count is not 12 or 24
	ErrInvalidWordCount = errors.New("word count must be 12 or 24")
	// ErrInvalidMnemonic is returned when mnemonic validation fails
	ErrInvalidMnemonic = errors.New("invalid mnemonic")
)

// GenerateMnemonic generates a new BIP39 mnemonic with the specified word count.
// wordCount must be 12 (128 bits entropy) or 24 (256 bits entropy).
func GenerateMnemonic(wordCount int) (string, error) {
	var entropyBits int
	switch wordCount {
	case 12:
		entropyBits = 128
	case 24:
		entropyBits = 256
	default:
		return "", ErrInvalidWordCount
	}

	entropy := make([]byte, entropyBits/8)
	if _, err := rand.Read(entropy); err != nil {
		return "", err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

// ValidateMnemonic checks if a mnemonic phrase is valid according to BIP39.
// Returns true if valid, false otherwise.
func ValidateMnemonic(mnemonic string) bool {
	// Normalize: lowercase and single spaces
	mnemonic = strings.ToLower(strings.TrimSpace(mnemonic))
	mnemonic = strings.Join(strings.Fields(mnemonic), " ")

	return bip39.IsMnemonicValid(mnemonic)
}

// MnemonicToSeed converts a BIP39 mnemonic to a 64-byte seed.
// passphrase is optional (can be empty string for standard wallets).
func MnemonicToSeed(mnemonic, passphrase string) ([]byte, error) {
	if !ValidateMnemonic(mnemonic) {
		return nil, ErrInvalidMnemonic
	}

	// Normalize mnemonic
	mnemonic = strings.ToLower(strings.TrimSpace(mnemonic))
	mnemonic = strings.Join(strings.Fields(mnemonic), " ")

	// BIP39: PBKDF2 with HMAC-SHA512, 2048 iterations
	seed := bip39.NewSeed(mnemonic, passphrase)

	return seed, nil
}
