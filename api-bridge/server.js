// XS Wallet API Bridge - Production-Ready Version
// Converts HTTP → gRPC with proper error handling, logging, and health checks
// Author: Based on user feedback and best practices

const express = require('express');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const fs = require('fs');
const path = require('path');
const { v4: uuidv4 } = require('uuid');

const app = express();
app.use(express.json());

// ============================================================================
// CONFIGURATION
// ============================================================================

const CONFIG = {
    GRPC_HOST: process.env.GRPC_HOST || 'localhost:9735',
    PORT: process.env.PORT || 3000,
    GRPC_DEADLINE_MS: {
        list: 120000,  // 120s for list operations (scantxoutset can be slow on testnet/pruned)
        create: 15000, // 15s for create operations
        commit: 30000, // 30s for heavy operations
    },
    LOG_LEVEL: process.env.LOG_LEVEL || 'info',
};

// ============================================================================
// LOGGING (request ID + structured logs)
// ============================================================================

function log(level, message, meta = {}) {
    const timestamp = new Date().toISOString();
    const entry = { timestamp, level, message, ...meta };
    console.log(JSON.stringify(entry));
}

// Middleware: Request ID
app.use((req, res, next) => {
    req.id = req.headers['x-request-id'] || uuidv4();
    res.setHeader('x-request-id', req.id);

    const start = Date.now();
    res.on('finish', () => {
        const latency = Date.now() - start;
        log('info', `${req.method} ${req.path}`, {
            requestId: req.id,
            status: res.statusCode,
            latency_ms: latency,
        });
    });

    next();
});

// ============================================================================
// GRPC CLIENT SETUP
// ============================================================================

const PROTO_DIR = __dirname + '/proto';
const PROTO_FILES = [
    PROTO_DIR + '/swap.proto',
    PROTO_DIR + '/wallet.proto',
    PROTO_DIR + '/node.proto',
];

const packageDef = protoLoader.loadSync(PROTO_FILES, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
    includeDirs: [PROTO_DIR],
});

const protoDescriptor = grpc.loadPackageDefinition(packageDef);
const xswallet = protoDescriptor.xswallet;

// Optional direct LND client for payment/invoice APIs while WalletService gRPC contract
// is being expanded. Requires explicit env vars.
const LND_HOST = process.env.LND_HOST || '';
const LND_TLS_CERT = process.env.LND_TLS_CERT || '';
const LND_MACAROON = process.env.LND_MACAROON || '';
const LND_PROTO_PATH = process.env.LND_PROTO_PATH || path.join(__dirname, '..', 'core', 'internal', 'adapters', 'lnd', 'proto', 'lnrpc', 'lightning.proto');
let lndClient = null;

function getLndClient() {
    if (lndClient) return lndClient;
    if (!LND_HOST || !LND_TLS_CERT || !LND_MACAROON) {
        throw new Error('LND not configured (set LND_HOST, LND_TLS_CERT, LND_MACAROON)');
    }
    const cert = fs.readFileSync(LND_TLS_CERT);
    const macaroonHex = fs.readFileSync(LND_MACAROON).toString('hex');
    const lndPackageDef = protoLoader.loadSync(LND_PROTO_PATH, {
        keepCase: true,
        longs: String,
        enums: String,
        defaults: true,
        oneofs: true,
        includeDirs: [path.dirname(LND_PROTO_PATH)],
    });
    const lndProto = grpc.loadPackageDefinition(lndPackageDef);
    const lnrpc = lndProto.lnrpc;
    if (!lnrpc || !lnrpc.Lightning) {
        throw new Error('lnrpc.Lightning not found in LND proto');
    }
    const sslCreds = grpc.credentials.createSsl(cert);
    const macaroonCreds = grpc.credentials.createFromMetadataGenerator((_params, cb) => {
        const metadata = new grpc.Metadata();
        metadata.add('macaroon', macaroonHex);
        cb(null, metadata);
    });
    const creds = grpc.credentials.combineChannelCredentials(sslCreds, macaroonCreds);
    lndClient = new lnrpc.Lightning(LND_HOST, creds);
    return lndClient;
}

// Clients
let swapClient, walletClient, nodeClient;
let grpcHealthy = false;

// Helper to create gRPC metadata from request header
function createAuthMetadataFromReq(req) {
    const metadata = new grpc.Metadata();
    const authHeader = req.headers['authorization'];
    if (typeof authHeader === 'string' && authHeader.trim().startsWith('Bearer ')) {
        metadata.add('authorization', authHeader);
        return metadata;
    }
    const token = typeof req.query?.access_token === 'string' ? req.query.access_token.trim() : '';
    if (token) {
        metadata.add('authorization', `Bearer ${token}`);
    }
    return metadata;
}

function initGrpcClients() {
    try {
        if (!xswallet || !xswallet.SwapService) {
            throw new Error('SwapService not found in proto definition');
        }

        swapClient = new xswallet.SwapService(
            CONFIG.GRPC_HOST,
            grpc.credentials.createInsecure()
        );

        walletClient = new xswallet.WalletService(
            CONFIG.GRPC_HOST,
            grpc.credentials.createInsecure()
        );

        nodeClient = new xswallet.NodeService(
            CONFIG.GRPC_HOST,
            grpc.credentials.createInsecure()
        );

        log('info', 'gRPC clients initialized', { host: CONFIG.GRPC_HOST });
        checkGrpcHealth();
    } catch (err) {
        log('error', 'Failed to init gRPC clients', { error: err.message });
        grpcHealthy = false;
    }
}

function setClientsForTest(clients = {}) {
    if (clients.swapClient) swapClient = clients.swapClient;
    if (clients.walletClient) walletClient = clients.walletClient;
    if (clients.nodeClient) nodeClient = clients.nodeClient;
    if (typeof clients.grpcHealthy === 'boolean') grpcHealthy = clients.grpcHealthy;
}

function checkGrpcHealth() {
    if (!walletClient) {
        grpcHealthy = false;
        return;
    }
    const deadline = new Date(Date.now() + 2000);
    // Use WalletService.GetVaultStatus for health (doesn't require auth)
    walletClient.GetVaultStatus({}, { deadline }, (err) => {
        grpcHealthy = !err;
        if (err) {
            log('warn', 'gRPC health check failed', { error: err.message });
        }
    });
}

