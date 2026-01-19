/**
 * SQLite Database Layer for XS Wallet Desktop App
 * Uses better-sqlite3 (synchronous) with WAL mode for optimal concurrency
 */

import Database from 'better-sqlite3';
import { join } from 'path';
import { app } from 'electron';

// ============================================================
// Types (same as before)
// ============================================================

export type SwapKind = 'submarine' | 'reverse' | 'chain';
export type SwapChain = 'btc' | 'liquid';
export type SwapEnv = 'regtest' | 'testnet' | 'mainnet';
export type SwapState =
    | 'open' | 'locked' | 'commit_started' | 'waiting'
    | 'waiting_claim_details' | 'signing_musig2_partial'
    | 'sent_partial_to_provider' | 'waiting_provider_broadcast'
    | 'refund_coop_waiting' | 'fallback_script_ready'
    | 'refunding' | 'completed' | 'failed' | 'canceled';
export type OpStatus = 'inflight' | 'ok' | 'fail';

export interface Swap {
    id: string;
    kind: SwapKind;
    env: SwapEnv;
    version: number;
    state: SwapState;
    provider: string;
    provider_url: string;
    provider_ws_url?: string;
    boltz_status?: string;
    boltz_raw?: Record<string, unknown>;
    from_asset: string;
    to_asset: string;
    locked_intent?: Record<string, unknown>;
    swap_key_index: number;
    preimage_hash_hex?: string;
    invoice_bolt11?: string;
    invoice_payment_hash_hex?: string;
    claim_pubkey_hex?: string;
    refund_pubkey_hex?: string;
    swap_tree_hex?: string;
    tap_tweak32_hex?: string;
    musig_session_id?: string;
    musig_user_pubnonce_hex?: string;
    musig_boltz_pubnonce_hex?: string;
    musig_txhash_hex?: string;
    musig_partial_sig_hex?: string;
    timeout_block_heights?: Record<string, unknown>;
    lockup_chain?: SwapChain;
    lockup_txid?: string;
    lockup_vout?: number;
    lockup_script_hex?: string;
    lockup_amount_sat?: bigint;
    lockup_seen_mempool_at?: Date;
    lockup_confirmed_height?: number;
    lockup_rawtx_hex?: string;
    claim_txid?: string;
    claim_confirmed_height?: number;
    refund_txid?: string;
    refund_confirmed_height?: number;
    liquid_tx_strategy: string;
    created_at?: Date;
    updated_at?: Date;
}

export interface SwapEvent {
    swap_id: string;
    seq: number;
    ts_ms: number;
    source: string;
    type: string;
    payload: Record<string, unknown>;
}

export interface SwapOp {
    swap_id: string;
    op_key: string;
    status: OpStatus;
    started_at: Date;
    heartbeat_at: Date | null;
    finished_at: Date | null;
    result: Record<string, unknown> | null;
    error: string | null;
}

export interface CASUpdateResult {
    success: boolean;
    currentVersion: number;
    currentState?: SwapState;
    noUpdates?: boolean;
}

// ============================================================
// Database Initialization with Critical Pragmas
// ============================================================

let db: Database.Database | null = null;

/**
 * Initialize SQLite database with optimal settings for desktop app
 */
export function initDatabase(dbPath?: string): Database.Database {
    const path = dbPath ?? join(app.getPath('userData'), 'xs-wallet.db');

    db = new Database(path);

    // CRITICAL: Set pragmas for stability and performance
    db.pragma('journal_mode = WAL');      // Write-Ahead Logging (better concurrency)
    db.pragma('synchronous = NORMAL');    // Balance safety/speed (FULL is overkill with WAL)
    db.pragma('busy_timeout = 5000');     // Wait 5s on lock (prevents "database is locked")
    db.pragma('foreign_keys = ON');       // Enforce FK constraints
    db.pragma('temp_store = MEMORY');     // Temp tables in memory (faster)
    db.pragma('cache_size = -20000');     // ~20MB cache (adjustable)

    // Load schema
    const schemaPath = join(__dirname, '../../database/schema.sqlite.sql');
    const schema = require('fs').readFileSync(schemaPath, 'utf8');
    db.exec(schema);

    return db;
}

