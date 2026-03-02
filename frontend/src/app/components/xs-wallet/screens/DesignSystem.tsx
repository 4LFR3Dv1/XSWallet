import React from 'react';
import { Bitcoin, Zap, Droplet, Lock, Check, AlertTriangle, Info } from 'lucide-react';
import { TerminalCard } from '../TerminalCard';
import { StatusChip } from '../StatusChip';
import { MetricCard } from '../MetricCard';
import { PrimaryButton, SecondaryButton, DestructiveButton } from '../PrimaryButton';
import { ProgressStepper } from '../ProgressStepper';

export function DesignSystem() {
  return (
    <div className="min-h-screen bg-[#0B0D10] p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-12">
          <h1 className="text-4xl text-[#E7EDF5] mb-2">Domini Wallet Design System</h1>
          <p className="text-[#9AA7B5]">Crypto Enterprise Terminal Components</p>
        </div>

        {/* Colors */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Color Palette</h2>
          <div className="grid grid-cols-4 gap-4">
            <div className="space-y-3">
              <h3 className="text-sm text-[#9AA7B5] mb-2">Backgrounds</h3>
              <div className="bg-[#0B0D10] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#E7EDF5] font-mono">#0B0D10</div>
                <div className="text-xs text-[#6C7A89]">Background</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#E7EDF5] font-mono">#11151B</div>
                <div className="text-xs text-[#6C7A89]">Surface-1</div>
              </div>
              <div className="bg-[#151B23] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#E7EDF5] font-mono">#151B23</div>
                <div className="text-xs text-[#6C7A89]">Surface-2</div>
              </div>
            </div>

            <div className="space-y-3">
              <h3 className="text-sm text-[#9AA7B5] mb-2">Text & Borders</h3>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#E7EDF5] font-mono">#E7EDF5</div>
                <div className="text-xs text-[#6C7A89]">Primary Text</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#9AA7B5] font-mono">#9AA7B5</div>
                <div className="text-xs text-[#6C7A89]">Muted</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#6C7A89] font-mono">#242C36</div>
                <div className="text-xs text-[#6C7A89]">Border</div>
              </div>
            </div>

            <div className="space-y-3">
              <h3 className="text-sm text-[#9AA7B5] mb-2">Asset Colors</h3>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#F7931A] font-mono">#F7931A</div>
                <div className="text-xs text-[#6C7A89]">BTC</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#FFD700] font-mono">#FFD700</div>
                <div className="text-xs text-[#6C7A89]">Lightning</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#00B4D8] font-mono">#00B4D8</div>
                <div className="text-xs text-[#6C7A89]">Liquid</div>
              </div>
            </div>

            <div className="space-y-3">
              <h3 className="text-sm text-[#9AA7B5] mb-2">Status Colors</h3>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#10B981] font-mono">#10B981</div>
                <div className="text-xs text-[#6C7A89]">Success</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#F59E0B] font-mono">#F59E0B</div>
                <div className="text-xs text-[#6C7A89]">Warning</div>
              </div>
              <div className="bg-[#11151B] border border-[#242C36] p-4 rounded-xl">
                <div className="text-xs text-[#EF4444] font-mono">#EF4444</div>
                <div className="text-xs text-[#6C7A89]">Error</div>
              </div>
            </div>
          </div>
        </section>

        {/* Typography */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Typography</h2>
          <TerminalCard>
            <div className="space-y-4">
              <div>
                <h1 className="text-4xl text-[#E7EDF5]">Heading 1 - Inter 36px</h1>
                <p className="text-xs text-[#6C7A89] mt-1">Large page titles</p>
              </div>
              <div>
                <h2 className="text-3xl text-[#E7EDF5]">Heading 2 - Inter 30px</h2>
                <p className="text-xs text-[#6C7A89] mt-1">Section titles</p>
              </div>
              <div>
                <h3 className="text-xl text-[#E7EDF5]">Heading 3 - Inter 20px</h3>
                <p className="text-xs text-[#6C7A89] mt-1">Card headers</p>
              </div>
              <div>
                <p className="text-base text-[#E7EDF5]">Body - Inter 16px</p>
                <p className="text-xs text-[#6C7A89] mt-1">Standard content</p>
              </div>
              <div>
                <p className="text-sm text-[#9AA7B5]">Small - Inter 14px</p>
                <p className="text-xs text-[#6C7A89] mt-1">Secondary text</p>
              </div>
              <div>
                <p className="text-base text-[#E7EDF5] font-mono">Mono - JetBrains Mono 16px</p>
                <p className="text-xs text-[#6C7A89] mt-1">Addresses, hashes, amounts</p>
              </div>
            </div>
          </TerminalCard>
        </section>

        {/* Buttons */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Buttons</h2>
          <TerminalCard>
            <div className="flex items-center gap-4">
              <PrimaryButton>Primary Button</PrimaryButton>
              <SecondaryButton>Secondary Button</SecondaryButton>
              <DestructiveButton>Destructive Button</DestructiveButton>
              <PrimaryButton disabled>Disabled</PrimaryButton>
            </div>
          </TerminalCard>
        </section>

        {/* Status Chips */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Status Chips</h2>
          <TerminalCard>
            <div className="flex flex-wrap items-center gap-3">
              <StatusChip label="Default" variant="default" />
              <StatusChip icon={<Check size={14} />} label="Success" variant="success" />
              <StatusChip icon={<AlertTriangle size={14} />} label="Warning" variant="warning" />
              <StatusChip icon={<Info size={14} />} label="Error" variant="error" />
              <StatusChip icon={<Bitcoin size={14} />} label="Bitcoin" variant="btc" />
              <StatusChip icon={<Zap size={14} />} label="Lightning" variant="lightning" />
              <StatusChip icon={<Droplet size={14} />} label="Liquid" variant="liquid" />
            </div>
          </TerminalCard>
        </section>

        {/* Metric Cards */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Metric Cards</h2>
          <div className="grid grid-cols-3 gap-6">
            <MetricCard
              icon={<Bitcoin size={24} />}
              asset="Bitcoin"
              balance="2.45870000"
              fiat="$124,587.00 USD"
              delta="+2.4%"
              deltaPositive={true}
              accentColor="#F7931A"
            />
            <MetricCard
              icon={<Droplet size={24} />}
              asset="Liquid"
              balance="5.12340000"
              fiat="$259,012.00 USD"
              delta="+1.8%"
              deltaPositive={true}
              accentColor="#00B4D8"
            />
            <MetricCard
              icon={<Zap size={24} />}
              asset="Lightning"
              balance="12,500,000"
              fiat="$6,325.00 USD"
              delta="-0.5%"
              deltaPositive={false}
              accentColor="#FFD700"
            />
          </div>
        </section>

        {/* Cards */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Terminal Cards</h2>
          <div className="grid grid-cols-2 gap-6">
            <TerminalCard>
              <h3 className="text-lg text-[#E7EDF5] mb-2">Card without Header</h3>
              <p className="text-sm text-[#9AA7B5]">This is a basic terminal card component with rounded corners and subtle shadows.</p>
            </TerminalCard>
            <TerminalCard
              header={<h3 className="text-lg text-[#E7EDF5]">Card with Header</h3>}
            >
              <p className="text-sm text-[#9AA7B5]">This card includes a header section separated by a border.</p>
            </TerminalCard>
          </div>
        </section>

        {/* Progress Stepper */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Progress Stepper</h2>
          <TerminalCard>
            <div className="space-y-4">
              <div>
                <p className="text-sm text-[#9AA7B5] mb-3">Step 1 of 4</p>
                <ProgressStepper currentStep={1} totalSteps={4} />
              </div>
              <div>
                <p className="text-sm text-[#9AA7B5] mb-3">Step 2 of 4</p>
                <ProgressStepper currentStep={2} totalSteps={4} />
              </div>
              <div>
                <p className="text-sm text-[#9AA7B5] mb-3">Step 4 of 4</p>
                <ProgressStepper currentStep={4} totalSteps={4} />
              </div>
            </div>
          </TerminalCard>
        </section>

        {/* Inputs */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Form Inputs</h2>
          <TerminalCard>
            <div className="grid grid-cols-2 gap-6">
              <div>
                <label className="text-sm text-[#9AA7B5] mb-2 block">Text Input</label>
                <input
                  type="text"
                  className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  placeholder="Enter text..."
                />
              </div>
              <div>
                <label className="text-sm text-[#9AA7B5] mb-2 block">Monospace Input</label>
                <input
                  type="text"
                  className="w-full px-4 py-3 bg-[#151B23] border border-[#242C36] rounded-xl text-[#E7EDF5] font-mono focus:outline-none focus:border-[#E7EDF5]/30 transition-colors"
                  placeholder="0.00000000"
                />
              </div>
            </div>
          </TerminalCard>
        </section>

        {/* Border Radius */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Border Radius</h2>
          <TerminalCard>
            <div className="grid grid-cols-3 gap-6">
              <div className="text-center">
                <div className="bg-[#151B23] border border-[#242C36] rounded-2xl p-6 mb-2">
                  <p className="text-sm text-[#E7EDF5]">16px</p>
                </div>
                <p className="text-xs text-[#6C7A89]">Cards</p>
              </div>
              <div className="text-center">
                <div className="bg-[#151B23] border border-[#242C36] rounded-xl p-6 mb-2">
                  <p className="text-sm text-[#E7EDF5]">12px</p>
                </div>
                <p className="text-xs text-[#6C7A89]">Inputs/Buttons</p>
              </div>
              <div className="text-center">
                <div className="bg-[#151B23] border border-[#242C36] rounded-full px-6 py-3 mb-2">
                  <p className="text-sm text-[#E7EDF5]">999px</p>
                </div>
                <p className="text-xs text-[#6C7A89]">Chips</p>
              </div>
            </div>
          </TerminalCard>
        </section>

        {/* Spacing */}
        <section className="mb-12">
          <h2 className="text-2xl text-[#E7EDF5] mb-6">Spacing (8pt Grid)</h2>
          <TerminalCard>
            <div className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="w-8 h-8 bg-[#E7EDF5]"></div>
                <span className="text-sm text-[#9AA7B5]">8px / 0.5rem</span>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 bg-[#E7EDF5]"></div>
                <span className="text-sm text-[#9AA7B5]">12px / 0.75rem</span>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-16 h-16 bg-[#E7EDF5]"></div>
                <span className="text-sm text-[#9AA7B5]">16px / 1rem</span>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-24 h-24 bg-[#E7EDF5]"></div>
                <span className="text-sm text-[#9AA7B5]">24px / 1.5rem</span>
              </div>
            </div>
          </TerminalCard>
        </section>
      </div>
    </div>
  );
}
