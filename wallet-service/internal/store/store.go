package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrInsufficient   = errors.New("insufficient balance")
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

type Account struct {
	UUID         string `json:"uuid"`
	AccountIndex int64  `json:"account_index"`
	UserXpub     string `json:"user_xpub"`
	Status       string `json:"status"`
}

type Address struct {
	ID             int64  `json:"id"`
	AccountUUID    string `json:"account_uuid"`
	Network        string `json:"network"`
	Asset          string `json:"asset"`
	Address        string `json:"address"`
	DerivationPath string `json:"derivation_path"`
	IsActive       bool   `json:"is_active"`
}

type Balance struct {
	Network        string `json:"network"`
	Asset          string `json:"asset"`
	Available      int64  `json:"available"`
	Reserved       int64  `json:"reserved"`
}

type Withdrawal struct {
	ID          int64  `json:"id"`
	AccountUUID string `json:"account_uuid"`
	Network     string `json:"network"`
	Asset       string `json:"asset"`
	Amount      int64  `json:"amount"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
}

type TxRecord struct {
	Network       string    `json:"network"`
	Asset         string    `json:"asset"`
	TxID          string    `json:"txid"`
	Amount        int64     `json:"amount"`
	Confirmations int       `json:"confirmations"`
	ConfirmedAt   time.Time `json:"confirmed_at"`
}

func (s *Store) CreateAccount(ctx context.Context, uuidStr, userXpub string) (*Account, error) {
	if uuidStr == "" {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(uuidStr); err != nil {
		return nil, ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO accounts (uuid, user_xpub, status)
		VALUES ($1, $2, 'ACTIVE')
		ON CONFLICT (uuid) DO NOTHING
	`, uuidStr, userXpub)
	if err != nil {
		return nil, err
	}

	return s.GetAccount(ctx, uuidStr)
}

func (s *Store) GetAccount(ctx context.Context, uuidStr string) (*Account, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT uuid, account_index, COALESCE(user_xpub,''), status
		FROM accounts WHERE uuid = $1
	`, uuidStr)
	var acc Account
	if err := row.Scan(&acc.UUID, &acc.AccountIndex, &acc.UserXpub, &acc.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &acc, nil
}

func (s *Store) CreateAddress(ctx context.Context, accountUUID, network, asset, address, derivation string) (*Address, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO addresses (account_uuid, network, asset, address, derivation_path, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id
	`, accountUUID, network, asset, address, derivation)
	var id int64
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return &Address{ID: id, AccountUUID: accountUUID, Network: network, Asset: asset, Address: address, DerivationPath: derivation, IsActive: true}, nil
}