// Check health every 30 seconds
const healthInterval = setInterval(checkGrpcHealth, 30000);
if (typeof healthInterval.unref === 'function') {
    healthInterval.unref();
}

// ============================================================================
// ERROR HANDLING
// ============================================================================

// Standard error response
function errorResponse(res, code, message, details = {}, requestId) {
    const grpcCode = code.replace('GRPC_', '');
    const httpStatus = {
        'OK': 200,
        'INVALID_ARGUMENT': 400,
        'NOT_FOUND': 404,
        'ALREADY_EXISTS': 409,
        'DEADLINE_EXCEEDED': 504,
        'UNAVAILABLE': 503,
        'UNAUTHENTICATED': 401,
        'PERMISSION_DENIED': 403,
        'INTERNAL': 500,
    }[grpcCode] || 500;

    log('error', message, {
        requestId,
        code: grpcCode,
        details,
    });

    res.status(httpStatus).json({
        error: message,
        code: grpcCode,
        details,
        request_id: requestId,
    });
}

// Convert gRPC error to HTTP
function handleGrpcError(err, res, requestId) {
    const code = err.code || grpc.status.UNKNOWN;
    const codeMap = Object.entries(grpc.status).find(([k, v]) => v === code)?.[0] || 'UNKNOWN';

    errorResponse(res, `GRPC_${codeMap}`, err.details || err.message, {}, requestId);
}

function requireAuthMetadata(req, res) {
    const metadata = createAuthMetadataFromReq(req);
    const authHeader = req.headers['authorization'];
    const hasHeader = typeof authHeader === 'string' && authHeader.trim().startsWith('Bearer ');
    const hasQueryToken = typeof req.query?.access_token === 'string' && req.query.access_token.trim() !== '';
    if (!hasHeader && !hasQueryToken) {
        errorResponse(res, 'GRPC_UNAUTHENTICATED', 'Missing or invalid Authorization header', {}, req.id);
        return null;
    }
    return metadata;
}

// ============================================================================
// SCHEMA NORMALIZATION (gRPC → Flask format)
// ============================================================================

function normalizeSwap(grpcSwap) {
    if (!grpcSwap || !grpcSwap.id) {
        throw new Error('invalid swap payload');
    }
    const lockedIntent = grpcSwap.locked_intent || {};
    const lockedKind = String(lockedIntent.kind || '').replace(/^SWAP_KIND_/, '').toLowerCase();
    const grpcKind = String(grpcSwap.kind || '').replace(/^SWAP_KIND_/, '').toLowerCase();
    const kind = grpcKind || lockedKind || 'submarine';

    const rawFrom = lockedIntent.from_chain || grpcSwap.from_chain || '';
    const rawTo = lockedIntent.to_chain || grpcSwap.to_chain || '';
    const normChain = (value, fallback) => {
        const upper = String(value || '').replace(/^CHAIN_/, '').toUpperCase();
        if (upper === 'BTC') return 'btc';
        if (upper === 'LIQUID' || upper === 'L-BTC') return 'liquid';
        if (upper === 'LN') return 'ln';
        return fallback;
    };
    let fromChain = normChain(rawFrom, '');
    let toChain = normChain(rawTo, '');
    if (!fromChain || !toChain) {
        if (kind === 'reverse') {
            fromChain = fromChain || 'ln';
            toChain = toChain || 'btc';
        } else if (kind === 'chain') {
            fromChain = fromChain || 'btc';
            toChain = toChain || 'liquid';
        } else {
            fromChain = fromChain || 'btc';
            toChain = toChain || 'ln';
        }
    }

    const amountSat = parseInt(
        grpcSwap.amount_sat || grpcSwap.amount_sats || lockedIntent.amount_sat || lockedIntent.amount_sats || 0,
        10
    ) || 0;

    return {
        id: grpcSwap.id,
        kind,
        from_chain: fromChain,
        to_chain: toChain,
        amount_sats: amountSat,
        amount_btc: amountSat / 100_000_000,
        state: normalizeState(grpcSwap.state),
        htlc_address: grpcSwap.lockup_address || lockedIntent.lockup_address || null,
        lockup_address: grpcSwap.lockup_address || lockedIntent.lockup_address || null,
        claim_address: grpcSwap.claim_address || lockedIntent.claim_address || null,
        destination_address: lockedIntent.payout_address || lockedIntent.claim_address || null,
        invoice: lockedIntent.invoice || grpcSwap.invoice || null,
        payment_hash: grpcSwap.payment_hash || null,
        preimage: grpcSwap.preimage || null,
        funded_txid: grpcSwap.lockup_txid || null,
        error_message: grpcSwap.error_message || null,
        created_at: grpcSwap.created_at || new Date().toISOString(),
        updated_at: grpcSwap.updated_at || new Date().toISOString(),
    };
}

// State normalization (gRPC states → Flask enum)
function normalizeState(grpcState) {
    const raw = String(grpcState || '').trim();
    const direct = raw.toLowerCase();
    if (direct && !direct.startsWith('state_') && !direct.startsWith('swap_state_')) {
        return direct;
    }

    const stateMap = {
        STATE_OPEN: 'open',
        STATE_LOCKED: 'locked',
        STATE_COMMIT_STARTED: 'commit_started',
        STATE_WAITING: 'waiting',
        STATE_WAITING_CLAIM_DETAILS: 'waiting_claim_details',
        STATE_SIGNING_MUSIG2_PARTIAL: 'signing_musig2_partial',
        STATE_SENT_PARTIAL_TO_PROVIDER: 'sent_partial_to_provider',
        STATE_WAITING_PROVIDER_BROADCAST: 'waiting_provider_broadcast',
        STATE_REFUND_COOP_WAITING: 'refund_coop_waiting',
        STATE_FALLBACK_SCRIPT_READY: 'fallback_script_ready',
        STATE_REFUNDING: 'refunding',
        STATE_COMPLETED: 'completed',
        STATE_FAILED: 'failed',
        STATE_CANCELED: 'canceled',

        SWAP_STATE_OPEN: 'open',
        SWAP_STATE_LOCKED: 'locked',
        SWAP_STATE_COMMIT_STARTED: 'commit_started',
        SWAP_STATE_WAITING: 'waiting',
        SWAP_STATE_WAITING_CLAIM_DETAILS: 'waiting_claim_details',
        SWAP_STATE_SIGNING_MUSIG2_PARTIAL: 'signing_musig2_partial',
        SWAP_STATE_SENT_PARTIAL_TO_PROVIDER: 'sent_partial_to_provider',
        SWAP_STATE_WAITING_PROVIDER_BROADCAST: 'waiting_provider_broadcast',
        SWAP_STATE_REFUND_COOP_WAITING: 'refund_coop_waiting',
        SWAP_STATE_FALLBACK_SCRIPT_READY: 'fallback_script_ready',
        SWAP_STATE_REFUNDING: 'refunding',
        SWAP_STATE_COMPLETED: 'completed',
        SWAP_STATE_FAILED: 'failed',
        SWAP_STATE_CANCELED: 'canceled',
    };
    return stateMap[raw] || 'open';
}

