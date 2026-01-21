// XS Wallet State Store (Zustand)
// Global state for vault and app

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import * as api from './api';

// ============================================================================
// VAULT STORE
// ============================================================================

interface VaultState {
    status: 'not_initialized' | 'locked' | 'unlocked' | 'locked_out' | 'loading';
    sessionId: string | null;
    mnemonic: string | null; // Temporarily stored during onboarding

    // Actions
    checkStatus: () => Promise<void>;
    initVault: (pin: string) => Promise<api.InitVaultResponse>;
    unlock: (pin: string) => Promise<boolean>;
    lock: () => Promise<void>;
    clearMnemonic: () => void;
}

export const useVaultStore = create<VaultState>()((set, get) => ({
    status: 'loading',
    sessionId: null,
    mnemonic: null,

    checkStatus: async () => {
        try {
            const data = await api.getVaultStatus();
            set({ status: data.state });
        } catch (e) {
            // If API fails, assume locked
            set({ status: 'locked' });
        }
    },

    initVault: async (pin: string) => {
        const result = await api.initVault({ action: 'generate', pin });
        if (result.success) {
            set({
                status: 'unlocked',
                sessionId: result.session_id,
                mnemonic: result.mnemonic || null,
            });
        }
        return result;
    },

    unlock: async (pin: string) => {
        const result = await api.unlockVault(pin);
        if (result.success) {
            set({
                status: 'unlocked',
                sessionId: result.session_id || null,
            });
            return true;
        }
        return false;
    },

    lock: async () => {
        try {
            await api.lockVault();
        } catch (e) {
            // Ignore errors
        }
        set({ status: 'locked', sessionId: null });
    },

    clearMnemonic: () => {
        set({ mnemonic: null });
    },
}));

// ============================================================================
// APP STORE
// ============================================================================

interface AppState {
    currentPage: string;
    setPage: (page: string) => void;
}

export const useAppStore = create<AppState>()((set) => ({
    currentPage: 'home',
    setPage: (page) => set({ currentPage: page }),
}));

// ============================================================================
// SWAP STORE
// ============================================================================

interface SwapState {
    swaps: api.Swap[];
    loading: boolean;
    fetchSwaps: () => Promise<void>;
}

export const useSwapStore = create<SwapState>()((set) => ({
    swaps: [],
    loading: false,

    fetchSwaps: async () => {
        set({ loading: true });
        try {
            const swaps = await api.listSwaps();
            set({ swaps });
        } catch (e) {
            // Ignore
        } finally {
            set({ loading: false });
        }
    },
}));
