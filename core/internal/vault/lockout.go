// Package vault - PIN lockout and attempt tracking
package vault

import (
	"database/sql"
	"errors"
	"time"

	"github.com/xs-wallet/xscore/internal/db"
)

const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
	PermanentLockout  = 10 // After 10 failed attempts, permanent lockout
)

var (
	ErrTooManyAttempts  = errors.New("too many failed attempts, vault locked")
	ErrPermanentLockout = errors.New("permanent lockout, recovery required")
)

// Lockout manages PIN attempt tracking
type Lockout struct {
	db *db.DB
}

// NewLockout creates a new Lockout instance
func NewLockout(database *db.DB) *Lockout {
	return &Lockout{db: database}
}

// CheckLockout verifies if vault is currently locked out
func (l *Lockout) CheckLockout() error {
	var failedAttempts int
	var lockedUntil sql.NullString

	err := l.db.QueryRow(`
		SELECT failed_attempts, locked_until 
		FROM vault_lockout WHERE id = 1
	`).Scan(&failedAttempts, &lockedUntil)

	if err == sql.ErrNoRows {
		// Initialize lockout record
		_, err = l.db.Exec(`
			INSERT INTO vault_lockout (id, failed_attempts) VALUES (1, 0)
		`)
		return err
	}

	if err != nil {
		return err
	}

	// Check for permanent lockout
	if failedAttempts >= PermanentLockout {
		return ErrPermanentLockout
	}

	// Check temporary lockout
	if lockedUntil.Valid {
		lockTime, err := time.Parse(time.RFC3339, lockedUntil.String)
		if err == nil && time.Now().Before(lockTime) {
			return ErrTooManyAttempts
		}
	}

	return nil
}

// RecordFailedAttempt increments failed attempt counter
func (l *Lockout) RecordFailedAttempt() error {
	var failedAttempts int
	err := l.db.QueryRow(`
		SELECT failed_attempts FROM vault_lockout WHERE id = 1
	`).Scan(&failedAttempts)

	if err != nil {
		return err
	}

	failedAttempts++

	// Calculate lockout duration (exponential backoff)
	var lockedUntil *string
	if failedAttempts >= MaxFailedAttempts && failedAttempts < PermanentLockout {
		backoffMultiplier := failedAttempts - MaxFailedAttempts + 1
		lockDuration := LockoutDuration * time.Duration(backoffMultiplier)
		lockTime := time.Now().Add(lockDuration).Format(time.RFC3339)
		lockedUntil = &lockTime
	}

	_, err = l.db.Exec(`
		UPDATE vault_lockout 
		SET failed_attempts = ?, 
		    locked_until = ?,
		    last_attempt_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = 1
	`, failedAttempts, lockedUntil)

	return err
}

// ResetAttempts clears failed attempt counter (after successful unlock)
func (l *Lockout) ResetAttempts() error {
	_, err := l.db.Exec(`
		UPDATE vault_lockout 
		SET failed_attempts = 0, 
		    locked_until = NULL,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = 1
	`)
	return err
}

// GetStatus returns current lockout status
func (l *Lockout) GetStatus() (failedAttempts int, lockedUntil *time.Time, err error) {
	var lockedUntilStr sql.NullString

	err = l.db.QueryRow(`
		SELECT failed_attempts, locked_until 
		FROM vault_lockout WHERE id = 1
	`).Scan(&failedAttempts, &lockedUntilStr)

	if err == sql.ErrNoRows {
		return 0, nil, nil
	}

	if err != nil {
		return 0, nil, err
	}

	if lockedUntilStr.Valid {
		t, err := time.Parse(time.RFC3339, lockedUntilStr.String)
		if err == nil {
			lockedUntil = &t
		}
	}

	return failedAttempts, lockedUntil, nil
}
