import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Download, ShieldCheck, Loader2 } from 'lucide-react';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { useVaultStore } from '@/services/store';

export function OnboardingWelcome() {
  const navigate = useNavigate();
  const { initVault, checkStatus, status } = useVaultStore();
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pin, setPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');

  // Check vault status on mount
  useEffect(() => {
    const check = async () => {
      await checkStatus();
      setChecking(false);
    };
    check();
  }, [checkStatus]);

  // Redirect only when already unlocked. If locked, keep onboarding visible
  // and let create/restore flow handle "already exists" explicitly.
  useEffect(() => {
    if (!checking) {
      if (status === 'unlocked') {
        navigate('/');
      }
    }
  }, [checking, status, navigate]);

  const handleCreate = async () => {
    if (pin.length < 8) {
      setError('PIN must be at least 8 characters.');
      return;
    }
    if (pin !== confirmPin) {
      setError('PIN confirmation does not match.');
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const result = await initVault(pin);
      if (result.success && result.mnemonic) {
        // Navigate to mnemonic display
        navigate('/onboarding/mnemonic', { state: { mnemonic: result.mnemonic } });
      } else {
        setError('Failed to create wallet');
      }
    } catch (e: any) {
      // Check if vault already exists
      if (e.message?.includes('already exists') || e.message?.includes('ALREADY_EXISTS')) {
        navigate('/unlock');
      } else {
        setError(e?.message || 'Failed to connect to wallet service. Make sure api-bridge is running.');
      }
    } finally {
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center">
        <div className="text-center">
          <Loader2 size={32} className="animate-spin text-[#E7EDF5] mx-auto mb-4" />
          <p className="text-[#9AA7B5]">Checking wallet status...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
      <div className="w-full max-w-[520px]">
        {/* Logo */}
        <div className="text-center mb-12">
          <h1 className="text-4xl text-[#E7EDF5] mb-2">Domini Wallet</h1>
          <p className="text-[#9AA7B5]">Self-custody & Atomic Swaps</p>
        </div>

        {/* Main Card */}
        <TerminalCard>
          <div className="space-y-6">
            <div className="text-center mb-8">
              <h2 className="text-2xl text-[#E7EDF5] mb-2">Get Started</h2>
              <p className="text-sm text-[#9AA7B5]">Create a new wallet or restore existing</p>
            </div>

            {error && (
              <div className="p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl text-sm text-[#EF4444]">
                {error}
              </div>
            )}

            <div className="space-y-3">
              <label className="text-sm text-[#9AA7B5] block">Choose PIN (min 8 chars)</label>
              <input
                type="password"
                value={pin}
                onChange={(e) => {
                  setPin(e.target.value);
                  setError(null);
                }}
                placeholder="Enter PIN"
                className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] focus:outline-none focus:border-[#E7EDF5]/30"
                disabled={loading}
              />
              <input
                type="password"
                value={confirmPin}
                onChange={(e) => {
                  setConfirmPin(e.target.value);
                  setError(null);
                }}
                placeholder="Confirm PIN"
                className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] focus:outline-none focus:border-[#E7EDF5]/30"
                disabled={loading}
              />
            </div>

            <PrimaryButton fullWidth onClick={handleCreate} disabled={loading}>
              <div className="flex items-center justify-center gap-2">
                {loading ? (
                  <>
                    <Loader2 size={20} className="animate-spin" />
                    <span>Creating...</span>
                  </>
                ) : (
                  <>
                    <Plus size={20} />
                    <span>Create Wallet</span>
                  </>
                )}
              </div>
            </PrimaryButton>

            <SecondaryButton fullWidth disabled>
              <div className="flex items-center justify-center gap-2">
                <Download size={20} />
                <span>Restore Wallet (Coming Soon)</span>
              </div>
            </SecondaryButton>

            {/* Security Notice */}
            <div className="mt-8 pt-6 border-t border-[#242C36]">
              <div className="flex items-start gap-3 p-4 bg-[#151B23] rounded-xl border border-[#242C36]">
                <ShieldCheck size={20} className="text-[#10B981] flex-shrink-0 mt-0.5" />
                <div className="text-sm">
                  <p className="text-[#E7EDF5] mb-1">Your keys, your crypto</p>
                  <p className="text-[#9AA7B5]">Non-custodial. No accounts. Full control.</p>
                </div>
              </div>
            </div>
          </div>
        </TerminalCard>
      </div>
    </div>
  );
}
