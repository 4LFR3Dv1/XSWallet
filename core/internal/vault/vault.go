// Package vault - Secure seed storage with AES-256-GCM encryption
// Vault is the core secret manager for the wallet
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/xs-wallet/xscore/internal/db"
)

const (
	// AES-GCM parameters
	nonceLength = 12 // 96 bits as per GCM standard
	tagLength   = 16 // 128 bits

	// Session management
	sessionIDLength = 32 // 256 bits
	sessionTTL      = 30 * time.Minute
)

// Errors
var (
	ErrVaultNotInitialized = errors.New("vault not initialized")
	ErrVaultLocked         = errors.New("vault is locked")
	ErrVaultAlreadyExists  = errors.New("vault already exists")
	ErrInvalidPIN          = errors.New("invalid PIN")
	ErrInvalidSession      = errors.New("invalid or expired session")
	ErrDecryptionFailed    = errors.New("decryption failed: invalid PIN or corrupted data")
)

// VaultStatus represents the current state of the vault
type VaultStatus int

const (
	StatusNotInitialized VaultStatus = iota
	StatusLocked
	StatusUnlocked
	StatusLockedOut
)

// VaultBlob is the encrypted seed storage format (versioned for future migrations)
type VaultBlob struct {
	Version    uint32 // Format version for migrations
	Salt       []byte // Argon2id salt
	Nonce      []byte // AES-GCM nonce
	Ciphertext []byte // Encrypted seed
}

// Vault manages the encrypted seed and session state
type Vault struct {
	db *db.DB

	mu        sync.RWMutex
	unlocked  bool
	seed      []byte // Only in memory when unlocked
	sessionID string
	lastSeen  time.Time
}

// NewVault creates a new Vault instance
func NewVault(database *db.DB) *Vault {
	return &Vault{db: database}
}

// Initialize creates a new vault with a generated or imported mnemonic
// If mnemonic is empty, generates a new 24-word mnemonic
// Returns the mnemonic (MUST be shown to user once and never stored)
func (v *Vault) Initialize(mnemonic, passphrase, pin string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Check if vault already exists
	exists, err := v.vaultExists()
	if err != nil {
		return "", err
	}
	if exists {
		return "", ErrVaultAlreadyExists
	}

	// Generate mnemonic if not provided
	if mnemonic == "" {
		mnemonic, err = GenerateMnemonic(24)
		if err != nil {
			return "", err
		}
	} else {
		if !ValidateMnemonic(mnemonic) {
			return "", ErrInvalidMnemonic
		}
	}

	// Derive seed from mnemonic
	seed, err := MnemonicToSeed(mnemonic, passphrase)
	if err != nil {
		return "", err
	}

	// Generate salt
	salt, err := NewSalt()
	if err != nil {
		return "", err
	}

	// Derive encryption key from PIN
	key := DeriveKey(pin, salt)

	// Encrypt seed
	nonce, ciphertext, err := encrypt(seed, key[:])
	if err != nil {
		return "", err
	}

	// Persist to database
	blob := VaultBlob{
		Version:    1,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	if err := v.saveBlob(blob); err != nil {
		return "", err
	}

	return mnemonic, nil
}

// Unlock decrypts the vault and returns a session ID
func (v *Vault) Unlock(pin string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Load blob from database
	blob, err := v.loadBlob()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrVaultNotInitialized
		}
		return "", err
	}

	// Derive key from PIN
	key := DeriveKey(pin, blob.Salt)

	// Decrypt seed
	seed, err := decrypt(blob.Ciphertext, blob.Nonce, key[:])
	if err != nil {
		return "", ErrInvalidPIN
	}

	// Generate session ID
	sessionBytes := make([]byte, sessionIDLength)
	if _, err := rand.Read(sessionBytes); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(sessionBytes)

	// Update state
	v.unlocked = true
	v.seed = seed
	v.sessionID = sessionID
	v.lastSeen = time.Now()

	return sessionID, nil
}

// Lock clears the seed from memory and invalidates the session
func (v *Vault) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Zero out seed
	if v.seed != nil {
		for i := range v.seed {
			v.seed[i] = 0
		}
	}

	v.unlocked = false
	v.seed = nil
	v.sessionID = ""
	v.lastSeen = time.Time{}

	return nil
}

// Status returns the current vault status
func (v *Vault) Status() VaultStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()

	exists, _ := v.vaultExists()
	if !exists {
		return StatusNotInitialized
	}

	if v.unlocked {
		return StatusUnlocked
	}

	return StatusLocked
}

// RequireSession validates that the provided session ID is valid and not expired
func (v *Vault) RequireSession(sessionID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}

	if v.sessionID != sessionID {
		return ErrInvalidSession
	}

	if time.Since(v.lastSeen) > sessionTTL {
		// Session expired, lock vault
		v.unlocked = false
		v.seed = nil
		v.sessionID = ""
		return ErrInvalidSession
	}

	// Update last seen
	v.lastSeen = time.Now()
	return nil
}

// Seed returns a copy of the seed (only when unlocked)
func (v *Vault) Seed() ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked || v.seed == nil {
		return nil, ErrVaultLocked
	}

	// Return a copy to prevent external modification
	seedCopy := make([]byte, len(v.seed))
	copy(seedCopy, v.seed)
	return seedCopy, nil
}

// vaultExists checks if a vault blob exists in the database
func (v *Vault) vaultExists() (bool, error) {
	var count int
	err := v.db.QueryRow("SELECT COUNT(*) FROM vault WHERE id = 1").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// saveBlob persists the encrypted vault to the database
func (v *Vault) saveBlob(blob VaultBlob) error {
	_, err := v.db.Exec(`
		INSERT INTO vault (id, encrypted_seed, salt, iv, tag)
		VALUES (1, ?, ?, ?, ?)
	`, blob.Ciphertext, blob.Salt, blob.Nonce, []byte{}) // tag is embedded in ciphertext for GCM
	return err
}

// loadBlob retrieves the encrypted vault from the database
func (v *Vault) loadBlob() (VaultBlob, error) {
	var blob VaultBlob
	blob.Version = 1

	err := v.db.QueryRow(`
		SELECT encrypted_seed, salt, iv FROM vault WHERE id = 1
	`).Scan(&blob.Ciphertext, &blob.Salt, &blob.Nonce)

	return blob, err
}

// encrypt encrypts data using AES-256-GCM
func encrypt(plaintext, key []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

// decrypt decrypts data using AES-256-GCM
func decrypt(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}
