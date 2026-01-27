package db

const Schema = `
CREATE TABLE IF NOT EXISTS payouts (
  id BIGSERIAL PRIMARY KEY,
  payment_id TEXT,
  withdrawal_id BIGINT,
  network TEXT,
  asset TEXT,
  amount BIGINT,
  destination TEXT,
  priority TEXT NOT NULL DEFAULT 'normal',
  status TEXT NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT,
  txid TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE payouts ADD COLUMN IF NOT EXISTS payment_id TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS withdrawal_id BIGINT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS network TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS asset TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS amount BIGINT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS destination TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS priority TEXT DEFAULT 'normal';
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'PENDING';
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS attempts INT DEFAULT 0;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS txid TEXT;
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

CREATE UNIQUE INDEX IF NOT EXISTS payouts_payment_network_uidx
  ON payouts(payment_id, network)
  WHERE payment_id IS NOT NULL AND network IS NOT NULL;

CREATE INDEX IF NOT EXISTS payouts_status_idx ON payouts(status, next_attempt_at);

CREATE TABLE IF NOT EXISTS payout_events (
  id BIGSERIAL PRIMARY KEY,
  payout_id BIGINT NOT NULL REFERENCES payouts(id),
  status_from TEXT,
  status_to TEXT NOT NULL,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dlq (
  id BIGSERIAL PRIMARY KEY,
  payout_id BIGINT NOT NULL UNIQUE REFERENCES payouts(id),
  reason TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS circuit_breakers (
  network TEXT PRIMARY KEY,
  state TEXT NOT NULL DEFAULT 'CLOSED',
  failures INT NOT NULL DEFAULT 0,
  opened_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`
