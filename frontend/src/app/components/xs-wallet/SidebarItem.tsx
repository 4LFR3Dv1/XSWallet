import React from 'react';

interface SidebarItemProps {
  icon: React.ReactNode;
  label: string;
  active?: boolean;
  onClick?: () => void;
}

export function SidebarItem({ icon, label, active = false, onClick }: SidebarItemProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 ${
        active
          ? 'bg-[#151B23] text-[#E7EDF5]'
          : 'text-[#9AA7B5] hover:bg-[#151B23]/50 hover:text-[#E7EDF5]'
      }`}
    >
      <span className="flex items-center">{icon}</span>
      <span>{label}</span>
      {active && (
        <div className="ml-auto w-1.5 h-1.5 rounded-full bg-[#E7EDF5]" />
      )}
    </button>
  );
}
