import { Server, Zap, Droplet, Activity, Wallet, FileCode, ArrowLeftRight } from 'lucide-react';
import { StatusCard } from '@/app/components/status-card';
import { TimelineDock } from '@/app/components/timeline-dock';
import { useAllNodeStatuses, useSystemHealth, useRecentEvents, useSystemMetrics } from '@/app/services/hooks';
import { mockTimelineEvents } from '@/app/data/mock-data';

interface HomeProps {
  onNavigate: (page: string) => void;
}

export function HomePage({ onNavigate }: HomeProps) {
  const { data: nodeStatus, loading: statusLoading, error: statusError } = useAllNodeStatuses();
  const { data: health } = useSystemHealth();
  const { data: events, loading: eventsLoading } = useRecentEvents();
  const { data: metrics, loading: metricsLoading } = useSystemMetrics();

  // Use real data if available, fallback to mock
  const lndStatus = nodeStatus?.lnd || { synced: false, block_height: 0, peers: 0 };
  const btcStatus = nodeStatus?.bitcoin || { synced: false, block_height: 0, peers: 0 };
  const elementsStatus = nodeStatus?.elements || { synced: false, block_height: 0, peers: 0 };
  const apiHealthy = nodeStatus?.apiHealthy || false;

  // Use real events or fallback to mock
  const timelineEvents = events || mockTimelineEvents;

  return (
    <div className="space-y-8">
      {/* System Status */}
      <section>
        <h3 className="text-xs font-semibold text-[#555] mb-4 uppercase tracking-widest flex items-center gap-2">
          System Status
          {statusLoading && <span className="text-[#F59E0B] text-[10px]">(Loading...)</span>}
          {statusError && <span className="text-[#EF4444] text-[10px]">(API Offline - Using Mock)</span>}
        </h3>
        <div className="grid grid-cols-4 gap-4">
          <StatusCard
            icon={Zap}
            label="LND Node"
            value={lndStatus.synced ? 'Synced' : 'Syncing'}
            status={lndStatus.synced ? 'ok' : 'warn'}
            subtitle={`Height: ${lndStatus.block_height.toLocaleString()} • ${lndStatus.peers} Peers`}
            accentColor="ln"
          />
          <StatusCard
            icon={Server}
            label="Bitcoin Core"
            value={btcStatus.synced ? 'Synced' : 'Syncing'}
            status={btcStatus.synced ? 'ok' : 'warn'}
            subtitle={`Height: ${btcStatus.block_height.toLocaleString()} • ${btcStatus.peers} Peers`}
            accentColor="btc"
          />
          <StatusCard
            icon={Droplet}
            label="Elements (Liquid)"
            value={elementsStatus.synced ? 'Synced' : 'Syncing'}
            status={elementsStatus.synced ? 'ok' : 'warn'}
            subtitle={`Height: ${elementsStatus.block_height.toLocaleString()} • ${elementsStatus.peers} Peers`}
            accentColor="lq"
          />
          <StatusCard
            icon={Activity}
            label="API Service"
            value={apiHealthy ? 'Healthy' : 'Offline'}
            status={apiHealthy ? 'ok' : 'error'}
            subtitle={health ? `Uptime: ${nodeStatus?.uptime} • ${nodeStatus?.latency_ms}ms avg` : 'Disconnected'}
            accentColor="green"
          />
        </div>
      </section>

      {/* Quick Actions */}
      <section>
        <h3 className="text-xs font-semibold text-[#555] mb-4 uppercase tracking-widest">Quick Actions</h3>
        <div className="grid grid-cols-3 gap-4">
          <button
            onClick={() => onNavigate('wallet')}
            className="bg-[#111] border border-[#222] rounded-xl p-5 hover:border-[#333] transition-all hover:bg-[#151515] group text-left"
          >
            <div className="flex items-center gap-4">
              <div className="bg-[#1a1a1a] group-hover:bg-[#222] p-3.5 rounded-xl transition-colors">
                <Wallet className="w-6 h-6 text-white" />
              </div>
              <div>
                <p className="text-sm font-semibold text-white">Generate Wallet</p>
                <p className="text-xs text-[#555] mt-0.5">Create new HD wallet</p>
              </div>
            </div>
          </button>

          <button
            onClick={() => onNavigate('htlc')}
            className="bg-[#111] border border-[#222] rounded-xl p-5 hover:border-[#333] transition-all hover:bg-[#151515] group text-left"
          >
            <div className="flex items-center gap-4">
              <div className="bg-[#1a1a1a] group-hover:bg-[#222] p-3.5 rounded-xl transition-colors">
                <FileCode className="w-6 h-6 text-white" />
              </div>
              <div>
                <p className="text-sm font-semibold text-white">Create HTLC</p>
                <p className="text-xs text-[#555] mt-0.5">Build HTLC script</p>
              </div>
            </div>
          </button>

          <button
            onClick={() => onNavigate('swap')}
            className="bg-[#111] border border-[#222] rounded-xl p-5 hover:border-[#333] transition-all hover:bg-[#151515] group text-left"
          >
            <div className="flex items-center gap-4">
              <div className="bg-[#1a1a1a] group-hover:bg-[#222] p-3.5 rounded-xl transition-colors">
                <ArrowLeftRight className="w-6 h-6 text-white" />
              </div>
              <div>
                <p className="text-sm font-semibold text-white">New Swap</p>
                <p className="text-xs text-[#555] mt-0.5">Start atomic swap</p>
              </div>
            </div>
          </button>
        </div>
      </section>

      {/* System Metrics */}
      <section>
        <h3 className="text-xs font-semibold text-[#555] mb-4 uppercase tracking-widest flex items-center gap-2">
          System Metrics
          {metricsLoading && <span className="text-[#F59E0B] text-[10px]">(Loading...)</span>}
          {!metrics && !metricsLoading && <span className="text-[#EF4444] text-[10px]">(Using Mock Data)</span>}
        </h3>
        <div className="grid grid-cols-3 gap-6">
          <div className="bg-[#111] border border-[#222] rounded-xl p-6">
            <p className="text-[10px] text-[#444] uppercase tracking-widest mb-2">Total Swaps (24h)</p>
            <p className="text-4xl font-bold text-white tracking-tight">{metrics?.total_swaps_24h || 0}</p>
            <p className="text-xs text-[#10B981] mt-2">{metrics?.swap_change_percent || '0%'} from yesterday</p>
          </div>
          <div className="bg-[#111] border border-[#222] rounded-xl p-6">
            <p className="text-[10px] text-[#444] uppercase tracking-widest mb-2">Active HTLCs</p>
            <p className="text-4xl font-bold text-white tracking-tight">{metrics?.active_htlcs || 0}</p>
            <p className="text-xs text-[#444] mt-2">Across all chains</p>
          </div>
          <div className="bg-[#111] border border-[#222] rounded-xl p-6">
            <p className="text-[10px] text-[#444] uppercase tracking-widest mb-2">Volume (BTC)</p>
            <p className="text-4xl font-bold text-white tracking-tight">{metrics?.volume_btc?.toFixed(4) || '0.0000'}</p>
            <p className="text-xs text-[#444] mt-2">Last 24 hours</p>
          </div>
        </div>
      </section>

      {/* Activity Feed */}
      <section>
        <h3 className="text-xs font-semibold text-[#555] mb-4 uppercase tracking-widest flex items-center gap-2">
          Activity Feed
          {eventsLoading && <span className="text-[#F59E0B] text-[10px]">(Loading...)</span>}
          {!events && !eventsLoading && <span className="text-[#EF4444] text-[10px]">(Using Mock Data)</span>}
        </h3>
        <TimelineDock events={timelineEvents} />
      </section>
    </div>
  );
}
