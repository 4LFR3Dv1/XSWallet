// XS Wallet API Service
// Connects to api-bridge → xscore

const API_BASE = '/api/v1';

// ============================================================================
// VAULT
// ============================================================================

export interface VaultStatus {
    state: 'not_initialized' | 'locked' | 'unlocked' | 'locked_out';
    failed_attempts?: number;
}

export interface InitVaultRequest {
    action: 'generate' | 'import';
    mnemonic?: string;
    pin: string;
}

export interface InitVaultResponse {
    success: boolean;
    mnemonic?: string;
    session_id: string;
}

export interface UnlockResponse {
    success: boolean;
    session_id?: string;
    error_message?: string;
    remaining_attempts?: number;
}

export async function getVaultStatus(): Promise<VaultStatus> {
    const res = await fetch(`${API_BASE}/wallet/status`);
    if (!res.ok) throw new Error('Failed to get vault status');
    return res.json();
}

export async function initVault(request: InitVaultRequest): Promise<InitVaultResponse> {
    const res = await fetch(`${API_BASE}/wallet/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            word_count: 24,
            pin: request.pin,
        }),
    });
    if (!res.ok) throw new Error('Failed to initialize vault');
    return res.json();
}

export async function unlockVault(pin: string): Promise<UnlockResponse> {
    const res = await fetch(`${API_BASE}/wallet/unlock`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pin }),
    });
    if (!res.ok) throw new Error('Failed to unlock vault');
    return res.json();
}

export async function lockVault(): Promise<{ success: boolean }> {
    const res = await fetch(`${API_BASE}/wallet/lock`, { method: 'POST' });
    if (!res.ok) throw new Error('Failed to lock vault');
    return res.json();
}

// ============================================================================
// WALLET
// ============================================================================

export interface Balances {
    btc: { confirmed: number; unconfirmed: number; pending_swap: number };
    liquid: { confirmed: number; unconfirmed: number; pending_swap: number };
    ln: { balance: number; pending_open: number; pending_close: number };
}

export interface AddressResponse {
    address: string;
    chain: string;
    derivation_path: string;
}

export interface SendOnchainRequest {
    chain: 'btc' | 'liquid';
    address: string;
    amount_sats: number;
    fee_rate_sat_vb?: number;
    subtract_fee?: boolean;
    label?: string;
}

export interface SendOnchainResponse {
    success: boolean;
    txid?: string;
    fee_sat?: number;
}

export async function getBalances(): Promise<Balances> {
    const res = await fetch(`${API_BASE}/wallet/balances`);
    if (!res.ok) throw new Error('Failed to get balances');
    return res.json();
}

export async function getNewAddress(chain: 'btc' | 'liquid'): Promise<AddressResponse> {
    const res = await fetch(`${API_BASE}/wallet/derive`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ chain: chain === 'btc' ? 'CHAIN_BTC' : 'CHAIN_LIQUID' }),
    });
    if (!res.ok) throw new Error('Failed to get address');
    return res.json();
}

export async function sendOnchain(request: SendOnchainRequest): Promise<SendOnchainResponse> {
    const res = await fetch(`${API_BASE}/wallet/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            chain: request.chain === 'btc' ? 'CHAIN_BTC' : 'CHAIN_LIQUID',
            address: request.address,
            amount_sat: request.amount_sats,
            fee_rate_sat_vb: request.fee_rate_sat_vb,
            subtract_fee: request.subtract_fee ?? false,
            label: request.label ?? '',
        }),
    });
    if (!res.ok) throw new Error('Failed to send on-chain');
    return res.json();
}

// ============================================================================
// SWAPS
// ============================================================================

export interface Swap {
    id: string;
    kind: 'submarine' | 'reverse' | 'chain';
    state: string;
    from_chain: string;
    to_chain: string;
    amount_sats: number;
    htlc_address?: string;
    preimage_hash?: string;
    created_at: string;
    updated_at: string;
}

export interface QuoteRequest {
    kind: 'submarine' | 'reverse' | 'chain';
    from_chain: string;
    to_chain: string;
    amount_sats: number;
}

export interface Quote {
    quote_id: string;
    from_amount: number;
    to_amount: number;
    fee_sats: number;
    fee_percent: number;
    expires_at: string;
}

export async function listSwaps(): Promise<Swap[]> {
    const res = await fetch(`${API_BASE}/swaps`);
    if (!res.ok) throw new Error('Failed to list swaps');
    const data = await res.json();
    return data.swaps || data || [];
}

export async function getSwap(id: string): Promise<Swap> {
    const res = await fetch(`${API_BASE}/swaps/${id}`);
    if (!res.ok) throw new Error('Failed to get swap');
    return res.json();
}

export async function createSwap(request: { from_chain: string; to_chain: string; amount_sats: number }): Promise<Swap> {
    const res = await fetch(`${API_BASE}/swaps`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
    });
    if (!res.ok) throw new Error('Failed to create swap');
    return res.json();
}

// ============================================================================
// BITCOIN
// ============================================================================

export interface BitcoinInfo {
    chain: string;
    blocks: number;
    headers: number;
    bestblockhash: string;
    difficulty: number;
    synced: boolean;
}

export interface FeeEstimates {
    fast: number;
    medium: number;
    slow: number;
}

export async function getBitcoinInfo(): Promise<BitcoinInfo> {
    const res = await fetch(`${API_BASE}/bitcoin/info`);
    if (!res.ok) throw new Error('Failed to get bitcoin info');
    return res.json();
}

export async function getFeeEstimates(): Promise<FeeEstimates> {
    const res = await fetch(`${API_BASE}/bitcoin/fees`);
    if (!res.ok) throw new Error('Failed to get fee estimates');
    return res.json();
}

// ============================================================================
// SYSTEM
// ============================================================================

export interface SystemHealth {
    status: 'healthy' | 'degraded';
    services: {
        xscore: 'connected' | 'disconnected';
        bitcoind: 'connected' | 'disconnected';
    };
    vault: string;
}

export async function getSystemHealth(): Promise<SystemHealth> {
    const res = await fetch(`${API_BASE}/system/health`);
    if (!res.ok) throw new Error('Failed to get system health');
    return res.json();
}
