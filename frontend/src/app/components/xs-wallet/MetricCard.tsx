import React from 'react';

interface MetricCardProps {
  icon: React.ReactNode;
  asset: string;
  balance: string;
  fiat: string;
  delta?: string;
  deltaPositive?: boolean;
  accentColor?: string;
}

export function MetricCard({ icon, asset, balance, fiat, delta, deltaPositive, accentColor }: MetricCardProps) {
  return (
    <div className="bg-[#11151B] border border-[#242C36] rounded-2xl p-6 hover:border-[#242C36]/60 transition-all">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center`} style={{ backgroundColor: accentColor ? `${accentColor}15` : '#151B23' }}>
            <span style={{ color: accentColor }}>{icon}</span>
          </div>
          <span className="text-[#9AA7B5] text-sm">{asset}</span>
        </div>
        {delta && (
          <span className={`text-sm ${deltaPositive ? 'text-[#10B981]' : 'text-[#EF4444]'}`}>
            {deltaPositive ? '+' : ''}{delta}
          </span>
        )}
      </div>
      <div className="space-y-1">
        <div className="text-2xl text-[#E7EDF5] font-mono">{balance}</div>
        <div className="text-sm text-[#6C7A89]">{fiat}</div>
      </div>
    </div>
  );
}
