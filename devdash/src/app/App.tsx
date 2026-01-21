import { useState } from 'react';
import { ThemeProvider } from './context/theme-context';
import { Sidebar } from './components/sidebar';
import { Topbar } from './components/topbar';
import { ObservabilityRibbon } from './components/observability-ribbon';
import { HomePage } from './pages/home';
import { WalletStudioPage } from './pages/wallet-studio';
import { HTLCLaboratoryPage } from './pages/htlc-laboratory';
import { SwapCenterPage } from './pages/swap-center';
import { NetworkPage } from './pages/network';
import { APIExplorerPage } from './pages/api-explorer';
import { SettingsPage } from './pages/settings';
import { NetworkEnv } from './types';
import { mockObservabilityMetrics } from './data/mock-data';

type Page = 'home' | 'wallet' | 'htlc' | 'swap' | 'network' | 'api' | 'settings';

function AppContent() {
  const [currentPage, setCurrentPage] = useState<Page>('home');
  const [network, setNetwork] = useState<NetworkEnv>('testnet');

  const handleNavigate = (page: string) => {
    setCurrentPage(page as Page);
  };

  const renderPage = () => {
    switch (currentPage) {
      case 'home':
        return <HomePage onNavigate={handleNavigate} />;
      case 'wallet':
        return <WalletStudioPage />;
      case 'htlc':
        return <HTLCLaboratoryPage />;
      case 'swap':
        return <SwapCenterPage />;
      case 'network':
        return <NetworkPage />;
      case 'api':
        return <APIExplorerPage />;
      case 'settings':
        return <SettingsPage network={network} onNetworkChange={setNetwork} />;
      default:
        return <HomePage onNavigate={handleNavigate} />;
    }
  };

  return (
    <div className="h-screen w-screen flex flex-col overflow-hidden bg-[#0a0a0a]" style={{ fontFamily: 'var(--font-sans)' }}>
      {/* Topbar - Fixed height */}
      <header className="flex-shrink-0">
        <Topbar network={network} onNetworkChange={setNetwork} />
      </header>

      {/* Observability Ribbon - Fixed height */}
      <div className="flex-shrink-0">
        <ObservabilityRibbon
          latency={mockObservabilityMetrics.latency}
          errorRate={mockObservabilityMetrics.errorRate}
          throughput={mockObservabilityMetrics.throughput}
        />
      </div>

      {/* Main Layout - Takes remaining space */}
      <div className="flex-1 flex overflow-hidden min-h-0">
        {/* Sidebar - Fixed width 240px */}
        <aside className="flex-shrink-0 w-60">
          <Sidebar currentPage={currentPage} onPageChange={setCurrentPage} />
        </aside>

        {/* Content Area - Flexible */}
        <main className="flex-1 overflow-y-auto bg-[#0d0d0d]">
          <div className="p-8">
            {renderPage()}
          </div>
        </main>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <ThemeProvider>
      <AppContent />
    </ThemeProvider>
  );
}
