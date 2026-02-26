// XS Wallet API Service
// Connects to api-bridge → xscore

const API_BASE = '/api/v1';
const DEBUG_HTTP = import.meta.env.VITE_DEBUG_HTTP === '1';
const DEV_HTTP_AUTH_BEARER = DEBUG_HTTP ? (import.meta.env.VITE_API_AUTH_BEARER || 'dev') : '';
let sessionAuthToken: string | null = null;

type IpcChannel =
    | 'wallet.init'
    | 'wallet.getStatus'
    | 'wallet.unlock'
    | 'wallet.lock'
    | 'wallet.getBalances'
    | 'wallet.getNewAddress'
    | 'wallet.listAddresses'
    | 'wallet.listUtxos'
    | 'wallet.listTransactions'
    | 'wallet.sendOnchain'
    | 'swap.create'
    | 'swap.check'
    | 'swap.list'
    | 'swap.get'
    | 'swap.getEvents'
    | 'swap.watchAll'
    | 'nodes.list'
    | 'nodes.start'
    | 'nodes.stop'
    | 'nodes.restart';

function canUseIpc(): boolean {
    return !DEBUG_HTTP && typeof window !== 'undefined' && !!window.xs;
}

function ensureDebugHttpEnabled(operation: string): void {
    // Browser/dev mode (without Electron preload) keeps HTTP path available.
    if (typeof window !== 'undefined' && !window.xs) {
        return;
    }
    if (!DEBUG_HTTP) {
        throw new Error(`HTTP fallback disabled for ${operation}. Use IPC or set VITE_DEBUG_HTTP=1 for debug.`);
    }
}

async function invokeIpc<T>(channel: IpcChannel, payload?: unknown): Promise<T> {
    if (!window.xs) {
        throw new Error('IPC bridge is not available');
    }
    return window.xs.invoke(channel, payload) as Promise<T>;
}

export function setSessionAuthToken(token: string | null): void {
    sessionAuthToken = token;
}

function currentAuthToken(): string {
    return sessionAuthToken ?? DEV_HTTP_AUTH_BEARER;
}

async function readErrorMessage(res: Response, fallback: string): Promise<string> {
    const text = await res.text();
    if (!text) return fallback;
    try {
        const parsed = JSON.parse(text);
        return parsed?.error || parsed?.message || fallback;
    } catch {
        return text;
    }
}

async function requestJson<T>(path: string, init?: RequestInit): Promise<T> {
    const headers = new Headers(init?.headers ?? {});
    const authToken = currentAuthToken();
    if (authToken && !headers.has('authorization')) {
        headers.set('authorization', `Bearer ${authToken}`);
    }

    const res = await fetch(path, { ...init, headers });
    if (!res.ok) {
        throw new Error(await readErrorMessage(res, `Request failed (${res.status})`));
    }
    return res.json() as Promise<T>;
}

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
    if (canUseIpc()) {
        return invokeIpc<VaultStatus>('wallet.getStatus');
    }
    ensureDebugHttpEnabled('wallet.getStatus');
    return requestJson<VaultStatus>(`${API_BASE}/wallet/status`);
}

