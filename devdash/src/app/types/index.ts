// BRLN-OS DevDash Types

export type NetworkEnv = 'mainnet' | 'testnet' | 'regtest';

export type NodeStatus = 'ok' | 'warn' | 'error';

export interface StatusMetric {
  label: string;
  value: string | number;
  status: NodeStatus;
  subtitle?: string;
}

export interface SwapStatus {
  id: string;
  from: 'BTC' | 'LN' | 'L-BTC';
  to: 'BTC' | 'LN' | 'L-BTC';
  amount: number;
  state: 'pending_funding' | 'funded' | 'htlc_created' | 'claimed' | 'completed' | 'expired' | 'refunded';
  created: string;
  txid?: string;
}

export interface TimelineEvent {
  id: string;
  timestamp: string;
  category: 'wallet' | 'htlc' | 'swap' | 'network' | 'api';
  severity: 'info' | 'warn' | 'error';
  message: string;
  details?: any;
}

export interface HTLCParams {
  hashlock: string;
  timelock: number;
  senderPubkey: string;
  receiverPubkey: string;
  amount: number;
}
