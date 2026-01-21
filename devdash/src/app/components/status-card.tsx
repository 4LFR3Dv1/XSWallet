import { LucideIcon } from 'lucide-react';

// Color palette
const ACCENT_COLORS: Record<string, string> = {
  btc: '#F7931A',
  ln: '#9945FF',
  lq: '#00B4E6',
  green: '#10B981',
  red: '#EF4444',
  yellow: '#F59E0B',
};

interface StatusCardProps {
  icon: LucideIcon;
  label: string;
  value: string;
  status: 'ok' | 'warn' | 'error';
  subtitle?: string;
  accentColor?: keyof typeof ACCENT_COLORS | string;
  onClick?: () => void;
}

export function StatusCard({
  icon: Icon,
  label,
  value,
  status,
  subtitle,
  accentColor = 'green',
  onClick
}: StatusCardProps) {
  const color = ACCENT_COLORS[accentColor] || accentColor;

  const statusColors = {
    ok: 'bg-[#10B981] text-[#10B981]',
    warn: 'bg-[#F59E0B] text-[#F59E0B]',
    error: 'bg-[#EF4444] text-[#EF4444]',
  };

  const statusLabels = {
    ok: 'Active',
    warn: 'Warning',
    error: 'Error',
  };

  return (
    <div
      className={`bg-[#111] border border-[#222] rounded-xl p-5 space-y-3 ${onClick ? 'cursor-pointer hover:bg-[#161616] transition-colors' : ''}`}
      onClick={onClick}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div
            className="p-2 rounded-lg"
            style={{
              backgroundColor: `${color}1A`,
              color: color
            }}
          >
            <Icon className="w-5 h-5" />
          </div>
          <div>
            <h3 className="font-semibold text-white text-sm">{label}</h3>
            <div className="flex items-center gap-1.5 mt-0.5">
              <div className={`w-1.5 h-1.5 rounded-full ${statusColors[status].split(' ')[0]}`} />
              <span className={`text-xs ${statusColors[status].split(' ')[1]}`}>
                {statusLabels[status]}
              </span>
            </div>
          </div>
        </div>

        {status === 'ok' && (
          <div className="w-2 h-2 rounded-full animate-pulse" style={{ backgroundColor: color }} />
        )}
      </div>

      <div className="space-y-1">
        <p className="text-lg font-semibold text-white">{value}</p>
        {subtitle && (
          <p className="text-xs text-[#555] truncate" title={subtitle}>
            {subtitle}
          </p>
        )}
      </div>
    </div>
  );
}

