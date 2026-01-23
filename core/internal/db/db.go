// Package db - SQLite database layer
// Fonte autoritativa de estado. WAL mode, CAS via version.
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wrapper do database
type DB struct {
	*sql.DB
}

// Open abre o database com pragmas corretos
func Open(path string) (*DB, error) {
	// Pragmas via connection string
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_cache_size=-20000", path)

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool
	sqlDB.SetMaxOpenConns(1) // SQLite só suporta 1 writer
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{sqlDB}, nil
}

// Migrate executa migrations
func (db *DB) Migrate() error {
	// Execute schema (CREATE IF NOT EXISTS are idempotent)
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	// Run migrations for existing databases
	return db.migrateExistingSchema()
}

// migrateExistingSchema handles schema evolution for existing databases
func (db *DB) migrateExistingSchema() error {
	// Check and add new columns to swaps table
	columns := []struct {
		name string
		ddl  string
	}{
		{"encrypted_preimage", "ALTER TABLE swaps ADD COLUMN encrypted_preimage BLOB"},
		{"boltz_status", "ALTER TABLE swaps ADD COLUMN boltz_status TEXT"},
		{"boltz_raw", "ALTER TABLE swaps ADD COLUMN boltz_raw TEXT"},
		{"from_asset", "ALTER TABLE swaps ADD COLUMN from_asset TEXT"},
		{"to_asset", "ALTER TABLE swaps ADD COLUMN to_asset TEXT"},
	}

	for _, col := range columns {
		exists, err := db.columnExists("swaps", col.name)
		if err != nil {
			return fmt.Errorf("failed to check column %s: %w", col.name, err)
		}
		if !exists {
			if _, err := db.Exec(col.ddl); err != nil {
				return fmt.Errorf("failed to add column %s: %w", col.name, err)
			}
		}
	}

	// Initialize vault_lockout record if not exists
	_, err := db.Exec(`INSERT OR IGNORE INTO vault_lockout (id, failed_attempts) VALUES (1, 0)`)
	if err != nil {
		return fmt.Errorf("failed to initialize vault_lockout: %w", err)
	}

	return nil
}

