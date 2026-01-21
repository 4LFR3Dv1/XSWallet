import React, { useState, useEffect } from 'react';
import { ChevronDown, Clock, Info, Loader2 } from 'lucide-react';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { TerminalCard } from '../TerminalCard';
import { StatusChip } from '../StatusChip';
import { PrimaryButton, SecondaryButton, DestructiveButton } from '../PrimaryButton';
import { useSwaps, useCreateSwap } from '@/services/hooks';
import type { Swap } from '@/services/api';

// Map xscore states to UI display
const stateConfig: Record<string, { label: string; variant: 'success' | 'warning' | 'error' | 'pending' | 'btc' | 'liquid' | 'ln' | 'green' | 'default' }> = {
  'open': { label: 'Open', variant: 'pending' },
  'locked': { label: 'Locked', variant: 'warning' },
  'commit_started': { label: 'Broadcasting', variant: 'btc' },
  'waiting': { label: 'Processing', variant: 'liquid' },
  'waiting_claim_details': { label: 'Claiming', variant: 'green' },
  'signing_musig2_partial': { label: 'Signing', variant: 'green' },
  'sent_partial_to_provider': { label: 'Finalizing', variant: 'green' },
  'waiting_provider_broadcast': { label: 'Confirming', variant: 'green' },
  'completed': { label: 'Complete', variant: 'success' },
  'failed': { label: 'Failed', variant: 'error' },
  'canceled': { label: 'Canceled', variant: 'error' },
};

