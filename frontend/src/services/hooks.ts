// XS Wallet React Hooks
// Wraps API calls with loading/error states

import { useState, useEffect, useCallback } from 'react';
import * as api from './api';

// ============================================================================
// VAULT HOOKS
// ============================================================================

export function useVaultStatus() {
    const [status, setStatus] = useState<api.VaultStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const refetch = useCallback(async () => {
        setLoading(true);
        try {
            const data = await api.getVaultStatus();
            setStatus(data);
            setError(null);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refetch();
    }, [refetch]);

    return { status, loading, error, refetch };
}

export function useInitVault() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [result, setResult] = useState<api.InitVaultResponse | null>(null);

    const init = async (pin: string) => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.initVault({ action: 'generate', pin });
            setResult(data);
            return data;
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { init, loading, error, result };
}

export function useUnlockVault() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const unlock = async (pin: string) => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.unlockVault(pin);
            if (!data.success) {
                setError(data.error_message || 'Invalid PIN');
                return data;
            }
            return data;
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { unlock, loading, error };
}

// ============================================================================
// WALLET HOOKS
// ============================================================================

export function useBalances() {
    const [balances, setBalances] = useState<api.Balances | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const refetch = useCallback(async () => {
        setLoading(true);
        try {
            const data = await api.getBalances();
            setBalances(data);
            setError(null);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refetch();
    }, [refetch]);

    return { balances, loading, error, refetch };
}

export function useNewAddress() {
    const [loading, setLoading] = useState(false);
    const [address, setAddress] = useState<api.AddressResponse | null>(null);

    const generate = async (chain: 'btc' | 'liquid') => {
        setLoading(true);
        try {
            const data = await api.getNewAddress(chain);
            setAddress(data);
            return data;
        } finally {
            setLoading(false);
        }
    };

    return { generate, loading, address };
}

export function useSendOnchain() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [txid, setTxid] = useState<string | null>(null);

    const send = async (request: api.SendOnchainRequest) => {
        setLoading(true);
        setError(null);
        try {
            const resp = await api.sendOnchain(request);
            if (!resp.success) {
                setError('Send failed');
                return resp;
            }
            setTxid(resp.txid || null);
            return resp;
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { send, loading, error, txid };
}

// ============================================================================
// SWAP HOOKS
// ============================================================================

export function useSwaps() {
    const [swaps, setSwaps] = useState<api.Swap[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const refetch = useCallback(async () => {
        setLoading(true);
        try {
            const data = await api.listSwaps();
            setSwaps(data);
            setError(null);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refetch();
    }, [refetch]);

    return { swaps, loading, error, refetch };
}

export function useCreateSwap() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const create = async (request: { from_chain: string; to_chain: string; amount_sats: number }) => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.createSwap(request);
            return data;
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { create, loading, error };
}

// ============================================================================
// BITCOIN HOOKS
// ============================================================================

export function useBitcoinInfo() {
    const [info, setInfo] = useState<api.BitcoinInfo | null>(null);
    const [loading, setLoading] = useState(true);

    const refetch = useCallback(async () => {
        try {
            const data = await api.getBitcoinInfo();
            setInfo(data);
        } catch (e) {
            // Silently fail for now
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refetch();
        const interval = setInterval(refetch, 30000); // Poll every 30s
        return () => clearInterval(interval);
    }, [refetch]);

    return { info, loading, refetch };
}

export function useFeeEstimates() {
    const [fees, setFees] = useState<api.FeeEstimates | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        api.getFeeEstimates().then(setFees).catch(() => { }).finally(() => setLoading(false));
    }, []);

    return { fees, loading };
}

// ============================================================================
// SYSTEM HOOKS
// ============================================================================

export function useSystemHealth() {
    const [health, setHealth] = useState<api.SystemHealth | null>(null);
    const [loading, setLoading] = useState(true);

    const refetch = useCallback(async () => {
        try {
            const data = await api.getSystemHealth();
            setHealth(data);
        } catch (e) {
            // Silently fail
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refetch();
        const interval = setInterval(refetch, 10000); // Poll every 10s
        return () => clearInterval(interval);
    }, [refetch]);

    return { health, loading, refetch };
}
