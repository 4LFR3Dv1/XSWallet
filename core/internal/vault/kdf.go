// Package vault - Key Derivation Function using Argon2id
// Parameters aligned with technical specification
package vault

import (
	"crypto/rand"

	"golang.org/x/crypto/argon2"
)

// KDF parameters - aligned with technical spec
const (
	// Argon2id parameters
	argon2Memory      = 64 * 1024 // 64 MB
	argon2Iterations  = 3
	argon2Parallelism = 1
	argon2KeyLength   = 32 // 256 bits for AES-256

	// Salt length
	saltLength = 16
)

// NewSalt generates a cryptographically secure random salt.
func NewSalt() ([]byte, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// DeriveKey derives a 256-bit key from a PIN using Argon2id.
// This is memory-hard and resistant to GPU/ASIC attacks.
func DeriveKey(pin string, salt []byte) [32]byte {
	key := argon2.IDKey(
		[]byte(pin),
		salt,
		argon2Iterations,
		argon2Memory,
		argon2Parallelism,
		argon2KeyLength,
	)

	var result [32]byte
	copy(result[:], key)
	return result
}
