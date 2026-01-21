import { useState } from 'react';
import { Home, Wallet, FileCode, ArrowLeftRight, Network, Code, Settings, ChevronLeft, ChevronRight } from 'lucide-react';

type Page = 'home' | 'wallet' | 'htlc' | 'swap' | 'network' | 'api' | 'settings';

interface SidebarProps {
  currentPage: Page;
  onPageChange: (page: Page) => void;
}

export function Sidebar({ currentPage, onPageChange }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false);

  const menuItems = [
    { id: 'home' as Page, icon: Home, label: 'Overview' },
    { id: 'wallet' as Page, icon: Wallet, label: 'Wallet Studio' },
    { id: 'htlc' as Page, icon: FileCode, label: 'HTLC Laboratory' },
    { id: 'swap' as Page, icon: ArrowLeftRight, label: 'Swap Center' },
    { id: 'network' as Page, icon: Network, label: 'Network' },
    { id: 'api' as Page, icon: Code, label: 'API Explorer' },
    { id: 'settings' as Page, icon: Settings, label: 'Settings' }
  ];

  return (
    <div className="h-full bg-[#0a0a0a] border-r border-[#1f1f1f] flex flex-col">
      {/* Header */}
      <div className="h-14 px-5 border-b border-[#1f1f1f] flex items-center justify-between flex-shrink-0">
        {!collapsed && (
          <h1 className="text-base font-bold text-white tracking-tight">BRLN-OS</h1>
        )}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="p-2 rounded-lg hover:bg-[#1a1a1a] text-[#666] hover:text-white transition-colors"
        >
          {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
        {menuItems.map(item => (
          <button
            key={item.id}
            onClick={() => onPageChange(item.id)}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 ${currentPage === item.id
                ? 'bg-white text-black font-medium shadow-lg shadow-white/5'
                : 'text-[#777] hover:text-white hover:bg-[#151515]'
              }`}
            title={collapsed ? item.label : undefined}
          >
            <item.icon className="w-5 h-5 flex-shrink-0" />
            {!collapsed && (
              <span className="text-sm">{item.label}</span>
            )}
          </button>
        ))}
      </nav>

      {/* Footer */}
      <div className="p-4 border-t border-[#1f1f1f] flex-shrink-0">
        {!collapsed && (
          <div className="text-xs text-[#444]">
            <p className="font-medium">v1.0.0-beta</p>
            <p>DevDash</p>
          </div>
        )}
      </div>
    </div>
  );
}
