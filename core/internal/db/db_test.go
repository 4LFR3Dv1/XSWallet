// Package db - Tests for schema and migrations
package db

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
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

func columnExists(t *testing.T, database *DB, table, column string) bool {
	t.Helper()

	var count int
	err := database.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?
	`, table, column).Scan(&count)
	if err != nil {
		t.Fatalf("columnExists: %v", err)
	}
	return count > 0
}

func tableExists(t *testing.T, database *DB, table string) bool {
	t.Helper()

	var count int
	err := database.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?
	`, table).Scan(&count)
	if err != nil {
		t.Fatalf("tableExists: %v", err)
	}
	return count > 0
}

func TestSchema_Migrations(t *testing.T) {
	database := openTestDB(t)

	// New columns in swaps
	if !columnExists(t, database, "swaps", "encrypted_preimage") {
		t.Fatalf("missing column swaps.encrypted_preimage")
	}
	if !columnExists(t, database, "swaps", "boltz_status") {
		t.Fatalf("missing column swaps.boltz_status")
	}
	if !columnExists(t, database, "swaps", "boltz_raw") {
		t.Fatalf("missing column swaps.boltz_raw")
	}
	if !columnExists(t, database, "swaps", "from_asset") {
		t.Fatalf("missing column swaps.from_asset")
	}
	if !columnExists(t, database, "swaps", "to_asset") {
		t.Fatalf("missing column swaps.to_asset")
	}

	// Lockout table exists
	if !tableExists(t, database, "vault_lockout") {
		t.Fatalf("missing table vault_lockout")
	}

	// Lockout row initialized
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM vault_lockout`).Scan(&count); err != nil {
		t.Fatalf("count vault_lockout: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected vault_lockout row = 1, got %d", count)
	}
}