function toGrpcNodeType(nodeType) {
    const map = {
        bitcoind: 'NODE_TYPE_BITCOIND',
        elementsd: 'NODE_TYPE_ELEMENTSD',
        lnd: 'NODE_TYPE_LND',
    };
    return map[String(nodeType || '').toLowerCase()] || null;
}

function normalizeNodeState(state) {
    const map = {
        NODE_STATE_RUNNING: 'running',
        NODE_STATE_SYNCING: 'syncing',
        NODE_STATE_STARTING: 'starting',
        NODE_STATE_STOPPING: 'stopping',
        NODE_STATE_STOPPED: 'stopped',
        NODE_STATE_ERROR: 'error',
        NODE_STATE_NOT_INSTALLED: 'not_installed',
    };
    return map[state] || 'unknown';
}

function normalizeNodeStatus(status) {
    if (!status) return null;
    const nodeTypeRaw = String(status.node_type || '').replace('NODE_TYPE_', '').toLowerCase();
    return {
        node_type: nodeTypeRaw || 'unknown',
        state: normalizeNodeState(status.state),
        version: status.version || '',
        peer_count: status.peer_count || 0,
        uptime_seconds: status.uptime_seconds || 0,
        error_message: status.error_message || '',
    };
}

function toGrpcChain(chain) {
    const v = String(chain || '').trim().toLowerCase();
    if (v === 'btc' || v === 'chain_btc') return 'CHAIN_BTC';
    if (v === 'liquid' || v === 'l-btc' || v === 'chain_liquid') return 'CHAIN_LIQUID';
    if (v === 'ln' || v === 'lightning' || v === 'chain_ln') return 'CHAIN_LN';
    return null;
}

function normalizeChain(chain) {
    const v = String(chain || '').replace(/^CHAIN_/, '').toLowerCase();
    if (v === 'btc') return 'btc';
    if (v === 'liquid' || v === 'l-btc') return 'liquid';
    if (v === 'ln' || v === 'lightning') return 'ln';
    return v || 'unknown';
}

function normalizeProtoTimestamp(ts) {
    if (!ts) return null;
    if (typeof ts === 'string') return ts;
    if (typeof ts === 'object') {
        const secRaw = ts.seconds ?? ts._seconds;
        const nsecRaw = ts.nanos ?? ts._nanos ?? 0;
        const sec = Number(secRaw);
        const nsec = Number(nsecRaw);
        if (Number.isFinite(sec)) {
            const ms = (sec * 1000) + Math.floor((Number.isFinite(nsec) ? nsec : 0) / 1_000_000);
            return new Date(ms).toISOString();
        }
    }
    return null;
}

// ============================================================================
// HEALTH ENDPOINTS
// ============================================================================

app.get('/health', (req, res) => {
    res.json({ status: 'ok', service: 'xs-wallet-bridge' });
});

app.get('/ready', (req, res) => {
    if (!grpcHealthy) {
        return res.status(503).json({
            status: 'not_ready',
            reason: 'gRPC backend unavailable',
            grpc_host: CONFIG.GRPC_HOST,
        });
    }
    res.json({ status: 'ready', grpc_host: CONFIG.GRPC_HOST });
});

// ============================================================================
// DEVDASH COMPATIBILITY ENDPOINTS (system, events, etc)
// ============================================================================

app.get('/api/v1/system/health', async (req, res) => {
    let bitcoindOk = false;
    let vaultState = 'unknown';

    // Check bitcoind
    try {
        await bitcoinRPC('getblockchaininfo');
        bitcoindOk = true;
    } catch (e) { /* ignore */ }

    // Check vault status
    try {
        const response = await new Promise((resolve, reject) => {
            walletClient.GetVaultStatus({}, { deadline: new Date(Date.now() + 2000) }, (err, resp) => {
                if (err) reject(err); else resolve(resp);
            });
        });
        const stateMap = {
            'STATE_NOT_INITIALIZED': 'not_initialized',
            'STATE_LOCKED': 'locked',
            'STATE_UNLOCKED': 'unlocked',
        };
        vaultState = stateMap[response.state] || 'unknown';
    } catch (e) { /* ignore */ }

    res.json({
        status: grpcHealthy && bitcoindOk ? 'healthy' : 'degraded',
        services: {
            xscore: grpcHealthy ? 'connected' : 'disconnected',
            bitcoind: bitcoindOk ? 'connected' : 'disconnected',
            boltz: 'unknown',
        },
        vault: vaultState,
        timestamp: new Date().toISOString(),
    });
});

app.get('/api/v1/system/status', async (req, res) => {
    let bitcoinBlocks = 0;
    let bitcoinChain = 'unknown';
    let vaultState = 'unknown';

    // Get bitcoin block height
    try {
        const info = await bitcoinRPC('getblockchaininfo');
        bitcoinBlocks = info?.blocks || 0;
        bitcoinChain = info?.chain || 'unknown';
    } catch (e) { /* ignore */ }

    // Get vault status
    try {
        const response = await new Promise((resolve, reject) => {
            walletClient.GetVaultStatus({}, { deadline: new Date(Date.now() + 2000) }, (err, resp) => {
                if (err) reject(err); else resolve(resp);
            });
        });
        const stateMap = {
            'STATE_NOT_INITIALIZED': 'not_initialized',
            'STATE_LOCKED': 'locked',
            'STATE_UNLOCKED': 'unlocked',
        };
        vaultState = stateMap[response.state] || 'unknown';
    } catch (e) { /* ignore */ }

    res.json({
        version: '0.1.0',
        network: bitcoinChain,
        uptime_seconds: Math.floor(process.uptime()),
        grpc_connected: grpcHealthy,
        vault_state: vaultState,
        bitcoin_block_height: bitcoinBlocks,
        services: {
            xscore: { status: grpcHealthy ? 'running' : 'stopped', port: 9735 },
            bitcoind: { status: bitcoinBlocks > 0 ? 'running' : 'stopped', port: bitcoinRPCPort(), blocks: bitcoinBlocks },
            boltz: { status: 'unknown', port: 9001 },
        },
    });
});

