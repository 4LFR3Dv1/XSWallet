import React from 'react';

interface StatusChipProps {
  icon?: React.ReactNode;
  label: string;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'btc' | 'lightning' | 'liquid';
  className?: string;
}

export function StatusChip({ icon, label, variant = 'default', className = '' }: StatusChipProps) {
  const variantStyles = {
    default: 'bg-[#151B23] text-[#9AA7B5] border-[#242C36]',
    success: 'bg-[#10B981]/10 text-[#10B981] border-[#10B981]/20',
    warning: 'bg-[#F59E0B]/10 text-[#F59E0B] border-[#F59E0B]/20',
    error: 'bg-[#EF4444]/10 text-[#EF4444] border-[#EF4444]/20',
    btc: 'bg-[#F7931A]/10 text-[#F7931A] border-[#F7931A]/20',
    lightning: 'bg-[#FFD700]/10 text-[#FFD700] border-[#FFD700]/20',
    liquid: 'bg-[#00B4D8]/10 text-[#00B4D8] border-[#00B4D8]/20',
  };

  return (
    <div className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border text-sm ${variantStyles[variant]} ${className}`}>
      {icon && <span className="flex items-center">{icon}</span>}
      <span>{label}</span>
    </div>
  );
}