// columnExists checks if a column exists in a table
func (db *DB) columnExists(table, column string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?
	`, table, column).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Schema do banco - alinhado com documento técnico
const schema = `
-- =============================================================================
-- SWAPS - Estado autoritativo
-- =============================================================================
CREATE TABLE IF NOT EXISTS swaps (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK(kind IN ('submarine', 'reverse', 'chain')),
    env TEXT NOT NULL CHECK(env IN ('regtest', 'testnet', 'mainnet')),
    version INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'open' CHECK(state IN (
        'open', 'locked', 'commit_started', 'waiting', 
        'waiting_claim_details', 'signing_musig2_partial',
        'sent_partial_to_provider', 'waiting_provider_broadcast',
        'refund_coop_waiting', 'fallback_script_ready', 'refunding',
        'completed', 'failed', 'canceled'
    )),
    locked_intent TEXT CHECK(
        (state = 'open' OR state = 'canceled' OR state = 'failed') 
        OR locked_intent IS NOT NULL
    ),
    
    -- Keys (BIP85 derived)
    swap_key_index INTEGER NOT NULL,
    encrypted_preimage BLOB,  -- AES-256-GCM encrypted, NEVER store plaintext
    preimage_hash_hex TEXT CHECK(length(preimage_hash_hex) = 64 OR preimage_hash_hex IS NULL),
    claim_pubkey_hex TEXT CHECK(length(claim_pubkey_hex) IN (64, 66) OR claim_pubkey_hex IS NULL),
    refund_pubkey_hex TEXT CHECK(length(refund_pubkey_hex) IN (64, 66) OR refund_pubkey_hex IS NULL),
    
    -- MuSig2 session
    musig_session_id TEXT,
    musig_secnonce BLOB,
    musig_pubnonce BLOB,
    musig_agg_nonce BLOB,
    musig_partial_sig BLOB,
    
    -- Provider data
    boltz_id TEXT,
    boltz_status TEXT,  -- Provider status from WebSocket
    boltz_raw TEXT,     -- Raw JSON payload from provider
    lockup_address TEXT,
    claim_address TEXT,
    invoice TEXT,
    
    -- Asset tracking (for chain swaps)
    from_asset TEXT,
    to_asset TEXT,
    
    -- Transactions
    lockup_txid TEXT CHECK(length(lockup_txid) = 64 OR lockup_txid IS NULL),
    lockup_vout INTEGER,
    lockup_amount_sat TEXT CHECK(lockup_amount_sat GLOB '[0-9]*' OR lockup_amount_sat IS NULL),
    raw_funding_tx TEXT,  -- Raw hex of funding tx for idempotency/recovery
    claim_txid TEXT CHECK(length(claim_txid) = 64 OR claim_txid IS NULL),
    refund_txid TEXT CHECK(length(refund_txid) = 64 OR refund_txid IS NULL),
    
    -- Timeouts
    timeout_block_height INTEGER,
    
    -- Error
    error_message TEXT,
    
    -- Timestamps
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_swaps_state ON swaps(state);
CREATE INDEX IF NOT EXISTS idx_swaps_kind ON swaps(kind);
CREATE INDEX IF NOT EXISTS idx_swaps_env ON swaps(env);

-- =============================================================================
-- SWAP_EVENTS - Log de auditoria imutável
-- =============================================================================
CREATE TABLE IF NOT EXISTS swap_events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    swap_id TEXT NOT NULL REFERENCES swaps(id),
    from_state TEXT,
    to_state TEXT NOT NULL,
    trigger TEXT NOT NULL,
    details TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_swap_events_swap_id ON swap_events(swap_id);

-- =============================================================================
-- SWAP_OPS - Ledger de idempotência
-- =============================================================================
CREATE TABLE IF NOT EXISTS swap_ops (
    swap_id TEXT NOT NULL REFERENCES swaps(id),
    op_key TEXT NOT NULL,
    result TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (swap_id, op_key)
);

-- =============================================================================
-- UTXO_RESERVATIONS - Previne double-spend
-- =============================================================================
CREATE TABLE IF NOT EXISTS utxo_reservations (
    chain TEXT NOT NULL CHECK(chain IN ('btc', 'liquid')),
    txid TEXT NOT NULL CHECK(length(txid) = 64),
    vout INTEGER NOT NULL CHECK(vout >= 0),
    swap_id TEXT NOT NULL REFERENCES swaps(id),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (chain, txid, vout)
);

-- =============================================================================
-- LN_RESERVATIONS - Previne pagamentos duplicados
-- =============================================================================
CREATE TABLE IF NOT EXISTS ln_reservations (
    payment_hash_hex TEXT PRIMARY KEY CHECK(length(payment_hash_hex) = 64),
    swap_id TEXT NOT NULL REFERENCES swaps(id),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- =============================================================================
-- VAULT - Seed criptografado
-- =============================================================================
CREATE TABLE IF NOT EXISTS vault (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    encrypted_seed BLOB NOT NULL,
    salt BLOB NOT NULL,
    iv BLOB NOT NULL,
    tag BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- =============================================================================
-- VAULT_LOCKOUT - PIN attempt tracking and lockout
-- =============================================================================
CREATE TABLE IF NOT EXISTS vault_lockout (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TEXT,  -- ISO 8601 timestamp, NULL if not locked
    last_attempt_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- =============================================================================
-- QUOTES - Swap quotes temporários
-- =============================================================================
CREATE TABLE IF NOT EXISTS quotes (
    quote_id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    amount_sat INTEGER NOT NULL,
    data TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_quotes_expires ON quotes(expires_at);

-- =============================================================================
-- APP_CONFIG - Configurações do usuário
-- =============================================================================
CREATE TABLE IF NOT EXISTS app_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
`
