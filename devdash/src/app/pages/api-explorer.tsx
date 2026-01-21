import { useState } from 'react';
import { Send, Clock, CheckCircle, XCircle, Loader2 } from 'lucide-react';
import { CodeBlock } from '@/app/components/code-block';

// All available endpoints
const API_ENDPOINTS = [
  {
    category: 'System',
    endpoints: [
      { method: 'GET', path: '/api/v1/system/status', description: 'Get system status', body: null },
      { method: 'GET', path: '/api/v1/system/health', description: 'Health check', body: null },
      { method: 'GET', path: '/api/v1/events/recent', description: 'Recent events', body: null },
    ]
  },
  {
    category: 'Wallet',
    endpoints: [
      { method: 'POST', path: '/api/v1/wallet/generate', description: 'Generate HD wallet', body: { word_count: 24 } },
      { method: 'POST', path: '/api/v1/wallet/derive', description: 'Derive addresses', body: { seed_phrase: 'abandon abandon abandon...' } },
      { method: 'POST', path: '/api/v1/wallet/validate', description: 'Validate mnemonic', body: { mnemonic: 'abandon abandon abandon...' } },
    ]
  },
  {
    category: 'Preimage',
    endpoints: [
      { method: 'POST', path: '/api/v1/preimage/generate', description: 'Generate preimage', body: {} },
      { method: 'POST', path: '/api/v1/preimage/verify', description: 'Verify preimage', body: { preimage: 'abc123...', payment_hash: 'def456...' } },
    ]
  },
  {
    category: 'HTLC',
    endpoints: [
      { method: 'POST', path: '/api/v1/htlc/create', description: 'Create HTLC', body: { amount_sats: 10000, timeout_blocks: 144, receiver_pubkey: '03...', sender_pubkey: '02...' } },
      { method: 'POST', path: '/api/v1/htlc/decode', description: 'Decode HTLC script', body: { script_hex: '6382012088a820...' } },
    ]
  },
  {
    category: 'Lightning',
    endpoints: [
      { method: 'GET', path: '/api/v1/lightning/info', description: 'Lightning node info', body: null },
      { method: 'GET', path: '/api/v1/lightning/balance', description: 'Channel balances', body: null },
    ]
  },
  {
    category: 'Bitcoin',
    endpoints: [
      { method: 'GET', path: '/api/v1/bitcoin/info', description: 'Bitcoin node info', body: null },
      { method: 'GET', path: '/api/v1/bitcoin/fees', description: 'Fee estimates', body: null },
    ]
  },
  {
    category: 'Elements',
    endpoints: [
      { method: 'GET', path: '/api/v1/elements/info', description: 'Elements node info', body: null },
    ]
  }
];

interface RequestHistory {
  method: string;
  path: string;
  status: number;
  time: string;
  timestamp: string;
}

