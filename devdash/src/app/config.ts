// Configuration for DevDash

export const config = {
    // API base URL - proxied through Vite in dev, direct in production
    apiBaseUrl: import.meta.env.VITE_API_URL || '/api/v1',

    // Default network for testing
    defaultNetwork: (import.meta.env.VITE_DEFAULT_NETWORK || 'testnet') as 'mainnet' | 'testnet' | 'regtest',

    // Enable mock data (for development without backend)
    useMockData: import.meta.env.VITE_USE_MOCK === 'true' || false,
};

// Environment type for Vite
declare global {
    interface ImportMetaEnv {
        readonly VITE_API_URL: string;
        readonly VITE_DEFAULT_NETWORK: string;
        readonly VITE_USE_MOCK: string;
    }
}
