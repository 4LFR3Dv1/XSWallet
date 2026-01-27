package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not_found")
	ErrDuplicate    = errors.New("duplicate")
	ErrInvalidInput = errors.New("invalid_input")
)

const (
	StatusPending     = "PENDING"
	StatusExecuting   = "EXECUTING"
	StatusConfirming  = "CONFIRMING"
	StatusCompleted   = "COMPLETED"
	StatusFailedRetry = "FAILED_RETRYABLE"
	StatusFailedFinal = "FAILED_FINAL"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

type Payout struct {
	ID            int64
	PaymentID     string
	WithdrawalID  sql.NullInt64
	Network       string
	Asset         string
	Amount        int64
	Destination   string
	Priority      string
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	LastError     sql.NullString
	TxID          sql.NullString
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreatePayoutInput struct {
	PaymentID    string
	WithdrawalID sql.NullInt64
	Network      string
	Asset        string
	Amount       int64
	Destination  string
	Priority     string
}

func (s *Store) CreatePayout(ctx context.Context, in CreatePayoutInput) (*Payout, error) {
	if in.PaymentID == "" || in.Network == "" || in.Asset == "" || in.Amount <= 0 || in.Destination == "" {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existing Payout
	row := tx.QueryRowContext(ctx, `
		SELECT id, payment_id, withdrawal_id, network, asset, amount, destination, priority,
		       status, attempts, next_attempt_at, last_error, txid, created_at, updated_at
		FROM payouts WHERE payment_id = $1 AND network = $2 FOR UPDATE
	`, in.PaymentID, in.Network)
	switch err := row.Scan(&existing.ID, &existing.PaymentID, &existing.WithdrawalID, &existing.Network, &existing.Asset, &existing.Amount,
		&existing.Destination, &existing.Priority, &existing.Status, &existing.Attempts, &existing.NextAttemptAt, &existing.LastError,
		&existing.TxID, &existing.CreatedAt, &existing.UpdatedAt); {
	case err == nil:
		if existing.Status == StatusCompleted || existing.Status == StatusFailedFinal {
			return nil, ErrDuplicate
		}
		return &existing, nil
	case errors.Is(err, sql.ErrNoRows):
		// continue
	default:
		return nil, err
	}

	if in.Priority == "" {
		in.Priority = "normal"
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO payouts (
			payment_id, withdrawal_id, network, asset, amount, destination, priority, status,
			user_id, address, amount_cents
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, in.PaymentID, nullIntOrNil(in.WithdrawalID), in.Network, in.Asset, in.Amount, in.Destination, in.Priority, StatusPending,
		in.PaymentID, in.Destination, in.Amount).Scan(&id)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payout_events (payout_id, status_to) VALUES ($1, $2)
	`, id, StatusPending); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetPayout(ctx, id)
}

func (s *Store) GetPayout(ctx context.Context, id int64) (*Payout, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, payment_id, withdrawal_id, network, asset, amount, destination, priority,
		       status, attempts, next_attempt_at, last_error, txid, created_at, updated_at
		FROM payouts WHERE id = $1
	`, id)
	var p Payout
	if err := row.Scan(&p.ID, &p.PaymentID, &p.WithdrawalID, &p.Network, &p.Asset, &p.Amount, &p.Destination, &p.Priority,
		&p.Status, &p.Attempts, &p.NextAttemptAt, &p.LastError, &p.TxID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) ClaimNextPayout(ctx context.Context, now time.Time) (*Payout, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, payment_id, withdrawal_id, network, asset, amount, destination, priority,
		       status, attempts, next_attempt_at, last_error, txid, created_at, updated_at
		FROM payouts
		WHERE status IN ($1, $2) AND next_attempt_at <= $3
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, StatusPending, StatusFailedRetry, now)

	var p Payout
	if err := row.Scan(&p.ID, &p.PaymentID, &p.WithdrawalID, &p.Network, &p.Asset, &p.Amount, &p.Destination, &p.Priority,
		&p.Status, &p.Attempts, &p.NextAttemptAt, &p.LastError, &p.TxID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	nextStatus := StatusExecuting
	if _, err := tx.ExecContext(ctx, `
		UPDATE payouts
		SET status = $1, attempts = attempts + 1, updated_at = now()
		WHERE id = $2
	`, nextStatus, p.ID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payout_events (payout_id, status_from, status_to, error)
		VALUES ($1, $2, $3, $4)
	`, p.ID, p.Status, nextStatus, nullStringOrNil(p.LastError)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	p.Status = nextStatus
	p.Attempts++
	p.UpdatedAt = time.Now().UTC()
	return &p, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id int64, status, txid, errMsg string, nextAttemptAt *time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `SELECT status, last_error FROM payouts WHERE id = $1 FOR UPDATE`, id)
	var prevStatus string
	var prevErr sql.NullString
	if err := row.Scan(&prevStatus, &prevErr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payouts
		SET status = $1,
		    txid = COALESCE(NULLIF($2, ''), txid),
		    last_error = NULLIF($3, ''),
		    next_attempt_at = COALESCE($4, next_attempt_at),
		    updated_at = now()
		WHERE id = $5
	`, status, txid, errMsg, nextAttemptAt, id)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payout_events (payout_id, status_from, status_to, error)
		VALUES ($1, $2, $3, $4)
	`, id, prevStatus, status, errMsg); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) EnqueueDLQ(ctx context.Context, id int64, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dlq (payout_id, reason) VALUES ($1, $2)
		ON CONFLICT (payout_id) DO NOTHING
	`, id, reason)
	return err
}

func (s *Store) ListConfirming(ctx context.Context, limit int) ([]Payout, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, payment_id, withdrawal_id, network, asset, amount, destination, priority,
		       status, attempts, next_attempt_at, last_error, txid, created_at, updated_at
		FROM payouts
		WHERE status = $1
		ORDER BY updated_at ASC
		LIMIT $2
	`, StatusConfirming, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Payout
	for rows.Next() {
		var p Payout
		if err := rows.Scan(&p.ID, &p.PaymentID, &p.WithdrawalID, &p.Network, &p.Asset, &p.Amount, &p.Destination,
			&p.Priority, &p.Status, &p.Attempts, &p.NextAttemptAt, &p.LastError, &p.TxID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

type ReservedUTXO struct {
	TxID    string
	Vout    uint32
	Amount  int64
	Network string
	Asset   string
}

func (s *Store) GetReservedUTXOs(ctx context.Context, withdrawalID int64) ([]ReservedUTXO, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.txid, u.vout, u.amount, u.network, u.asset
		FROM utxo_reservations r
		JOIN chain_utxos u ON u.network = r.network AND u.asset = r.asset AND u.txid = r.txid AND u.vout = r.vout
		WHERE r.withdrawal_id = $1 AND r.status = 'RESERVED'
		ORDER BY u.id
	`, withdrawalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReservedUTXO
	for rows.Next() {
		var u ReservedUTXO
		if err := rows.Scan(&u.TxID, &u.Vout, &u.Amount, &u.Network, &u.Asset); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

type CircuitState struct {
	Network   string
	State     string
	Failures  int
	OpenedAt  sql.NullTime
	UpdatedAt time.Time
}

func (s *Store) GetCircuit(ctx context.Context, network string) (*CircuitState, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT network, state, failures, opened_at, updated_at
		FROM circuit_breakers WHERE network = $1
	`, network)
	var c CircuitState
	if err := row.Scan(&c.Network, &c.State, &c.Failures, &c.OpenedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) RecordCircuitFailure(ctx context.Context, network string, threshold int) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO circuit_breakers (network, state, failures)
		VALUES ($1, 'CLOSED', 0)
		ON CONFLICT (network) DO NOTHING
	`, network)
	if err != nil {
		return false, err
	}

	row := tx.QueryRowContext(ctx, `SELECT failures, state FROM circuit_breakers WHERE network = $1 FOR UPDATE`, network)
	var failures int
	var state string
	if err := row.Scan(&failures, &state); err != nil {
		return false, err
	}
	failures++
	open := failures >= threshold
	nextState := state
	var openedAt sql.NullTime
	if open {
		nextState = "OPEN"
		openedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE circuit_breakers
		SET failures = $1, state = $2, opened_at = $3, updated_at = now()
		WHERE network = $4
	`, failures, nextState, openedAt, network)
	if err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return open, nil
}

func (s *Store) RecordCircuitSuccess(ctx context.Context, network string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO circuit_breakers (network, state, failures)
		VALUES ($1, 'CLOSED', 0)
		ON CONFLICT (network)
		DO UPDATE SET state = 'CLOSED', failures = 0, opened_at = NULL, updated_at = now()
	`, network)
	return err
}

func (s *Store) IsCircuitOpen(ctx context.Context, network string, openDuration time.Duration) (bool, error) {
	c, err := s.GetCircuit(ctx, network)
	if err != nil || c == nil {
		return false, err
	}
	if c.State != "OPEN" || !c.OpenedAt.Valid {
		return false, nil
	}
	if time.Since(c.OpenedAt.Time) > openDuration {
		_, err := s.db.ExecContext(ctx, `
			UPDATE circuit_breakers
			SET state = 'CLOSED', failures = 0, opened_at = NULL, updated_at = now()
			WHERE network = $1
		`, network)
		return false, err
	}
	return true, nil
}

func nullIntOrNil(v sql.NullInt64) interface{} {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullStringOrNil(v sql.NullString) interface{} {
	if v.Valid {
		return v.String
	}
	return nil
}
