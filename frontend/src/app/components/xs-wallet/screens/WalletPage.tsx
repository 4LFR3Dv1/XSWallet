import React, { useEffect, useMemo, useState } from 'react';
import { Bitcoin, Droplet, Copy, Plus, Database, Lock, Eye, EyeOff, ShieldAlert, History, Search } from 'lucide-react';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { useAddressBook, useBalances, useNewAddress, useSendOnchain, useSwaps, useVaultStatus, useWalletTransactions, useWalletUtxos } from '@/services/hooks';

type ActivityEventType = 'RECEIVE' | 'SEND' | 'SWAP_LOCK' | 'SWAP_CLAIM' | 'REFUND' | 'RESERVATION_SET' | 'RESERVATION_RELEASE';
type ActivityStatus = 'pending' | 'confirmed' | 'failed' | 'unknown';
type EvidenceLevel = 'observed' | 'inferred';

export function WalletPage() {
    const [activeChain, setActiveChain] = useState<'btc' | 'liquid'>('btc');
    const { balances, loading: balancesLoading, error: balancesError, refetch: refetchBalances } = useBalances();
    const { generate, loading: addressLoading, error: generateAddressError } = useNewAddress();
    const { send, loading: sendLoading, error: sendError, txid } = useSendOnchain();
    const { addresses, loading: addressesLoading, error: addressesError, refetch: refetchAddresses } = useAddressBook(activeChain);
    const { utxos, loading: utxosLoading, error: utxosError, refetch: refetchUtxos } = useWalletUtxos(activeChain);
    const { transactions, loading: txLoading, error: txError, refetch: refetchTx } = useWalletTransactions(activeChain);
    const { swaps } = useSwaps();
    const { status: vaultStatus } = useVaultStatus();
    const [showSend, setShowSend] = useState(false);
    const [sendAddress, setSendAddress] = useState('');
    const [sendAmount, setSendAmount] = useState('');
    const [sendFeeRate, setSendFeeRate] = useState('');
    const [showXpub, setShowXpub] = useState(false);
    const [xpubRevealUntil, setXpubRevealUntil] = useState<number | null>(null);
    const [showExportPanel, setShowExportPanel] = useState(false);
    const [exportPin, setExportPin] = useState('');
    const [exportConfirmText, setExportConfirmText] = useState('');
    const [exportMessage, setExportMessage] = useState<string | null>(null);
    const [addressSearch, setAddressSearch] = useState('');
    const [addressUsageFilter, setAddressUsageFilter] = useState<'all' | 'used' | 'unused'>('all');
    const [addressPage, setAddressPage] = useState(1);
    const [optimisticAddresses, setOptimisticAddresses] = useState<Array<{
        address: string;
        chain: string;
        derivation_path: string;
        used: boolean;
        balance_sat: number;
    }>>([]);

    const handleGenerateAddress = async () => {
        const result = await generate(activeChain);
        if (result) {
            setOptimisticAddresses((prev) => {
                if (prev.some((a) => a.address === result.address)) return prev;
                return [...prev, {
                    address: result.address,
                    chain: result.chain || activeChain,
                    derivation_path: result.derivation_path || '',
                    used: false,
                    balance_sat: 0,
                }];
            });
            await refetchAddresses();
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
    };

    const formatSats = (sats: number) => {
        return (sats / 100_000_000).toFixed(8);
    };

    const totalReservedSat = useMemo(
        () => utxos.filter((u) => u.reserved).reduce((acc, u) => acc + u.amount_sat, 0),
        [utxos]
    );
    const totalUtxoSat = useMemo(() => utxos.reduce((acc, u) => acc + u.amount_sat, 0), [utxos]);

    const activityEvents = useMemo(() => {
        const txEvents = transactions.map((t) => ({
            id: `tx:${t.txid}:${t.chain}:${t.amount_sat}`,
            event_type: (t.amount_sat >= 0 ? 'RECEIVE' : 'SEND') as ActivityEventType,
            status: (t.confirmations > 0 ? 'confirmed' : 'pending') as ActivityStatus,
            chain: t.chain,
            txid: t.txid,
            swap_id: t.swap_id,
            timestamp: t.timestamp || '',
            evidence_level: 'observed' as EvidenceLevel,
        }));
        const reservationEvents = utxos
            .filter((u) => u.reserved)
            .map((u) => ({
                id: `reservation:${u.txid}:${u.vout}`,
                event_type: 'RESERVATION_SET' as ActivityEventType,
                status: (u.confirmations > 0 ? 'confirmed' : 'pending') as ActivityStatus,
                chain: u.chain,
                txid: u.txid,
                swap_id: u.reservation?.swap_id || null,
                timestamp: u.reservation?.reserved_at || '',
                evidence_level: (u.reservation?.reserved_at ? 'observed' : 'inferred') as EvidenceLevel,
            }));
        const swapEvents = swaps.flatMap((s) => {
            const out: Array<{
                id: string;
                event_type: string;
                status: string;
                chain: string;
                txid: string;
                swap_id: string;
                timestamp: string;
                evidence_level: string;
            }> = [];
            if (s.funded_txid) {
                out.push({
                    id: `swap-lock:${s.id}:${s.funded_txid}`,
                    event_type: 'SWAP_LOCK' as ActivityEventType,
                    status: 'confirmed' as ActivityStatus,
                    chain: s.from_chain,
                    txid: s.funded_txid,
                    swap_id: s.id,
                    timestamp: s.updated_at || s.created_at || '',
                    evidence_level: 'observed' as EvidenceLevel,
                });
            }
            return out;
        });
        return [...txEvents, ...reservationEvents, ...swapEvents]
            .sort((a, b) => String(b.timestamp || '').localeCompare(String(a.timestamp || '')))
            .slice(0, 30);
    }, [transactions, utxos, swaps]);

    const mockMaskedXpub = activeChain === 'btc'
        ? 'zpub6q...****************...XSW'
        : 'VJLC...****************...XSW';
    const revealActive = xpubRevealUntil != null && xpubRevealUntil > Date.now();
    const xpubValue = revealActive ? `${mockMaskedXpub} (reveal-temporary)` : mockMaskedXpub;
    const exportEnabled = import.meta.env.VITE_ENABLE_WALLET_EXPORT === '1';
    const exportCanSubmit = exportEnabled && exportPin.trim().length >= 8 && exportConfirmText.trim().toUpperCase() === 'EXPORT';

    const handleSend = async () => {
        const amount = Number(sendAmount);
        if (!sendAddress || !Number.isFinite(amount) || amount <= 0) {
            return;
        }
        const amountSats = Math.round(amount * 100_000_000);
        const feeRate = sendFeeRate ? Number(sendFeeRate) : undefined;

        await send({
            chain: activeChain,
            address: sendAddress,
            amount_sats: amountSats,
            fee_rate_sat_vb: feeRate && feeRate > 0 ? feeRate : undefined,
            label: 'send',
        });
        await refetchUtxos();
        await refetchTx();
    };

    const balance = activeChain === 'btc'
        ? balances?.btc
        : balances?.liquid;
    const combinedAddresses = useMemo(() => {
        const byAddress = new Map<string, {
            address: string;
            chain: string;
            derivation_path: string;
            used: boolean;
            balance_sat: number;
        }>();
        for (const a of addresses) byAddress.set(a.address, a);
        for (const a of optimisticAddresses) if (!byAddress.has(a.address)) byAddress.set(a.address, a);
        return Array.from(byAddress.values());
    }, [addresses, optimisticAddresses]);
    const identityAnchorAddress = combinedAddresses.length > 0 ? combinedAddresses[0] : null;
    const latestAddress = combinedAddresses.length > 0 ? combinedAddresses[combinedAddresses.length - 1] : null;
    const allAddressesLatestFirst = useMemo(() => [...combinedAddresses].reverse(), [combinedAddresses]);
    const filteredAddresses = useMemo(() => {
        const query = addressSearch.trim().toLowerCase();
        return allAddressesLatestFirst.filter((addr) => {
            if (addressUsageFilter === 'used' && !addr.used) return false;
            if (addressUsageFilter === 'unused' && addr.used) return false;
            if (!query) return true;
            return addr.address.toLowerCase().includes(query) || (addr.derivation_path || '').toLowerCase().includes(query);
        });
    }, [allAddressesLatestFirst, addressSearch, addressUsageFilter]);
    const pageSize = 10;
    const totalAddressPages = Math.max(1, Math.ceil(filteredAddresses.length / pageSize));
    const pagedAddresses = useMemo(() => {
        const start = (addressPage - 1) * pageSize;
        return filteredAddresses.slice(start, start + pageSize);
    }, [filteredAddresses, addressPage]);
    const featuredLatestAddress = filteredAddresses.length > 0 ? filteredAddresses[0] : null;
    const pagedAddressesWithoutFeatured = useMemo(
        () => pagedAddresses.filter((a) => !featuredLatestAddress || a.address !== featuredLatestAddress.address),
        [pagedAddresses, featuredLatestAddress]
    );
    const swapLockupAddresses = useMemo(() => {
        const unique = new Set<string>();
        for (const s of swaps) {
            if (s.lockup_address) unique.add(s.lockup_address);
        }
        return Array.from(unique);
    }, [swaps]);
    const walletFingerprint = identityAnchorAddress
        ? `${identityAnchorAddress.address.slice(0, 10)}...${identityAnchorAddress.address.slice(-8)}`
        : 'not-generated';

    useEffect(() => {
        setAddressPage(1);
    }, [addressSearch, addressUsageFilter, activeChain]);

    useEffect(() => {
        if (addressPage > totalAddressPages) {
            setAddressPage(totalAddressPages);
        }
    }, [addressPage, totalAddressPages]);

    const hasSessionError = [balancesError, utxosError, txError].some((msg) =>
        (msg || '').toLowerCase().includes('session') || (msg || '').toLowerCase().includes('unauth')
    );

    const walletPageVaultLocked =
        vaultStatus?.state === 'locked' ||
        vaultStatus?.state === 'locked_out' ||
        vaultStatus?.state === 'not_initialized';

    return (
        <AppShell activePage="wallet" vaultLocked={walletPageVaultLocked}>
            <div className="p-8">
                <PageHeader
                    title="Wallet Control Center"
                    subtitle="Addresses, UTXOs, reservations and operational control"
                />

                {/* Chain Tabs */}
                <div className="flex gap-2 mb-6">
                    <button
                        onClick={() => setActiveChain('btc')}
                        className={`flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-all ${activeChain === 'btc'
                                ? 'bg-[#F7931A]/20 text-[#F7931A] border border-[#F7931A]/30'
                                : 'bg-[#151B23] text-[#6C7A89] border border-[#242C36] hover:border-[#E7EDF5]/20'
                            }`}
                    >
                        <Bitcoin size={18} />
                        Bitcoin
                    </button>
                    <button
                        onClick={() => setActiveChain('liquid')}
                        className={`flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-all ${activeChain === 'liquid'
                                ? 'bg-[#00B4D8]/20 text-[#00B4D8] border border-[#00B4D8]/30'
                                : 'bg-[#151B23] text-[#6C7A89] border border-[#242C36] hover:border-[#E7EDF5]/20'
                            }`}
                    >
                        <Droplet size={18} />
                        Liquid
                    </button>
                </div>

                <div className="grid grid-cols-3 gap-4 mb-6">
                    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-5">
                        <div className="text-xs text-[#6C7A89] mb-1">{activeChain.toUpperCase()} confirmed</div>
                        <div className="text-2xl font-mono text-[#E7EDF5]">
                            {balancesLoading ? '...' : formatSats(balance?.confirmed || 0)}
                        </div>
                    </div>
                    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-5">
                        <div className="text-xs text-[#6C7A89] mb-1">Unconfirmed</div>
                        <div className="text-2xl font-mono text-[#F59E0B]">
                            {balancesLoading ? '...' : formatSats(balance?.unconfirmed || 0)}
                        </div>
                    </div>
                    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-5">
                        <div className="text-xs text-[#6C7A89] mb-1">Reserved (source of reservation)</div>
                        <div className="text-2xl font-mono text-[#00B4D8]">
                            {utxosLoading ? '...' : formatSats(totalReservedSat)}
                        </div>
                        <div className="text-xs text-[#6C7A89] mt-1">
                            UTXO set: {formatSats(totalUtxoSat)} {activeChain === 'btc' ? 'BTC' : 'L-BTC'}
                        </div>
                    </div>
                </div>

                {(balancesError || utxosError || txError) && (
                    <div className="mb-6 p-4 bg-[#EF4444]/10 border border-[#EF4444]/30 rounded-2xl">
                        <div className="text-sm text-[#FCA5A5] font-medium mb-2">
                            Wallet API error
                        </div>
                        {balancesError && <div className="text-xs text-[#FCA5A5]">balances: {balancesError}</div>}
                        {utxosError && <div className="text-xs text-[#FCA5A5]">utxos: {utxosError}</div>}
                        {txError && <div className="text-xs text-[#FCA5A5]">transactions: {txError}</div>}
                        <div className="mt-2 text-xs text-[#9AA7B5]">
                            {hasSessionError
                                ? 'Sessao expirada/invalida: desbloqueie o vault novamente para renovar o session_id.'
                                : 'Se o node BTC ainda esta sincronizando, saldo e UTXOs podem ficar zerados ate o bloco da transacao ser indexado.'}
                        </div>
                        <div className="mt-3 flex gap-2">
                            <SecondaryButton onClick={() => void refetchBalances()} disabled={balancesLoading}>
                                Refresh balances
                            </SecondaryButton>
                            <SecondaryButton onClick={() => void refetchUtxos()} disabled={utxosLoading}>
                                Refresh UTXOs
                            </SecondaryButton>
                            <SecondaryButton onClick={() => void refetchTx()} disabled={txLoading}>
                                Refresh tx
                            </SecondaryButton>
                        </div>
                    </div>
                )}

                <div className="mb-6 p-4 bg-[#00B4D8]/10 border border-[#00B4D8]/30 rounded-2xl">
                    <div className="text-sm text-[#B9EFFF] font-medium mb-1">Funding context</div>
                    <div className="text-xs text-[#9DD7E5]">
                        Wallet balance/UTXO aqui mostra apenas endereços derivados da wallet. Endereços de lockup de swap são externos ao saldo da wallet e aparecem no Swap Center.
                    </div>
                    {swapLockupAddresses.length > 0 && (
                        <div className="mt-2 text-xs text-[#9DD7E5] font-mono break-all">
                            Lockups ativos: {swapLockupAddresses.slice(0, 2).join(' | ')}{swapLockupAddresses.length > 2 ? ' ...' : ''}
                        </div>
                    )}
                </div>

                <div className="mb-6">
                    <TerminalCard
                        header={<h3 className="text-lg text-[#E7EDF5]">Wallet Identity</h3>}
                    >
                        <div className="grid grid-cols-2 gap-4">
                            <div className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                <div className="text-xs text-[#6C7A89]">Vault State</div>
                                <div className="text-sm text-[#E7EDF5] font-mono mt-1">{vaultStatus?.state || 'unknown'}</div>
                            </div>
                            <div className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                <div className="text-xs text-[#6C7A89]">Persisted Address Count ({activeChain.toUpperCase()})</div>
                                <div className="text-sm text-[#E7EDF5] font-mono mt-1">{combinedAddresses.length}</div>
                            </div>
                            <div className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                <div className="text-xs text-[#6C7A89]">Identity Anchor (first derived)</div>
                                <div className="text-sm text-[#E7EDF5] font-mono mt-1 break-all">
                                    {identityAnchorAddress?.address || 'none yet'}
                                </div>
                                <div className="text-xs text-[#6C7A89] mt-1">
                                    {identityAnchorAddress?.derivation_path || 'generate first address to anchor wallet identity'}
                                </div>
                            </div>
                            <div className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                <div className="text-xs text-[#6C7A89]">Wallet Fingerprint (UI)</div>
                                <div className="text-sm text-[#E7EDF5] font-mono mt-1">{walletFingerprint}</div>
                                <div className="text-xs text-[#6C7A89] mt-1">
                                    Source of truth: core DB (wallet_addresses + vault state)
                                </div>
                            </div>
                        </div>
                    </TerminalCard>
                </div>

                <div className="grid grid-cols-2 gap-6">
                    {/* Addresses */}
                    <TerminalCard
                        header={
                            <div className="flex items-center justify-between">
                                <h3 className="text-lg text-[#E7EDF5]">Address Control</h3>
                                <SecondaryButton onClick={handleGenerateAddress} disabled={addressLoading || addressesLoading}>
                                    <div className="flex items-center gap-2">
                                        <Plus size={16} />
                                        <span>{addressLoading ? 'Generating...' : 'New'}</span>
                                    </div>
                                </SecondaryButton>
                            </div>
                        }
                    >
                        <div className="space-y-3">
                            <div className="grid grid-cols-3 gap-2">
                                <div className="col-span-2 relative">
                                    <Search size={14} className="absolute left-3 top-3 text-[#6C7A89]" />
                                    <input
                                        value={addressSearch}
                                        onChange={(e) => setAddressSearch(e.target.value)}
                                        placeholder="Search address or derivation path"
                                        className="w-full pl-8 pr-3 py-2 bg-[#0D1117] border border-[#242C36] rounded-lg text-sm text-[#E7EDF5] placeholder-[#6C7A89]"
                                    />
                                </div>
                                <select
                                    value={addressUsageFilter}
                                    onChange={(e) => setAddressUsageFilter(e.target.value as 'all' | 'used' | 'unused')}
                                    className="bg-[#0D1117] border border-[#242C36] rounded-lg px-3 py-2 text-sm text-[#E7EDF5]"
                                >
                                    <option value="all">All</option>
                                    <option value="used">Used</option>
                                    <option value="unused">Unused</option>
                                </select>
                            </div>
                            <div className="text-xs text-[#6C7A89]">
                                Showing {pagedAddresses.length} of {filteredAddresses.length} addresses
                            </div>
                            {addressesLoading ? (
                                <div className="text-center py-8 text-[#6C7A89]">Loading addresses...</div>
                            ) : featuredLatestAddress && (
                                <div className="p-3 bg-[#151B23] border border-[#10B981]/30 rounded-xl">
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-[#10B981]">Latest</span>
                                        <button onClick={() => copyToClipboard(featuredLatestAddress.address)} className="text-[#6C7A89] hover:text-[#E7EDF5]" title="Copy address">
                                            <Copy size={14} />
                                        </button>
                                    </div>
                                    <div className="text-sm text-[#E7EDF5] font-mono break-all">
                                        {featuredLatestAddress.address}
                                    </div>
                                    <div className="text-xs text-[#6C7A89] mt-1">
                                        {featuredLatestAddress.derivation_path}
                                    </div>
                                </div>
                            )}
                            {pagedAddressesWithoutFeatured.map((addr) => (
                                <div key={`${addr.address}:${addr.derivation_path}`} className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-[#6C7A89]">{addr.derivation_path}</span>
                                        <div className="flex items-center gap-2">
                                            <span className={`text-[10px] px-2 py-0.5 rounded-full ${addr.used ? 'bg-[#F59E0B]/20 text-[#F59E0B]' : 'bg-[#10B981]/20 text-[#10B981]'}`}>
                                                {addr.used ? 'used' : 'unused'}
                                            </span>
                                            <button onClick={() => copyToClipboard(addr.address)} className="text-[#6C7A89] hover:text-[#E7EDF5]">
                                                <Copy size={14} />
                                            </button>
                                        </div>
                                    </div>
                                    <div className="text-sm text-[#E7EDF5] font-mono break-all">
                                        {addr.address}
                                    </div>
                                </div>
                            ))}
                            <div className="flex items-center justify-between">
                                <SecondaryButton onClick={() => setAddressPage((p) => Math.max(1, p - 1))} disabled={addressPage <= 1}>
                                    Prev
                                </SecondaryButton>
                                <span className="text-xs text-[#6C7A89]">Page {addressPage} / {totalAddressPages}</span>
                                <SecondaryButton onClick={() => setAddressPage((p) => Math.min(totalAddressPages, p + 1))} disabled={addressPage >= totalAddressPages}>
                                    Next
                                </SecondaryButton>
                            </div>
                            {!latestAddress && filteredAddresses.length === 0 && (
                                <div className="text-center py-8 text-[#6C7A89]">
                                    No persisted address yet for {activeChain.toUpperCase()}. Click "New" to generate the first identity anchor.
                                </div>
                            )}
                            {(generateAddressError || addressesError) && (
                                <div className="p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl text-xs text-[#FCA5A5]">
                                    Address error: {generateAddressError || addressesError}
                                    <div className="mt-1 text-[#9AA7B5]">
                                        Verifique se o api-bridge está atualizado/reiniciado e se o vault está desbloqueado.
                                    </div>
                                </div>
                            )}
                        </div>
                    </TerminalCard>

                    {/* Actions */}
                    <TerminalCard
                        header={<h3 className="text-lg text-[#E7EDF5]">Actions</h3>}
                    >
                        <div className="space-y-3">
                            <button
                                className="w-full p-4 bg-[#10B981]/10 border border-[#10B981]/30 rounded-xl hover:bg-[#10B981]/20 transition-all text-left"
                                onClick={handleGenerateAddress}
                            >
                                <div className="text-[#10B981] font-medium mb-1">Receive</div>
                                <div className="text-sm text-[#6C7A89]">Generate address to receive funds</div>
                            </button>
                            <button
                                className="w-full p-4 bg-[#00B4D8]/10 border border-[#00B4D8]/30 rounded-xl hover:bg-[#00B4D8]/20 transition-all text-left"
                                onClick={() => setShowSend(!showSend)}
                            >
                                <div className="text-[#00B4D8] font-medium mb-1">Send</div>
                                <div className="text-sm text-[#6C7A89]">Transfer funds to another address</div>
                            </button>
                            {showSend && (
                                <div className="p-4 bg-[#151B23] border border-[#242C36] rounded-xl space-y-3">
                                    <input
                                        className="w-full bg-[#0D1117] border border-[#242C36] rounded-lg px-3 py-2 text-sm text-[#E7EDF5] placeholder-[#6C7A89]"
                                        placeholder="Destination address"
                                        value={sendAddress}
                                        onChange={(e) => setSendAddress(e.target.value)}
                                    />
                                    <div className="grid grid-cols-2 gap-3">
                                        <input
                                            className="w-full bg-[#0D1117] border border-[#242C36] rounded-lg px-3 py-2 text-sm text-[#E7EDF5] placeholder-[#6C7A89]"
                                            placeholder={`Amount (${activeChain === 'btc' ? 'BTC' : 'L-BTC'})`}
                                            value={sendAmount}
                                            onChange={(e) => setSendAmount(e.target.value)}
                                        />
                                        <input
                                            className="w-full bg-[#0D1117] border border-[#242C36] rounded-lg px-3 py-2 text-sm text-[#E7EDF5] placeholder-[#6C7A89]"
                                            placeholder="Fee rate (sat/vB)"
                                            value={sendFeeRate}
                                            onChange={(e) => setSendFeeRate(e.target.value)}
                                        />
                                    </div>
                                    <PrimaryButton onClick={handleSend} disabled={sendLoading}>
                                        {sendLoading ? 'Sending...' : 'Send now'}
                                    </PrimaryButton>
                                    {sendError && (
                                        <div className="text-xs text-[#F87171]">
                                            {sendError}
                                        </div>
                                    )}
                                    {txid && (
                                        <div className="text-xs text-[#10B981] break-all">
                                            Sent: {txid}
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    </TerminalCard>
                </div>

                <div className="mt-6">
                    <TerminalCard
                        header={
                            <div className="flex items-center justify-between">
                                <h3 className="text-lg text-[#E7EDF5] flex items-center gap-2">
                                    <Database size={18} />
                                    UTXO & Reservation Control
                                </h3>
                                <SecondaryButton onClick={() => void refetchUtxos()} disabled={utxosLoading}>
                                    Refresh
                                </SecondaryButton>
                            </div>
                        }
                    >
                        {utxosLoading ? (
                            <div className="text-center py-6 text-[#6C7A89]">Loading UTXOs...</div>
                        ) : utxosError ? (
                            <div className="text-center py-6 text-[#FCA5A5]">
                                Failed to load UTXOs: {utxosError}
                            </div>
                        ) : utxos.length === 0 ? (
                            <div className="text-center py-6 text-[#6C7A89]">No UTXOs found for this chain</div>
                        ) : (
                            <div className="overflow-x-auto">
                                <table className="w-full text-sm">
                                    <thead>
                                        <tr className="text-left text-[#6C7A89] border-b border-[#242C36]">
                                            <th className="py-2 pr-3">UTXO</th>
                                            <th className="py-2 pr-3">Amount</th>
                                            <th className="py-2 pr-3">Conf</th>
                                            <th className="py-2 pr-3">Reservation Type</th>
                                            <th className="py-2 pr-3">Reserved By</th>
                                            <th className="py-2 pr-3">Swap ID</th>
                                            <th className="py-2 pr-3">Reserved At</th>
                                            <th className="py-2 pr-3">Expires At</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {utxos.map((u) => (
                                            <tr key={`${u.txid}:${u.vout}`} className="border-b border-[#1b222c]">
                                                <td className="py-2 pr-3 text-[#E7EDF5] font-mono">
                                                    {u.txid.slice(0, 10)}...:{u.vout}
                                                </td>
                                                <td className="py-2 pr-3 text-[#E7EDF5] font-mono">{u.amount_sat.toLocaleString()} sats</td>
                                                <td className="py-2 pr-3 text-[#9AA7B5]">{u.confirmations}</td>
                                                <td className="py-2 pr-3">
                                                    {u.reservation ? (
                                                        <span className="inline-flex items-center gap-1 text-[#F59E0B]">
                                                            <Lock size={12} />
                                                            {u.reservation.reservation_type}
                                                        </span>
                                                    ) : (
                                                        <span className="text-[#6C7A89]">none</span>
                                                    )}
                                                </td>
                                                <td className="py-2 pr-3 text-[#9AA7B5]">{u.reservation?.reserved_by || '-'}</td>
                                                <td className="py-2 pr-3 text-[#9AA7B5] font-mono">
                                                    {u.reservation?.swap_id ? `${u.reservation.swap_id.slice(0, 12)}...` : '-'}
                                                </td>
                                                <td className="py-2 pr-3 text-[#9AA7B5]">{u.reservation?.reserved_at || '-'}</td>
                                                <td className="py-2 pr-3 text-[#9AA7B5]">{u.reservation?.expires_at || '-'}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </TerminalCard>
                </div>

                <div className="mt-6 grid grid-cols-2 gap-6">
                    <TerminalCard
                        header={
                            <div className="flex items-center gap-2">
                                <ShieldAlert size={18} />
                                <h3 className="text-lg text-[#E7EDF5]">Security Control</h3>
                            </div>
                        }
                    >
                        <div className="space-y-3">
                            <div className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                <div className="text-xs text-[#6C7A89] mb-1">xpub view (masked by default)</div>
                                <div className="text-sm text-[#E7EDF5] font-mono break-all">{showXpub ? xpubValue : mockMaskedXpub}</div>
                                <div className="flex items-center gap-2 mt-3">
                                    <SecondaryButton
                                        onClick={() => {
                                            setShowXpub((v) => !v);
                                            if (!showXpub) setXpubRevealUntil(Date.now() + 15_000);
                                        }}
                                    >
                                        <span className="flex items-center gap-2">{showXpub ? <EyeOff size={14} /> : <Eye size={14} />}{showXpub ? 'Hide' : 'Reveal 15s'}</span>
                                    </SecondaryButton>
                                    <SecondaryButton onClick={() => copyToClipboard(mockMaskedXpub)}>Copy</SecondaryButton>
                                    <SecondaryButton onClick={() => setShowExportPanel((v) => !v)}>
                                        {showExportPanel ? 'Close Export' : 'Open Export'}
                                    </SecondaryButton>
                                </div>
                                <div className="text-xs text-[#F59E0B] mt-2">
                                    Export de chave privada desabilitado por padrão (guard rails de produção).
                                </div>
                                {showExportPanel && (
                                    <div className="mt-3 p-3 bg-[#0D1117] border border-[#EF4444]/40 rounded-xl space-y-2">
                                        <div className="text-xs text-[#FCA5A5]">
                                            Fluxo sensível: exige confirmação dupla. Digite <span className="font-mono">EXPORT</span> e PIN para continuar.
                                        </div>
                                        <input
                                            type="password"
                                            placeholder="PIN (mín. 8)"
                                            value={exportPin}
                                            onChange={(e) => setExportPin(e.target.value)}
                                            className="w-full px-3 py-2 bg-[#151B23] border border-[#242C36] rounded-lg text-sm text-[#E7EDF5]"
                                        />
                                        <input
                                            type="text"
                                            placeholder='Digite EXPORT para confirmar'
                                            value={exportConfirmText}
                                            onChange={(e) => setExportConfirmText(e.target.value)}
                                            className="w-full px-3 py-2 bg-[#151B23] border border-[#242C36] rounded-lg text-sm text-[#E7EDF5]"
                                        />
                                        <PrimaryButton
                                            disabled={!exportCanSubmit}
                                            onClick={() => {
                                                if (!exportEnabled) {
                                                    setExportMessage('Export desabilitado por feature flag (VITE_ENABLE_WALLET_EXPORT=1).');
                                                    return;
                                                }
                                                setExportMessage('Export ainda não implementado no backend. Guard rails já aplicados no frontend.');
                                            }}
                                        >
                                            Export (Guarded)
                                        </PrimaryButton>
                                        {exportMessage && <div className="text-xs text-[#F59E0B]">{exportMessage}</div>}
                                    </div>
                                )}
                            </div>
                        </div>
                    </TerminalCard>

                    <TerminalCard
                        header={
                            <div className="flex items-center justify-between">
                                <h3 className="text-lg text-[#E7EDF5] flex items-center gap-2">
                                    <History size={18} />
                                    Activity Timeline
                                </h3>
                                <SecondaryButton onClick={() => void refetchTx()} disabled={txLoading}>Refresh</SecondaryButton>
                            </div>
                        }
                    >
                        {txLoading ? (
                            <div className="text-center py-6 text-[#6C7A89]">Loading activity...</div>
                        ) : txError ? (
                            <div className="text-center py-6 text-[#FCA5A5]">Failed to load activity: {txError}</div>
                        ) : activityEvents.length === 0 ? (
                            <div className="text-center py-6 text-[#6C7A89]">No activity yet</div>
                        ) : (
                            <div className="space-y-2 max-h-[360px] overflow-auto">
                                {activityEvents.map((e) => (
                                    <div key={e.id} className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                        <div className="flex items-center justify-between">
                                            <div className="text-sm text-[#E7EDF5] font-medium">{e.event_type}</div>
                                            <div className="text-xs text-[#9AA7B5]">{e.status}</div>
                                        </div>
                                        <div className="text-xs text-[#6C7A89] font-mono mt-1">
                                            chain={e.chain} txid={e.txid ? `${e.txid.slice(0, 16)}...` : '-'} swap_id={e.swap_id ? `${e.swap_id.slice(0, 12)}...` : '-'}
                                        </div>
                                        <div className="text-xs text-[#6C7A89] mt-1">
                                            evidence_level={e.evidence_level} status={e.status} timestamp={e.timestamp || '-'}
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </TerminalCard>
                </div>
            </div>
        </AppShell>
    );
}
