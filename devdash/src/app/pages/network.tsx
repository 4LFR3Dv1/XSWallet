import { useRef, useEffect } from 'react';
import { Database, Zap, Server, Droplet, Activity, Network as NetworkIcon, Wifi } from 'lucide-react';
import { StatusCard } from '@/app/components/status-card';
import { ChannelGraph } from '@/app/components/channel-graph';
import { useLightningInfo, useLightningBalance, useBitcoinInfo, useBitcoinFees, useBitcoinMempool, useElementsInfo } from '@/app/services/hooks';
import { NetworkBadge } from '@/app/components/network-badge';

export function NetworkPage() {
    const { data: lndInfo, loading: lndLoading } = useLightningInfo();
    const { data: lndBalance } = useLightningBalance();
    const { data: btcInfo, loading: btcLoading } = useBitcoinInfo();
    const { data: btcFees } = useBitcoinFees();
    const { data: btcMempool } = useBitcoinMempool();
    const { data: elementsInfo, loading: elementsLoading } = useElementsInfo();

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-2xl font-bold text-white mb-2">Network Status</h2>
                <p className="text-sm text-[#666]">
                    Real-time metrics from Bitcoin, Lightning Network clusters, and Sidechains
                </p>
            </div>

            <div className="grid grid-cols-3 gap-6">
                {/* Bitcoin Node */}
                <StatusCard
                    icon={Database}
                    label="Bitcoin Node"
                    value={btcInfo ? 'Connected' : btcLoading ? 'Connecting...' : 'Offline'}
                    status={btcInfo ? 'ok' : btcLoading ? 'warn' : 'error'}
                    subtitle={btcInfo ? `Height: ${btcInfo.blocks?.toLocaleString()} • ${btcInfo.connections} peers` : 'No connection'}
                    accentColor="btc"
                />

                {/* Lightning Node */}
                <StatusCard
                    icon={Zap}
                    label="Lightning Node"
                    value={lndInfo ? lndInfo.alias || 'Connected' : lndLoading ? 'Connecting...' : 'Offline'}
                    status={lndInfo ? 'ok' : lndLoading ? 'warn' : 'error'}
                    subtitle={lndInfo ? `${lndInfo.num_active_channels} channels • ${lndInfo.num_peers} peers` : 'No connection'}
                    accentColor="ln"
                />

                {/* Liquid Node */}
                <StatusCard
                    icon={Droplet}
                    label="Liquid Sidechain"
                    value={elementsInfo ? 'Connected' : elementsLoading ? 'Connecting...' : 'Offline'}
                    status={elementsInfo ? 'ok' : elementsLoading ? 'warn' : 'error'}
                    subtitle={elementsInfo ? `Height: ${elementsInfo.blocks?.toLocaleString()}` : 'No connection'}
                    accentColor="lq"
                />
            </div>

            <div className="grid grid-cols-3 gap-6">
                {/* Channel Graph */}
                <div className="col-span-2 bg-[#111] border border-[#222] rounded-xl p-6 flex flex-col h-[320px]">
                    <div className="flex items-center justify-between mb-6">
                        <div className="flex items-center gap-3">
                            <div className="p-2 bg-[#7C3AED]/10 rounded-lg">
                                <NetworkIcon className="w-5 h-5 text-[#7C3AED]" />
                            </div>
                            <div>
                                <h3 className="font-semibold text-white">Channel Topology</h3>
                                <p className="text-xs text-[#666]">Visualizing {lndInfo?.num_active_channels || 0} active channels</p>
                            </div>
                        </div>
                        <div className="flex gap-2">
                            <span className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-[#7C3AED]/10 border border-[#7C3AED]/20 text-[10px] text-[#7C3AED]">
                                <div className="w-1.5 h-1.5 rounded-full bg-[#7C3AED]" />
                                Your Node
                            </span>
                            <span className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-[#222] border border-[#333] text-[10px] text-[#888]">
                                <div className="w-1.5 h-1.5 rounded-full bg-[#444]" />
                                Peers
                            </span>
                        </div>
                    </div>

                    <div className="flex-1 relative overflow-hidden rounded-lg bg-[#0d0d0d] border border-[#222]">
                        <ChannelGraph
                            numChannels={lndInfo?.num_active_channels || 5}
                            totalCapacityBTC={lndBalance ? lndBalance.local_balance / 100000000 : 0.05}
                        />
                    </div>
                </div>

                {/* Mempool Stats */}
                <div className="col-span-1 space-y-6">
                    <div className="bg-[#111] border border-[#222] rounded-xl p-6 h-full">
                        <div className="flex items-center gap-3 mb-6">
                            <div className="p-2 bg-[#F7931A]/10 rounded-lg">
                                <Activity className="w-5 h-5 text-[#F7931A]" />
                            </div>
                            <div>
                                <h3 className="font-semibold text-white">Mempool Status</h3>
                                <p className="text-xs text-[#666]">Real-time fee market</p>
                            </div>
                        </div>

                        <div className="space-y-4">
                            <div className="p-4 bg-[#0d0d0d] rounded-lg border border-[#222]">
                                <div className="flex justify-between items-center mb-2">
                                    <span className="text-xs text-[#666]">Fastest Fee (Next Block)</span>
                                    <Wifi className="w-4 h-4 text-[#10B981]" />
                                </div>
                                <div className="text-2xl font-bold text-white mb-1">
                                    {btcFees?.fastestFee || 15} <span className="text-sm font-normal text-[#666]">sat/vB</span>
                                </div>
                                <div className="w-full bg-[#222] h-1.5 rounded-full overflow-hidden">
                                    <div className="bg-[#10B981] h-full rounded-full" style={{ width: '35%' }} />
                                </div>
                            </div>

                            <div className="p-4 bg-[#0d0d0d] rounded-lg border border-[#222]">
                                <div className="flex justify-between items-center mb-2">
                                    <span className="text-xs text-[#666]">Minimum Relay</span>
                                    <Server className="w-4 h-4 text-[#666]" />
                                </div>
                                <div className="text-xl font-bold text-white mb-1">
                                    1.0 <span className="text-sm font-normal text-[#666]">sat/vB</span>
                                </div>
                                <p className="text-[10px] text-[#555]">Mempool Size: {btcMempool?.size_mb || 45} MB</p>
                            </div>

                            <div className="grid grid-cols-2 gap-3">
                                <div className="p-3 bg-[#0d0d0d] rounded-lg border border-[#222] text-center">
                                    <p className="text-[10px] text-[#666]">1h Fee</p>
                                    <p className="text-lg font-bold text-white">{btcFees?.hourFee || 5}</p>
                                </div>
                                <div className="p-3 bg-[#0d0d0d] rounded-lg border border-[#222] text-center">
                                    <p className="text-[10px] text-[#666]">Tx Count</p>
                                    <p className="text-lg font-bold text-white">{btcMempool?.tx_count.toLocaleString() || '---'}</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
