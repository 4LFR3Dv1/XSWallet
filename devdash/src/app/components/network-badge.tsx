type NetworkType = 'BTC' | 'LN' | 'L-BTC' | 'Liquid';

interface NetworkBadgeProps {
  network: NetworkType;
  size?: 'sm' | 'md';
}

export function NetworkBadge({ network, size = 'md' }: NetworkBadgeProps) {
  const colors = {
    'BTC': 'bg-[#F7931A]/10 text-[#F7931A] border-[#F7931A]/20',
    'LN': 'bg-[#9945FF]/10 text-[#9945FF] border-[#9945FF]/20',
    'L-BTC': 'bg-[#00B4E6]/10 text-[#00B4E6] border-[#00B4E6]/20',
    'Liquid': 'bg-[#00B4E6]/10 text-[#00B4E6] border-[#00B4E6]/20'
  };

  const sizeClass = size === 'sm' ? 'px-2 py-0.5 text-[10px]' : 'px-2.5 py-1 text-xs';

  return (
    <span className={`inline-flex items-center font-medium rounded-[4px] border ${colors[network]} ${sizeClass}`}>
      {network}
    </span>
  );
}
