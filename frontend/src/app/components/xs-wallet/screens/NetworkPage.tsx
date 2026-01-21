import React from 'react';
import { Server, Wifi, WifiOff, Activity, TrendingUp, TrendingDown } from 'lucide-react';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { TerminalCard } from '../TerminalCard';
import { StatusChip } from '../StatusChip';
import { useBitcoinInfo, useFeeEstimates, useSystemHealth } from '@/services/hooks';

export function NetworkPage() {
    const { info: bitcoinInfo, loading: btcLoading } = useBitcoinInfo();
    const { fees, loading: feesLoading } = useFeeEstimates();
    const { health, loading: healthLoading } = useSystemHealth();

    return (
        <AppShell activePage="network" vaultLocked={false}>
            <div className="p-8">
                <PageHeader
                    title="Network Status"
                    subtitle="Monitor connected services and blockchain"
                />

                {/* Services Grid */}
                <div className="grid grid-cols-3 gap-4 mb-8">
                    {/* XSCore */}
                    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-6">
                        <div className="flex items-center justify-between mb-4">
                            <div className="w-12 h-12 rounded-xl bg-[#10B981]/10 flex items-center justify-center">
                                <Server size={24} className="text-[#10B981]" />
                            </div>
                            <StatusChip
                                label={health?.services.xscore === 'connected' ? 'Online' : 'Offline'}
                                variant={health?.services.xscore === 'connected' ? 'green' : 'error'}
                            />
                        </div>
                        <div className="text-lg text-[#E7EDF5] mb-1">XSCore</div>
                        <div className="text-sm text-[#6C7A89]">Port 9735</div>
                    </div>

                    {/* Bitcoind */}
                    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-6">
                        <div className="flex items-center justify-between mb-4">
                            <div className="w-12 h-12 rounded-xl bg-[#F7931A]/10 flex items-center justify-center">
                                {health?.services.bitcoind === 'connected' ? (
                                    <Wifi size={24} className="text-[#F7931A]" />
                                ) : (
                                    <WifiOff size={24} className="text-[#EF4444]" />
                                )}
                            </div>
                            <StatusChip
                                label={health?.services.bitcoind === 'connected' ? 'Synced' : 'Offline'}
                                variant={health?.services.bitcoind === 'connected' ? 'btc' : 'error'}
                            />
                        </div>
                        <div className="text-lg text-[#E7EDF5] mb-1">Bitcoin</div>
                        <div className="text-sm text-[#6C7A89]">
                            {bitcoinInfo ? `Block ${bitcoinInfo.blocks.toLocaleString()}` : 'Port 18443'}
                        </div>
                    </div>

                    {/* Vault */}
                    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-6">
                        <div className="flex items-center justify-between mb-4">
                            <div className="w-12 h-12 rounded-xl bg-[#E7EDF5]/10 flex items-center justify-center">
                                <Activity size={24} className="text-[#E7EDF5]" />
                            </div>
                            <StatusChip
                                label={health?.vault === 'unlocked' ? 'Unlocked' : 'Locked'}
                                variant={health?.vault === 'unlocked' ? 'green' : 'pending'}
                            />
                        </div>
                        <div className="text-lg text-[#E7EDF5] mb-1">Vault</div>
                        <div className="text-sm text-[#6C7A89]">
                            {health?.vault || 'Unknown'}
                        </div>
                    </div>
                </div>

                <div className="grid grid-cols-2 gap-6">
                    {/* Bitcoin Info */}
                    <TerminalCard
                        header={<h3 className="text-lg text-[#E7EDF5]">Bitcoin Network</h3>}
                    >
                        {btcLoading ? (
                            <div className="text-[#6C7A89]">Loading...</div>
                        ) : bitcoinInfo ? (
                            <div className="space-y-3">
                                <div className="flex justify-between py-2 border-b border-[#242C36]">
                                    <span className="text-[#6C7A89]">Chain</span>
                                    <span className="text-[#E7EDF5] font-mono">{bitcoinInfo.chain}</span>
                                </div>
                                <div className="flex justify-between py-2 border-b border-[#242C36]">
                                    <span className="text-[#6C7A89]">Block Height</span>
                                    <span className="text-[#E7EDF5] font-mono">{bitcoinInfo.blocks.toLocaleString()}</span>
                                </div>
                                <div className="flex justify-between py-2 border-b border-[#242C36]">
                                    <span className="text-[#6C7A89]">Headers</span>
                                    <span className="text-[#E7EDF5] font-mono">{bitcoinInfo.headers.toLocaleString()}</span>
                                </div>
                                <div className="flex justify-between py-2 border-b border-[#242C36]">
                                    <span className="text-[#6C7A89]">Difficulty</span>
                                    <span className="text-[#E7EDF5] font-mono">{bitcoinInfo.difficulty}</span>
                                </div>
                                <div className="flex justify-between py-2">
                                    <span className="text-[#6C7A89]">Best Block</span>
                                    <span className="text-[#E7EDF5] font-mono text-xs">
                                        {bitcoinInfo.bestblockhash?.slice(0, 16)}...
                                    </span>
                                </div>
                            </div>
                        ) : (
                            <div className="text-[#EF4444]">Failed to connect to bitcoind</div>
                        )}
                    </TerminalCard>

                    {/* Fee Estimates */}
                    <TerminalCard
                        header={<h3 className="text-lg text-[#E7EDF5]">Fee Estimates</h3>}
                    >
                        {feesLoading ? (
                            <div className="text-[#6C7A89]">Loading...</div>
                        ) : fees ? (
                            <div className="space-y-4">
                                <div className="p-4 bg-[#10B981]/10 border border-[#10B981]/30 rounded-xl">
                                    <div className="flex items-center justify-between mb-2">
                                        <div className="flex items-center gap-2">
                                            <TrendingUp size={18} className="text-[#10B981]" />
                                            <span className="text-[#10B981] font-medium">Fast</span>
                                        </div>
                                        <span className="text-[#E7EDF5] font-mono">{fees.fast} sat/vB</span>
                                    </div>
                                    <div className="text-xs text-[#6C7A89]">~1 block confirmation</div>
                                </div>
                                <div className="p-4 bg-[#F59E0B]/10 border border-[#F59E0B]/30 rounded-xl">
                                    <div className="flex items-center justify-between mb-2">
                                        <div className="flex items-center gap-2">
                                            <Activity size={18} className="text-[#F59E0B]" />
                                            <span className="text-[#F59E0B] font-medium">Medium</span>
                                        </div>
                                        <span className="text-[#E7EDF5] font-mono">{fees.medium} sat/vB</span>
                                    </div>
                                    <div className="text-xs text-[#6C7A89]">~6 blocks confirmation</div>
                                </div>
                                <div className="p-4 bg-[#6C7A89]/10 border border-[#6C7A89]/30 rounded-xl">
                                    <div className="flex items-center justify-between mb-2">
                                        <div className="flex items-center gap-2">
                                            <TrendingDown size={18} className="text-[#6C7A89]" />
                                            <span className="text-[#9AA7B5] font-medium">Slow</span>
                                        </div>
                                        <span className="text-[#E7EDF5] font-mono">{fees.slow} sat/vB</span>
                                    </div>
                                    <div className="text-xs text-[#6C7A89]">~24 blocks confirmation</div>
                                </div>
                            </div>
                        ) : (
                            <div className="text-[#6C7A89]">No fee data available</div>
                        )}
                    </TerminalCard>
                </div>
            </div>
        </AppShell>
    );
}
