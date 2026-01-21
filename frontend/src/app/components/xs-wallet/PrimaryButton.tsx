import React from 'react';

interface ButtonProps {
  children: React.ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  className?: string;
  variant?: 'primary' | 'secondary' | 'destructive';
  fullWidth?: boolean;
}

export function PrimaryButton({ children, onClick, disabled, className = '', variant = 'primary', fullWidth }: ButtonProps) {
  const variants = {
    primary: 'bg-[#E7EDF5] text-[#0B0D10] hover:bg-[#E7EDF5]/90',
    secondary: 'bg-[#151B23] text-[#E7EDF5] border border-[#242C36] hover:bg-[#151B23]/80',
    destructive: 'bg-[#EF4444]/10 text-[#EF4444] border border-[#EF4444]/20 hover:bg-[#EF4444]/20',
  };

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`px-6 py-3 rounded-xl transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed ${
        variants[variant]
      } ${fullWidth ? 'w-full' : ''} ${className}`}
    >
      {children}
    </button>
  );
}

export const SecondaryButton = (props: ButtonProps) => <PrimaryButton {...props} variant="secondary" />;
export const DestructiveButton = (props: ButtonProps) => <PrimaryButton {...props} variant="destructive" />;