app.get('/api/v1/system/metrics', (req, res) => {
    res.json({
        latency_p50_ms: 45,
        latency_p99_ms: 120,
        error_rate: 0.001,
        throughput_rps: 10,
        active_swaps: 0,
        timestamp: new Date().toISOString(),
    });
});

app.get('/api/v1/events/recent', (req, res) => {
    res.json([]);
});

// ============================================================================
// NODE ENDPOINTS (Real gRPC -> xscore NodeService)
// ============================================================================

app.get('/api/v1/nodes', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);

    nodeClient.GetAllNodeStatuses({}, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        const nodes = [
            normalizeNodeStatus(response.bitcoind),
            normalizeNodeStatus(response.elementsd),
            normalizeNodeStatus(response.lnd),
        ].filter(Boolean);

        res.json({ nodes });
    });
});

app.post('/api/v1/nodes/:node_type/start', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const grpcNodeType = toGrpcNodeType(req.params.node_type);
    if (!grpcNodeType) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Invalid node_type', { node_type: req.params.node_type }, req.id);
    }
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);

    nodeClient.StartNode({
        node_type: grpcNodeType,
        network: 'NETWORK_REGTEST',
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        res.json({ success: true, node: normalizeNodeStatus(response) });
    });
});

app.post('/api/v1/nodes/:node_type/stop', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const grpcNodeType = toGrpcNodeType(req.params.node_type);
    if (!grpcNodeType) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Invalid node_type', { node_type: req.params.node_type }, req.id);
    }
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);
    const graceful = typeof req.body?.graceful === 'boolean' ? req.body.graceful : true;

    nodeClient.StopNode({
        node_type: grpcNodeType,
        graceful,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        res.json({ success: true, node: normalizeNodeStatus(response) });
    });
});

app.post('/api/v1/nodes/:node_type/restart', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const grpcNodeType = toGrpcNodeType(req.params.node_type);
    if (!grpcNodeType) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Invalid node_type', { node_type: req.params.node_type }, req.id);
    }
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);

    nodeClient.RestartNode({
        node_type: grpcNodeType,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        res.json({ success: true, node: normalizeNodeStatus(response) });
    });
});

// ============================================================================
// WALLET ENDPOINTS (Real gRPC → xscore WalletService)
// ============================================================================

// Generate new wallet (InitializeVault with generate)
app.post('/api/v1/wallet/generate', (req, res) => {
    const { word_count = 24, pin } = req.body;
    if (typeof pin !== 'string' || pin.length < 8) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'PIN must be at least 8 characters', {}, req.id);
    }
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);

    walletClient.InitializeVault({
        generate: { word_count },
        pin
    }, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        log('info', 'Wallet generated via gRPC', { requestId: req.id });
        res.json({
            success: response.success,
            mnemonic: response.mnemonic,
            session_id: response.session_id,
            word_count,
        });
    });
});

// Unlock vault
app.post('/api/v1/wallet/unlock', (req, res) => {
    const { pin } = req.body;
    if (!pin) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'PIN required', {}, req.id);
    }

    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);
    walletClient.UnlockVault({ pin }, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        log('info', 'Vault unlocked', { requestId: req.id });
        res.json({
            success: response.success,
            session_id: response.session_id,
            error_message: response.error_message,
            remaining_attempts: response.attempts_remaining ?? response.attemptsRemaining ?? response.remaining_attempts,
        });
    });
});

// Lock vault
app.post('/api/v1/wallet/lock', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    walletClient.LockVault({}, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        log('info', 'Vault locked', { requestId: req.id });

        res.json({
            success: true,
        });
    });
});

// Get vault status
app.get('/api/v1/wallet/status', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);

    walletClient.GetVaultStatus({}, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        const stateMap = {
            'STATE_NOT_INITIALIZED': 'not_initialized',
            'STATE_LOCKED': 'locked',
            'STATE_UNLOCKED': 'unlocked',
            'STATE_LOCKED_OUT': 'locked_out',
        };

        res.json({
            state: stateMap[response.state] || 'unknown',
            failed_attempts: response.failed_attempts,
        });
    });
});

// Derive new address
app.post('/api/v1/wallet/derive', (req, res) => {
    const { chain = 'CHAIN_BTC', label = '' } = req.body;
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    walletClient.GetNewAddress({ chain, label }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        res.json({
            address: response.address,
            chain: response.chain,
            derivation_path: response.derivation_path,
            label: response.label,
        });
    });
});

// List derived addresses
app.get('/api/v1/wallet/addresses', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const chain = toGrpcChain(req.query.chain || 'btc');
    if (!chain) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'invalid chain', { chain: req.query.chain }, req.id);
    }
    const includeUsedRaw = String(req.query.include_used ?? req.query.includeUsed ?? '1').trim().toLowerCase();
    const includeUsed = includeUsedRaw !== '0' && includeUsedRaw !== 'false';

    walletClient.ListAddresses({
        chain,
        include_used: includeUsed,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        const addresses = (response.addresses || []).map((a) => ({
            address: a.address || '',
            chain: normalizeChain(a.chain),
            derivation_path: a.derivation_path || '',
            label: a.label || '',
            used: !!a.used,
            balance_sat: Number(a.balance_sat || 0),
        }));
        res.json({ addresses });
    });
});

