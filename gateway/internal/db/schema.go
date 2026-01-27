package db

const Schema = `
CREATE TABLE IF NOT EXISTS payment_intents (
  payment_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  amount_cents BIGINT NOT NULL,
  status TEXT NOT NULL,
  provider_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS deposits (
  id BIGSERIAL PRIMARY KEY,
  payment_id TEXT NOT NULL UNIQUE REFERENCES payment_intents(payment_id),
  asset TEXT NOT NULL,
  amount_cents BIGINT NOT NULL,
  status TEXT NOT NULL,
  chain_txid TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS balances (
  user_id TEXT PRIMARY KEY,
  available_cents BIGINT NOT NULL DEFAULT 0,
  locked_cents BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS destinations (
  id BIGSERIAL PRIMARY KEY,
  user_id TEXT NOT NULL,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, network, address)
);

CREATE TABLE IF NOT EXISTS payouts (
  id BIGSERIAL PRIMARY KEY,
  user_id TEXT NOT NULL,
  network TEXT NOT NULL,
  address TEXT NOT NULL,
  amount_cents BIGINT NOT NULL,
  status TEXT NOT NULL,
  txid TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`
