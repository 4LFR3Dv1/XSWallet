import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Lock, Repeat, Download, Send, Bitcoin, Zap, Droplet } from 'lucide-react';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { MetricCard } from '../MetricCard';
import { TerminalCard } from '../TerminalCard';
import { StatusChip } from '../StatusChip';
import { SecondaryButton } from '../PrimaryButton';
import { useBalances, useSwaps } from '@/services/hooks';
import { useVaultStore } from '@/services/store';

// State to variant mapping
const stateVariants: Record<string, 'btc' | 'liquid' | 'ln' | 'green' | 'error' | 'pending'> = {
  'open': 'pending',
  'locked': 'pending',
  'commit_started': 'btc',
  'waiting': 'liquid',
  'completed': 'green',
  'failed': 'error',
  'canceled': 'error',
};

export function Dashboard() {
  const navigate = useNavigate();
  const { lock } = useVaultStore();
  const { balances, loading: balancesLoading } = useBalances();
  const { swaps, loading: swapsLoading } = useSwaps();

  const handleLock = async () => {
    await lock();
    navigate('/unlock');
  };

  const formatSats = (sats: number) => {
    return (sats / 100_000_000).toFixed(8);
  };

  const activeSwaps = swaps.filter(s =>
    !['completed', 'failed', 'canceled'].includes(s.state.toLowerCase())
  );

  return (
    <AppShell activePage="home" vaultLocked={false}>
      <div className="p-8">
        <PageHeader
          title="Welcome"
          subtitle="Monitor your assets and active swaps"
          actions={
            <SecondaryButton onClick={handleLock}>
              <div className="flex items-center gap-2">
                <Lock size={18} />
                <span>Lock</span>
              </div>
            </SecondaryButton>
          }
        />

        {/* Metrics Grid */}
        <div className="grid grid-cols-3 gap-6 mb-8">
          <MetricCard
            icon={<Bitcoin size={24} />}
            asset="Bitcoin"
            balance={balancesLoading ? '...' : formatSats(balances?.btc?.confirmed || 0)}
            fiat={balances?.btc?.unconfirmed ? `+${formatSats(balances.btc.unconfirmed)} unconf` : 'On-chain'}
            delta=""
            deltaPositive={true}
            accentColor="#F7931A"
          />
          <MetricCard
            icon={<Droplet size={24} />}
            asset="Liquid"
            balance={balancesLoading ? '...' : formatSats(balances?.liquid?.confirmed || 0)}
            fiat={balances?.liquid?.unconfirmed ? `+${formatSats(balances.liquid.unconfirmed)} unconf` : 'L-BTC'}
            delta=""
            deltaPositive={true}
            accentColor="#00B4D8"
          />
          <MetricCard
            icon={<Zap size={24} />}
            asset="Lightning"
            balance={balancesLoading ? '...' : (balances?.ln?.balance || 0).toLocaleString()}
            fiat="sats"
            delta=""
            deltaPositive={true}
            accentColor="#FFD700"
          />
        </div>

        <div className="grid grid-cols-2 gap-6">
          {/* Active Swaps */}
          <TerminalCard
            header={
              <div className="flex items-center justify-between">
                <h3 className="text-lg text-[#E7EDF5]">Active Swaps</h3>
                <span className="text-sm text-[#6C7A89]">
                  {swapsLoading ? '...' : `${activeSwaps.length} active`}
                </span>
              </div>
            }
          >
            <div className="space-y-4">
              {activeSwaps.length === 0 && !swapsLoading && (
                <div className="text-center py-8 text-[#6C7A89]">
                  No active swaps
                </div>
              )}
              {activeSwaps.map((swap) => (
                <div
                  key={swap.id}
                  onClick={() => navigate('/swap')}
                  className="p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:border-[#242C36]/60 transition-all cursor-pointer"
                >
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                      <span className="text-sm text-[#E7EDF5] font-mono">{swap.from_chain}</span>
                      <span className="text-[#6C7A89]">→</span>
                      <span className="text-sm text-[#E7EDF5] font-mono">{swap.to_chain}</span>
                    </div>
                    <StatusChip
                      label={swap.state.replace(/_/g, ' ')}
                      variant={stateVariants[swap.state.toLowerCase()] || 'pending'}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-[#9AA7B5]">{swap.amount_sats.toLocaleString()} sats</span>
                    <span className="text-xs text-[#6C7A89] font-mono">{swap.id.slice(0, 8)}...</span>
                  </div>
                </div>
              ))}
            </div>
          </TerminalCard>

          {/* Quick Actions */}
          <TerminalCard
            header={<h3 className="text-lg text-[#E7EDF5]">Quick Actions</h3>}
          >
            <div className="space-y-3">
              <button
                onClick={() => navigate('/swap')}
                className="w-full p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:bg-[#151B23]/80 hover:border-[#E7EDF5]/20 transition-all text-left group"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-[#E7EDF5]/10 flex items-center justify-center group-hover:bg-[#E7EDF5]/20 transition-colors">
                    <Repeat size={20} className="text-[#E7EDF5]" />
                  </div>
                  <div>
                    <div className="text-sm text-[#E7EDF5] mb-0.5">New Swap</div>
                    <div className="text-xs text-[#6C7A89]">Start atomic swap</div>
                  </div>
                </div>
              </button>

              <button
                onClick={() => navigate('/wallet')}
                className="w-full p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:bg-[#151B23]/80 hover:border-[#E7EDF5]/20 transition-all text-left group"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-[#10B981]/10 flex items-center justify-center group-hover:bg-[#10B981]/20 transition-colors">
                    <Download size={20} className="text-[#10B981]" />
                  </div>
                  <div>
                    <div className="text-sm text-[#E7EDF5] mb-0.5">Receive</div>
                    <div className="text-xs text-[#6C7A89]">Generate address</div>
                  </div>
                </div>
              </button>

              <button
                onClick={() => navigate('/wallet')}
                className="w-full p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:bg-[#151B23]/80 hover:border-[#E7EDF5]/20 transition-all text-left group"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-[#00B4D8]/10 flex items-center justify-center group-hover:bg-[#00B4D8]/20 transition-colors">
                    <Send size={20} className="text-[#00B4D8]" />
                  </div>
                  <div>
                    <div className="text-sm text-[#E7EDF5] mb-0.5">Send</div>
                    <div className="text-xs text-[#6C7A89]">Transfer funds</div>
                  </div>
                </div>
              </button>
            </div>
          </TerminalCard>
        </div>
      </div>
    </AppShell>
  );
}