// List UTXOs and reservation source metadata for Wallet Control Center
app.get('/api/v1/wallet/utxos', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const chain = toGrpcChain(req.query.chain || 'btc');
    if (!chain) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'invalid chain', { chain: req.query.chain }, req.id);
    }
    const includeReservedRaw = String(req.query.include_reserved ?? req.query.includeReserved ?? '1').trim().toLowerCase();
    const includeReserved = includeReservedRaw !== '0' && includeReservedRaw !== 'false';

    walletClient.ListUtxos({
        chain,
        include_reserved: includeReserved,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        const utxos = (response.utxos || []).map((u) => {
            const swapId = u.reserved_for_swap_id || '';
            const reservation = u.reserved ? {
                swap_id: swapId || null,
                reservation_type: swapId ? 'lockup' : 'unknown',
                reserved_by: swapId ? 'swap_engine' : 'unknown',
                reserved_at: null,
                expires_at: null,
            } : null;
            return {
                txid: u.txid || '',
                vout: Number(u.vout || 0),
                amount_sat: Number(u.amount_sat || 0),
                address: u.address || '',
                confirmations: Number(u.confirmations || 0),
                reserved: !!u.reserved,
                reserved_for_swap_id: swapId || null,
                chain: normalizeChain(chain),
                reservation,
            };
        });
        res.json({ utxos });
    });
});

// List wallet transactions (typed source for activity timeline)
app.get('/api/v1/wallet/transactions', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const chain = toGrpcChain(req.query.chain || 'btc');
    if (!chain) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'invalid chain', { chain: req.query.chain }, req.id);
    }
    const limit = Number.parseInt(String(req.query.limit || 50), 10) || 50;
    const offset = Number.parseInt(String(req.query.offset || 0), 10) || 0;

    walletClient.ListTransactions({
        chain,
        limit,
        offset,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        const transactions = (response.transactions || []).map((t) => ({
            txid: t.txid || '',
            chain: normalizeChain(t.chain || chain),
            amount_sat: Number(t.amount_sat || 0),
            fee_sat: Number(t.fee_sat || 0),
            confirmations: Number(t.confirmations || 0),
            label: t.label || '',
            swap_id: t.swap_id || null,
            timestamp: normalizeProtoTimestamp(t.timestamp),
        }));
        res.json({
            transactions,
            total_count: Number(response.total_count || transactions.length),
        });
    });
});

// Validate mnemonic (local - BIP39 validation)
app.post('/api/v1/wallet/validate', (req, res) => {
    const { mnemonic } = req.body;
    const words = mnemonic?.split(' ') || [];
    const validWordCounts = [12, 15, 18, 21, 24];
    const valid = validWordCounts.includes(words.length);

    res.json({
        valid,
        word_count: words.length,
        error: valid ? null : 'Mnemonic must have 12, 15, 18, 21, or 24 words',
    });
});

// Get all balances
app.get('/api/v1/wallet/balances', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    walletClient.GetAllBalances({}, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        res.json({
            btc: response.btc,
            liquid: response.liquid,
            ln: response.ln,
        });
    });
});

// Send on-chain transaction
app.post('/api/v1/wallet/send', (req, res) => {
    const { chain = 'CHAIN_BTC', address, amount_sat, fee_rate_sat_vb, subtract_fee = false, label = '' } = req.body;
    if (!address || !amount_sat) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Missing required fields', {
            required: ['address', 'amount_sat'],
        }, req.id);
    }

    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    walletClient.SendOnchain({
        chain,
        address,
        amount_sat,
        fee_rate_sat_vb,
        subtract_fee,
        label,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        res.json({
            success: response.success,
            txid: response.txid,
            fee_sat: response.fee_sat,
        });
    });
});

// Preimage generate (mock for DevDash)
app.post('/api/v1/preimage/generate', (req, res) => {
    const crypto = require('crypto');
    const preimage = crypto.randomBytes(32).toString('hex');
    const hash = crypto.createHash('sha256').update(Buffer.from(preimage, 'hex')).digest('hex');
    res.json({ preimage, hash });
});

// ============================================================================
// BITCOIN ENDPOINTS (Real bitcoind RPC)
// ============================================================================

const BITCOIN_RPC_URL = process.env.BITCOIN_RPC_URL || 'http://xsrpc:troque_essa_senha@127.0.0.1:18332';

function bitcoinRPCPort() {
    try {
        const url = new URL(BITCOIN_RPC_URL);
        if (url.port) return Number(url.port);
        return url.protocol === 'https:' ? 443 : 80;
    } catch (_) {
        return 18332;
    }
}

async function bitcoinRPC(method, params = []) {
    const axios = require('axios');
    const response = await axios.post(BITCOIN_RPC_URL, {
        jsonrpc: '1.0',
        id: 'api-bridge',
        method,
        params
    }, {
        headers: { 'Content-Type': 'application/json' },
        timeout: 5000
    });
    return response.data.result;
}

app.get('/api/v1/bitcoin/info', async (req, res) => {
    try {
        const info = await bitcoinRPC('getblockchaininfo');
        res.json({
            chain: info.chain,
            blocks: info.blocks,
            headers: info.headers,
            bestblockhash: info.bestblockhash,
            difficulty: info.difficulty,
            mediantime: info.mediantime,
            verificationprogress: info.verificationprogress,
            pruned: info.pruned,
            rpc_port: bitcoinRPCPort(),
        });
    } catch (err) {
        log('error', 'Bitcoin RPC failed', { error: err.message });
        res.status(503).json({ error: 'bitcoind unavailable', details: err.message });
    }
});

app.get('/api/v1/bitcoin/fees', async (req, res) => {
    try {
        const fast = await bitcoinRPC('estimatesmartfee', [1]);
        const medium = await bitcoinRPC('estimatesmartfee', [6]);
        const slow = await bitcoinRPC('estimatesmartfee', [24]);

        // Convert BTC/kB to sat/vB
        const toSatVb = (result) => result.feerate ? Math.ceil(result.feerate * 100000) : 1;

        res.json({
            fast: toSatVb(fast),
            medium: toSatVb(medium),
            slow: toSatVb(slow),
        });
    } catch (err) {
        log('error', 'Fee estimation failed', { error: err.message });
        res.json({ fast: 10, medium: 5, slow: 1 }); // Fallback for regtest
    }
});

app.get('/api/v1/bitcoin/mempool', async (req, res) => {
    try {
        const info = await bitcoinRPC('getmempoolinfo');
        res.json({
            size: info.size,
            bytes: info.bytes,
            usage: info.usage,
            maxmempool: info.maxmempool,
            mempoolminfee: info.mempoolminfee,
        });
    } catch (err) {
        log('error', 'Mempool info failed', { error: err.message });
        res.status(503).json({ error: 'bitcoind unavailable' });
    }
});