export function SwapCenter() {
  const [fromAsset, setFromAsset] = useState('btc');
  const [toAsset, setToAsset] = useState('liquid');
  const [amount, setAmount] = useState('100000');
  const { swaps, loading: swapsLoading, refetch } = useSwaps();
  const { create, loading: creating, error } = useCreateSwap();

  // Auto-refresh swaps every 10 seconds
  useEffect(() => {
    const interval = setInterval(refetch, 10000);
    return () => clearInterval(interval);
  }, [refetch]);

  const handleCreateSwap = async () => {
    try {
      await create({
        from_chain: fromAsset,
        to_chain: toAsset,
        amount_sats: parseInt(amount, 10),
      });
      refetch();
    } catch (e) {
      // Error is handled in hook
    }
  };

  const activeSwaps = swaps.filter(s =>
    !['completed', 'failed', 'canceled'].includes(s.state.toLowerCase())
  );

  const historySwaps = swaps.filter(s =>
    ['completed', 'failed', 'canceled'].includes(s.state.toLowerCase())
  );

  return (
    <AppShell activePage="swap" vaultLocked={false}>
      <div className="p-8">
        <PageHeader
          title="Atomic Swap"
          subtitle="Cross-chain asset exchange"
        />

        {/* Swap Wizard */}
        <div className="mb-8">
          <TerminalCard>
            <div className="grid grid-cols-2 gap-6 mb-6">
              {/* From */}
              <div>
                <label className="text-sm text-[#9AA7B5] mb-2 block">From</label>
                <div className="relative">
                  <select
                    value={fromAsset}
                    onChange={(e) => setFromAsset(e.target.value)}
                    className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] appearance-none cursor-pointer focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  >
                    <option value="btc">Bitcoin (BTC)</option>
                    <option value="liquid">Liquid BTC</option>
                    <option value="ln">Lightning</option>
                  </select>
                  <ChevronDown size={16} className="absolute right-4 top-1/2 -translate-y-1/2 text-[#9AA7B5] pointer-events-none" />
                </div>
                <input
                  type="text"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  className="mt-3 w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  placeholder="Amount in sats"
                />
              </div>

              {/* To */}
              <div>
                <label className="text-sm text-[#9AA7B5] mb-2 block">To</label>
                <div className="relative">
                  <select
                    value={toAsset}
                    onChange={(e) => setToAsset(e.target.value)}
                    className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] appearance-none cursor-pointer focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  >
                    <option value="liquid">Liquid BTC</option>
                    <option value="btc">Bitcoin (BTC)</option>
                    <option value="ln">Lightning</option>
                  </select>
                  <ChevronDown size={16} className="absolute right-4 top-1/2 -translate-y-1/2 text-[#9AA7B5] pointer-events-none" />
                </div>
                <div className="mt-3 px-4 py-3 bg-[#151B23]/50 border border-[#242C36] rounded-xl text-[#9AA7B5] font-mono">
                  ≈ {(parseInt(amount || '0', 10) * 0.995).toLocaleString()} sats
                </div>
              </div>
            </div>

            {error && (
              <div className="mb-4 p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl text-sm text-[#EF4444]">
                {error}
              </div>
            )}

            {/* Actions */}
            <div className="flex items-center gap-3">
              <SecondaryButton className="flex-1">Cancel</SecondaryButton>
              <PrimaryButton className="flex-1" onClick={handleCreateSwap} disabled={creating}>
                {creating ? (
                  <div className="flex items-center gap-2">
                    <Loader2 size={16} className="animate-spin" />
                    <span>Creating...</span>
                  </div>
                ) : (
                  'Create Swap'
                )}
              </PrimaryButton>
            </div>
          </TerminalCard>
        </div>

        {/* Active Swaps */}
        {activeSwaps.length > 0 && (
          <div className="mb-8">
            <TerminalCard
              header={
                <div className="flex items-center justify-between">
                  <h3 className="text-lg text-[#E7EDF5]">Active Swaps</h3>
                  <span className="text-sm text-[#6C7A89]">{activeSwaps.length} active</span>
                </div>
              }
            >
              <div className="space-y-4">
                {activeSwaps.map((swap) => {
                  const config = stateConfig[swap.state.toLowerCase()] || { label: swap.state, variant: 'pending' as const };
                  return (
                    <div
                      key={swap.id}
                      className="p-4 bg-[#151B23] border border-[#242C36] rounded-xl"
                    >
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <span className="text-sm text-[#E7EDF5] font-mono">{swap.from_chain}</span>
                          <span className="text-[#6C7A89]">→</span>
                          <span className="text-sm text-[#E7EDF5] font-mono">{swap.to_chain}</span>
                        </div>
                        <StatusChip label={config.label} variant={config.variant} />
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-sm text-[#9AA7B5]">{swap.amount_sats.toLocaleString()} sats</span>
                        <span className="text-xs text-[#6C7A89] font-mono">{swap.id.slice(0, 12)}...</span>
                      </div>
                      {swap.htlc_address && (
                        <div className="mt-2 p-2 bg-[#0B0D10] rounded text-xs text-[#6C7A89] font-mono break-all">
                          HTLC: {swap.htlc_address}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </TerminalCard>
          </div>
        )}

        {/* History */}
        <TerminalCard
          header={
            <div className="flex items-center justify-between">
              <h3 className="text-lg text-[#E7EDF5]">Swap History</h3>
              <span className="text-sm text-[#6C7A89]">{historySwaps.length} total</span>
            </div>
          }
        >
          {swapsLoading ? (
            <div className="text-center py-8 text-[#6C7A89]">Loading...</div>
          ) : historySwaps.length === 0 ? (
            <div className="text-center py-8 text-[#6C7A89]">No completed swaps yet</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="text-left text-sm text-[#6C7A89] border-b border-[#242C36]">
                    <th className="pb-3 font-medium">Swap ID</th>
                    <th className="pb-3 font-medium">From</th>
                    <th className="pb-3 font-medium">To</th>
                    <th className="pb-3 font-medium">Amount</th>
                    <th className="pb-3 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody className="text-sm">
                  {historySwaps.map((swap) => {
                    const config = stateConfig[swap.state.toLowerCase()] || { label: swap.state, variant: 'pending' as const };
                    return (
                      <tr key={swap.id} className="border-b border-[#242C36] last:border-0 hover:bg-[#151B23]/50 transition-colors">
                        <td className="py-3 font-mono text-[#9AA7B5]">{swap.id.slice(0, 12)}...</td>
                        <td className="py-3 text-[#E7EDF5]">{swap.from_chain}</td>
                        <td className="py-3 text-[#E7EDF5]">{swap.to_chain}</td>
                        <td className="py-3 font-mono text-[#E7EDF5]">{swap.amount_sats.toLocaleString()}</td>
                        <td className="py-3">
                          <StatusChip label={config.label} variant={config.variant} />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </TerminalCard>
      </div>
    </AppShell>
  );
}
