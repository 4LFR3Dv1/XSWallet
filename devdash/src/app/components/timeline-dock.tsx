import { useState } from 'react';
import { ArrowLeftRight, FileCode, Wallet, Network, Code, Info, AlertTriangle, AlertCircle } from 'lucide-react';
import { TimelineEvent } from '@/app/types';

interface TimelineDockProps {
  events: TimelineEvent[];
}

const categoryIcons = {
  swap: ArrowLeftRight,
  htlc: FileCode,
  wallet: Wallet,
  network: Network,
  api: Code
};

const categoryColors = {
  swap: 'bg-[#9945FF]/15 text-[#9945FF]',
  htlc: 'bg-[#F7931A]/15 text-[#F7931A]',
  wallet: 'bg-[#10B981]/15 text-[#10B981]',
  network: 'bg-[#00B4E6]/15 text-[#00B4E6]',
  api: 'bg-[#666]/15 text-[#888]'
};

export function TimelineDock({ events }: TimelineDockProps) {
  const [filter, setFilter] = useState<string>('All');
  const categories = ['All', 'WALLET', 'HTLC', 'SWAP', 'NETWORK', 'API'];

  const filteredEvents = filter === 'All'
    ? events
    : events.filter(e => e.category.toUpperCase() === filter);

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  return (
    <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
      {/* Header with Filter Tabs */}
      <div className="border-b border-[#222] px-5 py-3 flex items-center gap-4">
        <span className="text-xs font-semibold text-[#555] uppercase tracking-widest">Recent Activity</span>
        <div className="flex-1" />
        <div className="flex items-center gap-1">
          {categories.map(cat => (
            <button
              key={cat}
              onClick={() => setFilter(cat)}
              className={`px-3 py-1.5 text-[10px] font-medium rounded-lg transition-all uppercase tracking-wider ${filter === cat
                  ? 'bg-white text-black'
                  : 'text-[#555] hover:text-white hover:bg-[#1a1a1a]'
                }`}
            >
              {cat}
            </button>
          ))}
        </div>
      </div>

      {/* Events List */}
      <div className="divide-y divide-[#1a1a1a] max-h-[320px] overflow-y-auto">
        {filteredEvents.map(event => {
          const CategoryIcon = categoryIcons[event.category as keyof typeof categoryIcons] || Code;
          const catColor = categoryColors[event.category as keyof typeof categoryColors] || categoryColors.api;

          return (
            <div
              key={event.id}
              className="px-5 py-4 hover:bg-[#151515] transition-colors flex items-center gap-4 cursor-pointer"
            >
              <div className={`p-2.5 rounded-lg ${catColor}`}>
                <CategoryIcon className="w-4 h-4" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm text-white">{event.message}</p>
                <p className="text-[10px] text-[#444] mt-1">{formatTime(event.timestamp)}</p>
              </div>
              <span className={`px-2.5 py-1 text-[9px] font-semibold rounded-md uppercase tracking-wider ${catColor}`}>
                {event.category}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