/**
 * Get database instance (must call initDatabase first)
 */
export function getDb(): Database.Database {
    if (!db) throw new Error('Database not initialized. Call initDatabase() first.');
    return db;
}

/**
 * Close database connection
 */
export function closeDatabase(): void {
    if (db) {
        db.close();
        db = null;
    }
}

// ============================================================
// Field Whitelists & Helpers
// ============================================================

const JSONB_FIELDS = new Set(['locked_intent', 'boltz_raw', 'timeout_block_heights']);

const ALLOWED_SWAP_FIELDS = new Set<keyof Swap>([
    'state', 'boltz_status', 'boltz_raw', 'locked_intent',
    'preimage_hash_hex', 'invoice_bolt11', 'invoice_payment_hash_hex',
    'claim_pubkey_hex', 'refund_pubkey_hex', 'swap_tree_hex', 'tap_tweak32_hex',
    'musig_session_id', 'musig_user_pubnonce_hex', 'musig_boltz_pubnonce_hex',
    'musig_txhash_hex', 'musig_partial_sig_hex', 'timeout_block_heights',
    'lockup_chain', 'lockup_txid', 'lockup_vout', 'lockup_script_hex',
    'lockup_amount_sat', 'lockup_rawtx_hex', 'lockup_seen_mempool_at',
    'lockup_confirmed_height', 'claim_txid', 'claim_confirmed_height',
    'refund_txid', 'refund_confirmed_height',
]);

function toDbValue(key: string, value: unknown): unknown {
    if (value === undefined) return null;
    if (typeof value === 'bigint') return value.toString();
    if (value instanceof Date) return value.toISOString();
    if (JSONB_FIELDS.has(key)) return value == null ? null : JSON.stringify(value);
    return value;
}

function normalizeHex(hex: string, expectedLength?: number): string {
    const normalized = hex.toLowerCase().replace(/^0x/, '');
    if (expectedLength !== undefined && normalized.length !== expectedLength) {
        throw new Error(`Invalid hex length: expected ${expectedLength}, got ${normalized.length}`);
    }
    return normalized;
}

function normalizeHexLoose(hex: string): string {
    return hex.toLowerCase().replace(/^0x/, '');
}

function normalizeTxid(txid: string): string {
    return normalizeHex(txid, 64);
}

function normalizeHash(hash: string): string {
    return normalizeHex(hash, 64);
}

function normalizePubkey(pubkey: string): string {
    const p = normalizeHex(pubkey);
    if (p.length !== 64 && p.length !== 66) {
        throw new Error(`Invalid pubkey length: expected 64 or 66 hex, got ${p.length}`);
    }
    return p;
}

function mapSwapRow(r: Record<string, unknown>): Swap {
    return {
        ...r,
        lockup_amount_sat: r.lockup_amount_sat != null ? BigInt(r.lockup_amount_sat as string) : undefined,
        boltz_raw: r.boltz_raw ? JSON.parse(r.boltz_raw as string) : undefined,
        locked_intent: r.locked_intent ? JSON.parse(r.locked_intent as string) : undefined,
        timeout_block_heights: r.timeout_block_heights ? JSON.parse(r.timeout_block_heights as string) : undefined,
        created_at: r.created_at ? new Date(r.created_at as string) : undefined,
        updated_at: r.updated_at ? new Date(r.updated_at as string) : undefined,
        lockup_seen_mempool_at: r.lockup_seen_mempool_at ? new Date(r.lockup_seen_mempool_at as string) : undefined,
    } as Swap;
}

// ============================================================
// CAS Operations (with transactions)
// ============================================================

/**
 * Update swap with optimistic locking (CAS)
 * CRITICAL: Returns success=false if no valid updates (prevents silent no-ops)
 */
