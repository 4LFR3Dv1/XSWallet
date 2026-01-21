// Mock Data for BRLN-OS DevDash

import { StatusMetric, SwapStatus, TimelineEvent } from '@/app/types';

export const mockNodeStatus: StatusMetric[] = [
  {
    label: 'LND Node',
    value: 'Synced',
    status: 'ok',
    subtitle: 'Height: 850,234 • 12 Peers'
  },
  {
    label: 'Bitcoin Core',
    value: 'Synced',
    status: 'ok',
    subtitle: 'Height: 850,234 • 8 Peers'
  },
  {
    label: 'Elements (Liquid)',
    value: 'Synced',
    status: 'ok',
    subtitle: 'Height: 3,234,567 • 6 Peers'
  },
  {
    label: 'API Service',
    value: 'Healthy',
    status: 'ok',
    subtitle: 'Uptime: 99.9% • 45ms avg'
  }
];

export const mockObservabilityMetrics = {
  latency: '45ms',
  errorRate: '0.02%',
  throughput: '234 req/min'
};

export const mockSwaps: SwapStatus[] = [
  {
    id: '#1234',
    from: 'BTC',
    to: 'LN',
    amount: 0.0025,
    state: 'pending_funding',
    created: '2025-01-15T14:30:00Z'
  },
  {
    id: '#1233',
    from: 'LN',
    to: 'L-BTC',
    amount: 0.001,
    state: 'htlc_created',
    created: '2025-01-15T14:15:00Z',
    txid: 'abc123...def456'
  },
  {
    id: '#1232',
    from: 'BTC',
    to: 'L-BTC',
    amount: 0.005,
    state: 'completed',
    created: '2025-01-15T13:45:00Z',
    txid: '789xyz...012abc'
  }
];

export const mockTimelineEvents: TimelineEvent[] = [
  {
    id: '1',
    timestamp: '2025-01-15T14:35:22Z',
    category: 'swap',
    severity: 'info',
    message: 'Swap #1234 created: BTC → LN (0.0025 BTC)',
    details: { swapId: '#1234' }
  },
  {
    id: '2',
    timestamp: '2025-01-15T14:30:15Z',
    category: 'htlc',
    severity: 'info',
    message: 'HTLC funded successfully',
    details: { htlcId: 'htlc_xyz789' }
  },
  {
    id: '3',
    timestamp: '2025-01-15T14:20:08Z',
    category: 'wallet',
    severity: 'info',
    message: 'New HD wallet generated (24 words)',
    details: { path: "m/44'/0'/0'" }
  },
  {
    id: '4',
    timestamp: '2025-01-15T14:15:42Z',
    category: 'network',
    severity: 'warn',
    message: 'Mempool congestion detected (15 sat/vB)',
    details: { feeRate: 15 }
  },
  {
    id: '5',
    timestamp: '2025-01-15T14:10:30Z',
    category: 'api',
    severity: 'info',
    message: 'POST /api/htlc/create • 201 • 32ms',
    details: { endpoint: '/api/htlc/create', status: 201, latency: 32 }
  }
];

export const mockMnemonicWords = [
  'abandon', 'ability', 'able', 'about', 'above', 'absent',
  'absorb', 'abstract', 'absurd', 'abuse', 'access', 'accident',
  'account', 'accuse', 'achieve', 'acid', 'acoustic', 'acquire',
  'across', 'act', 'action', 'actor', 'actress', 'actual'
];

export const mockHTLCScript = `OP_IF
  OP_HASH160
  <hash160(preimage)>
  OP_EQUALVERIFY
  <receiver_pubkey>
OP_ELSE
  <timelock_blocks>
  OP_CHECKSEQUENCEVERIFY
  OP_DROP
  <sender_pubkey>
OP_ENDIF
OP_CHECKSIG`;

export const mockAPIEndpoints = [
  {
    category: 'Wallet',
    endpoints: [
      { method: 'POST', path: '/api/wallet/generate', description: 'Generate new HD wallet' },
      { method: 'POST', path: '/api/wallet/derive', description: 'Derive address from path' },
      { method: 'POST', path: '/api/wallet/encrypt', description: 'Encrypt seed/private key' },
      { method: 'POST', path: '/api/wallet/validate', description: 'Validate mnemonic' }
    ]
  },
  {
    category: 'HTLC',
    endpoints: [
      { method: 'POST', path: '/api/htlc/create', description: 'Create new HTLC script' },
      { method: 'POST', path: '/api/htlc/decode', description: 'Decode HTLC script' },
      { method: 'GET', path: '/api/htlc/{id}', description: 'Get HTLC details' },
      { method: 'POST', path: '/api/htlc/{id}/fund', description: 'Fund HTLC' },
      { method: 'POST', path: '/api/htlc/{id}/claim', description: 'Claim HTLC with preimage' }
    ]
  },
  {
    category: 'Preimage',
    endpoints: [
      { method: 'POST', path: '/api/preimage/generate', description: 'Generate random preimage' },
      { method: 'POST', path: '/api/preimage/verify', description: 'Verify preimage matches hash' }
    ]
  },
  {
    category: 'System',
    endpoints: [
      { method: 'GET', path: '/api/system/status', description: 'System health status' },
      { method: 'GET', path: '/api/system/health', description: 'Health check endpoint' }
    ]
  }
];
