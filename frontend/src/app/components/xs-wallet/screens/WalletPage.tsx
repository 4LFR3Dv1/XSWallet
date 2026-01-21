import React, { useState } from 'react';
import { Bitcoin, Droplet, Copy, Plus, ExternalLink } from 'lucide-react';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { useBalances, useNewAddress } from '@/services/hooks';

export function WalletPage() {
    const [activeChain, setActiveChain] = useState<'btc' | 'liquid'>('btc');
    const { balances, loading: balancesLoading } = useBalances();
    const { generate, loading: addressLoading, address } = useNewAddress();
    const [addresses, setAddresses] = useState<Array<{ address: string; path: string }>>([]);

    const handleGenerateAddress = async () => {
        const result = await generate(activeChain);
        if (result) {
            setAddresses([...addresses, { address: result.address, path: result.derivation_path }]);
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
    };

    const formatSats = (sats: number) => {
        return (sats / 100_000_000).toFixed(8);
    };

    const balance = activeChain === 'btc'
        ? balances?.btc
        : balances?.liquid;

    return (
        <AppShell activePage="wallet" vaultLocked={false}>
            <div className="p-8">
                <PageHeader
                    title="Wallet"
                    subtitle="Manage your addresses and UTXOs"
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

                {/* Balance Card */}
                <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-6 mb-6">
                    <div className="text-sm text-[#6C7A89] mb-2">
                        {activeChain === 'btc' ? 'Bitcoin' : 'Liquid'} Balance
                    </div>
                    <div className="text-3xl font-mono text-[#E7EDF5] mb-1">
                        {balancesLoading ? '...' : formatSats(balance?.confirmed || 0)} {activeChain === 'btc' ? 'BTC' : 'L-BTC'}
                    </div>
                    {balance?.unconfirmed ? (
                        <div className="text-sm text-[#F59E0B]">
                            + {formatSats(balance.unconfirmed)} unconfirmed
                        </div>
                    ) : null}
                </div>

                <div className="grid grid-cols-2 gap-6">
                    {/* Addresses */}
                    <TerminalCard
                        header={
                            <div className="flex items-center justify-between">
                                <h3 className="text-lg text-[#E7EDF5]">Addresses</h3>
                                <SecondaryButton onClick={handleGenerateAddress} disabled={addressLoading}>
                                    <div className="flex items-center gap-2">
                                        <Plus size={16} />
                                        <span>{addressLoading ? 'Generating...' : 'New'}</span>
                                    </div>
                                </SecondaryButton>
                            </div>
                        }
                    >
                        <div className="space-y-3">
                            {address && (
                                <div className="p-3 bg-[#151B23] border border-[#10B981]/30 rounded-xl">
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-[#10B981]">Latest</span>
                                        <button onClick={() => copyToClipboard(address.address)} className="text-[#6C7A89] hover:text-[#E7EDF5]">
                                            <Copy size={14} />
                                        </button>
                                    </div>
                                    <div className="text-sm text-[#E7EDF5] font-mono break-all">
                                        {address.address}
                                    </div>
                                    <div className="text-xs text-[#6C7A89] mt-1">
                                        {address.derivation_path}
                                    </div>
                                </div>
                            )}
                            {addresses.map((addr, i) => (
                                <div key={i} className="p-3 bg-[#151B23] border border-[#242C36] rounded-xl">
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-xs text-[#6C7A89]">{addr.path}</span>
                                        <button onClick={() => copyToClipboard(addr.address)} className="text-[#6C7A89] hover:text-[#E7EDF5]">
                                            <Copy size={14} />
                                        </button>
                                    </div>
                                    <div className="text-sm text-[#E7EDF5] font-mono break-all">
                                        {addr.address}
                                    </div>
                                </div>
                            ))}
                            {!address && addresses.length === 0 && (
                                <div className="text-center py-8 text-[#6C7A89]">
                                    Click "New" to generate an address
                                </div>
                            )}
                        </div>
                    </TerminalCard>

                    {/* Actions */}
                    <TerminalCard
                        header={<h3 className="text-lg text-[#E7EDF5]">Actions</h3>}
                    >
                        <div className="space-y-3">
                            <button className="w-full p-4 bg-[#10B981]/10 border border-[#10B981]/30 rounded-xl hover:bg-[#10B981]/20 transition-all text-left">
                                <div className="text-[#10B981] font-medium mb-1">Receive</div>
                                <div className="text-sm text-[#6C7A89]">Generate address to receive funds</div>
                            </button>
                            <button className="w-full p-4 bg-[#00B4D8]/10 border border-[#00B4D8]/30 rounded-xl hover:bg-[#00B4D8]/20 transition-all text-left">
                                <div className="text-[#00B4D8] font-medium mb-1">Send</div>
                                <div className="text-sm text-[#6C7A89]">Transfer funds to another address</div>
                            </button>
                        </div>
                    </TerminalCard>
                </div>
            </div>
        </AppShell>
    );
}
