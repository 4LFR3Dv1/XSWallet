import { useState } from 'react';
import { CopyButton } from './copy-button';

interface AddressDisplayProps {
  address: string;
  label?: string;
  showFull?: boolean;
}

export function AddressDisplay({ address, label, showFull = false }: AddressDisplayProps) {
  const [expanded, setExpanded] = useState(showFull);

  const truncated = address.length > 16
    ? `${address.slice(0, 8)}...${address.slice(-8)}`
    : address;

  return (
    <div className="flex items-center gap-2 bg-secondary border border-border rounded-[8px] px-3 py-2">
      {label && (
        <span className="text-xs text-muted-foreground font-medium">{label}:</span>
      )}
      <code
        className="flex-1 text-xs font-mono text-foreground cursor-pointer select-all"
        onClick={() => setExpanded(!expanded)}
        title={expanded ? 'Click to collapse' : 'Click to expand'}
      >
        {expanded ? address : truncated}
      </code>
      <CopyButton text={address} />
    </div>
  );
}
