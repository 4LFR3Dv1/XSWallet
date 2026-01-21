import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertCircle, RotateCcw, Loader2 } from 'lucide-react';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton } from '../PrimaryButton';
import { InputOTP, InputOTPGroup, InputOTPSlot } from '@/app/components/ui/input-otp';
import { useVaultStore } from '@/services/store';

export function UnlockScreen() {
  const navigate = useNavigate();
  const { unlock } = useVaultStore();
  const [pin, setPin] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [attempts, setAttempts] = useState(0);

  // Cooldown timer
  React.useEffect(() => {
    if (cooldown > 0) {
      const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
      return () => clearTimeout(timer);
    }
  }, [cooldown]);

  const handleUnlock = async () => {
    if (pin.length !== 6 || loading || cooldown > 0) return;

    setLoading(true);
    setError(null);

    try {
      const success = await unlock(pin);
      if (success) {
        navigate('/');
      } else {
        const newAttempts = attempts + 1;
        setAttempts(newAttempts);
        setError(`Incorrect PIN (${10 - newAttempts} attempts remaining)`);
        setPin('');

        // Exponential backoff
        if (newAttempts >= 3) {
          setCooldown(Math.min(60 * newAttempts, 300));
        }
      }
    } catch (e) {
      setError('Failed to connect to wallet service');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0B0D10] flex items-center justify-center p-8">
      <div className="w-full max-w-[440px]">
        {/* Logo */}
        <div className="text-center mb-12">
          <h1 className="text-4xl text-[#E7EDF5] mb-2">XS Wallet</h1>
          <p className="text-[#9AA7B5]">Enter PIN to unlock</p>
        </div>

        <TerminalCard>
          <div className="space-y-6">
            {/* PIN Input */}
            <div className="space-y-4">
              <label className="text-sm text-[#9AA7B5] block text-center">Enter PIN</label>
              <div className="flex justify-center">
                <InputOTP
                  maxLength={6}
                  value={pin}
                  onChange={(value) => {
                    setPin(value);
                    setError(null);
                  }}
                  disabled={cooldown > 0 || loading}
                >
                  <InputOTPGroup>
                    <InputOTPSlot index={0} className="w-12 h-12 bg-[#151B23] border-[#242C36] text-[#E7EDF5]" />
                    <InputOTPSlot index={1} className="w-12 h-12 bg-[#151B23] border-[#242C36] text-[#E7EDF5]" />
                    <InputOTPSlot index={2} className="w-12 h-12 bg-[#151B23] border-[#242C36] text-[#E7EDF5]" />
                    <InputOTPSlot index={3} className="w-12 h-12 bg-[#151B23] border-[#242C36] text-[#E7EDF5]" />
                    <InputOTPSlot index={4} className="w-12 h-12 bg-[#151B23] border-[#242C36] text-[#E7EDF5]" />
                    <InputOTPSlot index={5} className="w-12 h-12 bg-[#151B23] border-[#242C36] text-[#E7EDF5]" />
                  </InputOTPGroup>
                </InputOTP>
              </div>
            </div>

            {/* Error Message */}
            {error && (
              <div className="p-3 bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-xl flex items-center gap-2">
                <AlertCircle size={16} className="text-[#EF4444]" />
                <span className="text-sm text-[#EF4444]">{error}</span>
              </div>
            )}

            {/* Cooldown Message */}
            {cooldown > 0 && (
              <div className="p-3 bg-[#F59E0B]/10 border border-[#F59E0B]/20 rounded-xl flex items-center gap-2">
                <RotateCcw size={16} className="text-[#F59E0B]" />
                <span className="text-sm text-[#F59E0B]">Try again in {cooldown}s</span>
              </div>
            )}

            {/* Unlock Button */}
            <PrimaryButton
              fullWidth
              onClick={handleUnlock}
              disabled={pin.length !== 6 || cooldown > 0 || loading}
            >
              {loading ? (
                <div className="flex items-center gap-2">
                  <Loader2 size={18} className="animate-spin" />
                  <span>Unlocking...</span>
                </div>
              ) : (
                'Unlock'
              )}
            </PrimaryButton>

            {/* Restore Link */}
            <div className="text-center pt-4 border-t border-[#242C36]">
              <button
                onClick={() => navigate('/onboarding')}
                className="text-sm text-[#6C7A89] hover:text-[#9AA7B5] transition-colors"
              >
                Restore from seed
              </button>
            </div>
          </div>
        </TerminalCard>
      </div>
    </div>
  );
}