export function updateSwapCAS(
    swapId: string,
    expectedVersion: number,
    updates: Partial<Swap>
): CASUpdateResult {
    const database = getDb();

    const setClauses: string[] = [
        'version = version + 1',
        "updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')"
    ];
    const values: unknown[] = [];

    for (const [key, value] of Object.entries(updates)) {
        if (key === 'id' || key === 'version' || key === 'created_at') continue;
        if (!ALLOWED_SWAP_FIELDS.has(key as keyof Swap)) continue;

        setClauses.push(`${key} = ?`);
        values.push(toDbValue(key, value));
    }

    // No valid updates - return failure
    if (setClauses.length === 1) {
        return {
            success: false,
            currentVersion: expectedVersion,
            currentState: undefined,
            noUpdates: true,
        };
    }

    values.push(swapId, expectedVersion);

    const stmt = database.prepare(`
    UPDATE swaps
    SET ${setClauses.join(', ')}
    WHERE id = ? AND version = ?
    RETURNING version, state
  `);

    const result = stmt.get(...values) as { version: number; state: SwapState } | undefined;

    if (!result) {
        const current = database.prepare('SELECT version, state FROM swaps WHERE id = ?').get(swapId) as
            { version: number; state: SwapState } | undefined;
        return {
            success: false,
            currentVersion: current?.version ?? -1,
            currentState: current?.state,
        };
    }

    return {
        success: true,
        currentVersion: result.version,
        currentState: result.state,
    };
}

/**
 * Transition swap state with CAS (atomic with event logging via transaction)
 */
export function transitionSwapState(
    swapId: string,
    expectedVersion: number,
    fromState: SwapState,
    toState: SwapState,
    additionalUpdates?: Partial<Swap>
): CASUpdateResult {
    const database = getDb();

    // Use transaction for atomicity
    const result = database.transaction(() => {
        const updates = { ...additionalUpdates, state: toState };
        const setClauses: string[] = [
            'version = version + 1',
            "updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')"
        ];
        const values: unknown[] = [];

        for (const [key, value] of Object.entries(updates)) {
            if (key === 'id' || key === 'version' || key === 'created_at') continue;
            if (key !== 'state' && !ALLOWED_SWAP_FIELDS.has(key as keyof Swap)) continue;

            setClauses.push(`${key} = ?`);
            values.push(toDbValue(key, value));
        }

        values.push(swapId, expectedVersion, fromState);

        const updateStmt = database.prepare(`
      UPDATE swaps
      SET ${setClauses.join(', ')}
      WHERE id = ? AND version = ? AND state = ?
      RETURNING version, state
    `);

        const updateResult = updateStmt.get(...values) as { version: number; state: SwapState } | undefined;

        if (!updateResult) {
            const current = database.prepare('SELECT version, state FROM swaps WHERE id = ?').get(swapId) as
                { version: number; state: SwapState } | undefined;
            return {
                success: false,
                currentVersion: current?.version ?? -1,
                currentState: current?.state,
            };
        }

        // Log state transition event (atomic with state change)
        const eventStmt = database.prepare(`
      INSERT INTO swap_events (swap_id, ts_ms, source, type, payload)
      VALUES (?, ?, ?, ?, ?)
    `);

        eventStmt.run(
            swapId,
            Date.now(),
            'engine',
            'STATE_TRANSITION',
            JSON.stringify({ from: fromState, to: toState, version: updateResult.version })
        );

        return {
            success: true,
            currentVersion: updateResult.version,
            currentState: updateResult.state,
        };
    })();

    return result;
}

// ============================================================
// Event Logging
// ============================================================

export function appendEvent(
    swapId: string,
    source: string,
    type: string,
    payload: Record<string, unknown>
): number {
    const database = getDb();
    const stmt = database.prepare(`
    INSERT INTO swap_events (swap_id, ts_ms, source, type, payload)
    VALUES (?, ?, ?, ?, ?)
  `);
    const result = stmt.run(swapId, Date.now(), source, type, JSON.stringify(payload));
    return result.lastInsertRowid as number;
}

