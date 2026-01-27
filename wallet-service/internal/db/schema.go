package db

const Schema = `
CREATE TABLE IF NOT EXISTS accounts (
  uuid TEXT PRIMARY KEY,
  account_index BIGSERIAL UNIQUE,
  user_xpub TEXT,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS addresses (
  id BIGSERIAL PRIMARY KEY,
  account_uuid TEXT NOT NULL REFERENCES accounts(uuid),
  network TEXT NOT NULL,
  asset TEXT NOT NULL,
  address TEXT NOT NULL,
  derivation_path TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(network, address)
);

CREATE TABLE IF NOT EXISTS chain_utxos (
  id BIGSERIAL PRIMARY KEY,
  network TEXT NOT NULL,
  asset TEXT NOT NULL,
  txid TEXT NOT NULL,
  vout BIGINT NOT NULL,
  address_id BIGINT NOT NULL REFERENCES addresses(id),
  amount BIGINT NOT NULL,
  confirmations INT NOT NULL DEFAULT 0,
  confirmed_at TIMESTAMPTZ,
  spent_txid TEXT,
  block_hash TEXT,
  block_height BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS chain_utxos_unique ON chain_utxos(network, asset, txid, vout);

CREATE TABLE IF NOT EXISTS chain_accounts (
  id BIGSERIAL PRIMARY KEY,
  network TEXT NOT NULL,
  asset TEXT NOT NULL,
  address_id BIGINT NOT NULL REFERENCES addresses(id),
  balance BIGINT NOT NULL,
  last_block BIGINT,
  last_txid TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(network, asset, address_id)
);

CREATE TABLE IF NOT EXISTS utxo_reservations (
  id BIGSERIAL PRIMARY KEY,
  network TEXT NOT NULL,
  asset TEXT NOT NULL,
  txid TEXT NOT NULL,
  vout BIGINT NOT NULL,
  withdrawal_id BIGINT,
  reserved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  status TEXT NOT NULL DEFAULT 'RESERVED',
  UNIQUE(network, asset, txid, vout)
);

CREATE TABLE IF NOT EXISTS withdrawals (
  id BIGSERIAL PRIMARY KEY,
  account_uuid TEXT NOT NULL REFERENCES accounts(uuid),
  network TEXT NOT NULL,
  asset TEXT NOT NULL,
  amount BIGINT NOT NULL,
  destination TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'CREATED',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payouts (
  id BIGSERIAL PRIMARY KEY,
  withdrawal_id BIGINT NOT NULL REFERENCES withdrawals(id),
  status TEXT NOT NULL,
  txid TEXT,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`
