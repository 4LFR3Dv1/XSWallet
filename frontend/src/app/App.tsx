import React, { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { useVaultStore } from '@/services/store';

// Screens
import { OnboardingWelcome } from '@/app/components/xs-wallet/screens/OnboardingWelcome';
import { OnboardingMnemonic } from '@/app/components/xs-wallet/screens/OnboardingMnemonic';
import { OnboardingConfirm } from '@/app/components/xs-wallet/screens/OnboardingConfirm';
import { UnlockScreen } from '@/app/components/xs-wallet/screens/UnlockScreen';
import { Dashboard } from '@/app/components/xs-wallet/screens/Dashboard';
import { SwapCenter } from '@/app/components/xs-wallet/screens/SwapCenter';
import { WalletPage } from '@/app/components/xs-wallet/screens/WalletPage';
import { NetworkPage } from '@/app/components/xs-wallet/screens/NetworkPage';
import { SettingsPage } from '@/app/components/xs-wallet/screens/SettingsPage';

// Auth Guard Component
function AuthGuard({ children }: { children: React.ReactNode }) {
  const { status, checkStatus } = useVaultStore();
  const navigate = useNavigate();

  useEffect(() => {
    checkStatus();
  }, [checkStatus]);

  useEffect(() => {
    if (status === 'not_initialized') {
      navigate('/onboarding');
    } else if (status === 'locked' || status === 'locked_out') {
      navigate('/unlock');
    }
  }, [status, navigate]);

  if (status === 'loading') {
    return (
      <div className="w-full h-screen bg-[#0B0D10] flex items-center justify-center">
        <div className="text-[#E7EDF5] text-lg">Loading...</div>
      </div>
    );
  }

  if (status !== 'unlocked') {
    return null;
  }

  return <>{children}</>;
}

// Public Route (no auth required)
function PublicRoute({ children }: { children: React.ReactNode }) {
  const { status, checkStatus } = useVaultStore();
  const navigate = useNavigate();

  useEffect(() => {
    checkStatus();
  }, [checkStatus]);

  useEffect(() => {
    if (status === 'unlocked') {
      navigate('/');
    }
  }, [status, navigate]);

  if (status === 'loading') {
    return (
      <div className="w-full h-screen bg-[#0B0D10] flex items-center justify-center">
        <div className="text-[#E7EDF5] text-lg">Loading...</div>
      </div>
    );
  }

  return <>{children}</>;
}

function AppRoutes() {
  return (
    <Routes>
      {/* Public Routes */}
      <Route path="/onboarding" element={<PublicRoute><OnboardingWelcome /></PublicRoute>} />
      <Route path="/onboarding/mnemonic" element={<PublicRoute><OnboardingMnemonic /></PublicRoute>} />
      <Route path="/onboarding/confirm" element={<PublicRoute><OnboardingConfirm /></PublicRoute>} />
      <Route path="/unlock" element={<PublicRoute><UnlockScreen /></PublicRoute>} />

      {/* Protected Routes */}
      <Route path="/" element={<AuthGuard><Dashboard /></AuthGuard>} />
      <Route path="/swap" element={<AuthGuard><SwapCenter /></AuthGuard>} />
      <Route path="/wallet" element={<AuthGuard><WalletPage /></AuthGuard>} />
      <Route path="/network" element={<AuthGuard><NetworkPage /></AuthGuard>} />
      <Route path="/settings" element={<AuthGuard><SettingsPage /></AuthGuard>} />

      {/* Fallback */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <div className="w-full h-screen bg-[#0B0D10]">
        <AppRoutes />
      </div>
    </BrowserRouter>
  );
}
