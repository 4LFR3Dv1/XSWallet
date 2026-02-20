import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Home, Repeat, Wallet, Network, Settings, Lock, Unlock, CheckCircle } from 'lucide-react';
import { SidebarItem } from './SidebarItem';
import { StatusChip } from './StatusChip';
import { useBitcoinInfo, useSystemHealth } from '@/services/hooks';

interface AppShellProps {
  children: React.ReactNode;
  activePage?: string;
  vaultLocked?: boolean;
}

export function AppShell({ children, activePage, vaultLocked = false }: AppShellProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { info: bitcoinInfo } = useBitcoinInfo();
  const { health } = useSystemHealth();

  // Auto-detect active page from location if not provided
  const currentPage = activePage || location.pathname.replace('/', '') || 'home';

  const navItems = [
    { id: 'home', path: '/', icon: Home, label: 'Home' },
    { id: 'swap', path: '/swap', icon: Repeat, label: 'Swap' },
    { id: 'wallet', path: '/wallet', icon: Wallet, label: 'Wallet' },
    { id: 'network', path: '/network', icon: Network, label: 'Network' },
    { id: 'settings', path: '/settings', icon: Settings, label: 'Settings' },
  ];

  return (
    <div className="flex h-screen bg-[#0B0D10] text-[#E7EDF5]">
      {/* Sidebar */}
      <div className="w-60 border-r border-[#242C36] flex flex-col">
        {/* Logo */}
        <div className="px-6 py-6 border-b border-[#242C36]">
          <h1 className="text-xl text-[#E7EDF5]">XS Wallet</h1>
          <p className="text-xs text-[#6C7A89] mt-1">Enterprise Terminal</p>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-4 space-y-1">
          {navItems.map((item) => (
            <SidebarItem
              key={item.id}
              icon={<item.icon size={20} />}
              label={item.label}
              active={currentPage === item.id || (item.id === 'home' && currentPage === '')}
              onClick={() => navigate(item.path)}
            />
          ))}
        </nav>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col">
        <div className="flex-1 overflow-auto">
          {children}
        </div>

        {/* Status Bar */}
        <div className="h-11 border-t border-[#242C36] px-6 flex items-center gap-4 bg-[#0B0D10]">
          <StatusChip
            icon={vaultLocked ? <Lock size={14} /> : <Unlock size={14} />}
            label={vaultLocked ? 'Locked' : 'Unlocked'}
            variant={vaultLocked ? 'warning' : 'success'}
          />
          <StatusChip
            label={bitcoinInfo ? `BTC: ${bitcoinInfo.blocks.toLocaleString()}` : 'BTC: ...'}
            variant="btc"
          />
          <StatusChip
            icon={<CheckCircle size={14} />}
            label={health?.services.xscore === 'connected' ? 'gRPC: OK' : 'gRPC: ✗'}
            variant={health?.services.xscore === 'connected' ? 'success' : 'error'}
          />
          <StatusChip
            label={bitcoinInfo?.chain || 'unknown'}
            variant="default"
          />
        </div>
      </div>
    </div>
  );
}
