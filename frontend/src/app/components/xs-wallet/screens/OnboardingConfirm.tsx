import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { CheckCircle, XCircle } from 'lucide-react';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { ProgressStepper } from '../ProgressStepper';
import { useVaultStore } from '@/services/store';

export function OnboardingConfirm() {
  const navigate = useNavigate();
  const location = useLocation();
  const { clearMnemonic } = useVaultStore();

  const mnemonic = (location.state as any)?.mnemonic || '';
  const words = mnemonic.split(' ');

  // Pick 3 random word indices to verify
  const verifyIndices = [2, 10, 18]; // words 3, 11, 19 (0-indexed)

  const [inputs, setInputs] = useState<{ [key: number]: string }>({});
  const [verified, setVerified] = useState<{ [key: number]: boolean | null }>({});

  const handleInputChange = (index: number, value: string) => {
    setInputs({ ...inputs, [index]: value.toLowerCase().trim() });

    // Auto-verify as user types
    if (value.toLowerCase().trim() === words[index]) {
      setVerified({ ...verified, [index]: true });
    } else if (value.length > 0) {
      setVerified({ ...verified, [index]: false });
    } else {
      setVerified({ ...verified, [index]: null });
    }
  };

  const allVerified = verifyIndices.every(i => verified[i] === true);

  const handleComplete = () => {
    // Clear mnemonic from memory and navigate to dashboard
    clearMnemonic();
    navigate('/');
  };

  if (!mnemonic) {
    return (
      <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
        <div className="text-center text-[#6C7A89]">
          No mnemonic found. Please start over.
          <div className="mt-4">
            <SecondaryButton onClick={() => navigate('/onboarding')}>Start Over</SecondaryButton>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
      <div className="w-full max-w-[520px]">
        {/* Header */}
        <div className="text-center mb-8">
          <h1 className="text-3xl text-[#E7EDF5] mb-2">Confirm Backup</h1>
          <p className="text-[#9AA7B5]">Verify you've saved your recovery phrase</p>
          <div className="mt-4 flex justify-center">
            <ProgressStepper currentStep={3} totalSteps={4} />
          </div>
        </div>

        <div className="space-y-6">
          {/* Word Confirmation */}
          <TerminalCard>
            <h3 className="text-lg text-[#E7EDF5] mb-4">Enter the following words</h3>
            <div className="space-y-4">
              {verifyIndices.map((index) => (
                <div key={index}>
                  <label className="text-sm text-[#9AA7B5] mb-2 block">Word #{index + 1}</label>
                  <div className="relative">
                    <input
                      type="text"
                      value={inputs[index] || ''}
                      onChange={(e) => handleInputChange(index, e.target.value)}
                      className={`w-full px-4 py-3 bg-[#151B23] border rounded-xl text-[#E7EDF5] font-mono focus:outline-none transition-colors ${verified[index] === true
                          ? 'border-[#10B981]'
                          : verified[index] === false
                            ? 'border-[#EF4444]'
                            : 'border-[#242C36] focus:border-[#E7EDF5]/30'
                        }`}
                      placeholder={`Enter word #${index + 1}`}
                    />
                    {verified[index] === true && (
                      <CheckCircle size={20} className="absolute right-4 top-1/2 -translate-y-1/2 text-[#10B981]" />
                    )}
                    {verified[index] === false && (
                      <XCircle size={20} className="absolute right-4 top-1/2 -translate-y-1/2 text-[#EF4444]" />
                    )}
                  </div>
                </div>
              ))}
            </div>
          </TerminalCard>

          {/* Success Message */}
          {allVerified && (
            <div className="p-4 bg-[#10B981]/10 border border-[#10B981]/20 rounded-xl flex items-center gap-3">
              <CheckCircle size={20} className="text-[#10B981]" />
              <div className="text-sm">
                <p className="text-[#10B981]">All words verified! Your wallet is ready.</p>
              </div>
            </div>
          )}

          <div className="flex gap-4">
            <SecondaryButton onClick={() => navigate('/onboarding/mnemonic', { state: { mnemonic } })}>
              Back
            </SecondaryButton>
            <PrimaryButton fullWidth onClick={handleComplete} disabled={!allVerified}>
              Complete Setup
            </PrimaryButton>
          </div>
        </div>
      </div>
    </div>
  );
}
