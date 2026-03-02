// XS Wallet React Hooks
// Wraps API calls with loading/error states

import { useState, useEffect, useCallback, useRef } from 'react';
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
    const [error, setError] = useState<string | null>(null);

    const generate = async (chain: 'btc' | 'liquid') => {
        setLoading(true);
        setError(null);
        try {
            const data = await api.getNewAddress(chain);
            setAddress(data);
            return data;
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { generate, loading, address, error };
}

export function useAddressBook(chain: 'btc' | 'liquid') {
    const [addresses, setAddresses] = useState<api.AddressInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const requestSeqRef = useRef(0);

    const refetch = useCallback(async () => {
        const requestSeq = ++requestSeqRef.current;
        setLoading(true);
        try {
            const data = await api.listAddresses(chain, true);
            if (requestSeq !== requestSeqRef.current) return;
            setAddresses(data);
            setError(null);
        } catch (e) {
            if (requestSeq !== requestSeqRef.current) return;
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            if (requestSeq !== requestSeqRef.current) return;
            setLoading(false);
        }
    }, [chain]);

    useEffect(() => {
        // Clear previous chain state immediately to avoid cross-chain bleed while loading.
        setAddresses([]);
        setError(null);
    }, [chain]);

    useEffect(() => {
        void refetch();
    }, [refetch]);

    return { addresses, loading, error, refetch };
}

export function useWalletUtxos(chain: 'btc' | 'liquid') {
    const [utxos, setUtxos] = useState<api.WalletUtxo[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const requestSeqRef = useRef(0);

    const refetch = useCallback(async () => {
        const requestSeq = ++requestSeqRef.current;
        setLoading(true);
        try {
            const data = await api.listUtxos(chain, true);
            if (requestSeq !== requestSeqRef.current) return;
            setUtxos(data);
            setError(null);
        } catch (e) {
            if (requestSeq !== requestSeqRef.current) return;
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            if (requestSeq !== requestSeqRef.current) return;
            setLoading(false);
        }
    }, [chain]);

    useEffect(() => {
        setUtxos([]);
        setError(null);
    }, [chain]);

    useEffect(() => {
        void refetch();
    }, [refetch]);

    return { utxos, loading, error, refetch };
}

export function useWalletTransactions(chain: 'btc' | 'liquid') {
    const [transactions, setTransactions] = useState<api.WalletTransaction[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const requestSeqRef = useRef(0);

    const refetch = useCallback(async () => {
        const requestSeq = ++requestSeqRef.current;
        setLoading(true);
        try {
            const data = await api.listTransactions(chain, 100, 0);
            if (requestSeq !== requestSeqRef.current) return;
            setTransactions(data);
            setError(null);
        } catch (e) {
            if (requestSeq !== requestSeqRef.current) return;
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            if (requestSeq !== requestSeqRef.current) return;
            setLoading(false);
        }
    }, [chain]);

    useEffect(() => {
        setTransactions([]);
        setError(null);
    }, [chain]);

    useEffect(() => {
        void refetch();
    }, [refetch]);

    return { transactions, loading, error, refetch };
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
        let unsubscribe: (() => void) | undefined;
        setLoading(true);
        api.watchAllSwaps((nextSwaps) => {
            setSwaps(nextSwaps);
            setError(null);
            setLoading(false);
        }).then((stop) => {
            unsubscribe = stop;
        }).catch((e) => {
            setError(e instanceof Error ? e.message : 'Unknown error');
            void refetch();
        });

        return () => {
            if (unsubscribe) {
                unsubscribe();
            }
        };
    }, [refetch]);

    return { swaps, loading, error, refetch };
}

export function useCreateSwap() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const create = async (request: api.CreateSwapRequest) => {
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

export function useCheckSwap() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const check = async (id: string) => {
        setLoading(true);
        setError(null);
        try {
            return await api.checkSwap(id);
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { check, loading, error };
}

export function useSwapEvents(swapId: string | null) {
    const [events, setEvents] = useState<api.SwapEvent[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const lastSeqRef = useRef(0);

    const refetch = useCallback(async () => {
        if (!swapId) {
            setEvents([]);
            return;
        }
        setLoading(true);
        try {
            const data = await api.getSwapEvents(swapId, 0);
            setEvents(data);
            lastSeqRef.current = data.length > 0 ? data[data.length - 1].seq : 0;
            setError(null);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Unknown error');
        } finally {
            setLoading(false);
        }
    }, [swapId]);

    useEffect(() => {
        void refetch();
    }, [refetch]);

    useEffect(() => {
        if (!swapId) return;
        let disposed = false;
        let stopStream: (() => void) | null = null;
        let fallbackPoll: number | null = null;

        const stopPollingFallback = () => {
            if (fallbackPoll != null) {
                window.clearInterval(fallbackPoll);
                fallbackPoll = null;
            }
        };

        const upsert = (incoming: api.SwapEvent) => {
            setEvents((prev) => {
                if (prev.some((e) => e.seq === incoming.seq)) return prev;
                const merged = [...prev, incoming];
                merged.sort((a, b) => a.seq - b.seq);
                lastSeqRef.current = merged.length > 0 ? merged[merged.length - 1].seq : 0;
                return merged;
            });
            setError(null);
            stopPollingFallback();
        };

        const startPollingFallback = () => {
            if (fallbackPoll != null) return;
            fallbackPoll = window.setInterval(async () => {
                if (disposed || !swapId) return;
                try {
                    const delta = await api.getSwapEvents(swapId, lastSeqRef.current);
                    if (delta.length > 0) {
                        setEvents((prev) => {
                            const seen = new Set(prev.map((e) => e.seq));
                            const next = [...prev, ...delta.filter((e) => !seen.has(e.seq))];
                            next.sort((a, b) => a.seq - b.seq);
                            lastSeqRef.current = next.length > 0 ? next[next.length - 1].seq : 0;
                            return next;
                        });
                    }
                } catch {
                    // Keep trying while stream is unavailable.
                }
            }, 10000);
        };

        const startStream = async () => {
            try {
                const snapshot = await api.getSwapEvents(swapId, 0);
                if (!disposed) {
                    setEvents(snapshot);
                    lastSeqRef.current = snapshot.length > 0 ? snapshot[snapshot.length - 1].seq : 0;
                    setError(null);
                }
                stopStream = api.watchSwapEvents(
                    swapId,
                    lastSeqRef.current,
                    upsert,
                    (message) => {
                        if (disposed) return;
                        setError(message);
                        startPollingFallback();
                    },
                );
            } catch (e) {
                if (!disposed) {
                    setError(e instanceof Error ? e.message : 'Unknown error');
                    startPollingFallback();
                }
            }
        };

        void startStream();

        return () => {
            disposed = true;
            if (stopStream) stopStream();
            stopPollingFallback();
        };
    }, [swapId]);

    return { events, loading, error, refetch };
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

export function useNodes() {
    const [nodes, setNodes] = useState<api.NodeInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const refetch = useCallback(async () => {
        setLoading(true);
        try {
            const data = await api.listNodes();
            setNodes(data);
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

    return { nodes, loading, error, refetch };
}

export function useNodeActions() {
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const start = async (nodeType: 'bitcoind' | 'elementsd' | 'lnd') => {
        setLoading(true);
        setError(null);
        try {
            return await api.startNode(nodeType);
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    const stop = async (nodeType: 'bitcoind' | 'elementsd' | 'lnd', graceful = true) => {
        setLoading(true);
        setError(null);
        try {
            return await api.stopNode(nodeType, graceful);
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    const restart = async (nodeType: 'bitcoind' | 'elementsd' | 'lnd') => {
        setLoading(true);
        setError(null);
        try {
            return await api.restartNode(nodeType);
        } catch (e) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setError(msg);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    return { start, stop, restart, loading, error };
}
