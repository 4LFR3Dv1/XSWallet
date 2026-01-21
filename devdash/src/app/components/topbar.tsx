import { Search, User, Sun, Moon, Settings } from 'lucide-react';
import { NetworkEnv } from '@/app/types';
import { useTheme } from '@/app/context/theme-context';

interface TopbarProps {
  network: NetworkEnv;
  onNetworkChange: (network: NetworkEnv) => void;
}

export function Topbar({ network, onNetworkChange }: TopbarProps) {
  const { theme, toggleTheme } = useTheme();

  const networkColors = {
    mainnet: 'bg-[#10B981]/10 text-[#10B981] border-[#10B981]/30',
    testnet: 'bg-[#F59E0B]/10 text-[#F59E0B] border-[#F59E0B]/30',
    regtest: 'bg-[#EF4444]/10 text-[#EF4444] border-[#EF4444]/30'
  };

  const handleSearch = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formData = new FormData(e.currentTarget);
    const query = formData.get('search');
    console.log('Search:', query);
    // TODO: Implement search functionality
  };

  const handleSettings = () => {
    console.log('Settings clicked');
    // TODO: Navigate to settings or open settings modal
  };

  const handleProfile = () => {
    console.log('Profile clicked');
    // TODO: Open profile menu or navigate to profile
  };

  return (
    <div className="bg-[#0a0a0a] border-b border-[#1f1f1f] px-6 py-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h2 className="text-lg font-bold text-white tracking-tight">BRLN-OS DevDash</h2>
          <div className="flex items-center gap-2">
            <select
              value={network}
              onChange={(e) => onNetworkChange(e.target.value as NetworkEnv)}
              className="bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg px-3 py-1.5 text-sm text-white cursor-pointer focus:outline-none focus:ring-2 focus:ring-white/10 hover:border-[#3a3a3a] transition-colors"
            >
              <option value="mainnet">Mainnet</option>
              <option value="testnet">Testnet</option>
              <option value="regtest">Regtest</option>
            </select>
            <span className={`px-2.5 py-1 text-[10px] font-semibold rounded-md border uppercase tracking-wider ${networkColors[network]}`}>
              {network}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <form onSubmit={handleSearch} className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#555] pointer-events-none" />
            <input
              type="text"
              name="search"
              placeholder="Search..."
              className="bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg pl-9 pr-4 py-1.5 text-sm text-white placeholder:text-[#555] focus:outline-none focus:ring-2 focus:ring-white/10 w-64 hover:border-[#3a3a3a] transition-colors"
            />
          </form>

          <button
            onClick={handleSettings}
            className="p-2 rounded-lg hover:bg-[#1a1a1a] text-[#666] hover:text-white transition-colors"
            title="Settings"
            type="button"
          >
            <Settings className="w-5 h-5" />
          </button>

          <button
            onClick={toggleTheme}
            className="p-2 rounded-lg hover:bg-[#1a1a1a] text-[#666] hover:text-white transition-colors"
            title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
            type="button"
          >
            {theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
          </button>

          <button
            onClick={handleProfile}
            className="p-2 rounded-lg hover:bg-[#1a1a1a] text-[#666] hover:text-white transition-colors"
            title="Profile"
            type="button"
          >
            <User className="w-5 h-5" />
          </button>
        </div>
      </div>
    </div>
  );
}
