import React, { useMemo, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { TerminalCard } from '../TerminalCard';
import { StatusChip } from '../StatusChip';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { useCheckSwap, useCreateSwap, useSwapEvents, useSwaps } from '@/services/hooks';
import type { CreateSwapRequest, Swap } from '@/services/api';

type RoutePreset = {
  id: string;
  label: string;
  description: string;
  fromChain: 'btc' | 'liquid' | 'ln';
  toChain: 'btc' | 'liquid' | 'ln';
};

const ROUTES: RoutePreset[] = [
  { id: 'btc-ln', label: 'Submarine (BTC -> LN)', description: 'On-chain BTC to Lightning', fromChain: 'btc', toChain: 'ln' },
  { id: 'ln-btc', label: 'Reverse (LN -> BTC)', description: 'Lightning to on-chain BTC', fromChain: 'ln', toChain: 'btc' },
  { id: 'btc-liquid', label: 'Chain (BTC -> L-BTC)', description: 'Cross-chain BTC to Liquid', fromChain: 'btc', toChain: 'liquid' },
  { id: 'liquid-btc', label: 'Chain (L-BTC -> BTC)', description: 'Cross-chain Liquid to BTC', fromChain: 'liquid', toChain: 'btc' },
];

const ACTIVE_STATES = new Set([
  'open',
  'locked',
  'commit_started',
  'waiting',
  'waiting_claim_details',
  'signing_musig2_partial',
  'sent_partial_to_provider',
  'waiting_provider_broadcast',
  'refund_coop_waiting',
  'fallback_script_ready',
  'refunding',
  'pending_funding',
  'funded',
]);

const TERMINAL_STATES = new Set(['completed', 'failed', 'canceled', 'refunded']);

const stateConfig: Record<string, { label: string; variant: 'success' | 'warning' | 'error' | 'btc' | 'liquid' | 'lightning' | 'default' }> = {
  open: { label: 'Open', variant: 'warning' },
  locked: { label: 'Locked', variant: 'warning' },
  commit_started: { label: 'Commit Started', variant: 'btc' },
  waiting: { label: 'Waiting', variant: 'liquid' },
  waiting_claim_details: { label: 'Waiting Claim Details', variant: 'liquid' },
  signing_musig2_partial: { label: 'Signing MuSig2', variant: 'lightning' },
  sent_partial_to_provider: { label: 'Partial Sent', variant: 'lightning' },
  waiting_provider_broadcast: { label: 'Waiting Broadcast', variant: 'lightning' },
  refund_coop_waiting: { label: 'Refund Coop Waiting', variant: 'warning' },
  fallback_script_ready: { label: 'Fallback Script Ready', variant: 'warning' },
  refunding: { label: 'Refunding', variant: 'warning' },
  pending_funding: { label: 'Pending Funding', variant: 'warning' },
  funded: { label: 'Funded', variant: 'liquid' },
  completed: { label: 'Completed', variant: 'success' },
  failed: { label: 'Failed', variant: 'error' },
  canceled: { label: 'Canceled', variant: 'error' },
  refunded: { label: 'Refunded', variant: 'error' },
};

function stateMessage(state: string, swap: Swap): string {
  if (swap.error_message && swap.error_message.trim()) return swap.error_message.trim();
  const messages: Record<string, string> = {
    open: 'Swap criado; aguardando lock de parâmetros.',
    locked: 'Parâmetros travados; pronto para commit.',
    commit_started: 'Commit iniciado; aguardando progresso do provider.',
    waiting: 'Aguardando trigger de execução.',
    waiting_claim_details: 'Aguardando detalhes de claim para assinatura.',
    signing_musig2_partial: 'Assinando partial MuSig2.',
    sent_partial_to_provider: 'Partial enviada ao provider.',
    waiting_provider_broadcast: 'Aguardando broadcast/confirmação do provider.',
    refund_coop_waiting: 'Aguardando janela de refund cooperativo.',
    fallback_script_ready: 'Coop falhou; fallback script disponível.',
    refunding: 'Refund em andamento.',
    completed: 'Swap concluído com sucesso.',
    failed: 'Swap falhou.',
    canceled: 'Swap cancelado.',
    refunded: 'Swap refundado.',
    pending_funding: 'Aguardando funding.',
    funded: 'Funding detectado; aguardando próximo passo.',
  };
  return messages[state] || 'Estado em processamento.';
}

function normalizeState(value: string): string {
  return String(value || '').trim().toLowerCase().replace(/\s+/g, '_');
}

function inferKind(swap: Swap): 'submarine' | 'reverse' | 'chain' | 'unknown' {
  const kind = (swap.kind || '').toLowerCase();
  if (kind === 'submarine' || kind === 'reverse' || kind === 'chain') return kind;
  const from = (swap.from_chain || '').toLowerCase();
  const to = (swap.to_chain || '').toLowerCase();
  if (from === 'ln') return 'reverse';
  if ((from === 'btc' || from === 'liquid') && (to === 'btc' || to === 'liquid') && from !== to) return 'chain';
  if ((from === 'btc' || from === 'liquid') && to === 'ln') return 'submarine';
  return 'unknown';
}

function routeKind(route: RoutePreset): 'submarine' | 'reverse' | 'chain' {
  if (route.fromChain === 'ln') return 'reverse';
  if ((route.fromChain === 'btc' || route.fromChain === 'liquid') && (route.toChain === 'btc' || route.toChain === 'liquid') && route.fromChain !== route.toChain) {
    return 'chain';
  }
  return 'submarine';
}

function requiresInvoice(kind: 'submarine' | 'reverse' | 'chain', route: RoutePreset): boolean {
  return kind === 'submarine' && route.toChain === 'ln';
}

function requiresDestination(kind: 'submarine' | 'reverse' | 'chain'): boolean {
  return kind === 'reverse' || kind === 'chain';
}

function destinationLabel(kind: 'submarine' | 'reverse' | 'chain', route: RoutePreset): string {
  if (kind === 'reverse') {
    return route.toChain === 'btc' ? 'BTC Destination Address' : 'Destination Address';
  }
  if (kind === 'chain') {
    return route.toChain === 'liquid' ? 'Liquid Destination Address' : 'BTC Destination Address';
  }
  return 'Destination Address';
}

function validateCreateSwapForm(
  route: RoutePreset,
  selectedKind: 'submarine' | 'reverse' | 'chain',
  amount: string,
  invoice: string,
  destinationAddress: string,
): string[] {
  const errors: string[] = [];
  const amountSats = parseInt(amount, 10);
  if (!Number.isFinite(amountSats) || amountSats <= 0) {
    errors.push('Amount must be greater than zero.');
  }
  if (requiresInvoice(selectedKind, route) && !invoice.trim()) {
    errors.push('Lightning Invoice is required for submarine swaps.');
  }
  if (requiresDestination(selectedKind) && !destinationAddress.trim()) {
    errors.push('Destination address is required for reverse/chain swaps.');
  }
  return errors;
}

export function SwapCenter() {
  const [routeId, setRouteId] = useState(ROUTES[0].id);
  const [amount, setAmount] = useState('100000');
  const [invoice, setInvoice] = useState('');
  const [destinationAddress, setDestinationAddress] = useState('');
  const [selectedSwapId, setSelectedSwapId] = useState<string | null>(null);
  const [focusedSwapId, setFocusedSwapId] = useState<string | null>(null);

  const route = useMemo(() => ROUTES.find((r) => r.id === routeId) || ROUTES[0], [routeId]);
  const selectedKind = routeKind(route);
  const validationErrors = useMemo(
    () => validateCreateSwapForm(route, selectedKind, amount, invoice, destinationAddress),
    [route, selectedKind, amount, invoice, destinationAddress],
  );

  const { swaps, loading: swapsLoading, refetch } = useSwaps();
  const { create, loading: creating, error: createError } = useCreateSwap();
  const { check, loading: checking, error: checkError } = useCheckSwap();
  const { events: swapEvents, loading: eventsLoading, error: eventsError, refetch: refetchEvents } = useSwapEvents(focusedSwapId);

  const normalized = swaps.map((s) => ({ swap: s, state: normalizeState(s.state), kind: inferKind(s) }));
  const activeSwaps = normalized.filter((s) => ACTIVE_STATES.has(s.state)).map((s) => s.swap);
  const historySwaps = normalized.filter((s) => TERMINAL_STATES.has(s.state)).map((s) => s.swap);

  const handleCreateSwap = async () => {
    if (validationErrors.length > 0) return;
    const amountSats = parseInt(amount, 10);

    const payload: CreateSwapRequest = {
      from_chain: route.fromChain,
      to_chain: route.toChain,
      amount_sats: amountSats,
    };
    if (requiresInvoice(selectedKind, route) && invoice.trim()) {
      payload.invoice = invoice.trim();
    }
    if (requiresDestination(selectedKind) && destinationAddress.trim()) {
      payload.destination_address = destinationAddress.trim();
    }
    await create(payload);
    setInvoice('');
    setDestinationAddress('');
    await refetch();
  };

  const handleCheckSwap = async (id: string) => {
    setSelectedSwapId(id);
    try {
      await check(id);
      await refetch();
    } finally {
      setSelectedSwapId(null);
    }
  };

  return (
    <AppShell activePage="swap" vaultLocked={false}>
      <div className="p-8">
        <PageHeader title="Atomic Swap" subtitle="Cross-chain execution center" />

        <div className="mb-8">
          <TerminalCard
            header={
              <div className="flex items-center justify-between">
                <h3 className="text-lg text-[#E7EDF5]">Create Swap</h3>
                <StatusChip label={selectedKind} variant={selectedKind === 'chain' ? 'liquid' : selectedKind === 'reverse' ? 'lightning' : 'btc'} />
              </div>
            }
          >
            <div className="space-y-4">
              <div>
                <label className="text-sm text-[#9AA7B5] mb-2 block">Route</label>
                <select
                  value={routeId}
                  onChange={(e) => setRouteId(e.target.value)}
                  className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                >
                  {ROUTES.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.label}
                    </option>
                  ))}
                </select>
                <div className="text-xs text-[#6C7A89] mt-1">{route.description}</div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#9AA7B5] mb-2 block">From</label>
                  <div className="px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono">{route.fromChain.toUpperCase()}</div>
                </div>
                <div>
                  <label className="text-sm text-[#9AA7B5] mb-2 block">To</label>
                  <div className="px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono">{route.toChain.toUpperCase()}</div>
                </div>
              </div>

              <div>
                <label className="text-sm text-[#9AA7B5] mb-2 block">Amount (sats)</label>
                <input
                  type="text"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value.replace(/[^\d]/g, ''))}
                  className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                />
              </div>

              {requiresInvoice(selectedKind, route) && (
                <div>
                  <label className="text-sm text-[#9AA7B5] mb-2 block">Lightning Invoice (required)</label>
                  <input
                    type="text"
                    value={invoice}
                    onChange={(e) => setInvoice(e.target.value)}
                    placeholder="lnbc..."
                    className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  />
                </div>
              )}

              {requiresDestination(selectedKind) && (
                <div>
                  <label className="text-sm text-[#9AA7B5] mb-2 block">{destinationLabel(selectedKind, route)} (required)</label>
                  <input
                    type="text"
                    value={destinationAddress}
                    onChange={(e) => setDestinationAddress(e.target.value)}
                    placeholder={route.toChain === 'liquid' ? 'tlq1...' : 'bc1q...'}
                    className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  />
                </div>
              )}

              {validationErrors.length > 0 && (
                <div className="p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl text-sm text-[#EF4444]">
                  {validationErrors[0]}
                </div>
              )}

              <div className="p-3 bg-[#0B0D10] border border-[#242C36] rounded-xl">
                <div className="text-xs text-[#6C7A89] mb-2">Execution Preview</div>
                <div className="text-xs text-[#9AA7B5] space-y-1 font-mono">
                  <div>route: {route.fromChain.toUpperCase()} -&gt; {route.toChain.toUpperCase()} ({selectedKind})</div>
                  <div>amount: {amount || '0'} sats</div>
                  {requiresInvoice(selectedKind, route) && (
                    <div>invoice: {invoice.trim() ? 'provided' : 'missing'}</div>
                  )}
                  {requiresDestination(selectedKind) && (
                    <div>destination: {destinationAddress.trim() || 'missing'}</div>
                  )}
                </div>
              </div>

              {(createError || checkError) && (
                <div className="p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl text-sm text-[#EF4444]">
                  {createError || checkError}
                </div>
              )}

              <div className="flex items-center gap-3">
                <SecondaryButton className="flex-1" onClick={() => { setAmount('100000'); setInvoice(''); setDestinationAddress(''); }}>
                  Reset
                </SecondaryButton>
                <PrimaryButton className="flex-1" onClick={handleCreateSwap} disabled={creating || validationErrors.length > 0}>
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
            </div>
          </TerminalCard>
        </div>

        <div className="mb-8">
          <TerminalCard
            header={
              <div className="flex items-center justify-between">
                <h3 className="text-lg text-[#E7EDF5]">Active Swaps</h3>
                <span className="text-sm text-[#6C7A89]">{activeSwaps.length} active</span>
              </div>
            }
          >
            {swapsLoading ? (
              <div className="text-center py-8 text-[#6C7A89]">Loading...</div>
            ) : activeSwaps.length === 0 ? (
              <div className="text-center py-8 text-[#6C7A89]">No active swaps</div>
            ) : (
              <div className="space-y-4">
                {activeSwaps.map((swap) => {
                  const state = normalizeState(swap.state);
                  const kind = inferKind(swap);
                  const config = stateConfig[state] || { label: swap.state, variant: 'warning' as const };
                  const message = stateMessage(state, swap);
                  const isCurrentChecking = checking && selectedSwapId === swap.id;
                  return (
                    <div key={swap.id} className="p-4 bg-[#151B23] border border-[#242C36] rounded-xl">
                      <div className="flex items-center justify-between mb-2 gap-2">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-sm text-[#E7EDF5] font-mono">{swap.from_chain}</span>
                          <span className="text-[#6C7A89]">→</span>
                          <span className="text-sm text-[#E7EDF5] font-mono">{swap.to_chain}</span>
                          <StatusChip label={kind} variant={kind === 'chain' ? 'liquid' : kind === 'reverse' ? 'lightning' : 'btc'} />
                        </div>
                        <StatusChip label={config.label} variant={config.variant} />
                      </div>
                      <div className="flex items-center justify-between mb-3">
                        <span className="text-sm text-[#9AA7B5]">{swap.amount_sats.toLocaleString()} sats</span>
                        <span className="text-xs text-[#6C7A89] font-mono">{swap.id.slice(0, 12)}...</span>
                      </div>
                      <div className="mb-3 p-2 bg-[#0B0D10] rounded text-xs text-[#6C7A89] font-mono space-y-1 break-all">
                        {swap.lockup_address && <div>Funding: {swap.lockup_address}</div>}
                        {swap.destination_address && <div>Destination: {swap.destination_address}</div>}
                        {swap.claim_address && <div>Claim: {swap.claim_address}</div>}
                        {swap.invoice && <div>Invoice: {swap.invoice.slice(0, 28)}...</div>}
                        {!swap.lockup_address && !swap.destination_address && !swap.claim_address && !swap.invoice && (
                          <div>No funding/settlement details yet.</div>
                        )}
                      </div>
                      <div className={`mb-3 text-xs ${state === 'failed' || state === 'canceled' || state === 'refunded' ? 'text-[#EF4444]' : 'text-[#9AA7B5]'}`}>
                        {message}
                      </div>
                      <div className="flex justify-end">
                        <SecondaryButton className="mr-2" onClick={() => { setFocusedSwapId(swap.id); void refetchEvents(); }}>
                          Events
                        </SecondaryButton>
                        <PrimaryButton onClick={() => handleCheckSwap(swap.id)} disabled={isCurrentChecking}>
                          {isCurrentChecking ? (
                            <div className="flex items-center gap-2">
                              <Loader2 size={16} className="animate-spin" />
                              <span>Advancing...</span>
                            </div>
                          ) : (
                            'Advance'
                          )}
                        </PrimaryButton>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </TerminalCard>
        </div>

        <div className="mb-8">
          <TerminalCard
            header={
              <div className="flex items-center justify-between">
                <h3 className="text-lg text-[#E7EDF5]">Swap Events Timeline</h3>
                <span className="text-sm text-[#6C7A89]">{focusedSwapId ? focusedSwapId.slice(0, 12) + '...' : 'No swap selected'}</span>
              </div>
            }
          >
            {!focusedSwapId ? (
              <div className="text-center py-8 text-[#6C7A89]">Select an active swap and click Events.</div>
            ) : eventsLoading ? (
              <div className="text-center py-8 text-[#6C7A89]">Loading events...</div>
            ) : eventsError ? (
              <div className="p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl text-sm text-[#EF4444]">{eventsError}</div>
            ) : swapEvents.length === 0 ? (
              <div className="text-center py-8 text-[#6C7A89]">No events yet for this swap.</div>
            ) : (
              <div className="space-y-3">
                {swapEvents.map((ev) => {
                  const toState = normalizeState(ev.to_state || '');
                  const config = stateConfig[toState] || { label: ev.to_state || 'unknown', variant: 'warning' as const };
                  return (
                    <div key={`${ev.seq}-${ev.swap_id}`} className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                      <div className="flex items-center justify-between gap-2 mb-2">
                        <div className="text-xs text-[#9AA7B5] font-mono">#{ev.seq} {ev.trigger || 'event'}</div>
                        <StatusChip label={config.label} variant={config.variant} />
                      </div>
                      <div className="text-xs text-[#6C7A89] mb-2">
                        {normalizeState(ev.from_state || 'unknown')} → {toState}
                      </div>
                      {ev.details_json && (
                        <pre className="text-xs text-[#9AA7B5] bg-[#0B0D10] border border-[#242C36] rounded p-2 overflow-x-auto whitespace-pre-wrap break-all">
{ev.details_json}
                        </pre>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </TerminalCard>
        </div>

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
            <div className="text-center py-8 text-[#6C7A89]">No terminal swaps yet</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="text-left text-sm text-[#6C7A89] border-b border-[#242C36]">
                    <th className="pb-3 font-medium">Swap ID</th>
                    <th className="pb-3 font-medium">Kind</th>
                    <th className="pb-3 font-medium">Route</th>
                    <th className="pb-3 font-medium">Amount</th>
                    <th className="pb-3 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody className="text-sm">
                  {historySwaps.map((swap) => {
                    const state = normalizeState(swap.state);
                    const kind = inferKind(swap);
                    const config = stateConfig[state] || { label: swap.state, variant: 'warning' as const };
                    const message = stateMessage(state, swap);
                    return (
                      <tr key={swap.id} className="border-b border-[#242C36] last:border-0 hover:bg-[#151B23]/50 transition-colors">
                        <td className="py-3 font-mono text-[#9AA7B5]">{swap.id.slice(0, 12)}...</td>
                        <td className="py-3 text-[#E7EDF5]">{kind}</td>
                        <td className="py-3 text-[#E7EDF5]">{swap.from_chain} → {swap.to_chain}</td>
                        <td className="py-3 font-mono text-[#E7EDF5]">{swap.amount_sats.toLocaleString()}</td>
                        <td className="py-3">
                          <StatusChip label={config.label} variant={config.variant} />
                          <div className={`text-xs mt-1 ${state === 'failed' || state === 'canceled' || state === 'refunded' ? 'text-[#EF4444]' : 'text-[#6C7A89]'}`}>
                            {message}
                          </div>
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
