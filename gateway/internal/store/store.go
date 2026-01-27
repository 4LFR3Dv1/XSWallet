package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending = "PENDING"
	StatusPaid    = "PAID"

	DepositPending    = "PENDING_CONFIRMATION"
	DepositConfirmed  = "CONFIRMED"

	PayoutExecuting   = "EXECUTING"
	PayoutConfirming  = "CONFIRMING"
	PayoutCompleted   = "COMPLETED"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrPaymentNotPaid    = errors.New("payment not paid")
	ErrDestinationNotFound = errors.New("destination not registered")
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

type PaymentIntent struct {
	PaymentID   string
	UserID      string
	AmountCents int64
	Status      string
	ProviderRef sql.NullString
}

type Balance struct {
	UserID         string
	AvailableCents int64
	LockedCents    int64
}

type Deposit struct {
	PaymentID   string
	AmountCents int64
	Status      string
	Asset       string
	ChainTxid   sql.NullString
}

type Payout struct {
	ID          int64
	UserID      string
	Network     string
	Address     string
	AmountCents int64
	Status      string
	TxID        sql.NullString
}

func (s *Store) CreatePaymentIntent(ctx context.Context, userID string, amountCents int64) (*PaymentIntent, error) {
	if amountCents <= 0 {
		return nil, ErrInvalidAmount
	}
	paymentID := uuid.NewString()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO payment_intents (payment_id, user_id, amount_cents, status)
		VALUES ($1, $2, $3, $4)
	`, paymentID, userID, amountCents, StatusPending)
	if err != nil {
		return nil, err
	}
	return &PaymentIntent{PaymentID: paymentID, UserID: userID, AmountCents: amountCents, Status: StatusPending}, nil
}

func (s *Store) GetPaymentIntent(ctx context.Context, paymentID string) (*PaymentIntent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT payment_id, user_id, amount_cents, status, provider_ref
		FROM payment_intents WHERE payment_id = $1
	`, paymentID)
	var pi PaymentIntent
	if err := row.Scan(&pi.PaymentID, &pi.UserID, &pi.AmountCents, &pi.Status, &pi.ProviderRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &pi, nil
}

func (s *Store) MarkPaymentPaid(ctx context.Context, paymentID, providerRef string, amountCents int64) (*PaymentIntent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT payment_id, user_id, amount_cents, status, provider_ref
		FROM payment_intents WHERE payment_id = $1 FOR UPDATE
	`, paymentID)
	var pi PaymentIntent
	if err := row.Scan(&pi.PaymentID, &pi.UserID, &pi.AmountCents, &pi.Status, &pi.ProviderRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if pi.AmountCents != amountCents {
		return nil, fmt.Errorf("amount mismatch")
	}
	if pi.Status == StatusPaid {
		return &pi, tx.Commit()
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payment_intents SET status = $1, provider_ref = $2 WHERE payment_id = $3
	`, StatusPaid, providerRef, paymentID)
	if err != nil {
		return nil, err
	}

	pi.Status = StatusPaid
	pi.ProviderRef = sql.NullString{String: providerRef, Valid: providerRef != ""}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &pi, nil
}

