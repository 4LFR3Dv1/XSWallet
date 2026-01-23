// Package vault - Tests for vault encryption utilities
package vault

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/xs-wallet/xscore/internal/db"
)

func openVaultTestDB(t *testing.T) *db.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return database
}

func TestPreimageEncryptDecrypt(t *testing.T) {
	database := openVaultTestDB(t)
	v := NewVault(database)

	if _, err := v.Initialize("", "", "123456"); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := v.Unlock("123456"); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	plaintext := []byte("test-preimage-32-bytes-123456")
	encrypted, err := v.EncryptPreimage(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(encrypted, plaintext) {
		t.Fatalf("expected encrypted data to differ from plaintext")
	}

	decrypted, err := v.DecryptPreimage(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypt mismatch")
	}
}

func TestVaultStatus_LockedOut(t *testing.T) {
	database := openVaultTestDB(t)
	v := NewVault(database)

	if _, err := v.Initialize("", "", "123456"); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	for i := 0; i < MaxFailedAttempts; i++ {
		if err := v.lockout.RecordFailedAttempt(); err != nil {
			t.Fatalf("record failed attempt: %v", err)
		}
	}

	if v.Status() != StatusLockedOut {
		t.Fatalf("expected StatusLockedOut")
	}
}
