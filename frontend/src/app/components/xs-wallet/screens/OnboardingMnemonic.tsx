import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Eye, EyeOff, AlertTriangle, Copy, Check } from 'lucide-react';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { ProgressStepper } from '../ProgressStepper';
import { useVaultStore } from '@/services/store';

export function OnboardingMnemonic() {
  const navigate = useNavigate();
  const location = useLocation();
  const { mnemonic: storedMnemonic, clearMnemonic } = useVaultStore();
  const [revealed, setRevealed] = useState(false);
  const [copied, setCopied] = useState(false);

  // Get mnemonic from router state or store
  const routerMnemonic = (location.state as any)?.mnemonic;
  const isRestore = (location.state as any)?.restore;
  const mnemonic = routerMnemonic || storedMnemonic || '';
  const mnemonicWords = mnemonic ? mnemonic.split(' ') : [];

  const handleCopyAll = () => {
    navigator.clipboard.writeText(mnemonic);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleContinue = () => {
    navigate('/onboarding/confirm', { state: { mnemonic } });
  };

  // If restoring, show input fields instead
  if (isRestore) {
    return (
      <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
        <div className="w-full max-w-[720px]">
          <div className="text-center mb-8">
            <h1 className="text-3xl text-[#E7EDF5] mb-2">Restore Wallet</h1>
            <p className="text-[#9AA7B5]">Enter your 24-word recovery phrase</p>
          </div>
          <TerminalCard>
            <div className="text-center py-12 text-[#6C7A89]">
              Restore functionality coming soon...
              <div className="mt-4">
                <SecondaryButton onClick={() => navigate('/onboarding')}>Back</SecondaryButton>
              </div>
            </div>
          </TerminalCard>
        </div>
      </div>
    );
  }

  if (mnemonicWords.length === 0) {
    return (
      <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
        <div className="text-center text-[#6C7A89]">
          No mnemonic found. Please create a wallet first.
          <div className="mt-4">
            <SecondaryButton onClick={() => navigate('/onboarding')}>Back</SecondaryButton>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
      <div className="w-full max-w-[720px]">
        {/* Header */}
        <div className="text-center mb-8">
          <h1 className="text-3xl text-[#E7EDF5] mb-2">Recovery Phrase</h1>
          <p className="text-[#9AA7B5]">Write down these 24 words in order</p>
          <div className="mt-4 flex justify-center">
            <ProgressStepper currentStep={1} totalSteps={4} />
          </div>
        </div>

        {/* Warning */}
        <div className="mb-6 p-4 bg-[#F59E0B]/10 border border-[#F59E0B]/20 rounded-xl flex items-start gap-3">
          <AlertTriangle size={20} className="text-[#F59E0B] flex-shrink-0 mt-0.5" />
          <div className="text-sm">
            <p className="text-[#F59E0B] mb-1">Never share your recovery phrase</p>
            <p className="text-[#F59E0B]/80">Anyone with these words can access your funds.</p>
          </div>
        </div>

        {/* Mnemonic Grid */}
        <TerminalCard>
          <div className={`relative ${!revealed ? 'blur-sm select-none' : ''}`}>
            <div className="grid grid-cols-4 gap-3">
              {mnemonicWords.map((word, index) => (
                <div
                  key={index}
                  className="bg-[#151B23] border border-[#242C36] rounded-xl p-3 flex items-center gap-3"
                >
                  <span className="text-xs text-[#6C7A89] font-mono w-6">{index + 1}</span>
                  <span className="text-sm text-[#E7EDF5] font-mono">{word}</span>
                </div>
              ))}
            </div>
          </div>

          {!revealed && (
            <div className="absolute inset-0 flex items-center justify-center">
              <button
                onClick={() => setRevealed(true)}
                className="px-8 py-4 bg-[#E7EDF5] text-[#0B0D10] rounded-xl flex items-center gap-3 hover:bg-[#E7EDF5]/90 transition-all"
              >
                <Eye size={20} />
                <span>Click to Reveal</span>
              </button>
            </div>
          )}

          {revealed && (
            <div className="mt-6 pt-6 border-t border-[#242C36] flex items-center justify-between">
              <div className="flex items-center gap-4">
                <button
                  onClick={() => setRevealed(false)}
                  className="text-sm text-[#9AA7B5] hover:text-[#E7EDF5] flex items-center gap-2 transition-colors"
                >
                  <EyeOff size={16} />
                  <span>Hide</span>
                </button>
                <button
                  onClick={handleCopyAll}
                  className="text-sm text-[#9AA7B5] hover:text-[#E7EDF5] flex items-center gap-2 transition-colors"
                >
                  {copied ? <Check size={16} className="text-[#10B981]" /> : <Copy size={16} />}
                  <span>{copied ? 'Copied!' : 'Copy All'}</span>
                </button>
              </div>
              <PrimaryButton onClick={handleContinue}>I've Saved It</PrimaryButton>
            </div>
          )}
        </TerminalCard>
      </div>
    </div>
  );
}
