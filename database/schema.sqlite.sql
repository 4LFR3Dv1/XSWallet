-- ============================================================
-- SQLite schema for Boltz v2 Swap Engine (Desktop App)
-- Production-ready with all critical fixes applied
-- ============================================================

BEGIN TRANSACTION;

-- ----------------------------
-- Core table: swaps
-- ----------------------------

CREATE TABLE IF NOT EXISTS swaps (
  id                  TEXT PRIMARY KEY,

  kind                TEXT NOT NULL CHECK(kind IN ('submarine', 'reverse', 'chain')),
  env                 TEXT NOT NULL CHECK(env IN ('regtest', 'testnet', 'mainnet')),

  provider            TEXT NOT NULL DEFAULT 'boltz',
  provider_url        TEXT NOT NULL,
  provider_ws_url     TEXT,

  -- Optimistic locking (CAS)
  version             INTEGER NOT NULL DEFAULT 0,

  -- Authoritative engine state
  state               TEXT NOT NULL DEFAULT 'open' CHECK(state IN (
    'open', 'locked', 'commit_started', 'waiting', 'waiting_claim_details',
    'signing_musig2_partial', 'sent_partial_to_provider', 'waiting_provider_broadcast',
    'refund_coop_waiting', 'fallback_script_ready', 'refunding',
    'completed', 'failed', 'canceled'
  )),

  -- Provider status + raw payload
  boltz_status        TEXT,
  boltz_raw           TEXT, -- JSON string

  -- Assets
  from_asset          TEXT NOT NULL,
  to_asset            TEXT NOT NULL,

  -- Locked intent snapshot (phase 1)
  locked_intent       TEXT, -- JSON string

  -- Deterministic restore anchors
  swap_key_index      INTEGER NOT NULL,
  preimage_hash_hex   TEXT,
  invoice_bolt11      TEXT,
  invoice_payment_hash_hex TEXT,

  -- Public keys
  claim_pubkey_hex    TEXT,
  refund_pubkey_hex   TEXT,

  -- Taproot / swap tree
  swap_tree_hex       TEXT,
  tap_tweak32_hex     TEXT,

  -- MuSig2 context
  musig_session_id    TEXT,
  musig_user_pubnonce_hex  TEXT,
  musig_boltz_pubnonce_hex TEXT,
  musig_txhash_hex         TEXT,
  musig_partial_sig_hex    TEXT,

  -- Timeouts
  timeout_block_heights TEXT, -- JSON string

  -- On-chain proofs (lockup)
  lockup_chain        TEXT CHECK(lockup_chain IN ('btc', 'liquid')),
  lockup_txid         TEXT,
  lockup_vout         INTEGER CHECK(lockup_vout IS NULL OR lockup_vout >= 0),
  lockup_script_hex   TEXT,
  lockup_script_hash  TEXT,
  lockup_amount_sat   TEXT CHECK(lockup_amount_sat IS NULL OR lockup_amount_sat GLOB '[0-9]*'),
  lockup_seen_mempool_at TEXT, -- ISO 8601 UTC
  lockup_confirmed_height INTEGER,

  -- On-chain proofs (claim)
  claim_chain         TEXT CHECK(claim_chain IN ('btc', 'liquid')),
  claim_txid          TEXT,
  claim_seen_mempool_at TEXT,
  claim_confirmed_height INTEGER,

  -- On-chain proofs (refund)
  refund_chain        TEXT CHECK(refund_chain IN ('btc', 'liquid')),
  refund_txid         TEXT,
  refund_seen_mempool_at TEXT,
  refund_confirmed_height INTEGER,

  -- Raw transaction hex
  lockup_rawtx_hex    TEXT,
  claim_rawtx_hex     TEXT,
  refund_rawtx_hex    TEXT,

  -- Liquid strategy
  liquid_tx_strategy  TEXT NOT NULL DEFAULT 'elementsd',

  -- Timestamps (ISO 8601 UTC)
  created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

  -- Guardrails
  CHECK (preimage_hash_hex IS NULL OR length(preimage_hash_hex) = 64),
  CHECK (invoice_payment_hash_hex IS NULL OR length(invoice_payment_hash_hex) = 64),
  CHECK (lockup_txid IS NULL OR length(lockup_txid) = 64),
  CHECK (claim_txid IS NULL OR length(claim_txid) = 64),
  CHECK (refund_txid IS NULL OR length(refund_txid) = 64),
  CHECK (claim_pubkey_hex IS NULL OR length(claim_pubkey_hex) IN (64, 66)),
  CHECK (refund_pubkey_hex IS NULL OR length(refund_pubkey_hex) IN (64, 66)),
  CHECK (state IN ('open','failed','canceled') OR locked_intent IS NOT NULL)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_swaps_state      ON swaps(state);
CREATE INDEX IF NOT EXISTS idx_swaps_kind       ON swaps(kind);
CREATE INDEX IF NOT EXISTS idx_swaps_env        ON swaps(env);
CREATE INDEX IF NOT EXISTS idx_swaps_updated_at ON swaps(updated_at);
CREATE INDEX IF NOT EXISTS idx_swaps_boltz_status ON swaps(boltz_status);

-- Partial index for watchdog
CREATE INDEX IF NOT EXISTS idx_swaps_active
  ON swaps(state, updated_at)
  WHERE state NOT IN ('completed','failed','canceled');

-- ----------------------------
-- Event log: swap_events
-- ----------------------------

CREATE TABLE IF NOT EXISTS swap_events (
  swap_id     TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
  seq         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_ms       INTEGER NOT NULL,
  source      TEXT NOT NULL,
  type        TEXT NOT NULL,
  payload     TEXT NOT NULL -- JSON string
);

CREATE INDEX IF NOT EXISTS idx_swap_events_swap_id_seq ON swap_events(swap_id, seq);
CREATE INDEX IF NOT EXISTS idx_swap_events_type        ON swap_events(type);

-- ----------------------------
-- Op ledger: swap_ops
-- ----------------------------

CREATE TABLE IF NOT EXISTS swap_ops (
  swap_id     TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
  op_key      TEXT NOT NULL,
  status      TEXT NOT NULL CHECK(status IN ('inflight','ok','fail')),
  started_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  heartbeat_at TEXT,
  finished_at TEXT,
  result      TEXT, -- JSON string
  error       TEXT,

  PRIMARY KEY (swap_id, op_key)
);

CREATE INDEX IF NOT EXISTS idx_swap_ops_status ON swap_ops(status);
CREATE INDEX IF NOT EXISTS idx_swap_ops_heartbeat ON swap_ops(heartbeat_at)
  WHERE status = 'inflight';

-- ----------------------------
-- UTXO Reservations
-- ----------------------------

CREATE TABLE IF NOT EXISTS utxo_reservations (
  chain               TEXT NOT NULL CHECK(chain IN ('btc', 'liquid')),
  txid                TEXT NOT NULL,
  vout                INTEGER NOT NULL CHECK(vout >= 0),
  
  swap_id             TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
  
  -- Convenience field (computed)
  outpoint            TEXT GENERATED ALWAYS AS (txid || ':' || vout) STORED,
  
  amount_sat          TEXT NOT NULL CHECK(amount_sat GLOB '[0-9]*'),
  script_hex          TEXT,
  
  -- Lifecycle
  reserved_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  spent_txid          TEXT,
  spent_at            TEXT,
  released_at         TEXT,
  
  PRIMARY KEY (chain, txid, vout)
);

CREATE INDEX IF NOT EXISTS idx_utxo_reservations_swap_id ON utxo_reservations(swap_id);
CREATE INDEX IF NOT EXISTS idx_utxo_reservations_chain   ON utxo_reservations(chain);

-- ----------------------------
-- LN Reservations
-- ----------------------------

CREATE TABLE IF NOT EXISTS ln_reservations (
  payment_hash_hex    TEXT PRIMARY KEY CHECK(length(payment_hash_hex) = 64),
  
  swap_id             TEXT NOT NULL REFERENCES swaps(id) ON DELETE CASCADE,
  direction           TEXT NOT NULL CHECK(direction IN ('pay', 'receive')),
  
  invoice_bolt11      TEXT,
  amount_msat         TEXT CHECK(amount_msat IS NULL OR amount_msat GLOB '[0-9]*'),
  
  -- Lifecycle
  reserved_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  settled_at          TEXT,
  failed_at           TEXT,
  preimage_hex        TEXT
);

CREATE INDEX IF NOT EXISTS idx_ln_reservations_swap_id   ON ln_reservations(swap_id);
CREATE INDEX IF NOT EXISTS idx_ln_reservations_direction ON ln_reservations(direction);

-- ----------------------------
-- App Config (key-value store)
-- ----------------------------

CREATE TABLE IF NOT EXISTS app_config (
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL, -- JSON string
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Default configs
INSERT OR IGNORE INTO app_config (key, value) VALUES
  ('network', '"regtest"'),
  ('provider_url', '"https://api.boltz.exchange"'),
  ('provider_ws_url', '"wss://api.boltz.exchange/v2/ws"'),
  ('kdf_params', '{"algorithm":"argon2id","memory":65536,"iterations":3,"parallelism":1}');

COMMIT;

-- ============================================================
-- USAGE PATTERNS
-- ============================================================

-- 1) CAS with updated_at
-- UPDATE swaps 
-- SET state='locked', version=version+1, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'), locked_intent=?
-- WHERE id=? AND version=?

-- 2) Reserve UTXO
-- INSERT INTO utxo_reservations (chain,txid,vout,swap_id,amount_sat) 
-- VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING

-- 3) Reserve LN payment
-- INSERT INTO ln_reservations (payment_hash_hex,swap_id,direction,invoice_bolt11,amount_msat)
-- VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING

-- 4) Idempotent op
-- INSERT INTO swap_ops (swap_id,op_key,status) 
-- VALUES (?,'op_key','inflight') ON CONFLICT DO NOTHING

-- 5) Log event
-- INSERT INTO swap_events (swap_id,ts_ms,source,type,payload) 
-- VALUES (?,?,?,?,?)
