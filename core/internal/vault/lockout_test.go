// Package vault - Tests for lockout behavior
package vault

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xs-wallet/xscore/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
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

func TestLockout_Temporary(t *testing.T) {
	database := openTestDB(t)
	lockout := NewLockout(database)

	for i := 0; i < MaxFailedAttempts; i++ {
		if err := lockout.RecordFailedAttempt(); err != nil {
			t.Fatalf("record failed attempt %d: %v", i+1, err)
		}
	}

	if err := lockout.CheckLockout(); err != ErrTooManyAttempts {
		t.Fatalf("expected ErrTooManyAttempts, got %v", err)
	}

	failed, lockedUntil, err := lockout.GetStatus()
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if failed != MaxFailedAttempts {
		t.Fatalf("expected failed_attempts=%d, got %d", MaxFailedAttempts, failed)
	}
	if lockedUntil == nil || time.Now().After(*lockedUntil) {
		t.Fatalf("expected locked_until in the future")
	}
}

func TestLockout_Permanent(t *testing.T) {
	database := openTestDB(t)
	lockout := NewLockout(database)

	for i := 0; i < PermanentLockout; i++ {
		if err := lockout.RecordFailedAttempt(); err != nil {
			t.Fatalf("record failed attempt %d: %v", i+1, err)
		}
	}

	if err := lockout.CheckLockout(); err != ErrPermanentLockout {
		t.Fatalf("expected ErrPermanentLockout, got %v", err)
	}
}
