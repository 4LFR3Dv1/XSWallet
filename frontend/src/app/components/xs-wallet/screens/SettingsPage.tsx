import React from 'react';
import { Lock, Eye, EyeOff, Shield, Globe, Info, AlertTriangle } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { AppShell } from '../AppShell';
import { PageHeader } from '../PageHeader';
import { TerminalCard } from '../TerminalCard';
import { PrimaryButton, SecondaryButton } from '../PrimaryButton';
import { useVaultStore } from '@/services/store';

export function SettingsPage() {
    const navigate = useNavigate();
    const { lock } = useVaultStore();
    const [showMnemonic, setShowMnemonic] = React.useState(false);

    const handleLock = async () => {
        await lock();
        navigate('/unlock');
    };

    return (
        <AppShell activePage="settings" vaultLocked={false}>
            <div className="p-8">
                <PageHeader
                    title="Settings"
                    subtitle="Configure your wallet"
                />

                <div className="max-w-2xl space-y-6">
                    {/* Security */}
                    <TerminalCard
                        header={
                            <div className="flex items-center gap-2">
                                <Shield size={20} className="text-[#E7EDF5]" />
                                <h3 className="text-lg text-[#E7EDF5]">Security</h3>
                            </div>
                        }
                    >
                        <div className="space-y-4">
                            <button
                                onClick={handleLock}
                                className="w-full p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:bg-[#151B23]/80 hover:border-[#E7EDF5]/20 transition-all text-left flex items-center justify-between"
                            >
                                <div className="flex items-center gap-3">
                                    <div className="w-10 h-10 rounded-xl bg-[#E7EDF5]/10 flex items-center justify-center">
                                        <Lock size={20} className="text-[#E7EDF5]" />
                                    </div>
                                    <div>
                                        <div className="text-sm text-[#E7EDF5]">Lock Wallet</div>
                                        <div className="text-xs text-[#6C7A89]">Immediately lock the vault</div>
                                    </div>
                                </div>
                            </button>

                            <button
                                className="w-full p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:bg-[#151B23]/80 hover:border-[#E7EDF5]/20 transition-all text-left flex items-center justify-between"
                            >
                                <div className="flex items-center gap-3">
                                    <div className="w-10 h-10 rounded-xl bg-[#F59E0B]/10 flex items-center justify-center">
                                        {showMnemonic ? <EyeOff size={20} className="text-[#F59E0B]" /> : <Eye size={20} className="text-[#F59E0B]" />}
                                    </div>
                                    <div>
                                        <div className="text-sm text-[#E7EDF5]">View Recovery Phrase</div>
                                        <div className="text-xs text-[#6C7A89]">Requires PIN verification</div>
                                    </div>
                                </div>
                                <span className="text-xs text-[#F59E0B] bg-[#F59E0B]/10 px-2 py-1 rounded">⚠️ Sensitive</span>
                            </button>

                            <button
                                className="w-full p-4 bg-[#151B23] border border-[#242C36] rounded-xl hover:bg-[#151B23]/80 hover:border-[#E7EDF5]/20 transition-all text-left flex items-center justify-between"
                            >
                                <div className="flex items-center gap-3">
                                    <div className="w-10 h-10 rounded-xl bg-[#E7EDF5]/10 flex items-center justify-center">
                                        <Lock size={20} className="text-[#E7EDF5]" />
                                    </div>
                                    <div>
                                        <div className="text-sm text-[#E7EDF5]">Change PIN</div>
                                        <div className="text-xs text-[#6C7A89]">Update wallet unlock PIN</div>
                                    </div>
                                </div>
                            </button>
                        </div>
                    </TerminalCard>

                    {/* Network */}
                    <TerminalCard
                        header={
                            <div className="flex items-center gap-2">
                                <Globe size={20} className="text-[#E7EDF5]" />
                                <h3 className="text-lg text-[#E7EDF5]">Network</h3>
                            </div>
                        }
                    >
                        <div className="space-y-4">
                            <div className="p-4 bg-[#151B23] border border-[#242C36] rounded-xl flex items-center justify-between">
                                <div>
                                    <div className="text-sm text-[#E7EDF5]">Network</div>
                                    <div className="text-xs text-[#6C7A89]">Select Bitcoin network</div>
                                </div>
                                <select className="bg-[#11151B] border border-[#242C36] rounded-lg px-3 py-2 text-sm text-[#E7EDF5] focus:outline-none focus:border-[#E7EDF5]/30">
                                    <option value="regtest">Regtest</option>
                                    <option value="testnet">Testnet</option>
                                    <option value="mainnet">Mainnet</option>
                                </select>
                            </div>

                            <div className="p-4 bg-[#151B23] border border-[#242C36] rounded-xl flex items-center justify-between">
                                <div>
                                    <div className="text-sm text-[#E7EDF5]">API Bridge</div>
                                    <div className="text-xs text-[#6C7A89]">Connection endpoint</div>
                                </div>
                                <span className="text-sm text-[#6C7A89] font-mono">localhost:3000</span>
                            </div>
                        </div>
                    </TerminalCard>

                    {/* About */}
                    <TerminalCard
                        header={
                            <div className="flex items-center gap-2">
                                <Info size={20} className="text-[#E7EDF5]" />
                                <h3 className="text-lg text-[#E7EDF5]">About</h3>
                            </div>
                        }
                    >
                        <div className="space-y-3">
                            <div className="flex justify-between py-2 border-b border-[#242C36]">
                                <span className="text-[#6C7A89]">Version</span>
                                <span className="text-[#E7EDF5] font-mono">0.1.0</span>
                            </div>
                            <div className="flex justify-between py-2 border-b border-[#242C36]">
                                <span className="text-[#6C7A89]">XSCore</span>
                                <span className="text-[#E7EDF5] font-mono">v0.2.0</span>
                            </div>
                            <div className="flex justify-between py-2">
                                <span className="text-[#6C7A89]">Build</span>
                                <span className="text-[#E7EDF5] font-mono">2026.01.20</span>
                            </div>
                        </div>
                    </TerminalCard>

                    {/* Danger Zone */}
                    <TerminalCard
                        header={
                            <div className="flex items-center gap-2">
                                <AlertTriangle size={20} className="text-[#EF4444]" />
                                <h3 className="text-lg text-[#EF4444]">Danger Zone</h3>
                            </div>
                        }
                    >
                        <button
                            className="w-full p-4 bg-[#EF4444]/10 border border-[#EF4444]/30 rounded-xl hover:bg-[#EF4444]/20 transition-all text-left"
                        >
                            <div className="text-[#EF4444] font-medium mb-1">Reset Application</div>
                            <div className="text-xs text-[#EF4444]/70">
                                This will delete all data. Make sure you have backed up your recovery phrase.
                            </div>
                        </button>
                    </TerminalCard>
                </div>
            </div>
        </AppShell>
    );
}