export function getSwapEvents(swapId: string): SwapEvent[] {
    const database = getDb();
    const stmt = database.prepare('SELECT * FROM swap_events WHERE swap_id = ? ORDER BY seq ASC');
    const rows = stmt.all(swapId) as SwapEvent[];
    return rows.map(r => ({ ...r, payload: JSON.parse(r.payload as unknown as string) }));
}

// ============================================================
// Idempotent Operations (with stale reclaim via SQL interval)
// ============================================================

const STALE_OP_INTERVAL = '5 minutes';

export interface OpResult {
    alreadyExists: boolean;
    status: OpStatus;
    isStale?: boolean;
    result?: Record<string, unknown>;
    error?: string;
}

export function beginOp(swapId: string, opKey: string): OpResult {
    const database = getDb();

    // Try insert first
    try {
        const insertStmt = database.prepare(`
      INSERT INTO swap_ops (swap_id, op_key, status, heartbeat_at)
      VALUES (?, ?, 'inflight', datetime('now'))
    `);
        insertStmt.run(swapId, opKey);
        return { alreadyExists: false, status: 'inflight' };
    } catch (err: any) {
        if (!err.message.includes('UNIQUE constraint')) throw err;
    }

    // Already exists - check status
    const existingStmt = database.prepare(`
    SELECT status, result, error FROM swap_ops WHERE swap_id = ? AND op_key = ?
  `);
    const row = existingStmt.get(swapId, opKey) as SwapOp | undefined;

    if (!row) {
        // Race condition - retry
        return beginOp(swapId, opKey);
    }

    // If completed, return result
    if (row.status !== 'inflight') {
        return {
            alreadyExists: true,
            status: row.status,
            result: row.result ? JSON.parse(row.result as unknown as string) : undefined,
            error: row.error ?? undefined,
        };
    }

    // Try to reclaim stale op (SQL interval comparison)
    const reclaimStmt = database.prepare(`
    UPDATE swap_ops
    SET started_at = datetime('now'), heartbeat_at = datetime('now')
    WHERE swap_id = ? AND op_key = ? AND status = 'inflight'
      AND datetime(COALESCE(heartbeat_at, started_at)) < datetime('now', '-${STALE_OP_INTERVAL}')
  `);

    const reclaimResult = reclaimStmt.run(swapId, opKey);

    if (reclaimResult.changes > 0) {
        return { alreadyExists: false, status: 'inflight', isStale: true };
    }

    // Still inflight and not stale
    return { alreadyExists: true, status: 'inflight' };
}

export function heartbeatOp(swapId: string, opKey: string): void {
    const database = getDb();
    const stmt = database.prepare(`
    UPDATE swap_ops SET heartbeat_at = datetime('now')
    WHERE swap_id = ? AND op_key = ? AND status = 'inflight'
  `);
    stmt.run(swapId, opKey);
}

export function finishOpOk(swapId: string, opKey: string, result?: Record<string, unknown>): void {
    const database = getDb();
    const stmt = database.prepare(`
    UPDATE swap_ops
    SET status = 'ok', finished_at = datetime('now'), result = ?
    WHERE swap_id = ? AND op_key = ?
  `);
    stmt.run(result ? JSON.stringify(result) : null, swapId, opKey);
}

export function finishOpFail(swapId: string, opKey: string, error: string): void {
    const database = getDb();
    const stmt = database.prepare(`
    UPDATE swap_ops
    SET status = 'fail', finished_at = datetime('now'), error = ?
    WHERE swap_id = ? AND op_key = ?
  `);
    stmt.run(error, swapId, opKey);
}

// ============================================================
// UTXO & LN Reservations
// ============================================================

export interface ReserveUtxoResult {
    reserved: boolean;
    existingSwapId?: string;
}

