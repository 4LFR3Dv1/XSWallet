// BRLN-OS API Service Layer
// Connects DevDash to the Flask backend at /api/v1

const API_BASE = '/api/v1';

// Helper function for API calls
async function apiCall<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const response = await fetch(`${API_BASE}${endpoint}`, {
        headers: {
            'Content-Type': 'application/json',
            ...options?.headers,
        },
        ...options,
    });

    if (!response.ok) {
        throw new Error(`API Error: ${response.status} ${response.statusText}`);
    }

    return response.json();
}

// ============ SYSTEM ============

export async function getSystemStatus() {
    return apiCall('/system/status');
}

export async function getSystemHealth() {
    return apiCall('/system/health');
}

export async function getSystemMetrics() {
    return apiCall('/system/metrics');
}


// ============ WALLET ============

export async function generateWallet(wordCount: 12 | 24 = 24) {
    return apiCall('/wallet/generate', {
        method: 'POST',
        body: JSON.stringify({ word_count: wordCount }),
    });
}

export async function deriveAddresses(seedPhrase: string, passphrase?: string) {
    return apiCall('/wallet/derive', {
        method: 'POST',
        body: JSON.stringify({ seed_phrase: seedPhrase, passphrase }),
    });
}

export async function validateMnemonic(mnemonic: string) {
    return apiCall('/wallet/validate', {
        method: 'POST',
        body: JSON.stringify({ mnemonic }),
    });
}

export async function encryptWallet(seedPhrase: string, password: string) {
    return apiCall('/wallet/encrypt', {
        method: 'POST',
        body: JSON.stringify({ seed_phrase: seedPhrase, password }),
    });
}

// ============ HTLC (brln-swap-core) ============

export async function generatePreimage() {
    return apiCall('/preimage/generate', { method: 'POST' });
}

export async function verifyPreimage(preimage: string, paymentHash: string) {
    return apiCall('/preimage/verify', {
        method: 'POST',
        body: JSON.stringify({ preimage, payment_hash: paymentHash }),
    });
}

export async function createHTLC(params: {
    amount_sats: number;
    timeout_blocks: number;
    receiver_pubkey: string;
    sender_pubkey: string;
    network?: 'mainnet' | 'testnet' | 'regtest';
}) {
    return apiCall('/htlc/create', {
        method: 'POST',
        body: JSON.stringify(params),
    });
}

export async function decodeHTLCScript(scriptHex: string) {
    return apiCall('/htlc/decode', {
        method: 'POST',
        body: JSON.stringify({ script_hex: scriptHex }),
    });
}

// ============ LIGHTNING ============

export async function getLightningInfo() {
    return apiCall('/lightning/info');
}

export async function getLightningBalance() {
    return apiCall('/lightning/balance');
}

export async function getLightningChannels() {
    return apiCall('/lightning/channels');
}

export async function createInvoice(valueSats: number, memo?: string) {
    return apiCall('/lightning/invoices', {
        method: 'POST',
        body: JSON.stringify({ value: valueSats, memo }),
    });
}

// ============ BITCOIN ============

export async function getBitcoinInfo() {
    return apiCall('/bitcoin/info');
}

export async function getBitcoinFees() {
    return apiCall('/bitcoin/fees');
}

export async function getBitcoinMempool() {
    return apiCall('/bitcoin/mempool');
}

// ============ SWAPS ============

export async function listSwaps() {
    return apiCall('/swaps');
}

export async function createSwap(params: {
    from_chain: string;
    to_chain: string;
    amount_sats: number;
}) {
    return apiCall('/swaps', {
        method: 'POST',
        body: JSON.stringify(params),
    });
}

export async function getSwap(swapId: string) {
    return apiCall(`/swaps/${swapId}`);
}


// ============ ELEMENTS (LIQUID) ============

export async function getElementsInfo() {
    return apiCall('/elements/info');
}

export async function getElementsBalance() {
    return apiCall('/elements/balance');
}

// ============ CONVENIENCE ============

// Fetch all node statuses at once
export async function getAllNodeStatuses() {
    try {
        const status: any = await getSystemStatus();
        return {
            lnd: {
                synced: status.lnd?.synced || false,
                block_height: status.lnd?.block_height || 0,
                peers: status.lnd?.peers || 0,
                status: status.lnd?.status || 'unknown'
            },
            bitcoin: {
                synced: status.bitcoin?.synced || false,
                block_height: status.bitcoin?.block_height || 0,
                peers: status.bitcoin?.peers || 0,
                status: status.bitcoin?.status || 'unknown'
            },
            elements: {
                synced: status.elements?.synced || false,
                block_height: status.elements?.block_height || 0,
                peers: status.elements?.peers || 0,
                status: status.elements?.status || 'unknown'
            },
            apiHealthy: status.api?.status === 'ok',
            uptime: status.api?.uptime,
            latency_ms: status.api?.latency_ms
        };
    } catch (error) {
        throw new Error('Failed to fetch node statuses');
    }
}

// Get recent events
export async function getRecentEvents() {
    return apiCall('/events/recent');
}
