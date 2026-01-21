import { Activity, AlertTriangle, TrendingUp } from 'lucide-react';
import { useSystemHealth } from '@/app/services/hooks';

interface ObservabilityRibbonProps {
  latency?: string;
  errorRate?: string;
  throughput?: string;
}

export function ObservabilityRibbon({ latency: propLatency, errorRate: propErrorRate, throughput: propThroughput }: ObservabilityRibbonProps) {
  const { data: health } = useSystemHealth();

  // Use real data if available, fallback to props
  const latency = health?.latency || propLatency || '45ms';
  const errorRate = health?.error_rate || propErrorRate || '0.02%';
  const throughput = health?.throughput || propThroughput || '234 req/min';

  const metrics = [
    {
      icon: Activity,
      label: 'Latency',
      value: latency,
      sparkline: [12, 15, 14, 18, 16, 19, 17, 15, 14, 12, 13, 15]
    },
    {
      icon: AlertTriangle,
      label: 'Error Rate',
      value: errorRate,
      sparkline: [0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0]
    },
    {
      icon: TrendingUp,
      label: 'Throughput',
      value: throughput,
      sparkline: [45, 52, 48, 55, 58, 62, 59, 63, 61, 58, 55, 60]
    }
  ];

  const max = (data: number[]) => Math.max(...data);
  const min = (data: number[]) => Math.min(...data);

  const normalizeSparkline = (data: number[]) => {
    const maxVal = max(data);
    const minVal = min(data);
    const range = maxVal - minVal || 1;
    return data.map(v => ((v - minVal) / range) * 100);
  };

  return (
    <div className="border-b border-[#1f1f1f] bg-[#0d0d0d]">
      <div className="max-w-[1440px] mx-auto px-6 py-2">
        <div className="flex items-center justify-between gap-8">
          {metrics.map((metric, index) => {
            const normalized = normalizeSparkline(metric.sparkline);
            return (
              <div key={index} className="flex items-center gap-3">
                <metric.icon className="w-4 h-4 text-[#444] flex-shrink-0" />
                <div className="min-w-0">
                  <p className="text-[10px] text-[#555] uppercase tracking-wider font-medium">
                    {metric.label}
                  </p>
                  <p className="text-sm font-semibold text-white">{metric.value}</p>
                </div>
                <svg width="80" height="24" className="flex-shrink-0">
                  <defs>
                    <linearGradient id={`sparkline-gradient-${index}`} x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="white" stopOpacity="0.3" />
                      <stop offset="100%" stopColor="white" stopOpacity="0" />
                    </linearGradient>
                  </defs>
                  {/* Fill area */}
                  <path
                    d={`M0,24 ${normalized.map((v, i) => `L${(i / (normalized.length - 1)) * 80},${24 - (v / 100) * 20}`).join(' ')} L80,24 Z`}
                    fill={`url(#sparkline-gradient-${index})`}
                  />
                  {/* Line */}
                  <polyline
                    points={normalized.map((v, i) => `${(i / (normalized.length - 1)) * 80},${24 - (v / 100) * 20}`).join(' ')}
                    fill="none"
                    stroke="white"
                    strokeWidth="1.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    opacity="0.6"
                  />
                </svg>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