export function reserveUtxo(
    chain: SwapChain,
    txid: string,
    vout: number,
    swapId: string,
    amountSat: bigint,
    scriptHex?: string
): ReserveUtxoResult {
    const database = getDb();
    const normalizedTxid = normalizeTxid(txid);
    const normalizedScript = scriptHex ? normalizeHexLoose(scriptHex) : null;

    try {
        const stmt = database.prepare(`
      INSERT INTO utxo_reservations (chain, txid, vout, swap_id, amount_sat, script_hex)
      VALUES (?, ?, ?, ?, ?, ?)
    `);
        stmt.run(chain, normalizedTxid, vout, swapId, amountSat.toString(), normalizedScript);
        return { reserved: true };
    } catch (err: any) {
        if (!err.message.includes('UNIQUE constraint')) throw err;

        const existingStmt = database.prepare(`
      SELECT swap_id FROM utxo_reservations WHERE chain = ? AND txid = ? AND vout = ?
    `);
        const existing = existingStmt.get(chain, normalizedTxid, vout) as { swap_id: string } | undefined;
        return {
            reserved: false,
            existingSwapId: existing?.swap_id,
        };
    }
}

export interface ReserveLnResult {
    reserved: boolean;
    existingSwapId?: string;
}

export function reserveLnPayment(
    paymentHashHex: string,
    swapId: string,
    direction: 'pay' | 'receive',
    invoiceBolt11?: string,
    amountMsat?: bigint
): ReserveLnResult {
    const database = getDb();
    const normalizedHash = normalizeHash(paymentHashHex);

    try {
        const stmt = database.prepare(`
      INSERT INTO ln_reservations (payment_hash_hex, swap_id, direction, invoice_bolt11, amount_msat)
      VALUES (?, ?, ?, ?, ?)
    `);
        stmt.run(normalizedHash, swapId, direction, invoiceBolt11 ?? null, amountMsat?.toString() ?? null);
        return { reserved: true };
    } catch (err: any) {
        if (!err.message.includes('UNIQUE constraint')) throw err;

        const existingStmt = database.prepare(`
      SELECT swap_id FROM ln_reservations WHERE payment_hash_hex = ?
    `);
        const existing = existingStmt.get(normalizedHash) as { swap_id: string } | undefined;
        return {
            reserved: false,
            existingSwapId: existing?.swap_id,
        };
    }
}

// ============================================================
// Query Helpers
// ============================================================

export function getSwap(swapId: string): Swap | null {
    const database = getDb();
    const stmt = database.prepare('SELECT * FROM swaps WHERE id = ?');
    const row = stmt.get(swapId);
    if (!row) return null;
    return mapSwapRow(row as Record<string, unknown>);
}

export function getActiveSwaps(limit: number = 100): Swap[] {
    const database = getDb();
    const stmt = database.prepare(`
    SELECT * FROM swaps
    WHERE state NOT IN ('completed', 'failed', 'canceled')
    ORDER BY updated_at ASC
    LIMIT ?
  `);
    const rows = stmt.all(limit) as Record<string, unknown>[];
    return rows.map(mapSwapRow);
}

export function createSwap(swap: Omit<Swap, 'version' | 'created_at' | 'updated_at'>): Swap {
    const database = getDb();
    const entries = Object.entries(swap).filter(([k]) => k !== 'version');
    const columns = entries.map(([k]) => k);
    const values = entries.map(([k, v]) => toDbValue(k, v));
    const placeholders = columns.map(() => '?');

    const stmt = database.prepare(`
    INSERT INTO swaps (${columns.join(', ')})
    VALUES (${placeholders.join(', ')})
    RETURNING *
  `);

    const result = stmt.get(...values);
    return mapSwapRow(result as Record<string, unknown>);
}

// ============================================================
// Exports
// ============================================================

export {
    normalizeHex,
    normalizeHexLoose,
    normalizeTxid,
    normalizeHash,
    normalizePubkey,
    mapSwapRow,
};
