import { useState } from 'react';
import { ArrowRight, Plus, Clock, CheckCircle, ChevronDown, ChevronUp, Copy, RefreshCw, ExternalLink } from 'lucide-react';
import { NetworkBadge } from '@/app/components/network-badge';
import { useListSwaps, useCreateSwap } from '@/app/services/hooks';

const API_BASE = '/api/v1';

// SwapRow component with expandable details
function SwapRow({ swap, onUpdate }: { swap: any; onUpdate: () => void }) {
  const [expanded, setExpanded] = useState(false);
  const [checking, setChecking] = useState(false);
  const [checkResult, setCheckResult] = useState<any>(null);

  const handleCheckFunding = async () => {
    setChecking(true);
    try {
      const res = await fetch(`${API_BASE}/swaps/${swap.id}/check`, { method: 'POST' });
      const data = await res.json();
      setCheckResult(data);
      if (data.state === 'FUNDED') {
        onUpdate();
      }
    } catch (err) {
      setCheckResult({ error: 'Check failed' });
    } finally {
      setChecking(false);
    }
  };

  const handleClaim = async () => {
    if (!confirm('Claim this swap?')) return;
    try {
      await fetch(`${API_BASE}/swaps/${swap.id}/claim`, { method: 'POST' });
      onUpdate();
    } catch (err) {
      alert('Claim failed');
    }
  };

  const handleRefund = async () => {
    if (!confirm('Refund this swap? Only do this after timeout.')) return;
    try {
      await fetch(`${API_BASE}/swaps/${swap.id}/refund`, { method: 'POST' });
      onUpdate();
    } catch (err) {
      alert('Refund failed');
    }
  };

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    alert(`${label} copied!`);
  };

  // Full state machine mapping (14 states from xscore)
  const stateConfig: Record<string, { color: string; label: string; showActions: string[] }> = {
    // Creation phase
    'STATE_OPEN': { color: 'bg-[#6366F1]/10 text-[#6366F1]', label: 'CREATED', showActions: ['lock', 'cancel'] },
    'OPEN': { color: 'bg-[#6366F1]/10 text-[#6366F1]', label: 'CREATED', showActions: ['lock', 'cancel'] },

    // Funding phase
    'STATE_LOCKED': { color: 'bg-[#F59E0B]/10 text-[#F59E0B]', label: 'AWAITING PAYMENT', showActions: ['check', 'cancel'] },
    'LOCKED': { color: 'bg-[#F59E0B]/10 text-[#F59E0B]', label: 'AWAITING PAYMENT', showActions: ['check', 'cancel'] },
    'PENDING_FUNDING': { color: 'bg-[#F59E0B]/10 text-[#F59E0B]', label: 'AWAITING PAYMENT', showActions: ['check', 'cancel'] },

    // Commit phase
    'STATE_COMMIT_STARTED': { color: 'bg-[#F59E0B]/10 text-[#F59E0B]', label: 'BROADCASTING', showActions: [] },
    'COMMIT_STARTED': { color: 'bg-[#F59E0B]/10 text-[#F59E0B]', label: 'BROADCASTING', showActions: [] },

    // Waiting phase
    'STATE_WAITING': { color: 'bg-[#3B82F6]/10 text-[#3B82F6]', label: 'PROCESSING', showActions: ['check'] },
    'WAITING': { color: 'bg-[#3B82F6]/10 text-[#3B82F6]', label: 'PROCESSING', showActions: ['check'] },
    'FUNDED': { color: 'bg-[#3B82F6]/10 text-[#3B82F6]', label: 'FUNDED', showActions: ['claim'] },

    // Claim phase (auto - green)
    'STATE_WAITING_CLAIM_DETAILS': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'CLAIMING', showActions: [] },
    'STATE_SIGNING_MUSIG2_PARTIAL': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'SIGNING', showActions: [] },
    'STATE_SENT_PARTIAL_TO_PROVIDER': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'FINALIZING', showActions: [] },
    'STATE_WAITING_PROVIDER_BROADCAST': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'CONFIRMING', showActions: [] },

    // Refund phase (orange)
    'STATE_REFUND_COOP_WAITING': { color: 'bg-[#F97316]/10 text-[#F97316]', label: 'REFUNDING', showActions: [] },
    'STATE_FALLBACK_SCRIPT_READY': { color: 'bg-[#F97316]/10 text-[#F97316]', label: 'REFUND READY', showActions: ['refund'] },
    'STATE_REFUNDING': { color: 'bg-[#F97316]/10 text-[#F97316]', label: 'REFUNDING', showActions: [] },

    // Terminal states
    'STATE_COMPLETED': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'SUCCESS', showActions: [] },
    'COMPLETED': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'SUCCESS', showActions: [] },
    'CLAIMED': { color: 'bg-[#10B981]/10 text-[#10B981]', label: 'CLAIMED', showActions: [] },

    'STATE_FAILED': { color: 'bg-[#EF4444]/10 text-[#EF4444]', label: 'FAILED', showActions: [] },
    'FAILED': { color: 'bg-[#EF4444]/10 text-[#EF4444]', label: 'FAILED', showActions: [] },

    'STATE_CANCELED': { color: 'bg-[#6B7280]/10 text-[#6B7280]', label: 'CANCELED', showActions: [] },
    'CANCELED': { color: 'bg-[#6B7280]/10 text-[#6B7280]', label: 'CANCELED', showActions: [] },
    'REFUNDED': { color: 'bg-[#6B7280]/10 text-[#6B7280]', label: 'REFUNDED', showActions: [] },
  };

  const currentState = stateConfig[swap.state] || { color: 'bg-[#333] text-[#999]', label: swap.state, showActions: [] };
  const canCheck = currentState.showActions.includes('check');
  const canClaim = currentState.showActions.includes('claim');
  const canRefund = currentState.showActions.includes('refund');
  const canCancel = currentState.showActions.includes('cancel');

  return (
    <div className="border-b border-[#1a1a1a] last:border-0">
      {/* Header row */}
      <div
        className="flex items-center gap-4 px-4 py-3 cursor-pointer hover:bg-[#151515] transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex-shrink-0">
          {expanded ? <ChevronUp className="w-4 h-4 text-[#555]" /> : <ChevronDown className="w-4 h-4 text-[#555]" />}
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-mono text-white truncate">{swap.id}</p>
        </div>
        <div className="flex items-center gap-2">
          <NetworkBadge network={swap.from_chain} size="sm" />
          <ArrowRight className="w-3 h-3 text-[#555]" />
          <NetworkBadge network={swap.to_chain} size="sm" />
        </div>
        <div className="text-sm font-mono text-white w-32 text-right">
          {swap.amount_btc?.toFixed(8) || '0.00000000'} BTC
        </div>
        <div className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium ${currentState.color}`}>
          <Clock className="w-3 h-3" />
          {currentState.label}
        </div>
      </div>

      {/* Expanded details */}
      {expanded && (
        <div className="px-4 pb-4 pt-2 bg-[#0a0a0a] border-t border-[#1a1a1a]">
          <div className="grid grid-cols-2 gap-4">
            {/* Left: HTLC Details */}
            <div className="space-y-3">
              <div>
                <p className="text-xs text-[#555] uppercase tracking-wider mb-1">HTLC Address</p>
                <div className="flex items-center gap-2">
                  <p className="text-xs font-mono text-white break-all">{swap.htlc_address || 'N/A'}</p>
                  {swap.htlc_address && (
                    <>
                      <button
                        onClick={() => copyToClipboard(swap.htlc_address, 'Address')}
                        className="p-1 hover:bg-white/10 rounded"
                      >
                        <Copy className="w-3 h-3 text-[#666]" />
                      </button>
                      <a
                        href={`https://mempool.space/testnet/address/${swap.htlc_address}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="p-1 hover:bg-white/10 rounded"
                      >
                        <ExternalLink className="w-3 h-3 text-[#666]" />
                      </a>
                    </>
                  )}
                </div>
              </div>

              <div>
                <p className="text-xs text-[#555] uppercase tracking-wider mb-1">Payment Hash</p>
                <p className="text-xs font-mono text-[#666] break-all">{swap.payment_hash?.slice(0, 32) || 'N/A'}...</p>
              </div>

              {swap.state === 'FUNDED' && (
                <div className="bg-[#10B981]/10 border border-[#10B981]/20 rounded p-2">
                  <p className="text-xs text-[#555] uppercase tracking-wider mb-1">Preimage (use to claim)</p>
                  <div className="flex items-center gap-2">
                    <p className="text-xs font-mono text-[#10B981] break-all">{swap.preimage}</p>
                    <button
                      onClick={() => copyToClipboard(swap.preimage, 'Preimage')}
                      className="p-1 hover:bg-white/10 rounded"
                    >
                      <Copy className="w-3 h-3 text-[#10B981]" />
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* Right: Actions */}
            <div className="space-y-3">
              <div>
                <p className="text-xs text-[#555] uppercase tracking-wider mb-2">Actions</p>
                <div className="flex flex-wrap gap-2">
                  {canCheck && (
                    <button
                      onClick={handleCheckFunding}
                      disabled={checking}
                      className="flex items-center gap-2 px-3 py-1.5 bg-white/10 text-white rounded hover:bg-white/20 transition-colors text-xs disabled:opacity-50"
                    >
                      <RefreshCw className={`w-3 h-3 ${checking ? 'animate-spin' : ''}`} />
                      {checking ? 'Checking...' : 'Check Status'}
                    </button>
                  )}

                  {canClaim && (
                    <button
                      onClick={handleClaim}
                      className="flex items-center gap-2 px-3 py-1.5 bg-[#10B981]/20 text-[#10B981] rounded hover:bg-[#10B981]/30 transition-colors text-xs"
                    >
                      <CheckCircle className="w-3 h-3" />
                      Claim
                    </button>
                  )}

                  {canRefund && (
                    <button
                      onClick={handleRefund}
                      className="flex items-center gap-2 px-3 py-1.5 bg-[#EF4444]/10 text-[#EF4444] rounded hover:bg-[#EF4444]/20 transition-colors text-xs"
                    >
                      Refund
                    </button>
                  )}

                  {canCancel && (
                    <button
                      onClick={() => { if (confirm('Cancel this swap?')) { /* TODO: call cancel endpoint */ } }}
                      className="flex items-center gap-2 px-3 py-1.5 bg-[#6B7280]/10 text-[#6B7280] rounded hover:bg-[#6B7280]/20 transition-colors text-xs"
                    >
                      Cancel
                    </button>
                  )}
                </div>
              </div>

              {checkResult && (
                <div className={`p-2 rounded text-xs ${checkResult.status === 'funded' ? 'bg-[#10B981]/10 text-[#10B981]' : 'bg-[#333] text-[#999]'}`}>
                  {checkResult.message || checkResult.error}
                </div>
              )}

              <div className="text-xs text-[#555]">
                <p>Created: {new Date(swap.created_at).toLocaleString()}</p>
                {swap.funded_txid && <p>Funding TX: {swap.funded_txid.slice(0, 16)}...</p>}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function SwapCenterPage() {
  const [view, setView] = useState<'wizard' | 'active' | 'history'>('active');
  const { data: swaps, loading, refetch } = useListSwaps();
  const { create, loading: creating } = useCreateSwap();

  // Form state
  const [fromChain, setFromChain] = useState('BTC');
  const [toChain, setToChain] = useState('LN');
  const [amount, setAmount] = useState('');

  const handleCreateSwap = async () => {
    if (!amount || parseFloat(amount) <= 0) {
      alert('Please enter a valid amount');
      return;
    }

    try {
      await create({
        from_chain: fromChain,
        to_chain: toChain,
        amount_sats: Math.floor(parseFloat(amount) * 100_000_000)
      });

      // Reset form and refresh list
      setAmount('');
      refetch();
      setView('active');
    } catch (err) {
      alert('Failed to create swap: ' + (err instanceof Error ? err.message : 'Unknown error'));
    }
  };

  const activeSwaps = swaps?.filter((s: any) => s.state !== 'COMPLETED') || [];
  const completedSwaps = swaps?.filter((s: any) => s.state === 'COMPLETED') || [];

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-white mb-2">Swap Center</h2>
        <p className="text-sm text-[#666]">
          Create and manage atomic swaps across BTC, Lightning and Liquid networks
        </p>
      </div>

      <div className="flex gap-2">
        {[
          { id: 'wizard', label: 'New Swap' },
          { id: 'active', label: 'Active Swaps' },
          { id: 'history', label: 'History' }
        ].map(tab => (
          <button
            key={tab.id}
            onClick={() => setView(tab.id as any)}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${view === tab.id
              ? 'bg-white/10 text-white border border-white/20'
              : 'bg-[#111] border border-[#222] text-[#999] hover:border-white/20'
              }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {view === 'wizard' && (
        <div className="bg-[#111] border border-[#222] rounded-xl p-6 space-y-6">
          <div className="flex items-center justify-between pb-4 border-b border-[#222]">
            <div>
              <h3 className="text-lg font-semibold text-white">New Atomic Swap</h3>
              <p className="text-sm text-[#666]">Configure swap parameters</p>
            </div>
          </div>

          <div className="grid grid-cols-3 gap-6">
            {/* From */}
            <div className="space-y-3">
              <label className="text-xs font-medium text-[#555] uppercase tracking-wider">From</label>
              <select
                value={fromChain}
                onChange={(e) => setFromChain(e.target.value)}
                className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-white/10"
              >
                <option value="BTC">Bitcoin (BTC)</option>
                <option value="LN">Lightning Network (LN)</option>
                <option value="L-BTC">Liquid Bitcoin (L-BTC)</option>
              </select>
              <input
                type="number"
                placeholder="Amount (BTC)"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                step="0.00000001"
                className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
              />
            </div>

            {/* Arrow */}
            <div className="flex items-center justify-center pt-8">
              <div className="p-3 bg-white/10 rounded-full text-white">
                <ArrowRight className="w-6 h-6" />
              </div>
            </div>

            {/* To */}
            <div className="space-y-3">
              <label className="text-xs font-medium text-[#555] uppercase tracking-wider">To</label>
              <select
                value={toChain}
                onChange={(e) => setToChain(e.target.value)}
                className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-white/10"
              >
                <option value="LN">Lightning Network (LN)</option>
                <option value="BTC">Bitcoin (BTC)</option>
                <option value="L-BTC">Liquid Bitcoin (L-BTC)</option>
              </select>
              <input
                type="number"
                placeholder="Amount"
                value={amount ? (parseFloat(amount) * 0.999).toFixed(8) : ''}
                readOnly
                className="w-full bg-[#0d0d0d]/50 border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-[#666]"
              />
              <div className="text-xs text-[#555]">
                Fee: ~0.1%
              </div>
            </div>
          </div>

          <div className="flex gap-3 pt-4">
            <button
              onClick={handleCreateSwap}
              disabled={creating}
              className="flex-1 px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-medium"
            >
              <Plus className="w-4 h-4 inline mr-2" />
              {creating ? 'Creating...' : 'Create Swap'}
            </button>
            <button
              onClick={() => setAmount('')}
              className="px-4 py-2 bg-[#0d0d0d] border border-[#222] text-white rounded-lg hover:bg-[#151515] transition-colors"
            >
              Reset
            </button>
          </div>
        </div>
      )}

      {view === 'active' && (
        <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
          {loading ? (
            <div className="p-8 text-center text-[#666]">Loading swaps...</div>
          ) : activeSwaps.length === 0 ? (
            <div className="p-8 text-center text-[#666]">No active swaps</div>
          ) : (
            <div className="divide-y divide-[#1a1a1a]">
              {activeSwaps.map((swap: any) => (
                <SwapRow key={swap.id} swap={swap} onUpdate={refetch} />
              ))}
            </div>
          )}
        </div>
      )}

      {view === 'history' && (
        <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
          {loading ? (
            <div className="p-8 text-center text-[#666]">Loading history...</div>
          ) : completedSwaps.length === 0 ? (
            <div className="p-8 text-center text-[#666]">No completed swaps</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-[#0d0d0d] border-b border-[#1a1a1a]">
                  <tr>
                    <th className="text-left px-4 py-3 text-[10px] font-medium text-[#555] uppercase tracking-wider">ID</th>
                    <th className="text-left px-4 py-3 text-[10px] font-medium text-[#555] uppercase tracking-wider">Pair</th>
                    <th className="text-left px-4 py-3 text-[10px] font-medium text-[#555] uppercase tracking-wider">Amount</th>
                    <th className="text-left px-4 py-3 text-[10px] font-medium text-[#555] uppercase tracking-wider">Status</th>
                    <th className="text-left px-4 py-3 text-[10px] font-medium text-[#555] uppercase tracking-wider">Date</th>
                  </tr>
                </thead>
                <tbody>
                  {completedSwaps.map((swap: any) => (
                    <tr key={swap.id} className="border-b border-[#1a1a1a] hover:bg-[#151515] transition-colors">
                      <td className="px-4 py-3 text-sm font-mono text-white">{swap.id.slice(0, 12)}...</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <NetworkBadge network={swap.from_chain} size="sm" />
                          <ArrowRight className="w-3 h-3 text-[#555]" />
                          <NetworkBadge network={swap.to_chain} size="sm" />
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm font-mono text-white">{swap.amount_btc.toFixed(8)} BTC</td>
                      <td className="px-4 py-3">
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-[#10B981]/10 text-[#10B981] text-xs font-medium">
                          <CheckCircle className="w-3 h-3" />
                          COMPLETED
                        </span>
                      </td>
                      <td className="px-4 py-3 text-sm text-[#666]">
                        {new Date(swap.created_at).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