func (s *Store) GetBalances(ctx context.Context, accountUUID string) ([]Balance, error) {
	// UTXO-based balances
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.network, a.asset,
		       COALESCE(SUM(u.amount),0) AS total,
		       COALESCE(SUM(CASE WHEN r.status = 'RESERVED' THEN u.amount ELSE 0 END),0) AS reserved
		FROM addresses a
		LEFT JOIN chain_utxos u ON u.address_id = a.id AND u.spent_txid IS NULL
		LEFT JOIN utxo_reservations r ON r.network = u.network AND r.asset = u.asset AND r.txid = u.txid AND r.vout = u.vout
		WHERE a.account_uuid = $1
		GROUP BY a.network, a.asset
	`, accountUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := []Balance{}
	for rows.Next() {
		var b Balance
		var total int64
		if err := rows.Scan(&b.Network, &b.Asset, &total, &b.Reserved); err != nil {
			return nil, err
		}
		b.Available = total - b.Reserved
		balances = append(balances, b)
	}

	// Account-based balances (Tron)
	rows2, err := s.db.QueryContext(ctx, `
		SELECT a.network, a.asset, COALESCE(c.balance,0)
		FROM addresses a
		LEFT JOIN chain_accounts c ON c.address_id = a.id
		WHERE a.account_uuid = $1 AND a.network = 'tron'
	`, accountUUID)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var b Balance
		var total int64
		if err := rows2.Scan(&b.Network, &b.Asset, &total); err != nil {
			return nil, err
		}
		b.Available = total
		b.Reserved = 0
		balances = append(balances, b)
	}

	return balances, nil
}

func (s *Store) CreateWithdrawal(ctx context.Context, accountUUID, network, asset, destination string, amount int64) (*Withdrawal, error) {
	if amount <= 0 {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Reserve UTXOs for UTXO-based networks.
	var reservedSum int64
	type utxoSel struct {
		TxID  string
		Vout  int64
		Amount int64
		Network string
		Asset string
	}
	selected := []utxoSel{}

	rows, err := tx.QueryContext(ctx, `
		SELECT u.txid, u.vout, u.amount, u.network, u.asset
		FROM chain_utxos u
		JOIN addresses a ON a.id = u.address_id
		WHERE a.account_uuid = $1 AND u.network = $2 AND u.asset = $3
		  AND u.spent_txid IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM utxo_reservations r
		    WHERE r.network = u.network AND r.asset = u.asset
		      AND r.txid = u.txid AND r.vout = u.vout
		      AND r.status = 'RESERVED'
		  )
		ORDER BY u.confirmed_at NULLS LAST, u.id
		FOR UPDATE SKIP LOCKED
	`, accountUUID, network, asset)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u utxoSel
		if err := rows.Scan(&u.TxID, &u.Vout, &u.Amount, &u.Network, &u.Asset); err != nil {
			rows.Close()
			return nil, err
		}
		selected = append(selected, u)
		reservedSum += u.Amount
		if reservedSum >= amount {
			break
		}
	}
	rows.Close()

	if reservedSum < amount {
		return nil, ErrInsufficient
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO withdrawals (account_uuid, network, asset, amount, destination, status)
		VALUES ($1, $2, $3, $4, $5, 'CREATED')
		RETURNING id
	`, accountUUID, network, asset, amount, destination)
	var id int64
	if err := row.Scan(&id); err != nil {
		return nil, err
	}

	for _, u := range selected {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO utxo_reservations (network, asset, txid, vout, withdrawal_id, status)
			VALUES ($1, $2, $3, $4, $5, 'RESERVED')
			ON CONFLICT DO NOTHING
		`, u.Network, u.Asset, u.TxID, u.Vout, id)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Withdrawal{ID: id, AccountUUID: accountUUID, Network: network, Asset: asset, Amount: amount, Destination: destination, Status: "CREATED"}, nil
}

func (s *Store) GetWithdrawal(ctx context.Context, id int64) (*Withdrawal, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_uuid, network, asset, amount, destination, status
		FROM withdrawals WHERE id = $1
	`, id)
	var w Withdrawal
	if err := row.Scan(&w.ID, &w.AccountUUID, &w.Network, &w.Asset, &w.Amount, &w.Destination, &w.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (s *Store) UpdateWithdrawalStatus(ctx context.Context, id int64, status, txid string) (*Withdrawal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, account_uuid, network, asset, amount, destination, status
		FROM withdrawals WHERE id = $1 FOR UPDATE
	`, id)
	var w Withdrawal
	if err := row.Scan(&w.ID, &w.AccountUUID, &w.Network, &w.Asset, &w.Amount, &w.Destination, &w.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `UPDATE withdrawals SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		return nil, err
	}

	if status == "COMPLETED" {
		// Mark reserved UTXOs as spent and insert change if needed.
		type resRow struct {
			TxID      string
			Vout      int64
			Amount    int64
			AddressID int64
		}
		resRows, err := tx.QueryContext(ctx, `
			SELECT r.txid, r.vout, u.amount, u.address_id
			FROM utxo_reservations r
			JOIN chain_utxos u ON u.network = r.network AND u.asset = r.asset AND u.txid = r.txid AND u.vout = r.vout
			WHERE r.withdrawal_id = $1 AND r.status = 'RESERVED'
			FOR UPDATE
		`, id)
		if err != nil {
			return nil, err
		}
		var sum int64
		var firstAddrID int64
		for resRows.Next() {
			var rr resRow
			if err := resRows.Scan(&rr.TxID, &rr.Vout, &rr.Amount, &rr.AddressID); err != nil {
				resRows.Close()
				return nil, err
			}
			sum += rr.Amount
			if firstAddrID == 0 {
				firstAddrID = rr.AddressID
			}
		}
		resRows.Close()

		_, err = tx.ExecContext(ctx, `
			UPDATE utxo_reservations SET status = 'SPENT' WHERE withdrawal_id = $1 AND status = 'RESERVED'
		`, id)
		if err != nil {
			return nil, err
		}

		if txid != "" {
			_, err = tx.ExecContext(ctx, `
				UPDATE chain_utxos SET spent_txid = $1
				WHERE (network, asset, txid, vout) IN (
					SELECT network, asset, txid, vout FROM utxo_reservations WHERE withdrawal_id = $2
				)
			`, txid, id)
			if err != nil {
				return nil, err
			}
		}

		if sum > w.Amount && firstAddrID != 0 {
			change := sum - w.Amount
			changeTxID := "change-" + txid
			_, err = tx.ExecContext(ctx, `
				INSERT INTO chain_utxos (network, asset, txid, vout, address_id, amount, confirmations, confirmed_at)
				SELECT network, asset, $1, 1, $2, $3, 0, now()
				FROM chain_utxos WHERE address_id = $2 LIMIT 1
			`, changeTxID, firstAddrID, change)
			if err != nil {
				return nil, err
			}
		}
	}

	w.Status = status
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Store) ListTransactions(ctx context.Context, accountUUID string, limit int) ([]TxRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.network, u.asset, u.txid, u.amount, u.confirmations, COALESCE(u.confirmed_at, now())
		FROM chain_utxos u
		JOIN addresses a ON a.id = u.address_id
		WHERE a.account_uuid = $1
		ORDER BY u.confirmed_at DESC NULLS LAST
		LIMIT $2
	`, accountUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TxRecord
	for rows.Next() {
		var r TxRecord
		if err := rows.Scan(&r.Network, &r.Asset, &r.TxID, &r.Amount, &r.Confirmations, &r.ConfirmedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Watcher ingestion helpers

func (s *Store) UpsertUTXO(ctx context.Context, network, asset, txid string, vout int64, address string, amount int64, confirmations int, confirmedAt time.Time, blockHash string, blockHeight int64) error {
	var addressID int64
	row := s.db.QueryRowContext(ctx, `SELECT id FROM addresses WHERE address = $1 AND network = $2`, address, network)
	if err := row.Scan(&addressID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chain_utxos (network, asset, txid, vout, address_id, amount, confirmations, confirmed_at, block_hash, block_height)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (network, asset, txid, vout) DO UPDATE
		SET confirmations = EXCLUDED.confirmations,
		    confirmed_at = EXCLUDED.confirmed_at,
		    block_hash = EXCLUDED.block_hash,
		    block_height = EXCLUDED.block_height
	`, network, asset, txid, vout, addressID, amount, confirmations, confirmedAt, blockHash, blockHeight)
	return err
}

func (s *Store) UpsertAccountBalance(ctx context.Context, network, asset, address string, balance int64, lastBlock int64, lastTxID string) error {
	var addressID int64
	row := s.db.QueryRowContext(ctx, `SELECT id FROM addresses WHERE address = $1 AND network = $2`, address, network)
	if err := row.Scan(&addressID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chain_accounts (network, asset, address_id, balance, last_block, last_txid, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (network, asset, address_id) DO UPDATE
		SET balance = EXCLUDED.balance,
		    last_block = EXCLUDED.last_block,
		    last_txid = EXCLUDED.last_txid,
		    updated_at = now()
	`, network, asset, addressID, balance, lastBlock, lastTxID)
	return err
}
