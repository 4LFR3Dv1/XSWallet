// React Hooks for BRLN-OS API
// Uses real data from Flask backend - no mocks

import { useState, useEffect, useCallback } from 'react';
import * as api from './api';

// Generic hook for API calls
function useApiCall<T>(
    apiFunction: () => Promise<T>,
    dependencies: any[] = []
) {
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const refetch = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const result = await apiFunction();
            setData(result);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Unknown error');
        } finally {
            setLoading(false);
        }
    }, dependencies);

    useEffect(() => {
        refetch();
    }, [refetch]);

    return { data, loading, error, refetch };
}

// ============ SYSTEM HOOKS ============

export function useSystemStatus() {
    return useApiCall(() => api.getSystemStatus(), []);
}

export function useSystemHealth() {
    return useApiCall(() => api.getSystemHealth(), []);
}

export function useAllNodeStatuses() {
    return useApiCall(() => api.getAllNodeStatuses(), []);
}

export function useRecentEvents() {
    return useApiCall(() => api.getRecentEvents(), []);
}

export function useSystemMetrics() {
    return useApiCall(() => api.getSystemMetrics(), []);
}


// ============ WALLET HOOKS ============

export function useGenerateWallet(wordCount: 12 | 24 = 24) {
    const [result, setResult] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const generate = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.generateWallet(wordCount);
            setResult(data);
            return data;
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'Failed to generate wallet';
            setError(msg);
            throw err;
        } finally {
            setLoading(false);
        }
    }, [wordCount]);

    return { result, loading, error, generate };
}

export function useDeriveAddresses() {
    const [result, setResult] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const derive = useCallback(async (seedPhrase: string, passphrase?: string) => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.deriveAddresses(seedPhrase, passphrase);
            setResult(data);
            return data;
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'Failed to derive addresses';
            setError(msg);
            throw err;
        } finally {
            setLoading(false);
        }
    }, []);

    return { result, loading, error, derive };
}

// ============ HTLC HOOKS ============

export function useGeneratePreimage() {
    const [result, setResult] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const generate = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.generatePreimage();
            setResult(data);
            return data;
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'Failed to generate preimage';
            setError(msg);
            throw err;
        } finally {
            setLoading(false);
        }
    }, []);

    return { result, loading, error, generate };
}

export function useCreateHTLC() {
    const [result, setResult] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const create = useCallback(async (params: {
        amount_sats: number;
        timeout_blocks: number;
        receiver_pubkey: string;
        sender_pubkey: string;
        network?: 'mainnet' | 'testnet' | 'regtest';
    }) => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.createHTLC(params);
            setResult(data);
            return data;
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'Failed to create HTLC';
            setError(msg);
            throw err;
        } finally {
            setLoading(false);
        }
    }, []);

    return { result, loading, error, create };
}

// ============ LIGHTNING HOOKS ============

export function useLightningInfo() {
    return useApiCall(() => api.getLightningInfo(), []);
}

export function useLightningBalance() {
    return useApiCall(() => api.getLightningBalance(), []);
}

export function useLightningChannels() {
    return useApiCall(() => api.getLightningChannels(), []);
}

// ============ BITCOIN HOOKS ============

export function useBitcoinInfo() {
    return useApiCall(() => api.getBitcoinInfo(), []);
}

export function useBitcoinFees() {
    return useApiCall(() => api.getBitcoinFees(), []);
}

export function useBitcoinMempool() {
    return useApiCall(() => api.getBitcoinMempool(), []);
}

// ============ SWAP HOOKS ============

export function useListSwaps() {
    return useApiCall(() => api.listSwaps(), []);
}

export function useCreateSwap() {
    const [result, setResult] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const create = useCallback(async (params: {
        from_chain: string;
        to_chain: string;
        amount_sats: number;
    }) => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.createSwap(params);
            setResult(data);
            return data;
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'Failed to create swap';
            setError(msg);
            throw err;
        } finally {
            setLoading(false);
        }
    }, []);

    return { result, loading, error, create };
}


// ============ ELEMENTS HOOKS ============

export function useElementsInfo() {
    return useApiCall(() => api.getElementsInfo(), []);
}

export function useElementsBalance() {
    return useApiCall(() => api.getElementsBalance(), []);
}