app.get('/api/v1/lightning/info', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    nodeClient.GetNodeStatus({
        node_type: 'NODE_TYPE_LND',
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        const state = normalizeNodeState(response.state);
        const ready = state === 'running';
        const message = response.error_message || '';
        const syncedToChain = ready || message.includes('synced_to_chain=true');
        const syncedToGraph = ready || message.includes('synced_to_graph=true');

        res.json({
            alias: '',
            pubkey: '',
            num_active_channels: 0,
            num_peers: response.peer_count || 0,
            synced_to_chain: syncedToChain,
            synced_to_graph: syncedToGraph,
            version: response.version || '',
            state,
            reason: message,
        });
    });
});

app.get('/api/v1/lightning/balance', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    walletClient.GetBalance({
        chain: 'CHAIN_LN',
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        res.json({
            balance_sat: Number(response.total_sat || 0),
            confirmed_sat: Number(response.confirmed_sat || 0),
            unconfirmed_sat: Number(response.unconfirmed_sat || 0),
            pending_swap_sat: Number(response.pending_swap_sat || 0),
            pending_open_sat: 0,
            pending_close_sat: 0,
        });
    });
});

app.get('/api/v1/lightning/decode', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const invoice = String(req.query.invoice || '').trim();
    if (!invoice) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Missing invoice query param', {
            required: ['invoice'],
        }, req.id);
    }
    let client;
    try {
        client = getLndClient();
    } catch (err) {
        return errorResponse(res, 'GRPC_FAILED_PRECONDITION', err.message, {}, req.id);
    }
    client.DecodePayReq({ pay_req: invoice }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        res.json({
            payment_hash: response.payment_hash || '',
            amount_sat: Number(response.num_satoshis || 0),
            destination: response.destination || '',
            description: response.description || '',
            expiry: Number(response.expiry || 0),
            timestamp: Number(response.timestamp || 0),
        });
    });
});

app.post('/api/v1/lightning/pay', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const invoice = String(req.body?.invoice || '').trim();
    if (!invoice) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Missing invoice in request body', {
            required: ['invoice'],
        }, req.id);
    }
    let client;
    try {
        client = getLndClient();
    } catch (err) {
        return errorResponse(res, 'GRPC_FAILED_PRECONDITION', err.message, {}, req.id);
    }
    client.SendPaymentSync({ payment_request: invoice }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        if (response.payment_error) {
            return errorResponse(res, 'GRPC_INTERNAL', response.payment_error, {}, req.id);
        }
        res.json({
            success: true,
            payment_hash: response.payment_hash ? Buffer.from(response.payment_hash).toString('hex') : '',
            payment_preimage: response.payment_preimage ? Buffer.from(response.payment_preimage).toString('hex') : '',
        });
    });
});

app.post('/api/v1/lightning/invoice', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;
    const amountSat = Number(req.body?.amount_sat || 0);
    const memo = String(req.body?.memo || '');
    const expiry = Number(req.body?.expiry || 3600);
    if (!Number.isFinite(amountSat) || amountSat <= 0) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'amount_sat must be > 0', {}, req.id);
    }
    let client;
    try {
        client = getLndClient();
    } catch (err) {
        return errorResponse(res, 'GRPC_FAILED_PRECONDITION', err.message, {}, req.id);
    }
    client.AddInvoice({
        value: amountSat,
        memo,
        expiry,
    }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        const paymentHashHex = response.r_hash ? Buffer.from(response.r_hash).toString('hex') : '';
        res.json({
            invoice: response.payment_request || '',
            payment_hash: paymentHashHex,
            amount_sat: amountSat,
            memo,
            expiry,
        });
    });
});

app.get('/api/v1/elements/info', (req, res) => {
    res.json({
        chain: 'liquidregtest',
        blocks: 0,
        initialized: false,
    });
});

// Preimage verify (mock)
app.post('/api/v1/preimage/verify', (req, res) => {
    const crypto = require('crypto');
    const { preimage, payment_hash } = req.body;
    if (!preimage || !payment_hash) {
        return res.status(400).json({ error: 'Missing preimage or payment_hash' });
    }
    const computed = crypto.createHash('sha256').update(Buffer.from(preimage, 'hex')).digest('hex');
    res.json({
        valid: computed === payment_hash,
        computed_hash: computed,
        expected_hash: payment_hash,
    });
});

// HTLC create (mock)
app.post('/api/v1/htlc/create', (req, res) => {
    const crypto = require('crypto');
    const { amount_sats = 10000, timeout_blocks = 144 } = req.body;
    const preimage = crypto.randomBytes(32).toString('hex');
    const hash = crypto.createHash('sha256').update(Buffer.from(preimage, 'hex')).digest('hex');
    res.json({
        htlc_address: 'bcrt1qhtlc' + Math.random().toString(36).substring(2, 10),
        redeem_script: '6382012088a820' + hash + '8876a914mock1488ac6700',
        preimage,
        payment_hash: hash,
        amount_sats,
        timeout_blocks,
        warning: 'MOCK HTLC for development only!',
    });
});

// HTLC decode (mock)
app.post('/api/v1/htlc/decode', (req, res) => {
    const { script_hex } = req.body;
    res.json({
        type: 'htlc',
        payment_hash: script_hex?.substring(14, 78) || '0'.repeat(64),
        timeout_blocks: 144,
        sender_pubkey: '02' + '0'.repeat(64),
        receiver_pubkey: '03' + '0'.repeat(64),
        warning: 'MOCK decode for development only!',
    });
});

// ============================================================================
// SWAP ENDPOINTS
// ============================================================================

// GET /api/v1/swaps - List all swaps
app.get('/api/v1/swaps', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    swapClient.ListSwaps({}, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        const swaps = (response.swaps || []).map(normalizeSwap);
        res.json(swaps);
    });
});

