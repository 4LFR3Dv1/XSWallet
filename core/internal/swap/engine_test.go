// Package swap - Tests for engine preimage handling
package swap

import (
	"bytes"
	"context"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/xs-wallet/xscore/internal/db"
)

type testVault struct{}

func (testVault) EncryptPreimage(b []byte) ([]byte, error) {
	return append([]byte("enc:"), b...), nil
}

func (testVault) DecryptPreimage(b []byte) ([]byte, error) {
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}

func openSwapTestDB(t *testing.T) *db.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	// Minimal table for preimage migration tests
	_, err = database.Exec(`
		CREATE TABLE swaps (
			id TEXT PRIMARY KEY,
			encrypted_preimage BLOB,
			preimage_hex TEXT
		)
	`)
	if err != nil {
		t.Fatalf("create swaps table: %v", err)
	}

	return database
}

func TestCreate_RequiresVault(t *testing.T) {
	database := openSwapTestDB(t)
	engine := NewEngine(database, nil)

	if _, err := engine.Create(context.Background(), KindSubmarine, "regtest", 0); err == nil {
		t.Fatalf("expected error when vault is nil")
	}
}

func TestGetPreimage_Encrypted(t *testing.T) {
	database := openSwapTestDB(t)
	engine := NewEngine(database, testVault{})

	plain := []byte("preimage-plaintext")
	encrypted, _ := testVault{}.EncryptPreimage(plain)

	_, err := database.Exec(`
		INSERT INTO swaps (id, encrypted_preimage, preimage_hex)
		VALUES (?, ?, NULL)
	`, "swap-1", encrypted)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := engine.GetPreimage(context.Background(), "swap-1")
	if err != nil {
		t.Fatalf("get preimage: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("expected plaintext back")
	}
}

func TestGetPreimage_LegacyMigration(t *testing.T) {
	database := openSwapTestDB(t)
	engine := NewEngine(database, testVault{})

	plain := []byte("legacy-preimage")
	legacyHex := hex.EncodeToString(plain)

	_, err := database.Exec(`
		INSERT INTO swaps (id, encrypted_preimage, preimage_hex)
		VALUES (?, NULL, ?)
	`, "swap-legacy", legacyHex)
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}

	got, err := engine.GetPreimage(context.Background(), "swap-legacy")
	if err != nil {
		t.Fatalf("get preimage: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("expected legacy plaintext back")
	}

	var encrypted []byte
	var preimageHex string
	err = database.QueryRow(`
		SELECT COALESCE(encrypted_preimage, X''), COALESCE(preimage_hex, '')
		FROM swaps WHERE id = ?
	`, "swap-legacy").Scan(&encrypted, &preimageHex)
	if err != nil {
		t.Fatalf("query after migration: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatalf("expected encrypted_preimage to be set")
	}
	if preimageHex != "" {
		t.Fatalf("expected preimage_hex to be cleared")
	}
}