export function APIExplorerPage() {
  const [selectedEndpoint, setSelectedEndpoint] = useState(API_ENDPOINTS[0].endpoints[0]);
  const [requestBody, setRequestBody] = useState(JSON.stringify(API_ENDPOINTS[0].endpoints[0].body, null, 2));
  const [response, setResponse] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [responseTime, setResponseTime] = useState<number>(0);
  const [statusCode, setStatusCode] = useState<number>(0);
  const [history, setHistory] = useState<RequestHistory[]>([]);

  const handleEndpointSelect = (endpoint: typeof selectedEndpoint) => {
    setSelectedEndpoint(endpoint);
    setRequestBody(JSON.stringify(endpoint.body, null, 2));
    setResponse(null);
    setError(null);
  };

  const handleSendRequest = async () => {
    setLoading(true);
    setError(null);
    const startTime = performance.now();

    try {
      const options: RequestInit = {
        method: selectedEndpoint.method,
        headers: {
          'Content-Type': 'application/json',
        },
      };

      if (selectedEndpoint.method === 'POST' && requestBody) {
        options.body = requestBody;
      }

      const res = await fetch(selectedEndpoint.path, options);
      const endTime = performance.now();
      const time = Math.round(endTime - startTime);

      setResponseTime(time);
      setStatusCode(res.status);

      const data = await res.json();
      setResponse(data);

      // Add to history
      const now = new Date();
      setHistory(prev => [{
        method: selectedEndpoint.method,
        path: selectedEndpoint.path,
        status: res.status,
        time: `${time}ms`,
        timestamp: now.toLocaleTimeString('pt-BR')
      }, ...prev].slice(0, 10));

    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed');
      setStatusCode(0);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="grid grid-cols-[320px_1fr] gap-6 h-full">
      {/* Sidebar - Endpoints List */}
      <div className="space-y-4">
        <div>
          <h2 className="text-2xl font-bold text-white mb-2">API Explorer</h2>
          <p className="text-sm text-[#666]">Test real API endpoints</p>
        </div>

        <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
          {API_ENDPOINTS.map(category => (
            <div key={category.category} className="border-b border-[#1a1a1a] last:border-0">
              <div className="px-4 py-2 bg-[#0d0d0d]">
                <p className="text-xs font-semibold text-[#555] uppercase tracking-wider">
                  {category.category}
                </p>
              </div>
              <div>
                {category.endpoints.map(endpoint => (
                  <button
                    key={endpoint.path}
                    onClick={() => handleEndpointSelect(endpoint)}
                    className={`w-full text-left px-4 py-2.5 hover:bg-[#151515] transition-colors border-l-2 ${selectedEndpoint.path === endpoint.path
                        ? 'border-l-white bg-[#151515]'
                        : 'border-l-transparent'
                      }`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${endpoint.method === 'GET'
                          ? 'bg-[#10B981]/10 text-[#10B981]'
                          : 'bg-[#F7931A]/10 text-[#F7931A]'
                        }`}>
                        {endpoint.method}
                      </span>
                      <span className="text-xs font-mono text-white truncate">
                        {endpoint.path.replace('/api/v1', '')}
                      </span>
                    </div>
                    <p className="text-xs text-[#555]">{endpoint.description}</p>
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Main Content - Request Builder */}
      <div className="space-y-6">
        <div className="bg-[#111] border border-[#222] rounded-xl p-6 space-y-6">
          {/* Endpoint Info */}
          <div>
            <div className="flex items-center gap-3 mb-2">
              <span className={`text-sm font-bold px-2.5 py-1 rounded ${selectedEndpoint.method === 'GET'
                  ? 'bg-[#10B981]/10 text-[#10B981]'
                  : 'bg-[#F7931A]/10 text-[#F7931A]'
                }`}>
                {selectedEndpoint.method}
              </span>
              <code className="text-sm font-mono text-white">{selectedEndpoint.path}</code>
            </div>
            <p className="text-sm text-[#666]">{selectedEndpoint.description}</p>
          </div>

          {/* Request Body */}
          {selectedEndpoint.method === 'POST' && (
            <div>
              <label className="text-xs font-medium text-[#555] uppercase tracking-wider mb-2 block">
                Request Body (JSON)
              </label>
              <textarea
                value={requestBody}
                onChange={(e) => setRequestBody(e.target.value)}
                className="w-full bg-[#0d0d0d] border border-[#222] rounded-lg px-3 py-2 text-sm font-mono text-white min-h-[120px] resize-none focus:outline-none focus:ring-2 focus:ring-white/10"
              />
            </div>
          )}

          {/* Send Button */}
          <button
            onClick={handleSendRequest}
            disabled={loading}
            className="w-full px-4 py-2.5 bg-white text-black rounded-lg hover:bg-[#eee] transition-colors flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed font-medium"
          >
            {loading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Sending...
              </>
            ) : (
              <>
                <Send className="w-4 h-4" />
                Send Request
              </>
            )}
          </button>
        </div>

        {/* Response */}
        {(response || error) && (
          <div className="bg-[#111] border border-[#222] rounded-xl p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Response</h3>
              <div className="flex items-center gap-4">
                <div className="flex items-center gap-2 text-xs">
                  <Clock className="w-3 h-3 text-[#555]" />
                  <span className="font-mono text-white">{responseTime}ms</span>
                </div>
                <div className="flex items-center gap-2">
                  {error ? (
                    <>
                      <XCircle className="w-3 h-3 text-[#EF4444]" />
                      <span className="text-xs font-bold text-[#EF4444]">ERROR</span>
                    </>
                  ) : (
                    <>
                      <CheckCircle className="w-3 h-3 text-[#10B981]" />
                      <span className="text-xs font-bold text-[#10B981]">{statusCode}</span>
                    </>
                  )}
                </div>
              </div>
            </div>

            <CodeBlock
              code={error || JSON.stringify(response, null, 2)}
              language="json"
            />
          </div>
        )}

        {/* Request History */}
        {history.length > 0 && (
          <div className="bg-[#111] border border-[#222] rounded-xl overflow-hidden">
            <div className="px-4 py-3 border-b border-[#1a1a1a] bg-[#0d0d0d]">
              <h3 className="text-sm font-semibold text-white uppercase tracking-wider">Recent Requests</h3>
            </div>
            <div className="divide-y divide-[#1a1a1a]">
              {history.map((req, i) => (
                <div key={i} className="px-4 py-2.5 hover:bg-[#151515] transition-colors flex items-center gap-3">
                  <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${req.method === 'GET'
                      ? 'bg-[#10B981]/10 text-[#10B981]'
                      : 'bg-[#F7931A]/10 text-[#F7931A]'
                    }`}>
                    {req.method}
                  </span>
                  <code className="flex-1 text-xs font-mono text-white">{req.path}</code>
                  <span className={`text-xs font-mono ${req.status >= 200 && req.status < 300 ? 'text-[#10B981]' : 'text-[#EF4444]'}`}>
                    {req.status}
                  </span>
                  <span className="text-xs font-mono text-[#555]">{req.time}</span>
                  <span className="text-xs text-[#555]">{req.timestamp}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
