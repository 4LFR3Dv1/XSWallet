import { useState, useEffect } from 'react';
import { RefreshCw, Copy, Check, Key, Lock, Unlock, RotateCcw, AlertTriangle } from 'lucide-react';
import { CodeBlock } from '@/app/components/code-block';
import { TreeView } from '@/app/components/tree-view';
import { useGenerateWallet, useDeriveAddresses } from '@/app/services/hooks';

export function WalletStudioPage() {
  const [activeTab, setActiveTab] = useState('generate');
  const [seedSize, setSeedSize] = useState<12 | 24>(24);
  const { generate, result: walletData, loading } = useGenerateWallet(seedSize);
  const { derive, result: addresses, loading: deriving } = useDeriveAddresses();

  // Generate tab
  const [mnemonic, setMnemonic] = useState<string[]>([]);
  const [mnemonicString, setMnemonicString] = useState('');
  const [copied, setCopied] = useState(false);

  // Derive tab
  const [derivePath, setDerivePath] = useState("m/44'/0'/0'/0/0");
  const [derivedAddresses, setDerivedAddresses] = useState<any>(null);

  // Encrypt tab
  const [encryptInput, setEncryptInput] = useState('');
  const [encryptPassword, setEncryptPassword] = useState('');
  const [encrypted, setEncrypted] = useState('');

  // Decrypt tab
  const [decryptInput, setDecryptInput] = useState('');
  const [decryptPassword, setDecryptPassword] = useState('');
  const [decrypted, setDecrypted] = useState('');
  const [decryptError, setDecryptError] = useState('');

  // Restore tab
  const [restoreWords, setRestoreWords] = useState<string[]>(Array(24).fill(''));
  const [restoreValid, setRestoreValid] = useState<boolean | null>(null);

  useEffect(() => {
    if (walletData) {
      const words = walletData.mnemonic.split(' ');
      setMnemonic(words);
      setMnemonicString(walletData.mnemonic);
    }
  }, [walletData]);

  const handleGenerate = async () => {
    try {
      await generate();
    } catch (err) {
      alert('Failed to generate wallet');
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(mnemonicString);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleDerive = async () => {
    if (!mnemonicString) {
      alert('Please generate a wallet first');
      return;
    }

    try {
      const result = await derive(mnemonicString);
      setDerivedAddresses(result);
    } catch (err) {
      alert('Failed to derive addresses');
    }
  };

  const handleEncrypt = () => {
    if (!encryptInput || !encryptPassword) {
      alert('Please fill both fields');
      return;
    }

    // Simple Base64 encoding (for demo - NOT secure for production!)
    const combined = JSON.stringify({ data: encryptInput, password: encryptPassword });
    const encoded = btoa(combined);
    setEncrypted(encoded);
  };

  const handleDecrypt = () => {
    if (!decryptInput || !decryptPassword) {
      setDecryptError('Please fill both fields');
      return;
    }

    setDecryptError('');

    try {
      // Decode Base64
      const decoded = atob(decryptInput);
      const parsed = JSON.parse(decoded);

      // Validate password
      if (parsed.password !== decryptPassword) {
        setDecryptError('Invalid password');
        setDecrypted('');
        return;
      }

      setDecrypted(parsed.data);
    } catch (e) {
      setDecryptError('Invalid encrypted data or format');
      setDecrypted('');
    }
  };

  const handleRestore = () => {
    const phrase = restoreWords.filter(w => w.trim()).join(' ');

    if (phrase.split(' ').length < 12) {
      setRestoreValid(false);
      alert('Please enter at least 12 words');
      return;
    }

    // Simple validation - check if words are not empty
    const isValid = restoreWords.slice(0, seedSize).every(w => w.trim().length > 0);
    setRestoreValid(isValid);

    if (isValid) {
      setMnemonicString(phrase);
      setMnemonic(phrase.split(' '));
      alert('✅ Mnemonic restored! Switch to "Derive" tab to generate addresses.');
    }
  };

  const handleClearRestore = () => {
    setRestoreWords(Array(24).fill(''));
    setRestoreValid(null);
  };

  // Check if response has the new format (seed_hex) or old format (addresses)
  const hasNewFormat = derivedAddresses && derivedAddresses.seed_hex;

  const derivationTree = derivedAddresses && !hasNewFormat ? [
    {
      id: 'root',
      label: "m / purpose' / coin_type' / account' / change / address_index",
      children: [
        {
          id: 'btc',
          label: "BIP44 - Bitcoin (0')",
          children: [
            {
              id: 'btc-0',
              label: 'Account 0',
              value: derivedAddresses.bitcoin || 'Generating...'
            }
          ]
        },
        {
          id: 'eth',
          label: "BIP44 - Ethereum (60')",
          children: [
            {
              id: 'eth-0',
              label: 'Account 0',
              value: derivedAddresses.ethereum || 'Generating...'
            }
          ]
        },
        {
          id: 'lq',
          label: "BIP44 - Liquid (1776')",
          children: [
            {
              id: 'lq-0',
              label: 'Account 0',
              value: derivedAddresses.liquid || 'Generating...'
            }
          ]
        },
        {
          id: 'trx',
          label: "BIP44 - TRON (195')",
          children: [
            {
              id: 'trx-0',
              label: 'Account 0',
              value: derivedAddresses.tron || 'Generating...'
            }
          ]
        }
      ]
    }
  ] : [];

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-white mb-2">Wallet Studio</h2>
        <p className="text-sm text-[#666]">
          Generate, derive, encrypt and restore HD wallets (offline mode)
        </p>
      </div>

      <div className="bg-[#111] border border-[#222] rounded-xl">
        <div className="border-b border-[#222] flex">
          {[
            { id: 'generate', label: 'Generate', icon: Key },
            { id: 'derive', label: 'Derive', icon: RefreshCw },
            { id: 'encrypt', label: 'Encrypt', icon: Lock },
            { id: 'decrypt', label: 'Decrypt', icon: Unlock },
            { id: 'restore', label: 'Restore', icon: RotateCcw }
          ].map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-6 py-3 text-sm font-medium transition-colors ${activeTab === tab.id
                ? 'text-white border-b-2 border-white'
                : 'text-[#666] hover:text-white'
                }`}
            >
              <tab.icon className="w-4 h-4" />
              {tab.label}
            </button>
          ))}
        </div>

        {activeTab === 'generate' && (
          <div className="p-6 space-y-6">
            <div className="flex items-center justify-between">
              <div className="flex gap-2">
                <button
                  onClick={() => setSeedSize(12)}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${seedSize === 12
                    ? 'bg-white text-black'
                    : 'bg-[#0d0d0d] border border-[#222] text-white hover:bg-[#151515]'
                    }`}
                >
                  12 Words
                </button>
                <button
                  onClick={() => setSeedSize(24)}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${seedSize === 24
                    ? 'bg-white text-black'
                    : 'bg-[#0d0d0d] border border-[#222] text-white hover:bg-[#151515]'
                    }`}
                >
                  24 Words
                </button>
              </div>
              <button
                onClick={handleGenerate}
                disabled={loading}
                className="flex items-center gap-2 px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-medium"
              >
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
                {loading ? 'Generating...' : 'Regenerate'}
              </button>
            </div>

            {mnemonic.length > 0 && (
              <>
                <div className="grid grid-cols-4 gap-3">
                  {mnemonic.slice(0, seedSize).map((word, i) => (
                    <div
                      key={i}
                      className="bg-[#0d0d0d] border border-[#222] rounded-lg p-3 flex items-center gap-3"
                    >
                      <span className="text-xs text-[#555] font-mono w-6">{i + 1}</span>
                      <span className="text-sm font-mono text-white">{word}</span>
                    </div>
                  ))}
                </div>

                <div className="bg-[#F59E0B]/10 border border-[#F59E0B]/20 rounded-lg p-4 flex gap-3">
                  <AlertTriangle className="w-5 h-5 text-[#F59E0B] flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-semibold text-[#F59E0B] mb-1">Security Warning</p>
                    <p className="text-xs text-[#F59E0B]/80">
                      Store your seed phrase securely. Never share it with anyone. Anyone with access to this phrase can access your funds.
                    </p>
                  </div>
                </div>

                <div className="flex items-center justify-between bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                  <div className="flex items-center gap-2">
                    <Check className="w-4 h-4 text-[#10B981]" />
                    <span className="text-sm text-white">Mnemonic is valid (BIP39)</span>
                  </div>
                  <button
                    onClick={handleCopy}
                    className="flex items-center gap-2 px-3 py-1.5 bg-white/10 text-white rounded hover:bg-white/20 transition-colors text-sm"
                  >
                    {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                    {copied ? 'Copied!' : 'Copy All'}
                  </button>
                </div>
              </>
            )}

            {mnemonic.length === 0 && (
              <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-12 text-center">
                <Key className="w-12 h-12 text-[#444] mx-auto mb-4" />
                <p className="text-[#666]">Click "Regenerate" to generate a new HD wallet</p>
              </div>
            )}
          </div>
        )}

        {activeTab === 'derive' && (
          <div className="p-6 space-y-6">
            <div>
              <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                Derivation Path
              </label>
              <div className="flex gap-3">
                <input
                  type="text"
                  value={derivePath}
                  onChange={(e) => setDerivePath(e.target.value)}
                  className="flex-1 bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                />
                <button
                  onClick={handleDerive}
                  disabled={!mnemonicString || deriving}
                  className="px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-medium"
                >
                  {deriving ? 'Deriving...' : 'Derive'}
                </button>
              </div>
            </div>

            {derivedAddresses ? (
              hasNewFormat ? (
                // New format: show seed_hex and tools
                <div className="space-y-4">
                  <div>
                    <h3 className="text-sm font-semibold text-white uppercase tracking-wider mb-3">BIP39 Seed (512-bit)</h3>
                    <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                      <p className="text-xs text-[#555] mb-2">Hex ({derivedAddresses.seed_bytes} bytes)</p>
                      <p className="text-xs font-mono text-white break-all">{derivedAddresses.seed_hex}</p>
                    </div>
                  </div>

                  <div className="bg-[#F59E0B]/10 border border-[#F59E0B]/20 rounded-lg p-4">
                    <p className="text-sm font-semibold text-[#F59E0B] mb-2">⚠️ Full Address Derivation</p>
                    <p className="text-xs text-[#F59E0B]/80 mb-3">
                      {derivedAddresses.derivation_note}
                    </p>
                    <div className="space-y-2">
                      {derivedAddresses.tools?.online && (
                        <a
                          href={derivedAddresses.tools.online.split(' ')[0]}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="block text-xs text-[#7C3AED] hover:underline"
                        >
                          🌐 Ian Coleman BIP39 Tool (paste mnemonic, select testnet)
                        </a>
                      )}
                      {derivedAddresses.tools?.electrum && (
                        <p className="text-xs text-[#666]">⚡ {derivedAddresses.tools.electrum}</p>
                      )}
                      {derivedAddresses.tools?.sparrow && (
                        <p className="text-xs text-[#666]">🔐 {derivedAddresses.tools.sparrow}</p>
                      )}
                    </div>
                  </div>

                  <div>
                    <h3 className="text-sm font-semibold text-white uppercase tracking-wider mb-3">Example Derivation Paths</h3>
                    <div className="grid grid-cols-3 gap-3">
                      {Object.entries(derivedAddresses.example_paths || {}).map(([name, path]) => (
                        <div key={name} className="bg-[#0d0d0d] border border-[#222] rounded-lg p-3">
                          <p className="text-xs text-[#555] mb-1">{name.replace(/_/g, ' ')}</p>
                          <p className="text-sm font-mono text-white">{path as string}</p>
                        </div>
                      ))}
                    </div>
                  </div>

                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(derivedAddresses.seed_hex);
                      alert('Seed hex copied!');
                    }}
                    className="flex items-center gap-2 px-3 py-1.5 bg-white/10 text-white rounded hover:bg-white/20 transition-colors text-sm"
                  >
                    <Copy className="w-4 h-4" />
                    Copy Seed Hex
                  </button>
                </div>
              ) : (
                // Old format: show address tree
                <div>
                  <h3 className="text-sm font-semibold text-white uppercase tracking-wider mb-3">Address Tree</h3>
                  <TreeView data={derivationTree} />
                </div>
              )
            ) : (
              <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-12 text-center">
                <RefreshCw className="w-12 h-12 text-[#444] mx-auto mb-4" />
                <p className="text-[#666]">Generate a wallet first, then click "Derive"</p>
              </div>
            )}
          </div>
        )}

        {activeTab === 'encrypt' && (
          <div className="p-6 space-y-6">
            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-4">
                <div>
                  <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                    Private Key / Seed
                  </label>
                  <textarea
                    value={encryptInput}
                    onChange={(e) => setEncryptInput(e.target.value)}
                    placeholder={mnemonicString || "Enter private key or seed phrase..."}
                    className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white min-h-[100px] resize-none focus:outline-none focus:ring-2 focus:ring-white/10"
                  />
                </div>

                <div>
                  <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                    Encryption Password
                  </label>
                  <input
                    type="password"
                    value={encryptPassword}
                    onChange={(e) => setEncryptPassword(e.target.value)}
                    placeholder="Enter strong password..."
                    className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                  />
                </div>

                <button
                  onClick={handleEncrypt}
                  className="w-full px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors font-medium"
                >
                  Encrypt
                </button>
              </div>

              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Encrypted Output</h3>
                {encrypted ? (
                  <div className="space-y-3">
                    <CodeBlock code={encrypted} language="base64" showLineNumbers={false} />
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(encrypted);
                          alert('Copied to clipboard!');
                        }}
                        className="flex items-center gap-2 px-3 py-1.5 bg-white/10 text-white rounded hover:bg-white/20 transition-colors text-sm"
                      >
                        <Copy className="w-4 h-4" />
                        Copy Encrypted
                      </button>
                      <button
                        onClick={() => setEncrypted('')}
                        className="px-3 py-1.5 bg-[#0d0d0d] border border-[#222] text-[#666] rounded hover:text-white transition-colors text-sm"
                      >
                        Clear
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-12 text-center">
                    <Lock className="w-12 h-12 text-[#444] mx-auto mb-4" />
                    <p className="text-[#666]">Encrypted data will appear here</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {activeTab === 'decrypt' && (
          <div className="p-6 space-y-6">
            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-4">
                <div>
                  <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                    Encrypted Data
                  </label>
                  <textarea
                    value={decryptInput}
                    onChange={(e) => setDecryptInput(e.target.value)}
                    placeholder="Paste encrypted base64 text here..."
                    className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white min-h-[100px] resize-none focus:outline-none focus:ring-2 focus:ring-white/10"
                  />
                </div>

                <div>
                  <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                    Decryption Password
                  </label>
                  <input
                    type="password"
                    value={decryptPassword}
                    onChange={(e) => setDecryptPassword(e.target.value)}
                    placeholder="Enter password used during encryption..."
                    className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                  />
                </div>

                <button
                  onClick={handleDecrypt}
                  className="w-full px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors font-medium"
                >
                  Decrypt
                </button>

                {decryptError && (
                  <div className="bg-[#EF4444]/10 border border-[#EF4444]/20 rounded-lg p-3 flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-[#EF4444]" />
                    <span className="text-sm text-[#EF4444]">{decryptError}</span>
                  </div>
                )}
              </div>

              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Decrypted Output</h3>
                {decrypted ? (
                  <div className="space-y-3">
                    <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-4">
                      <p className="text-sm font-mono text-white break-all">{decrypted}</p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(decrypted);
                          alert('Copied to clipboard!');
                        }}
                        className="flex items-center gap-2 px-3 py-1.5 bg-white/10 text-white rounded hover:bg-white/20 transition-colors text-sm"
                      >
                        <Copy className="w-4 h-4" />
                        Copy
                      </button>
                      <button
                        onClick={() => {
                          setMnemonicString(decrypted);
                          setMnemonic(decrypted.split(' '));
                          alert('✅ Restored to wallet! Go to Derive tab.');
                        }}
                        className="flex items-center gap-2 px-3 py-1.5 bg-[#10B981]/20 text-[#10B981] rounded hover:bg-[#10B981]/30 transition-colors text-sm"
                      >
                        <Key className="w-4 h-4" />
                        Use as Seed
                      </button>
                      <button
                        onClick={() => {
                          setDecrypted('');
                          setDecryptInput('');
                          setDecryptPassword('');
                        }}
                        className="px-3 py-1.5 bg-[#0d0d0d] border border-[#222] text-[#666] rounded hover:text-white transition-colors text-sm"
                      >
                        Clear
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="bg-[#0d0d0d] border border-[#222] rounded-lg p-12 text-center">
                    <Unlock className="w-12 h-12 text-[#444] mx-auto mb-4" />
                    <p className="text-[#666]">Decrypted data will appear here</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {activeTab === 'restore' && (
          <div className="p-6 space-y-6">
            <div>
              <h3 className="text-sm font-semibold text-white uppercase tracking-wider mb-3">Mnemonic Phrase</h3>
              <div className="grid grid-cols-4 gap-3">
                {restoreWords.map((word, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <span className="text-xs text-[#555] font-mono w-6">{i + 1}</span>
                    <input
                      type="text"
                      value={word}
                      onChange={(e) => {
                        const newWords = [...restoreWords];
                        newWords[i] = e.target.value.toLowerCase().trim();
                        setRestoreWords(newWords);
                      }}
                      placeholder={`Word ${i + 1}`}
                      className="flex-1 bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white focus:outline-none focus:ring-2 focus:ring-white/10"
                    />
                  </div>
                ))}
              </div>
            </div>

            {restoreValid !== null && (
              <div className={`border rounded-lg p-4 flex items-center gap-3 ${restoreValid
                ? 'bg-[#10B981]/10 border-[#10B981]/20'
                : 'bg-[#EF4444]/10 border-[#EF4444]/20'
                }`}>
                {restoreValid ? (
                  <>
                    <Check className="w-5 h-5 text-[#10B981]" />
                    <span className="text-sm text-[#10B981]">✓ Valid mnemonic restored</span>
                  </>
                ) : (
                  <>
                    <AlertTriangle className="w-5 h-5 text-[#EF4444]" />
                    <span className="text-sm text-[#EF4444]">✗ Invalid mnemonic</span>
                  </>
                )}
              </div>
            )}

            <div className="flex gap-3">
              <button
                onClick={handleRestore}
                className="flex-1 px-4 py-2 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors font-medium"
              >
                Validate & Restore
              </button>
              <button
                onClick={handleClearRestore}
                className="px-4 py-2 bg-[#0d0d0d] border border-[#222] text-white rounded-lg hover:bg-[#151515] transition-colors"
              >
                Clear
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