export async function initVault(request: InitVaultRequest): Promise<InitVaultResponse> {
    if (canUseIpc()) {
        return invokeIpc<InitVaultResponse>('wallet.init', request);
    }
    ensureDebugHttpEnabled('wallet.init');
    return requestJson<InitVaultResponse>(`${API_BASE}/wallet/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            word_count: 24,
            pin: request.pin,
        }),
    });
}

export async function unlockVault(pin: string): Promise<UnlockResponse> {
    if (canUseIpc()) {
        return invokeIpc<UnlockResponse>('wallet.unlock', { pin });
    }
    ensureDebugHttpEnabled('wallet.unlock');
    return requestJson<UnlockResponse>(`${API_BASE}/wallet/unlock`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pin }),
    });
}

export async function lockVault(): Promise<{ success: boolean }> {
    if (canUseIpc()) {
        return invokeIpc<{ success: boolean }>('wallet.lock');
    }
    ensureDebugHttpEnabled('wallet.lock');
    return requestJson<{ success: boolean }>(`${API_BASE}/wallet/lock`, { method: 'POST' });
}

// ============================================================================
// WALLET
// ============================================================================

export interface Balances {
    btc: { confirmed: number; unconfirmed: number; pending_swap: number };
    liquid: { confirmed: number; unconfirmed: number; pending_swap: number };
    ln: { balance: number; pending_open: number; pending_close: number };
}

function normalizeBalanceChain(input: any): { confirmed: number; unconfirmed: number; pending_swap: number } {
    return {
        confirmed: Number(input?.confirmed ?? input?.confirmed_sat ?? 0),
        unconfirmed: Number(input?.unconfirmed ?? input?.unconfirmed_sat ?? 0),
        pending_swap: Number(input?.pending_swap ?? input?.pending_swap_sat ?? 0),
    };
}

export interface AddressResponse {
    address: string;
    chain: string;
    derivation_path: string;
}

export interface AddressInfo {
    address: string;
    chain: 'btc' | 'liquid' | 'ln' | string;
    derivation_path: string;
    label?: string;
    used: boolean;
    balance_sat: number;
}

export interface UtxoReservation {
    swap_id: string | null;
    reservation_type: 'lockup' | 'fee' | 'manual' | 'unknown';
    reserved_by: 'swap_engine' | 'manual' | 'recovery' | 'unknown';
    reserved_at: string | null;
    expires_at: string | null;
}

export interface WalletUtxo {
    txid: string;
    vout: number;
    amount_sat: number;
    address: string;
    confirmations: number;
    reserved: boolean;
    reserved_for_swap_id: string | null;
    chain: 'btc' | 'liquid' | 'ln' | string;
    reservation: UtxoReservation | null;
}

export interface WalletTransaction {
    txid: string;
    chain: 'btc' | 'liquid' | 'ln' | string;
    amount_sat: number;
    fee_sat: number;
    confirmations: number;
    label: string;
    swap_id: string | null;
    timestamp: string | null;
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
    const normalizeAll = (raw: any): Balances => ({
        btc: normalizeBalanceChain(raw?.btc),
        liquid: normalizeBalanceChain(raw?.liquid),
        ln: {
            balance: Number(raw?.ln?.balance ?? raw?.ln?.total_sat ?? raw?.ln?.confirmed_sat ?? 0),
            pending_open: Number(raw?.ln?.pending_open ?? 0),
            pending_close: Number(raw?.ln?.pending_close ?? 0),
        },
    });

    if (canUseIpc()) {
        const raw = await invokeIpc<any>('wallet.getBalances');
        return normalizeAll(raw);
    }
    ensureDebugHttpEnabled('wallet.getBalances');
    const raw = await requestJson<any>(`${API_BASE}/wallet/balances`);
    return normalizeAll(raw);
}

export async function getNewAddress(chain: 'btc' | 'liquid'): Promise<AddressResponse> {
    if (canUseIpc()) {
        return invokeIpc<AddressResponse>('wallet.getNewAddress', { chain });
    }
    ensureDebugHttpEnabled('wallet.derive');
    return requestJson<AddressResponse>(`${API_BASE}/wallet/derive`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ chain: chain === 'btc' ? 'CHAIN_BTC' : 'CHAIN_LIQUID' }),
    });
}

export async function listAddresses(chain: 'btc' | 'liquid', includeUsed = true): Promise<AddressInfo[]> {
    if (canUseIpc()) {
        const data = await invokeIpc<{ addresses?: AddressInfo[] }>('wallet.listAddresses', { chain, includeUsed });
        return data.addresses || [];
    }
    ensureDebugHttpEnabled('wallet.addresses');
    return requestJson<{ addresses: AddressInfo[] }>(
        `${API_BASE}/wallet/addresses?chain=${encodeURIComponent(chain)}&include_used=${includeUsed ? 1 : 0}`
    ).then((r) => r.addresses || []);
}

export async function listUtxos(chain: 'btc' | 'liquid', includeReserved = true): Promise<WalletUtxo[]> {
    if (canUseIpc()) {
        const data = await invokeIpc<{ utxos?: WalletUtxo[] }>('wallet.listUtxos', { chain, includeReserved });
        return data.utxos || [];
    }
    ensureDebugHttpEnabled('wallet.utxos');
    return requestJson<{ utxos: WalletUtxo[] }>(
        `${API_BASE}/wallet/utxos?chain=${encodeURIComponent(chain)}&include_reserved=${includeReserved ? 1 : 0}`
    ).then((r) => r.utxos || []);
}

export async function listTransactions(chain: 'btc' | 'liquid', limit = 50, offset = 0): Promise<WalletTransaction[]> {
    if (canUseIpc()) {
        const data = await invokeIpc<{ transactions?: WalletTransaction[] }>('wallet.listTransactions', { chain, limit, offset });
        return data.transactions || [];
    }
    ensureDebugHttpEnabled('wallet.transactions');
    return requestJson<{ transactions: WalletTransaction[] }>(
        `${API_BASE}/wallet/transactions?chain=${encodeURIComponent(chain)}&limit=${limit}&offset=${offset}`
    ).then((r) => r.transactions || []);
}

export async function sendOnchain(request: SendOnchainRequest): Promise<SendOnchainResponse> {
    if (canUseIpc()) {
        return invokeIpc<SendOnchainResponse>('wallet.sendOnchain', request);
    }
    ensureDebugHttpEnabled('wallet.send');
    return requestJson<SendOnchainResponse>(`${API_BASE}/wallet/send`, {
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
}

// ============================================================================
// SWAPS
// ============================================================================

export interface Swap {
    id: string;
    kind?: 'submarine' | 'reverse' | 'chain';
    state: string;
    from_chain: string;
    to_chain: string;
    amount_sats: number;
    htlc_address?: string;
    lockup_address?: string;
    claim_address?: string;
    destination_address?: string;
    invoice?: string;
    error_message?: string | null;
    preimage_hash?: string;
    funded_txid?: string;
    created_at: string;
    updated_at: string;
}

export interface SwapEvent {
    seq: number;
    swap_id: string;
    from_state: string;
    to_state: string;
    trigger: string;
    details_json?: string;
    created_at?: string;
}

function upsertSwapById(current: Swap[], incoming: Swap): Swap[] {
    const idx = current.findIndex((s) => s.id === incoming.id);
    if (idx === -1) return [incoming, ...current];
    const next = current.slice();
    next[idx] = incoming;
    return next;
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

export interface CreateSwapRequest {
    from_chain: string;
    to_chain: string;
    amount_sats: number;
    invoice?: string;
    destination_address?: string;
}

function isUnimplementedStreamMessage(message: string): boolean {
    const normalized = String(message || '').toLowerCase();
    return normalized.includes('unimplemented') || normalized.includes('not implemented');
}

export async function watchAllSwaps(onUpdate: (swaps: Swap[]) => void): Promise<() => void> {
    if (canUseIpc() && window.xs?.onSwapWatchAll) {
        const unsubscribeEvent = window.xs.onSwapWatchAll((payload) => {
            const swaps = (payload as { swaps?: Swap[] })?.swaps;
            if (Array.isArray(swaps)) {
                onUpdate(swaps);
            }
        });
        await invokeIpc<{ subscribed: boolean }>('swap.watchAll', { action: 'subscribe' });
        return () => {
            unsubscribeEvent();
            void invokeIpc<{ subscribed: boolean }>('swap.watchAll', { action: 'unsubscribe' }).catch(() => { });
        };
    }
    ensureDebugHttpEnabled('swap.watchAll');

    // HTTP path: try native SSE stream first, with polling fallback.
    let stopped = false;
    let stream: EventSource | null = null;
    let fallbackInterval: number | null = null;
    let reconnectTimer: number | null = null;
    let watchdogTimer: number | null = null;
    let current: Swap[] = [];
    let lastSeq = 0;
    let reconnectDelayMs = 1000;
    let lastActivityAt = Date.now();
    let disableStreamUntil = 0;

    const pollSnapshot = async () => {
        try {
            const data = await listSwaps();
            current = data;
            onUpdate(data);
        } catch {
            // Keep fallback alive even on transient failures.
        }
    };

    const startFallback = () => {
        if (fallbackInterval != null) return;
        fallbackInterval = window.setInterval(() => {
            void pollSnapshot();
        }, 10000);
    };

    const stopFallback = () => {
        if (fallbackInterval != null) {
            window.clearInterval(fallbackInterval);
            fallbackInterval = null;
        }
    };

    const clearWatchdog = () => {
        if (watchdogTimer != null) {
            window.clearInterval(watchdogTimer);
            watchdogTimer = null;
        }
    };

    const markActivity = () => {
        lastActivityAt = Date.now();
    };

    const startWatchdog = () => {
        clearWatchdog();
        watchdogTimer = window.setInterval(() => {
            if (stopped || stream == null) return;
            if (Date.now() - lastActivityAt > 45000) {
                try { stream.close(); } catch { }
                stream = null;
                startFallback();
                if (reconnectTimer == null) {
                    const jitter = Math.floor(Math.random() * 250);
                    reconnectTimer = window.setTimeout(() => {
                        reconnectTimer = null;
                        connect();
                    }, reconnectDelayMs + jitter);
                    reconnectDelayMs = Math.min(reconnectDelayMs * 2, 30000);
                }
            }
        }, 5000);
    };

    await pollSnapshot();

    const connect = () => {
        if (stopped) return;
        if (Date.now() < disableStreamUntil) {
            startFallback();
            return;
        }
        const token = currentAuthToken();
        const url = new URL(`${API_BASE}/swaps/events/stream`, window.location.origin);
        url.searchParams.set('from_seq', String(lastSeq));
        if (token) {
            url.searchParams.set('access_token', token);
        }
        stream = new EventSource(url.toString());
        stream.onopen = () => {
            reconnectDelayMs = 1000;
            markActivity();
            stopFallback();
            startWatchdog();
        };

        const onSwapEvent = async (event: MessageEvent) => {
            if (stopped) return;
            markActivity();
            try {
                const ev = JSON.parse(event.data) as SwapEvent;
                if (ev.seq > lastSeq) lastSeq = ev.seq;
                if (!ev.swap_id) return;
                const latest = await getSwap(ev.swap_id);
                current = upsertSwapById(current, latest);
                onUpdate(current);
            } catch {
                // Ignore malformed events and keep stream alive.
            }
        };

        stream.addEventListener('swap_event', onSwapEvent as EventListener);
        stream.addEventListener('ready', (() => markActivity()) as EventListener);
        stream.addEventListener('ping', (() => markActivity()) as EventListener);
        stream.addEventListener('stream_error', ((event: MessageEvent) => {
            markActivity();
            try {
                const payload = JSON.parse(String((event as MessageEvent).data || '{}')) as { message?: string };
                if (isUnimplementedStreamMessage(payload.message || '')) {
                    disableStreamUntil = Date.now() + (5 * 60 * 1000);
                    try { stream?.close(); } catch { }
                    stream = null;
                    clearWatchdog();
                    startFallback();
                }
            } catch {
                // ignore custom error payload parsing errors
            }
        }) as EventListener);
        // Backward compatibility for old bridge event name.
        stream.addEventListener('error', ((event: MessageEvent) => {
            markActivity();
            try {
                const payload = JSON.parse(String((event as MessageEvent).data || '{}')) as { message?: string };
                if (isUnimplementedStreamMessage(payload.message || '')) {
                    disableStreamUntil = Date.now() + (5 * 60 * 1000);
                    try { stream?.close(); } catch { }
                    stream = null;
                    clearWatchdog();
                    startFallback();
                }
            } catch {
                // ignore custom error payload parsing errors
            }
        }) as EventListener);
        stream.onerror = () => {
            if (stopped) return;
            try { stream?.close(); } catch { }
            stream = null;
            clearWatchdog();
            startFallback();
            if (Date.now() < disableStreamUntil) {
                return;
            }
            if (reconnectTimer == null) {
                const jitter = Math.floor(Math.random() * 250);
                reconnectTimer = window.setTimeout(() => {
                    reconnectTimer = null;
                    connect();
                }, reconnectDelayMs + jitter);
                reconnectDelayMs = Math.min(reconnectDelayMs * 2, 30000);
            }
        };
    };

    try {
        connect();
    } catch {
        startFallback();
    }

    return () => {
        stopped = true;
        if (stream) {
            try { stream.close(); } catch { }
            stream = null;
        }
        clearWatchdog();
        if (reconnectTimer != null) {
            window.clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
        stopFallback();
    };
}

export async function listSwaps(): Promise<Swap[]> {
    if (canUseIpc()) {
        const data = await invokeIpc<{ swaps?: Swap[] } | Swap[]>('swap.list');
        return Array.isArray(data) ? data : (data.swaps || []);
    }
    ensureDebugHttpEnabled('swap.list');
    const data = await requestJson<{ swaps?: Swap[] } | Swap[]>(`${API_BASE}/swaps`);
    return Array.isArray(data) ? data : (data.swaps || []);
}

export async function getSwap(id: string): Promise<Swap> {
    if (canUseIpc()) {
        return invokeIpc<Swap>('swap.get', { id });
    }
    ensureDebugHttpEnabled('swap.get');
    return requestJson<Swap>(`${API_BASE}/swaps/${id}`);
}

export async function getSwapEvents(id: string, afterSeq = 0): Promise<SwapEvent[]> {
    if (canUseIpc()) {
        return invokeIpc<SwapEvent[]>('swap.getEvents', { id, afterSeq });
    }
    ensureDebugHttpEnabled('swap.events');
    return requestJson<SwapEvent[]>(`${API_BASE}/swaps/${encodeURIComponent(id)}/events?after_seq=${afterSeq}`);
}

export function watchSwapEvents(
    id: string,
    fromSeq: number,
    onEvent: (ev: SwapEvent) => void,
    onError?: (message: string) => void,
): () => void {
    if (canUseIpc()) {
        let stopped = false;
        let timer: number | null = null;
        let lastSeq = fromSeq;
        const poll = async () => {
            if (stopped) return;
            try {
                const events = await invokeIpc<SwapEvent[]>('swap.getEvents', { id, afterSeq: lastSeq });
                for (const ev of events) {
                    if (ev.seq > lastSeq) lastSeq = ev.seq;
                    onEvent(ev);
                }
            } catch (e) {
                const msg = e instanceof Error ? e.message : 'swap events IPC unavailable';
                onError?.(msg);
            }
        };
        void poll();
        timer = window.setInterval(() => {
            void poll();
        }, 5000);
        return () => {
            stopped = true;
            if (timer != null) {
                window.clearInterval(timer);
                timer = null;
            }
        };
    }
    ensureDebugHttpEnabled('swap.events.stream');
    let stopped = false;
    let source: EventSource | null = null;
    let reconnectTimer: number | null = null;
    let watchdogTimer: number | null = null;
    let lastSeq = fromSeq;
    let reconnectDelayMs = 1000;
    let lastActivityAt = Date.now();
    let disableStreamUntil = 0;

    const markActivity = () => {
        lastActivityAt = Date.now();
    };

    const clearWatchdog = () => {
        if (watchdogTimer != null) {
            window.clearInterval(watchdogTimer);
            watchdogTimer = null;
        }
    };

    const connect = () => {
        if (stopped) return;
        if (Date.now() < disableStreamUntil) {
            onError?.('stream indisponível no backend; usando fallback por polling');
            return;
        }
        const token = currentAuthToken();
        const url = new URL(`${API_BASE}/swaps/${encodeURIComponent(id)}/events/stream`, window.location.origin);
        url.searchParams.set('from_seq', String(lastSeq));
        if (token) {
            url.searchParams.set('access_token', token);
        }
        source = new EventSource(url.toString());
        source.onopen = () => {
            reconnectDelayMs = 1000;
            markActivity();
            clearWatchdog();
            watchdogTimer = window.setInterval(() => {
                if (stopped || source == null) return;
                if (Date.now() - lastActivityAt > 45000) {
                    try { source.close(); } catch { }
                    source = null;
                    onError?.('swap events stream stalled');
                    if (reconnectTimer == null) {
                        const jitter = Math.floor(Math.random() * 250);
                        reconnectTimer = window.setTimeout(() => {
                            reconnectTimer = null;
                            connect();
                        }, reconnectDelayMs + jitter);
                        reconnectDelayMs = Math.min(reconnectDelayMs * 2, 30000);
                    }
                }
            }, 5000);
        };

        const handleSwapEvent = (event: MessageEvent) => {
            markActivity();
            try {
                const payload = JSON.parse(event.data) as SwapEvent;
                if (payload.seq > lastSeq) lastSeq = payload.seq;
                onEvent(payload);
            } catch {
                onError?.('invalid stream payload');
            }
        };

        source.addEventListener('swap_event', handleSwapEvent as EventListener);
        source.addEventListener('ready', (() => markActivity()) as EventListener);
        source.addEventListener('ping', (() => markActivity()) as EventListener);
        source.addEventListener('stream_error', ((event: MessageEvent) => {
            markActivity();
            try {
                const payload = JSON.parse(String((event as MessageEvent).data || '{}')) as { message?: string };
                if (isUnimplementedStreamMessage(payload.message || '')) {
                    disableStreamUntil = Date.now() + (5 * 60 * 1000);
                    onError?.('stream de eventos não implementado no backend; modo degradado');
                    try { source?.close(); } catch { }
                    source = null;
                    clearWatchdog();
                }
            } catch {
                // ignore custom error payload parsing errors
            }
        }) as EventListener);
        // Backward compatibility for old bridge event name.
        source.addEventListener('error', ((event: MessageEvent) => {
            markActivity();
            try {
                const payload = JSON.parse(String((event as MessageEvent).data || '{}')) as { message?: string };
                if (isUnimplementedStreamMessage(payload.message || '')) {
                    disableStreamUntil = Date.now() + (5 * 60 * 1000);
                    onError?.('stream de eventos não implementado no backend; modo degradado');
                    try { source?.close(); } catch { }
                    source = null;
                    clearWatchdog();
                }
            } catch {
                // ignore custom error payload parsing errors
            }
        }) as EventListener);
        source.onerror = () => {
            if (stopped) return;
            try { source?.close(); } catch { }
            source = null;
            clearWatchdog();
            onError?.('swap events stream disconnected');
            if (Date.now() < disableStreamUntil) {
                return;
            }
            if (reconnectTimer == null) {
                const jitter = Math.floor(Math.random() * 250);
                reconnectTimer = window.setTimeout(() => {
                    reconnectTimer = null;
                    connect();
                }, reconnectDelayMs + jitter);
                reconnectDelayMs = Math.min(reconnectDelayMs * 2, 30000);
            }
        };
    };

    connect();

    return () => {
        stopped = true;
        if (source) {
            try { source.close(); } catch { }
            source = null;
        }
        clearWatchdog();
        if (reconnectTimer != null) {
            window.clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
    };
}

export async function createSwap(request: CreateSwapRequest): Promise<Swap> {
    if (canUseIpc()) {
        return invokeIpc<Swap>('swap.create', request);
    }
    ensureDebugHttpEnabled('swap.create');
    return requestJson<Swap>(`${API_BASE}/swaps`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
    });
}

export async function checkSwap(id: string): Promise<Swap> {
    if (canUseIpc()) {
        const data = await invokeIpc<{ swap?: Swap } | Swap>('swap.check', { id });
        return (data as { swap?: Swap })?.swap || (data as Swap);
    }
    ensureDebugHttpEnabled('swap.check');
    const data = await requestJson<{ swap?: Swap } | Swap>(`${API_BASE}/swaps/${id}/check`, {
        method: 'POST',
    });
    return (data as { swap?: Swap })?.swap || (data as Swap);
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
    rpc_port?: number;
}

export interface FeeEstimates {
    fast: number;
    medium: number;
    slow: number;
}

export async function getBitcoinInfo(): Promise<BitcoinInfo> {
    return requestJson<BitcoinInfo>(`${API_BASE}/bitcoin/info`);
}

export async function getFeeEstimates(): Promise<FeeEstimates> {
    return requestJson<FeeEstimates>(`${API_BASE}/bitcoin/fees`);
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
    return requestJson<SystemHealth>(`${API_BASE}/system/health`);
}

export interface NodeInfo {
    node_type: 'bitcoind' | 'elementsd' | 'lnd' | string;
    state: 'running' | 'stopped' | 'starting' | 'stopping' | 'syncing' | 'error' | 'unknown' | string;
    version?: string;
    peer_count?: number;
    uptime_seconds?: number;
    error_message?: string;
    blocks?: number;
}

export async function listNodes(): Promise<NodeInfo[]> {
    if (canUseIpc()) {
        const data = await invokeIpc<{ nodes?: NodeInfo[] } | NodeInfo[]>('nodes.list');
        return Array.isArray(data) ? data : (data.nodes || []);
    }
    ensureDebugHttpEnabled('nodes.list');
    const data = await requestJson<{ nodes?: NodeInfo[] }>(`${API_BASE}/nodes`);
    return data.nodes || [];
}

export async function startNode(nodeType: 'bitcoind' | 'elementsd' | 'lnd'): Promise<{ success: boolean; node?: NodeInfo }> {
    if (canUseIpc()) {
        return invokeIpc<{ success: boolean; node?: NodeInfo }>('nodes.start', { nodeType });
    }
    ensureDebugHttpEnabled('nodes.start');
    return requestJson<{ success: boolean; node?: NodeInfo }>(`${API_BASE}/nodes/${encodeURIComponent(nodeType)}/start`, { method: 'POST' });
}

export async function stopNode(nodeType: 'bitcoind' | 'elementsd' | 'lnd', graceful = true): Promise<{ success: boolean; node?: NodeInfo }> {
    if (canUseIpc()) {
        return invokeIpc<{ success: boolean; node?: NodeInfo }>('nodes.stop', { nodeType, graceful });
    }
    ensureDebugHttpEnabled('nodes.stop');
    return requestJson<{ success: boolean; node?: NodeInfo }>(`${API_BASE}/nodes/${encodeURIComponent(nodeType)}/stop`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ graceful }),
    });
}

export async function restartNode(nodeType: 'bitcoind' | 'elementsd' | 'lnd'): Promise<{ success: boolean; node?: NodeInfo }> {
    if (canUseIpc()) {
        return invokeIpc<{ success: boolean; node?: NodeInfo }>('nodes.restart', { nodeType });
    }
    ensureDebugHttpEnabled('nodes.restart');
    return requestJson<{ success: boolean; node?: NodeInfo }>(`${API_BASE}/nodes/${encodeURIComponent(nodeType)}/restart`, { method: 'POST' });
}