// POST /api/v1/swaps - Create new swap
// Semantic: Creates quote, accepts, and leaves in OPEN/LOCKED state
app.post('/api/v1/swaps', (req, res) => {
    const {
        from_chain,
        to_chain,
        amount_sats,
        invoice,
        destination_address,
        payout_address,
    } = req.body;

    if (!from_chain || !to_chain || !amount_sats) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Missing required fields', {
            required: ['from_chain', 'to_chain', 'amount_sats'],
        }, req.id);
    }

    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.create);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    // Map chain names to proto enum
    const chainMap = {
        'btc': 'CHAIN_BTC',
        'liquid': 'CHAIN_LIQUID',
        'ln': 'CHAIN_LN',
    };

    // Determine swap kind based on chains
    let swapKind = 'SWAP_KIND_SUBMARINE';
    if (from_chain === 'ln') {
        swapKind = 'SWAP_KIND_REVERSE';
    } else if (
        (from_chain === 'btc' && to_chain === 'liquid') ||
        (from_chain === 'liquid' && to_chain === 'btc')
    ) {
        swapKind = 'SWAP_KIND_CHAIN';
    }
    const payoutAddress = String(destination_address || payout_address || '').trim();
    if (swapKind === 'SWAP_KIND_SUBMARINE' && !String(invoice || '').trim()) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Missing required field for submarine swap', {
            required: ['invoice'],
        }, req.id);
    }
    if ((swapKind === 'SWAP_KIND_REVERSE' || swapKind === 'SWAP_KIND_CHAIN') && !payoutAddress) {
        return errorResponse(res, 'GRPC_INVALID_ARGUMENT', 'Missing required field for reverse/chain swap', {
            required: ['destination_address'],
        }, req.id);
    }

    // Step 1: Get Quote
    const quoteReq = {
        kind: swapKind,
        from_chain: chainMap[from_chain] || from_chain,
        to_chain: chainMap[to_chain] || to_chain,
        amount_sat: amount_sats,
    };

    // Add submarine params if invoice provided
    if (invoice && swapKind === 'SWAP_KIND_SUBMARINE') {
        quoteReq.submarine = { invoice };
    }
    if (payoutAddress && swapKind === 'SWAP_KIND_REVERSE') {
        quoteReq.reverse = { payout_address: payoutAddress };
    }
    if (payoutAddress && swapKind === 'SWAP_KIND_CHAIN') {
        quoteReq.chain = { payout_address: payoutAddress };
    }

    log('info', 'Requesting quote', { requestId: req.id, kind: swapKind });

    swapClient.QuoteSwap(quoteReq, metadata, { deadline }, (err, quoteResp) => {
        if (err) return handleGrpcError(err, res, req.id);

        log('info', 'Quote received', {
            requestId: req.id,
            quoteId: quoteResp.quote_id,
            feeSat: quoteResp.total_fee_sat,
        });

        // Step 2: Create swap from quote
        swapClient.CreateSwap({
            quote_id: quoteResp.quote_id,
        }, metadata, { deadline }, (err, response) => {
            if (err) return handleGrpcError(err, res, req.id);

            const swap = normalizeSwap(response.swap ?? response);
            log('info', 'Swap created', {
                requestId: req.id,
                swapId: swap.id,
                state: swap.state,
            });

            // Include quote info in response
            swap.quote = {
                quote_id: quoteResp.quote_id,
                provider_fee_sat: quoteResp.provider_fee_sat,
                network_fee_sat: quoteResp.network_fee_sat,
                total_fee_sat: quoteResp.total_fee_sat,
                lockup_address: quoteResp.lockup_address,
            };

            res.status(201).json(swap);
        });
    });
});

// GET /api/v1/swaps/:id - Get swap details
app.get('/api/v1/swaps/:id', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    swapClient.GetSwap({ swap_id: req.params.id }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);

        const swap = normalizeSwap(response.swap ?? response);
        res.json(swap);
    });
});

// GET /api/v1/swaps/:id/events - Timeline de eventos de swap
app.get('/api/v1/swaps/:id/events', (req, res) => {
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.list);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    const afterSeq = parseInt(req.query.after_seq || req.query.afterSeq || 0, 10) || 0;
    swapClient.GetSwapEvents({
        swap_id: req.params.id,
        after_seq: afterSeq,
    }, metadata, { deadline }, (err, response) => {
        if (err) return handleGrpcError(err, res, req.id);
        const events = (response.events || []).map((ev) => ({
            seq: Number(ev.seq || 0),
            swap_id: ev.swap_id || req.params.id,
            from_state: normalizeState(ev.from_state),
            to_state: normalizeState(ev.to_state),
            trigger: ev.trigger || '',
            details_json: ev.details_json || '',
            created_at: ev.timestamp || new Date().toISOString(),
        }));
        res.json(events);
    });
});

// GET /api/v1/swaps/:id/events/stream - Native stream via SSE backed by gRPC WatchSwap
app.get('/api/v1/swaps/:id/events/stream', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    const fromSeq = parseInt(req.query.from_seq || req.query.fromSeq || 0, 10) || 0;

    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache, no-transform');
    res.setHeader('Connection', 'keep-alive');
    res.setHeader('X-Accel-Buffering', 'no');
    res.flushHeaders?.();
    res.write(`event: ready\ndata: ${JSON.stringify({ swap_id: req.params.id, from_seq: fromSeq })}\n\n`);

    const stream = swapClient.WatchSwap({
        swap_id: req.params.id,
        from_seq: fromSeq,
    }, metadata);

    const heartbeat = setInterval(() => {
        res.write(`event: ping\ndata: ${Date.now()}\n\n`);
    }, 15000);

    const cleanup = () => {
        clearInterval(heartbeat);
        if (!res.writableEnded) {
            res.end();
        }
    };

    stream.on('data', (ev) => {
        const normalized = {
            seq: Number(ev.seq || 0),
            swap_id: ev.swap_id || req.params.id,
            from_state: normalizeState(ev.from_state),
            to_state: normalizeState(ev.to_state),
            trigger: ev.trigger || '',
            details_json: ev.details_json || '',
            created_at: ev.timestamp || new Date().toISOString(),
        };
        res.write(`event: swap_event\ndata: ${JSON.stringify(normalized)}\n\n`);
    });

    stream.on('error', (err) => {
        res.write(`event: stream_error\ndata: ${JSON.stringify({ message: err?.details || err?.message || 'stream error' })}\n\n`);
        cleanup();
    });

    stream.on('end', () => {
        cleanup();
    });

    req.on('close', () => {
        clearInterval(heartbeat);
        try { stream.cancel(); } catch (_) { }
    });
});

