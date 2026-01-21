import React from 'react';

interface TerminalCardProps {
  children: React.ReactNode;
  header?: React.ReactNode;
  className?: string;
}

export function TerminalCard({ children, header, className = '' }: TerminalCardProps) {
  return (
    <div className={`bg-[#11151B] border border-[#242C36] rounded-2xl shadow-lg ${className}`}>
      {header && (
        <div className="px-6 py-4 border-b border-[#242C36]">
          {header}
        </div>
      )}
      <div className="p-6">
        {children}
      </div>
    </div>
  );
}
