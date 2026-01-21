import { Sun, Moon, Terminal, TestTube } from 'lucide-react';
import { useTheme } from '@/app/context/theme-context';
import { NetworkEnv } from '@/app/types';

interface SettingsPageProps {
  network: NetworkEnv;
  onNetworkChange: (network: NetworkEnv) => void;
}

export function SettingsPage({ network, onNetworkChange }: SettingsPageProps) {
  const { theme, toggleTheme } = useTheme();

  return (
    <div className="space-y-6 max-w-3xl">
      <div>
        <h2 className="text-2xl font-bold text-foreground mb-2">Settings</h2>
        <p className="text-sm text-muted-foreground">
          Configure your DevDash preferences
        </p>
      </div>

      {/* Appearance */}
      <div className="bg-card border border-border rounded-[12px] p-6 space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-foreground uppercase tracking-wide mb-4">Appearance</h3>
          
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium text-foreground mb-3 block">Theme</label>
              <div className="grid grid-cols-2 gap-3">
                <button
                  onClick={() => theme === 'dark' && toggleTheme()}
                  className={`flex items-center gap-3 p-4 rounded-[12px] border-2 transition-all ${
                    theme === 'light'
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/40'
                  }`}
                >
                  <div className={`p-2 rounded-[8px] ${
                    theme === 'light' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
                  }`}>
                    <Sun className="w-5 h-5" />
                  </div>
                  <div className="text-left">
                    <p className="text-sm font-semibold text-foreground">Light</p>
                    <p className="text-xs text-muted-foreground">Bright interface</p>
                  </div>
                </button>

                <button
                  onClick={() => theme === 'light' && toggleTheme()}
                  className={`flex items-center gap-3 p-4 rounded-[12px] border-2 transition-all ${
                    theme === 'dark'
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/40'
                  }`}
                >
                  <div className={`p-2 rounded-[8px] ${
                    theme === 'dark' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
                  }`}>
                    <Moon className="w-5 h-5" />
                  </div>
                  <div className="text-left">
                    <p className="text-sm font-semibold text-foreground">Dark</p>
                    <p className="text-xs text-muted-foreground">Easy on the eyes</p>
                  </div>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Network */}
      <div className="bg-card border border-border rounded-[12px] p-6 space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-foreground uppercase tracking-wide mb-4">Network Configuration</h3>
          
          <div>
            <label className="text-sm font-medium text-foreground mb-3 block">Active Network</label>
            <div className="grid grid-cols-3 gap-3">
              {(['mainnet', 'testnet', 'regtest'] as NetworkEnv[]).map(net => (
                <button
                  key={net}
                  onClick={() => onNetworkChange(net)}
                  className={`p-4 rounded-[12px] border-2 transition-all ${
                    network === net
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/40'
                  }`}
                >
                  <p className="text-sm font-semibold text-foreground capitalize">{net}</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    {net === 'mainnet' && 'Production network'}
                    {net === 'testnet' && 'Public test network'}
                    {net === 'regtest' && 'Local development'}
                  </p>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Developer Options */}
      <div className="bg-card border border-border rounded-[12px] p-6 space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-foreground uppercase tracking-wide mb-4">Developer Options</h3>
          
          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 bg-secondary rounded-[12px]">
              <div className="flex items-center gap-3">
                <Terminal className="w-5 h-5 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium text-foreground">Verbose Logging</p>
                  <p className="text-xs text-muted-foreground">Show detailed debug information</p>
                </div>
              </div>
              <button className="relative inline-flex h-6 w-11 items-center rounded-full bg-muted transition-colors">
                <span className="inline-block h-4 w-4 transform rounded-full bg-white transition-transform translate-x-1" />
              </button>
            </div>

            <div className="flex items-center justify-between p-4 bg-secondary rounded-[12px]">
              <div className="flex items-center gap-3">
                <TestTube className="w-5 h-5 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium text-foreground">Mock Mode</p>
                  <p className="text-xs text-muted-foreground">Use simulated API responses</p>
                </div>
              </div>
              <button className="relative inline-flex h-6 w-11 items-center rounded-full bg-primary transition-colors">
                <span className="inline-block h-4 w-4 transform rounded-full bg-white transition-transform translate-x-6" />
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* About */}
      <div className="bg-card border border-border rounded-[12px] p-6">
        <h3 className="text-sm font-semibold text-foreground uppercase tracking-wide mb-4">About</h3>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Version</span>
            <span className="font-mono text-foreground">1.0.0-beta</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Build</span>
            <span className="font-mono text-foreground">2025.01.15</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">License</span>
            <span className="font-mono text-foreground">MIT</span>
          </div>
        </div>
      </div>
    </div>
  );
}