func (s *Store) EnsureDepositPending(ctx context.Context, paymentID, asset string, amountCents int64) (*Deposit, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT status FROM payment_intents WHERE payment_id = $1 FOR UPDATE
	`, paymentID)
	var status string
	if err := row.Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != StatusPaid {
		return nil, ErrPaymentNotPaid
	}

	// Insert if not exists
	_, err = tx.ExecContext(ctx, `
		INSERT INTO deposits (payment_id, asset, amount_cents, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (payment_id) DO NOTHING
	`, paymentID, asset, amountCents, DepositPending)
	if err != nil {
		return nil, err
	}

	row = tx.QueryRowContext(ctx, `
		SELECT payment_id, amount_cents, status, asset, chain_txid
		FROM deposits WHERE payment_id = $1
	`, paymentID)
	var dep Deposit
	if err := row.Scan(&dep.PaymentID, &dep.AmountCents, &dep.Status, &dep.Asset, &dep.ChainTxid); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &dep, nil
}

func (s *Store) ConfirmDeposit(ctx context.Context, paymentID, chainTxID string) (*Deposit, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT user_id, amount_cents, status FROM payment_intents WHERE payment_id = $1 FOR UPDATE
	`, paymentID)
	var payStatus string
	var userID string
	var intentAmount int64
	if err := row.Scan(&userID, &intentAmount, &payStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if payStatus != StatusPaid {
		return nil, ErrPaymentNotPaid
	}

	row = tx.QueryRowContext(ctx, `
		SELECT payment_id, amount_cents, status, asset, chain_txid
		FROM deposits WHERE payment_id = $1 FOR UPDATE
	`, paymentID)
	var dep Deposit
	if err := row.Scan(&dep.PaymentID, &dep.AmountCents, &dep.Status, &dep.Asset, &dep.ChainTxid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO deposits (payment_id, asset, amount_cents, status)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (payment_id) DO NOTHING
			`, paymentID, "unknown", intentAmount, DepositPending)
			if err != nil {
				return nil, err
			}

			row = tx.QueryRowContext(ctx, `
				SELECT payment_id, amount_cents, status, asset, chain_txid
				FROM deposits WHERE payment_id = $1 FOR UPDATE
			`, paymentID)
			if err := row.Scan(&dep.PaymentID, &dep.AmountCents, &dep.Status, &dep.Asset, &dep.ChainTxid); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if dep.Status == DepositConfirmed {
		return &dep, tx.Commit()
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE deposits SET status = $1, chain_txid = $2 WHERE payment_id = $3
	`, DepositConfirmed, chainTxID, paymentID)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO balances (user_id, available_cents, locked_cents, updated_at)
		VALUES ($1, $2, 0, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET available_cents = balances.available_cents + EXCLUDED.available_cents,
		    updated_at = EXCLUDED.updated_at
	`, userID, dep.AmountCents, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	dep.Status = DepositConfirmed
	dep.ChainTxid = sql.NullString{String: chainTxID, Valid: chainTxID != ""}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &dep, nil
}

func (s *Store) RegisterDestination(ctx context.Context, userID, network, address string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO destinations (user_id, network, address)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, userID, network, address)
	return err
}

func (s *Store) DestinationExists(ctx context.Context, userID, network, address string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM destinations WHERE user_id = $1 AND network = $2 AND address = $3
	`, userID, network, address)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) GetBalance(ctx context.Context, userID string) (*Balance, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, available_cents, locked_cents FROM balances WHERE user_id = $1
	`, userID)
	var b Balance
	if err := row.Scan(&b.UserID, &b.AvailableCents, &b.LockedCents); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &Balance{UserID: userID, AvailableCents: 0, LockedCents: 0}, nil
		}
		return nil, err
	}
	return &b, nil
}

func (s *Store) CreatePayout(ctx context.Context, userID, network, address string, amountCents int64, executor Executor) (*Payout, error) {
	if amountCents <= 0 {
		return nil, ErrInvalidAmount
	}

	ok, err := s.DestinationExists(ctx, userID, network, address)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrDestinationNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT user_id, available_cents, locked_cents FROM balances WHERE user_id = $1 FOR UPDATE
	`, userID)
	var b Balance
	if err := row.Scan(&b.UserID, &b.AvailableCents, &b.LockedCents); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInsufficientFunds
		}
		return nil, err
	}

	if b.AvailableCents < amountCents {
		return nil, ErrInsufficientFunds
	}

	// Lock funds
	newAvail := b.AvailableCents - amountCents
	newLocked := b.LockedCents + amountCents
	_, err = tx.ExecContext(ctx, `
		UPDATE balances SET available_cents = $1, locked_cents = $2, updated_at = $3 WHERE user_id = $4
	`, newAvail, newLocked, time.Now().UTC(), userID)
	if err != nil {
		return nil, err
	}

	// Create payout
	row = tx.QueryRowContext(ctx, `
		INSERT INTO payouts (user_id, network, address, amount_cents, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status
	`, userID, network, address, amountCents, PayoutExecuting)
	var payoutID int64
	var status string
	if err := row.Scan(&payoutID, &status); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Execute outside transaction
	txid, err := executor.Send(ctx, network, address, amountCents)
	if err != nil {
		return nil, err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE payouts SET status = $1, txid = $2 WHERE id = $3
	`, PayoutConfirming, txid, payoutID)
	if err != nil {
		return nil, err
	}

	return &Payout{ID: payoutID, UserID: userID, Network: network, Address: address, AmountCents: amountCents, Status: PayoutConfirming, TxID: sql.NullString{String: txid, Valid: txid != ""}}, nil
}

func (s *Store) ConfirmPayout(ctx context.Context, payoutID int64) (*Payout, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, user_id, network, address, amount_cents, status, txid
		FROM payouts WHERE id = $1 FOR UPDATE
	`, payoutID)
	var p Payout
	if err := row.Scan(&p.ID, &p.UserID, &p.Network, &p.Address, &p.AmountCents, &p.Status, &p.TxID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if p.Status == PayoutCompleted {
		return &p, tx.Commit()
	}

	// Move locked -> spent
	_, err = tx.ExecContext(ctx, `
		UPDATE balances SET locked_cents = locked_cents - $1, updated_at = $2 WHERE user_id = $3
	`, p.AmountCents, time.Now().UTC(), p.UserID)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payouts SET status = $1 WHERE id = $2
	`, PayoutCompleted, p.ID)
	if err != nil {
		return nil, err
	}

	p.Status = PayoutCompleted
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &p, nil
}

type Executor interface {
	Send(ctx context.Context, network, address string, amountCents int64) (string, error)
}