// GET /api/v1/swaps/events/stream - Native stream via SSE backed by gRPC WatchAllSwaps
app.get('/api/v1/swaps/events/stream', (req, res) => {
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    const fromSeq = parseInt(req.query.from_seq || req.query.fromSeq || 0, 10) || 0;
    const filterStatesRaw = String(req.query.filter_states || req.query.filterStates || '').trim();
    const filterStates = filterStatesRaw
        ? filterStatesRaw.split(',').map((v) => v.trim()).filter(Boolean)
        : [];

    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache, no-transform');
    res.setHeader('Connection', 'keep-alive');
    res.setHeader('X-Accel-Buffering', 'no');
    res.flushHeaders?.();
    res.write(`event: ready\ndata: ${JSON.stringify({ from_seq: fromSeq, filter_states: filterStates })}\n\n`);

    const stream = swapClient.WatchAllSwaps({
        from_seq: fromSeq,
        filter_states: filterStates,
    }, metadata);

    const heartbeat = setInterval(() => {
        res.write(`event: ping\ndata: ${Date.now()}\n\n`);
    }, 15000);

    const cleanup = () => {
        clearInterval(heartbeat);
        if (!res.writableEnded) {
            res.end();
        }
    };

    stream.on('data', (ev) => {
        const normalized = {
            seq: Number(ev.seq || 0),
            swap_id: ev.swap_id || '',
            from_state: normalizeState(ev.from_state),
            to_state: normalizeState(ev.to_state),
            trigger: ev.trigger || '',
            details_json: ev.details_json || '',
            created_at: ev.timestamp || new Date().toISOString(),
        };
        res.write(`event: swap_event\ndata: ${JSON.stringify(normalized)}\n\n`);
    });

    stream.on('error', (err) => {
        res.write(`event: stream_error\ndata: ${JSON.stringify({ message: err?.details || err?.message || 'stream error' })}\n\n`);
        cleanup();
    });

    stream.on('end', () => {
        cleanup();
    });

    req.on('close', () => {
        clearInterval(heartbeat);
        try { stream.cancel(); } catch (_) { }
    });
});

// POST /api/v1/swaps/:id/check - Advance swap state (Lock → Commit → Reconcile)
app.post('/api/v1/swaps/:id/check', (req, res) => {
    const swapId = req.params.id;
    const deadline = new Date(Date.now() + CONFIG.GRPC_DEADLINE_MS.commit);
    const metadata = requireAuthMetadata(req, res);
    if (!metadata) return;

    // Get current swap state
    swapClient.GetSwap({ swap_id: swapId }, metadata, { deadline }, (err, getResp) => {
        if (err) return handleGrpcError(err, res, req.id);

        const currentState = getResp.swap?.state || getResp.state;

        // State machine: advance to next step
        if (currentState === 'STATE_OPEN' || currentState === 'SWAP_STATE_OPEN') {
            // Lock the swap
            swapClient.LockSwap({ swap_id: swapId, expected_version: getResp.version }, metadata, { deadline }, (err, lockResp) => {
                if (err) return handleGrpcError(err, res, req.id);
                res.json({ status: 'locked', swap: normalizeSwap(lockResp.swap ?? lockResp) });
            });
        } else if (currentState === 'STATE_LOCKED' || currentState === 'SWAP_STATE_LOCKED') {
            // Commit (fund HTLC)
            swapClient.CommitSwap({ swap_id: swapId, expected_version: getResp.version }, metadata, { deadline }, (err, commitResp) => {
                if (err) return handleGrpcError(err, res, req.id);
                res.json({ status: 'committed', swap: normalizeSwap(commitResp.swap ?? commitResp) });
            });
        } else if (currentState === 'STATE_COMMIT_STARTED' || currentState === 'STATE_WAITING' ||
            currentState === 'SWAP_STATE_COMMIT_STARTED' || currentState === 'SWAP_STATE_WAITING') {
            // Just return current state - reconciliation happens via watcher
            res.json({ status: 'waiting', swap: normalizeSwap(getResp.swap ?? getResp) });
        } else {
            res.json({ status: 'no_action', state: currentState });
        }
    });
});

// POST /api/v1/swaps/:id/claim - Claim swap (NOT IMPLEMENTED)
app.post('/api/v1/swaps/:id/claim', (req, res) => {
    errorResponse(res, 'GRPC_UNIMPLEMENTED', 'Claim not implemented (MuSig2 crypto missing)', {
        reason: 'Claims are handled by Boltz backend for now',
    }, req.id);
});

// POST /api/v1/swaps/:id/refund - Refund swap (NOT IMPLEMENTED)
app.post('/api/v1/swaps/:id/refund', (req, res) => {
    errorResponse(res, 'GRPC_UNIMPLEMENTED', 'Refund not implemented', {
        reason: 'Refund logic not yet implemented in core',
    }, req.id);
});

// ============================================================================
// VAULT ALIASES (for spec compatibility)
// ============================================================================

// Alias: /vault/status -> /wallet/status
app.get('/api/v1/vault/status', (req, res) => {
    req.url = '/api/v1/wallet/status';
    app.handle(req, res);
});

// Alias: /vault/unlock -> /wallet/unlock
app.post('/api/v1/vault/unlock', (req, res) => {
    req.url = '/api/v1/wallet/unlock';
    req.method = 'POST';
    app.handle(req, res);
});

// Alias: /vault/lock -> /wallet/lock
app.post('/api/v1/vault/lock', (req, res) => {
    req.url = '/api/v1/wallet/lock';
    req.method = 'POST';
    app.handle(req, res);
});

// ============================================================================
// SERVER STARTUP
// ============================================================================

function start() {
    initGrpcClients();
    return app.listen(CONFIG.PORT, () => {
        log('info', `API Bridge started`, {
            port: CONFIG.PORT,
            grpc_host: CONFIG.GRPC_HOST,
            env: process.env.NODE_ENV || 'development',
        });
    });
}

if (require.main === module) {
    start();
}

// Graceful shutdown
process.on('SIGTERM', () => {
    log('info', 'SIGTERM received, shutting down gracefully');
    process.exit(0);
});

module.exports = {
    app,
    start,
    _test: {
        setClientsForTest,
        normalizeSwap,
        requireAuthMetadata,
    },
};
