import { useState, useEffect } from 'react';
import { Copy, Check } from 'lucide-react';
import { CodeBlock } from '@/app/components/code-block';
import { StateMachine } from '@/app/components/state-machine';
import { useGeneratePreimage } from '@/app/services/hooks';

export function HTLCLaboratoryPage() {
  const [activeTab, setActiveTab] = useState('builder');
  const { generate: generatePreimage, result: preimageData } = useGeneratePreimage();

  // Builder state
  const [hashlock, setHashlock] = useState('');
  const [preimage, setPreimage] = useState('');
  const [timelock, setTimelock] = useState('144');
  const [senderPubkey, setSenderPubkey] = useState('');
  const [receiverPubkey, setReceiverPubkey] = useState('');
  const [amount, setAmount] = useState('100000');
  const [generatedScript, setGeneratedScript] = useState('');
  const [copied, setCopied] = useState(false);

  // Decoder state
  const [scriptHex, setScriptHex] = useState('');
  const [decodedScript, setDecodedScript] = useState('');

  // Simulator state
  const [currentBlock, setCurrentBlock] = useState(850234);
  const [expiryBlock, setExpiryBlock] = useState(850378);
  const [htlcState, setHtlcState] = useState<'created' | 'funded' | 'claimed' | 'expired' | 'refunded'>('funded');

  // Witness state
  const [witnessPreimage, setWitnessPreimage] = useState('');
  const [witnessSignature, setWitnessSignature] = useState('');
  const [witnessPreview, setWitnessPreview] = useState('');
  const [preimageValid, setPreimageValid] = useState<boolean | null>(null);

  const handleGeneratePreimage = async () => {
    try {
      await generatePreimage();
    } catch (err) {
      alert('Failed to generate preimage');
    }
  };

  useEffect(() => {
    if (preimageData) {
      setPreimage(preimageData.preimage);
      setHashlock(preimageData.payment_hash);
    }
  }, [preimageData]);

  const handleGenerateScript = () => {
    if (!hashlock) {
      alert('Please generate a preimage first (click "Generate Preimage")');
      return;
    }

    // Use placeholders if pubkeys not provided (educational mode)
    const sender = senderPubkey || '02a1633cafcc01ebfb6d78e39f687a1f0995c62fc95f51ead10a02ee0be551b5dc';
    const receiver = receiverPubkey || '03c9f4836b9a4f77fc0d81f7bcb01b7f1b35916864b9476c241ce9fc198bd25432';

    const script = `OP_IF
  OP_HASH160
  <${hashlock.slice(0, 40)}>
  OP_EQUALVERIFY
  <${receiver.slice(0, 40)}>
OP_ELSE
  <${timelock}>
  OP_CHECKSEQUENCEVERIFY
  OP_DROP
  <${sender.slice(0, 40)}>
OP_ENDIF
OP_CHECKSIG`;

    setGeneratedScript(script);
  };

  const handleCopyScript = () => {
    navigator.clipboard.writeText(generatedScript);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleDecodeScript = () => {
    if (!scriptHex) {
      alert('Please paste a script hex');
      return;
    }

    // Simple mock decode - in real implementation would parse hex
    const decoded = `OP_IF
  OP_HASH160
  <hash160>
  OP_EQUALVERIFY
  <receiver_pubkey>
OP_ELSE
  <${timelock}>
  OP_CHECKSEQUENCEVERIFY
  OP_DROP
  <sender_pubkey>
OP_ENDIF
OP_CHECKSIG`;

    setDecodedScript(decoded);
  };

  const handleSimulateClaim = () => {
    setHtlcState('claimed');
    setTimeout(() => {
      alert('✅ HTLC Claimed! Receiver provided correct preimage and signature.');
    }, 300);
  };

  const handleSimulateRefund = () => {
    setHtlcState('refunded');
    setTimeout(() => {
      alert('🔄 HTLC Refunded! Timelock expired, sender recovered funds.');
    }, 300);
  };

  const handleResetSimulator = () => {
    setHtlcState('funded');
    setCurrentBlock(850234);
    setExpiryBlock(850378);
  };

  const handleBuildWitness = () => {
    if (!witnessPreimage) {
      alert('Please enter a preimage');
      return;
    }

    // Validate preimage against hashlock
    if (hashlock) {
      // Simple validation - in real implementation would hash and compare
      const isValid = witnessPreimage.length === 64; // Mock validation
      setPreimageValid(isValid);
    }

    const witness = `<${witnessSignature || 'signature'}>
<${witnessPreimage}>
<htlc_script>`;

    setWitnessPreview(witness);
  };

  const htlcStates = [
    { id: 'created', label: 'Created', status: htlcState === 'created' ? 'completed' as const : 'pending' as const },
    { id: 'funded', label: 'Funded', status: htlcState === 'funded' ? 'active' as const : htlcState === 'created' ? 'pending' as const : 'completed' as const },
    { id: 'claimed', label: 'Claimed', status: htlcState === 'claimed' ? 'completed' as const : 'pending' as const },
    { id: 'expired', label: 'Expired', status: htlcState === 'expired' ? 'completed' as const : 'pending' as const },
    { id: 'refunded', label: 'Refunded', status: htlcState === 'refunded' ? 'completed' as const : 'pending' as const }
  ];

  const blocksRemaining = Math.max(0, expiryBlock - currentBlock);
  const hoursRemaining = Math.round((blocksRemaining * 10) / 60);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-white mb-2">HTLC Laboratory</h2>
        <p className="text-sm text-[#666]">
          Build, decode and test Hash Time-Locked Contracts (offline simulation)
        </p>
      </div>

      <div className="bg-[#111] border border-[#222] rounded-xl">
        <div className="border-b border-[#222] flex">
          {[
            { id: 'builder', label: 'Script Builder' },
            { id: 'decoder', label: 'Script Decoder' },
            { id: 'simulator', label: 'Lifecycle Simulator' },
            { id: 'witness', label: 'Witness Builder' }
          ].map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-6 py-3 text-sm font-medium transition-colors ${activeTab === tab.id
                ? 'text-white border-b-2 border-white'
                : 'text-[#666] hover:text-white'
                }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {activeTab === 'builder' && (
          <div className="grid grid-cols-2 gap-6 p-6">
            {/* Parameters */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Parameters</h3>
                <button
                  onClick={handleGeneratePreimage}
                  className="text-xs px-3 py-1 bg-white/10 text-white rounded hover:bg-white/20 transition-colors"
                >
                  Generate Preimage
                </button>
              </div>

              <div>
                <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                  Hashlock (SHA256)
                </label>
                <input
                  type="text"
                  value={hashlock}
                  onChange={(e) => setHashlock(e.target.value)}
                  placeholder="Click 'Generate Preimage' to create"
                  className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                  Preimage (keep secret!)
                </label>
                <input
                  type="text"
                  value={preimage}
                  readOnly
                  placeholder="Generated with hashlock"
                  className="w-full bg-[#0d0d0d]/50 border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-[#666]"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                  Timelock (Blocks)
                </label>
                <input
                  type="number"
                  value={timelock}
                  onChange={(e) => setTimelock(e.target.value)}
                  className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                  Sender Pubkey
                </label>
                <input
                  type="text"
                  value={senderPubkey}
                  onChange={(e) => setSenderPubkey(e.target.value)}
                  placeholder="02a1633cafcc01ebfb6d78e39f687a1f..."
                  className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                  Receiver Pubkey
                </label>
                <input
                  type="text"
                  value={receiverPubkey}
                  onChange={(e) => setReceiverPubkey(e.target.value)}
                  placeholder="03c9f4836b9a4f77fc0d81f7bcb01b7f..."
                  className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                  Amount (sats)
                </label>
                <input
                  type="number"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                />
              </div>

              <button
                onClick={handleGenerateScript}
                className="w-full px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors font-medium"
              >
                Generate Script
              </button>
            </div>

            {/* Generated Script */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Generated Script</h3>
                {generatedScript && (
                  <button
                    onClick={handleCopyScript}
                    className="text-xs px-3 py-1 bg-white/10 text-white rounded hover:bg-white/20 transition-colors flex items-center gap-1"
                  >
                    {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                    {copied ? 'Copied!' : 'Copy'}
                  </button>
                )}
              </div>
              {generatedScript ? (
                <CodeBlock code={generatedScript} language="bitcoin-script" />
              ) : (
                <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-8 text-center text-[#555]">
                  Fill parameters and click "Generate Script"
                </div>
              )}

              {generatedScript && (
                <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4 space-y-2">
                  <div className="flex justify-between">
                    <span className="text-xs text-[#555]">Script Size</span>
                    <span className="text-xs font-mono text-white">~156 bytes</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-xs text-[#555]">Est. Fee (10 sat/vB)</span>
                    <span className="text-xs font-mono text-white">~1,560 sats</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-xs text-[#555]">Amount</span>
                    <span className="text-xs font-mono text-white">{parseInt(amount).toLocaleString()} sats</span>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === 'decoder' && (
          <div className="p-6 space-y-6">
            <div>
              <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                HTLC Script (Hex)
              </label>
              <textarea
                value={scriptHex}
                onChange={(e) => setScriptHex(e.target.value)}
                placeholder="Paste HTLC script in hexadecimal format..."
                className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white min-h-[120px] resize-none focus:outline-none focus:ring-2 focus:ring-white/10"
              />
            </div>

            <button
              onClick={handleDecodeScript}
              className="px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors font-medium"
            >
              Decode Script
            </button>

            {decodedScript && (
              <div>
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider mb-3">Decoded Opcodes</h3>
                <CodeBlock code={decodedScript} language="bitcoin-script" />
              </div>
            )}
          </div>
        )}

        {activeTab === 'simulator' && (
          <div className="p-6 space-y-6">
            <div>
              <h3 className="text-sm font-semibold text-white uppercase tracking-wider mb-4">Lifecycle States</h3>
              <div className="overflow-x-auto pb-4">
                <StateMachine states={htlcStates} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                <p className="text-xs text-[#555] uppercase tracking-wider mb-2">Current Block</p>
                <p className="text-2xl font-bold text-white">{currentBlock.toLocaleString()}</p>
              </div>
              <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                <p className="text-xs text-[#555] uppercase tracking-wider mb-2">Expiry Block</p>
                <p className="text-2xl font-bold text-white">{expiryBlock.toLocaleString()}</p>
              </div>
              <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                <p className="text-xs text-[#555] uppercase tracking-wider mb-2">Blocks Remaining</p>
                <p className="text-2xl font-bold text-[#F59E0B]">{blocksRemaining}</p>
              </div>
              <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                <p className="text-xs text-[#555] uppercase tracking-wider mb-2">Time Remaining</p>
                <p className="text-2xl font-bold text-white">~{hoursRemaining}h</p>
              </div>
            </div>

            <div className="flex gap-3">
              <button
                onClick={handleSimulateClaim}
                disabled={htlcState === 'claimed'}
                className="px-4 py-2 bg-[#10B981] text-white rounded-lg hover:bg-[#10B981]/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Simulate Claim
              </button>
              <button
                onClick={handleSimulateRefund}
                disabled={htlcState === 'refunded'}
                className="px-4 py-2 bg-[#EF4444] text-white rounded-lg hover:bg-[#EF4444]/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Simulate Refund
              </button>
              <button
                onClick={handleResetSimulator}
                className="px-4 py-2 bg-[#0d0d0d] border border-[#222] text-white rounded-lg hover:bg-[#151515] transition-colors"
              >
                Reset
              </button>
            </div>
          </div>
        )}

        {activeTab === 'witness' && (
          <div className="p-6 space-y-6">
            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Witness Data</h3>

                <div>
                  <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                    Preimage
                  </label>
                  <input
                    type="text"
                    value={witnessPreimage}
                    onChange={(e) => setWitnessPreimage(e.target.value)}
                    placeholder={preimage || "Secret preimage value..."}
                    className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                  />
                  {preimageValid !== null && (
                    <p className={`text-xs mt-1 ${preimageValid ? 'text-[#10B981]' : 'text-[#EF4444]'}`}>
                      {preimageValid ? '✓ Valid preimage format' : '✗ Invalid preimage'}
                    </p>
                  )}
                </div>

                <div>
                  <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                    Signature
                  </label>
                  <input
                    type="text"
                    value={witnessSignature}
                    onChange={(e) => setWitnessSignature(e.target.value)}
                    placeholder="ECDSA signature..."
                    className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                  />
                </div>

                <button
                  onClick={handleBuildWitness}
                  className="w-full px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors font-medium"
                >
                  Build Witness
                </button>
              </div>

              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Witness Preview</h3>
                {witnessPreview ? (
                  <CodeBlock
                    code={witnessPreview}
                    language="witness"
                    showLineNumbers={false}
                  />
                ) : (
                  <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-8 text-center text-[#555]">
                    Enter witness data and click "Build Witness"
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
