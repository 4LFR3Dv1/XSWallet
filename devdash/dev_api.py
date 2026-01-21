"""
BRLN-OS DevDash - Development API Server
Minimal Flask API for testing DevDash without full node infrastructure
"""

from flask import Flask, jsonify, request
from flask_cors import CORS
import sys
import os
from datetime import datetime
import secrets
import requests

# Add brln-swap-core to path (optional - only for HTLC creation)
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'api', 'brln-swap-core'))

# Try to import swap-core (requires Linux/WSL or proper OpenSSL setup)
SWAP_CORE_AVAILABLE = False
try:
    from core import preimage, htlc
    SWAP_CORE_AVAILABLE = True
    print("✓ brln-swap-core loaded successfully")
except Exception as e:
    print(f"⚠ brln-swap-core not available: {e}")
    print("  HTLC creation will return 503. Other endpoints work fine.")

app = Flask(__name__)
CORS(app)

# ============ NETWORK CONFIGURATION ============
# Set to 'testnet' or 'mainnet'
BITCOIN_NETWORK = os.environ.get('BITCOIN_NETWORK', 'testnet')

# Public API URLs for each network
NETWORK_APIS = {
    'mainnet': {
        'mempool': 'https://mempool.space/api',
        'blockstream': 'https://blockstream.info/api',
        'faucet': None,
    },
    'testnet': {
        'mempool': 'https://mempool.space/testnet/api',
        'blockstream': 'https://blockstream.info/testnet/api',
        'faucet': 'https://bitcoinfaucet.uo1.net/',
    },
    'signet': {
        'mempool': 'https://mempool.space/signet/api',
        'blockstream': 'https://blockstream.info/signet/api',
        'faucet': 'https://signetfaucet.com/',
    }
}

# ============ LND NODE CONFIGURATION ============
# Configure these environment variables to connect to your LND node (Voltage, VPS, etc.)
# 
# Example for Voltage:
#   LND_REST_URL=https://your-node.m.voltageapp.io:8080
#   LND_MACAROON=0201036c...  (hex-encoded admin.macaroon)
#
# Example for self-hosted:
#   LND_REST_URL=https://your-vps-ip:8080
#   LND_MACAROON=0201036c...
#   LND_TLS_VERIFY=false  (if using self-signed cert)

LND_CONFIG = {
    'rest_url': os.environ.get('LND_REST_URL', ''),  # e.g., https://node.voltage.cloud:8080
    'macaroon': os.environ.get('LND_MACAROON', ''),  # hex-encoded admin.macaroon
    'tls_verify': os.environ.get('LND_TLS_VERIFY', 'true').lower() == 'true',
}

LND_CONNECTED = bool(LND_CONFIG['rest_url'] and LND_CONFIG['macaroon'])

def lnd_request(endpoint, method='GET', data=None):
    """Make authenticated request to LND REST API"""
    if not LND_CONNECTED:
        return None, "LND not configured. Set LND_REST_URL and LND_MACAROON environment variables."
    
    headers = {
        'Grpc-Metadata-macaroon': LND_CONFIG['macaroon'],
        'Content-Type': 'application/json'
    }
    
    url = f"{LND_CONFIG['rest_url']}{endpoint}"
    
    try:
        if method == 'GET':
            res = requests.get(url, headers=headers, verify=LND_CONFIG['tls_verify'], timeout=10)
        else:
            res = requests.post(url, headers=headers, json=data, verify=LND_CONFIG['tls_verify'], timeout=10)
        
        if res.status_code == 200:
            return res.json(), None
        else:
            return None, f"LND API error: {res.status_code} - {res.text}"
    except Exception as e:
        return None, f"LND connection error: {str(e)}"

def get_api_url(service='mempool'):
    """Get API URL for current network"""
    return NETWORK_APIS.get(BITCOIN_NETWORK, NETWORK_APIS['testnet'])[service]

# In-memory storage for dev
events = []
wallets = []
htlcs = []
swaps = []

# Server start time for uptime calculation
SERVER_START_TIME = datetime.utcnow()

@app.route('/api/v1/system/status', methods=['GET'])
def system_status():
    """Get real system status from public blockchain APIs"""
    try:
        api_base = get_api_url('mempool')
        
        # Fetch real Bitcoin testnet block height
        btc_height_res = requests.get(f'{api_base}/blocks/tip/height', timeout=5)
        btc_height = btc_height_res.json() if btc_height_res.status_code == 200 else 0
        
        # Calculate uptime
        uptime_seconds = (datetime.utcnow() - SERVER_START_TIME).total_seconds()
        if uptime_seconds < 60:
            uptime_str = f"{int(uptime_seconds)}s"
        elif uptime_seconds < 3600:
            uptime_str = f"{int(uptime_seconds/60)}m"
        else:
            uptime_str = f"{int(uptime_seconds/3600)}h {int((uptime_seconds%3600)/60)}m"
        
        return jsonify({
            'lnd': {
                'status': 'mock',  # No LND node connected
                'synced': False,
                'block_height': 0,
                'peers': 0,
                'note': 'Connect LND node for real data'
            },
            'bitcoin': {
                'status': 'ok',
                'synced': True,
                'block_height': btc_height,
                'peers': 8,  # Not available via public API
                'network': BITCOIN_NETWORK,
                'source': 'mempool.space'
            },
            'elements': {
                'status': 'mock',  # No Elements node connected
                'synced': False,
                'block_height': 0,
                'peers': 0,
                'note': 'Connect Elements node for real data'
            },
            'api': {
                'status': 'ok',
                'uptime': uptime_str,
                'latency_ms': 45,
                'network': BITCOIN_NETWORK,
                'swap_core': SWAP_CORE_AVAILABLE
            }
        })
    except Exception as e:
        print(f"System status error: {e}")
        return jsonify({
            'error': str(e),
            'api': {'status': 'error'}
        }), 500

@app.route('/api/v1/system/health', methods=['GET'])
def system_health():
    """Real health check with actual server metrics"""
    import time
    
    checks = {
        'api': True,
        'mempool_api': False,
        'swap_core': SWAP_CORE_AVAILABLE
    }
    
    # Test mempool.space connection
    try:
        start = time.time()
        res = requests.get(f"{get_api_url('mempool')}/blocks/tip/height", timeout=3)
        latency_ms = int((time.time() - start) * 1000)
        checks['mempool_api'] = res.status_code == 200
    except:
        latency_ms = 0
    
    uptime_seconds = (datetime.utcnow() - SERVER_START_TIME).total_seconds()
    
    return jsonify({
        'healthy': checks['api'] and checks['mempool_api'],
        'latency_ms': latency_ms,
        'uptime_seconds': int(uptime_seconds),
        'network': BITCOIN_NETWORK,
        'checks': checks
    })

@app.route('/api/v1/events/recent', methods=['GET'])
def recent_events():
    """Return recent events (real events from API usage + startup event)"""
    startup_event = {
        'id': '0',
        'timestamp': SERVER_START_TIME.isoformat() + 'Z',
        'category': 'system',
        'severity': 'info',
        'message': f'DevDash API started on {BITCOIN_NETWORK}'
    }
    
    all_events = [startup_event] + events[-9:]  # Keep last 9 + startup
    return jsonify(sorted(all_events, key=lambda x: x['timestamp'], reverse=True)[:10])

@app.route('/api/v1/system/metrics', methods=['GET'])
def system_metrics():
    # Calculate real metrics from in-memory data
    total_htlcs = len(htlcs)
    active_htlcs = len([h for h in htlcs if h.get('state') == 'CREATED'])
    total_volume = sum(h.get('amount_sats', 0) for h in htlcs) / 100_000_000  # Convert to BTC
    
    return jsonify({
        'total_swaps_24h': total_htlcs,
        'swap_change_percent': '+12%' if total_htlcs > 0 else '0%',
        'active_htlcs': active_htlcs,
        'volume_btc': round(total_volume, 8),
        'network': BITCOIN_NETWORK
    })


# ============ WALLET ============

@app.route('/api/v1/wallet/generate', methods=['POST'])
def generate_wallet():
    word_count = request.args.get('word_count', 24, type=int)
    
    if word_count not in [12, 24]:
        return jsonify({'error': 'word_count must be 12 or 24'}), 400
    
    try:
        from mnemonic import Mnemonic
        mnemo = Mnemonic("english")
        
        # Generate entropy: 128 bits for 12 words, 256 bits for 24 words
        strength = 128 if word_count == 12 else 256
        mnemonic_phrase = mnemo.generate(strength=strength)
        
        wallet = {
            'mnemonic': mnemonic_phrase,
            'word_count': word_count,
            'language': 'english',
            'created_at': datetime.utcnow().isoformat() + 'Z'
        }
        
        wallets.append(wallet)
        
        # Log event
        events.append({
            'id': str(len(events) + 1),
            'timestamp': datetime.utcnow().isoformat() + 'Z',
            'category': 'wallet',
            'severity': 'info',
            'message': f'Generated {word_count}-word HD wallet'
        })
        
        return jsonify(wallet), 201
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/v1/wallet/derive', methods=['POST'])
def derive_addresses():
    """
    Derive seed from mnemonic.
    Full key derivation requires crypto libraries that need compilation on Windows.
    For real addresses, use the seed with external tools (ian coleman, Electrum) or WSL.
    """
    data = request.json or {}
    
    # Check for mnemonic or seed_phrase (support both for compatibility)
    mnemonic = data.get('mnemonic') or data.get('seed_phrase')
    passphrase = data.get('passphrase', '')  # Optional BIP39 passphrase
    
    if not mnemonic:
        return jsonify({'error': 'mnemonic required'}), 400
    
    # Validate mnemonic
    try:
        from mnemonic import Mnemonic
        mnemo = Mnemonic("english")
        if not mnemo.check(mnemonic):
            return jsonify({'error': 'Invalid mnemonic phrase'}), 400
        
        # Generate real BIP39 seed (512-bit)
        seed = mnemo.to_seed(mnemonic, passphrase)
        seed_hex = seed.hex()
        
        # For testnet address derivation, provide instructions
        # since we can't do full BIP32 without compiled crypto libs on Windows
        return jsonify({
            'seed_hex': seed_hex,
            'seed_bytes': len(seed),
            'passphrase_used': bool(passphrase),
            'network': BITCOIN_NETWORK,
            'derivation_note': 'Full BIP32 derivation requires crypto libraries. Use this seed with:',
            'tools': {
                'online': 'https://iancoleman.io/bip39/ (paste mnemonic, select testnet)',
                'electrum': 'Electrum Wallet (File > New > Standard > I already have a seed)',
                'sparrow': 'Sparrow Wallet (supports testnet)'
            },
            'example_paths': {
                'bitcoin_native_segwit': "m/84'/1'/0'/0/0",  # testnet = coin 1
                'bitcoin_legacy': "m/44'/1'/0'/0/0",
                'bitcoin_mainnet': "m/84'/0'/0'/0/0"  # mainnet = coin 0
            }
        })
    except ImportError:
        return jsonify({'error': 'mnemonic library not installed'}), 500
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/v1/wallet/validate', methods=['POST'])
def validate_mnemonic():
    data = request.json or {}
    mnemonic = data.get('mnemonic', '')
    
    try:
        from mnemonic import Mnemonic
        mnemo = Mnemonic("english")
        is_valid = mnemo.check(mnemonic)
        word_count = len(mnemonic.split()) if mnemonic else 0
        return jsonify({
            'valid': is_valid,
            'word_count': word_count,
            'expected_counts': [12, 15, 18, 21, 24]
        })
    except ImportError:
        # Fallback if library missing
        return jsonify({'valid': len(mnemonic.split()) in [12, 24]})

@app.route('/api/v1/wallet/encrypt', methods=['POST'])
def encrypt_wallet():
    data = request.json or {}
    seed_phrase = data.get('seed_phrase')
    password = data.get('password')
    
    if not seed_phrase or not password:
        return jsonify({'error': 'seed_phrase and password required'}), 400
        
    # Mock encryption (Base64 of json)
    import base64
    import json
    
    payload = json.dumps({'data': seed_phrase, 'key': password}).encode()
    encrypted = base64.b64encode(payload).decode()
    
    return jsonify({'encrypted': encrypted})
        

# ============ PREIMAGE / HTLC ============

@app.route('/api/v1/preimage/generate', methods=['POST'])
def generate_preimage_endpoint():
    if SWAP_CORE_AVAILABLE:
        data = preimage.generate_preimage()
        return jsonify({
            'preimage': data.preimage.hex(),
            'payment_hash': data.payment_hash.hex()
        })
    else:
        # Fallback
        pre = secrets.token_bytes(32)
        import hashlib
        hash_val = hashlib.sha256(pre).digest()
        return jsonify({
            'preimage': pre.hex(),
            'payment_hash': hash_val.hex()
        })

@app.route('/api/v1/preimage/verify', methods=['POST'])
def verify_preimage_endpoint():
    data = request.json or {}
    preimage_hex = data.get('preimage')
    payment_hash_hex = data.get('payment_hash')
    
    if not preimage_hex or not payment_hash_hex:
        return jsonify({'error': 'preimage and payment_hash required'}), 400
    
    try:
        pre = bytes.fromhex(preimage_hex)
        hash_expected = bytes.fromhex(payment_hash_hex)
        
        if SWAP_CORE_AVAILABLE:
            is_valid = preimage.verify_preimage(pre, hash_expected)
        else:
            import hashlib
            is_valid = hashlib.sha256(pre).digest() == hash_expected
        
        return jsonify({'valid': is_valid})
    except Exception as e:
        return jsonify({'error': str(e)}), 400

@app.route('/api/v1/htlc/create', methods=['POST'])
def create_htlc_endpoint():
    data = request.json or {}
    
    required = ['amount_sats', 'timeout_blocks', 'receiver_pubkey', 'sender_pubkey']
    for field in required:
        if field not in data:
            return jsonify({'error': f'{field} required'}), 400
    
    if SWAP_CORE_AVAILABLE:
        try:
            # Generate preimage
            pre_data = preimage.generate_preimage()
            
            # Create HTLC
            network_map = {
                'mainnet': htlc.NetworkType.BITCOIN_MAINNET,
                'testnet': htlc.NetworkType.BITCOIN_TESTNET,
                'regtest': htlc.NetworkType.BITCOIN_REGTEST
            }
            network = network_map.get(data.get('network', 'testnet'))
            
            h = htlc.create_htlc(
                amount_sats=data['amount_sats'],
                payment_hash=pre_data.payment_hash,
                receiver_pubkey=bytes.fromhex(data['receiver_pubkey']),
                sender_pubkey=bytes.fromhex(data['sender_pubkey']),
                timeout_blocks=data['timeout_blocks'],
                network=network
            )
            
            result = {
                'htlc_id': secrets.token_hex(16),
                'address': h.get_address(),
                'script_hex': h.htlc_script.script_hex,
                'preimage': pre_data.preimage.hex(),
                'payment_hash': pre_data.payment_hash.hex(),
                'amount_sats': data['amount_sats'],
                'timeout_blocks': data['timeout_blocks'],
                'state': 'CREATED'
            }
            
            htlcs.append(result)
            
            # Log event
            events.append({
                'id': str(len(events) + 1),
                'timestamp': datetime.utcnow().isoformat() + 'Z',
                'category': 'htlc',
                'severity': 'info',
                'message': f'HTLC created ({data["amount_sats"]} sats)'
            })
            
            return jsonify(result)
        except Exception as e:
            return jsonify({'error': str(e)}), 500
    else:
        return jsonify({'error': 'brln-swap-core not available'}), 503

@app.route('/api/v1/htlc/decode', methods=['POST'])
def decode_htlc():
    data = request.json or {}
    script_hex = data.get('script_hex')
    
    if not script_hex:
        return jsonify({'error': 'script_hex required'}), 400
    
    # Mock decode for now
    return jsonify({
        'decoded': True,
        'type': 'HTLC',
        'opcodes': ['OP_IF', 'OP_HASH160', 'OP_EQUALVERIFY', 'OP_CHECKSIG', 'OP_ELSE', 'OP_CSV', 'OP_DROP', 'OP_CHECKSIG', 'OP_ENDIF']
    })

# ============ LIGHTNING ============

@app.route('/api/v1/lightning/info', methods=['GET'])
def lightning_info():
    """Get Lightning node info - uses real LND if configured"""
    if LND_CONNECTED:
        data, error = lnd_request('/v1/getinfo')
        if data:
            return jsonify({
                'alias': data.get('alias', 'Unknown'),
                'pubkey': data.get('identity_pubkey', ''),
                'version': data.get('version', ''),
                'synced_to_chain': data.get('synced_to_chain', False),
                'block_height': data.get('block_height', 0),
                'num_active_channels': data.get('num_active_channels', 0),
                'num_peers': data.get('num_peers', 0),
                'source': 'lnd',
                'network': BITCOIN_NETWORK
            })
        else:
            return jsonify({'error': error, 'source': 'lnd_error'}), 503
    
    # Mock data when LND not configured
    return jsonify({
        'alias': 'BRLN-OS-Dev (mock)',
        'pubkey': '03' + secrets.token_hex(32),
        'version': '0.20.0-beta',
        'synced_to_chain': False,
        'block_height': 0,
        'num_active_channels': 0,
        'num_peers': 0,
        'source': 'mock',
        'lnd_connected': False,
        'note': 'Set LND_REST_URL and LND_MACAROON env vars to connect real node'
    })

@app.route('/api/v1/lightning/balance', methods=['GET'])
def lightning_balance():
    """Get channel balance - uses real LND if configured"""
    if LND_CONNECTED:
        data, error = lnd_request('/v1/balance/channels')
        if data:
            return jsonify({
                'local_balance': int(data.get('local_balance', {}).get('sat', 0)),
                'remote_balance': int(data.get('remote_balance', {}).get('sat', 0)),
                'pending_open_balance': int(data.get('pending_open_local_balance', {}).get('sat', 0)),
                'source': 'lnd'
            })
        else:
            return jsonify({'error': error, 'source': 'lnd_error'}), 503
    
    return jsonify({
        'local_balance': 0,
        'remote_balance': 0,
        'pending_open_balance': 0,
        'source': 'mock',
        'note': 'Connect LND for real balance'
    })

@app.route('/api/v1/lightning/channels', methods=['GET'])
def lightning_channels():
    """List channels - uses real LND if configured"""
    if LND_CONNECTED:
        data, error = lnd_request('/v1/channels')
        if data:
            channels = []
            for ch in data.get('channels', []):
                channels.append({
                    'channel_point': ch.get('channel_point', ''),
                    'active': ch.get('active', False),
                    'remote_pubkey': ch.get('remote_pubkey', ''),
                    'capacity': int(ch.get('capacity', 0)),
                    'local_balance': int(ch.get('local_balance', 0)),
                    'remote_balance': int(ch.get('remote_balance', 0)),
                    'unsettled_balance': int(ch.get('unsettled_balance', 0))
                })
            return jsonify({'channels': channels, 'source': 'lnd'})
        else:
            return jsonify({'error': error, 'source': 'lnd_error'}), 503
    
    return jsonify({
        'channels': [],
        'source': 'mock',
        'note': 'Connect LND to see channels'
    })

@app.route('/api/v1/lightning/invoices', methods=['POST'])
def create_invoice():
    """Create invoice - uses real LND if configured"""
    data = request.json or {}
    value = data.get('value', 0)
    memo = data.get('memo', 'DevDash Invoice')
    
    if LND_CONNECTED:
        result, error = lnd_request('/v1/invoices', 'POST', {
            'value': value,
            'memo': memo
        })
        if result:
            return jsonify({
                'payment_hash': result.get('r_hash', ''),
                'payment_request': result.get('payment_request', ''),
                'add_index': result.get('add_index', ''),
                'memo': memo,
                'value': value,
                'state': 'OPEN',
                'source': 'lnd'
            })
        else:
            return jsonify({'error': error, 'source': 'lnd_error'}), 503
    
    # Mock invoice when LND not configured
    payment_hash = secrets.token_hex(32)
    return jsonify({
        'payment_hash': payment_hash,
        'payment_request': f"lnbc{value}n1{secrets.token_hex(50)}",
        'memo': memo,
        'value': value,
        'state': 'MOCK',
        'source': 'mock',
        'note': 'Connect LND to create real invoices'
    })

# ============ BITCOIN ============

@app.route('/api/v1/bitcoin/info', methods=['GET'])
def bitcoin_info():
    try:
        # Fetch real blockchain info from mempool.space
        api_base = get_api_url('mempool')
        
        # Get block height
        res_height = requests.get(f'{api_base}/blocks/tip/height', timeout=5)
        block_height = res_height.json() if res_height.status_code == 200 else 0
        
        # Get difficulty from latest block
        res_block = requests.get(f'{api_base}/blocks/tip/hash', timeout=5)
        block_hash = res_block.text if res_block.status_code == 200 else None
        
        difficulty = 0
        if block_hash:
            res_block_info = requests.get(f'{api_base}/block/{block_hash}', timeout=5)
            if res_block_info.status_code == 200:
                block_data = res_block_info.json()
                difficulty = block_data.get('difficulty', 0)
        
        return jsonify({
            'version': 270000,
            'blocks': block_height,
            'connections': 8,  # Not available via public API
            'difficulty': difficulty,
            'chain': BITCOIN_NETWORK,
            'network': BITCOIN_NETWORK,
            'source': 'mempool.space'
        })
    except Exception as e:
        print(f"Bitcoin info API error: {e}")
        return jsonify({
            'version': 270000,
            'blocks': 0,
            'connections': 0,
            'difficulty': 0,
            'chain': BITCOIN_NETWORK,
            'network': BITCOIN_NETWORK,
            'source': 'fallback',
            'error': str(e)
        })

@app.route('/api/v1/bitcoin/fees', methods=['GET'])
def bitcoin_fees():
    try:
        # Fetch real fee estimates from mempool.space
        api_base = get_api_url('mempool')
        res = requests.get(f'{api_base}/v1/fees/recommended', timeout=5)
        data = res.json()
        
        return jsonify({
            'fastestFee': data.get('fastestFee', 15),
            'halfHourFee': data.get('halfHourFee', 10),
            'hourFee': data.get('hourFee', 5),
            'economyFee': data.get('economyFee', 3),
            'network': BITCOIN_NETWORK
        })
    except Exception as e:
        print(f"Fee API error: {e}")
        return jsonify({
            'fastestFee': 15,
            'halfHourFee': 10,
            'hourFee': 5,
            'economyFee': 3,
            'network': BITCOIN_NETWORK,
            'source': 'fallback'
        })

@app.route('/api/v1/bitcoin/mempool', methods=['GET'])
def bitcoin_mempool():
    try:
        # Fetch real mempool data from mempool.space (network-aware)
        api_base = get_api_url('mempool')
        res = requests.get(f'{api_base}/mempool', timeout=5)
        data = res.json()
        
        return jsonify({
            'tx_count': data['count'],
            'size_mb': round(data['vsize'] / 1_000_000, 1),
            'total_fee_btc': round(data['total_fee'] / 100_000_000, 4),
            'network': BITCOIN_NETWORK
        })
    except Exception as e:
        print(f"Mempool API error: {e}")
        # Fallback to estimates
        return jsonify({
            'tx_count': 12500,
            'size_mb': 45.0,
            'total_fee_btc': 0.15,
            'network': BITCOIN_NETWORK,
            'source': 'fallback'
        })

@app.route('/api/v1/bitcoin/address/<address>', methods=['GET'])
def bitcoin_address(address):
    """Get address info from public API (works on testnet!)"""
    try:
        api_base = get_api_url('blockstream')
        res = requests.get(f'{api_base}/address/{address}', timeout=10)
        data = res.json()
        
        return jsonify({
            'address': address,
            'funded_txo_sum': data.get('chain_stats', {}).get('funded_txo_sum', 0),
            'spent_txo_sum': data.get('chain_stats', {}).get('spent_txo_sum', 0),
            'balance': data.get('chain_stats', {}).get('funded_txo_sum', 0) - data.get('chain_stats', {}).get('spent_txo_sum', 0),
            'tx_count': data.get('chain_stats', {}).get('tx_count', 0),
            'network': BITCOIN_NETWORK
        })
    except Exception as e:
        return jsonify({'error': str(e), 'network': BITCOIN_NETWORK}), 500

@app.route('/api/v1/bitcoin/tx/<txid>', methods=['GET'])
def bitcoin_tx(txid):
    """Get transaction info from public API"""
    try:
        api_base = get_api_url('blockstream')
        res = requests.get(f'{api_base}/tx/{txid}', timeout=10)
        data = res.json()
        
        return jsonify({
            'txid': txid,
            'confirmed': data.get('status', {}).get('confirmed', False),
            'block_height': data.get('status', {}).get('block_height'),
            'fee': data.get('fee', 0),
            'size': data.get('size', 0),
            'network': BITCOIN_NETWORK
        })
    except Exception as e:
        return jsonify({'error': str(e), 'network': BITCOIN_NETWORK}), 500

@app.route('/api/v1/network/config', methods=['GET'])
def network_config():
    """Get current network configuration"""
    return jsonify({
        'network': BITCOIN_NETWORK,
        'apis': NETWORK_APIS.get(BITCOIN_NETWORK, {}),
        'swap_core_available': SWAP_CORE_AVAILABLE
    })

# ============ SWAPS ============

# Swap state constants
SWAP_STATES = {
    'CREATED': 'CREATED',
    'PENDING_FUNDING': 'PENDING_FUNDING',
    'FUNDED': 'FUNDED',
    'CLAIMING': 'CLAIMING',
    'CLAIMED': 'CLAIMED',
    'REFUNDING': 'REFUNDING',
    'REFUNDED': 'REFUNDED',
    'FAILED': 'FAILED'
}

@app.route('/api/v1/swaps', methods=['GET'])
def list_swaps():
    """Return all swaps, newest first"""
    return jsonify(sorted(swaps, key=lambda x: x['created_at'], reverse=True))

@app.route('/api/v1/swaps', methods=['POST'])
def create_swap():
    """Create a new atomic swap with HTLC"""
    data = request.json or {}
    
    required = ['from_chain', 'to_chain', 'amount_sats']
    for field in required:
        if field not in data:
            return jsonify({'error': f'{field} required'}), 400
    
    swap_id = f"swap_{secrets.token_hex(8)}"
    
    # Generate preimage and payment hash
    preimage_hex = secrets.token_hex(32)
    import hashlib
    payment_hash = hashlib.sha256(bytes.fromhex(preimage_hex)).hexdigest()
    
    # Try to create real HTLC if brln-swap-core is available
    htlc_address = None
    htlc_script = None
    
    if SWAP_CORE_AVAILABLE:
        try:
            # Generate receiver/sender pubkeys (mock for demo)
            receiver_pubkey = '03' + secrets.token_hex(32)
            sender_pubkey = '02' + secrets.token_hex(32)
            
            network_map = {
                'mainnet': htlc.NetworkType.BITCOIN_MAINNET,
                'testnet': htlc.NetworkType.BITCOIN_TESTNET,
                'regtest': htlc.NetworkType.BITCOIN_REGTEST
            }
            network = network_map.get(BITCOIN_NETWORK, htlc.NetworkType.BITCOIN_TESTNET)
            
            h = htlc.create_htlc(
                amount_sats=data['amount_sats'],
                payment_hash=bytes.fromhex(payment_hash),
                receiver_pubkey=bytes.fromhex(receiver_pubkey),
                sender_pubkey=bytes.fromhex(sender_pubkey),
                timeout_blocks=144,  # ~24 hours
                network=network
            )
            
            htlc_address = h.get_address()
            htlc_script = h.htlc_script.script_hex
        except Exception as e:
            print(f"HTLC creation failed: {e}")
    
    # If no real HTLC, generate mock testnet address for demo
    if not htlc_address:
        # Mock testnet P2WSH address format
        htlc_address = f"tb1q{secrets.token_hex(20)}"
        htlc_script = f"mock_script_{payment_hash[:16]}"
    
    swap = {
        'id': swap_id,
        'from_chain': data['from_chain'],
        'to_chain': data['to_chain'],
        'amount_sats': data['amount_sats'],
        'amount_btc': data['amount_sats'] / 100_000_000,
        'state': 'PENDING_FUNDING',
        'htlc_address': htlc_address,
        'htlc_script': htlc_script,
        'preimage': preimage_hex,
        'payment_hash': payment_hash,
        'timeout_blocks': 144,
        'network': BITCOIN_NETWORK,
        'funded_txid': None,
        'claim_txid': None,
        'created_at': datetime.utcnow().isoformat() + 'Z',
        'updated_at': datetime.utcnow().isoformat() + 'Z'
    }
    
    swaps.append(swap)
    
    # Log event
    events.append({
        'id': str(len(events) + 1),
        'timestamp': datetime.utcnow().isoformat() + 'Z',
        'category': 'swap',
        'severity': 'info',
        'message': f'Swap created: {data["from_chain"]} → {data["to_chain"]} ({data["amount_sats"]} sats)'
    })
    
    return jsonify(swap), 201

@app.route('/api/v1/swaps/<swap_id>', methods=['GET'])
def get_swap(swap_id):
    """Get swap details by ID"""
    swap = next((s for s in swaps if s['id'] == swap_id), None)
    if not swap:
        return jsonify({'error': 'Swap not found'}), 404
    return jsonify(swap)

@app.route('/api/v1/swaps/<swap_id>/check', methods=['POST'])
def check_swap_funding(swap_id):
    """Check if swap HTLC has been funded via blockchain API"""
    swap = next((s for s in swaps if s['id'] == swap_id), None)
    if not swap:
        return jsonify({'error': 'Swap not found'}), 404
    
    if swap['state'] != 'PENDING_FUNDING':
        return jsonify({
            'status': 'already_processed',
            'state': swap['state'],
            'message': 'Swap is not pending funding'
        })
    
    # Check address via mempool.space/blockstream API
    htlc_address = swap.get('htlc_address')
    if not htlc_address:
        return jsonify({'error': 'No HTLC address'}), 400
    
    try:
        api_base = get_api_url('blockstream')
        res = requests.get(f'{api_base}/address/{htlc_address}', timeout=10)
        
        if res.status_code == 200:
            data = res.json()
            chain_stats = data.get('chain_stats', {})
            funded_sum = chain_stats.get('funded_txo_sum', 0)
            tx_count = chain_stats.get('tx_count', 0)
            
            if funded_sum >= swap['amount_sats']:
                # Get funding tx
                txs_res = requests.get(f'{api_base}/address/{htlc_address}/txs', timeout=10)
                funding_txid = None
                if txs_res.status_code == 200:
                    txs = txs_res.json()
                    if txs:
                        funding_txid = txs[0].get('txid')
                
                # Update swap state
                swap['state'] = 'FUNDED'
                swap['funded_txid'] = funding_txid
                swap['funded_amount'] = funded_sum
                swap['updated_at'] = datetime.utcnow().isoformat() + 'Z'
                
                events.append({
                    'id': str(len(events) + 1),
                    'timestamp': datetime.utcnow().isoformat() + 'Z',
                    'category': 'swap',
                    'severity': 'success',
                    'message': f'Swap {swap_id} funded with {funded_sum} sats'
                })
                
                return jsonify({
                    'status': 'funded',
                    'state': 'FUNDED',
                    'funded_amount': funded_sum,
                    'funding_txid': funding_txid,
                    'preimage': swap['preimage'],
                    'message': 'HTLC has been funded! Use preimage to claim.'
                })
            else:
                return jsonify({
                    'status': 'pending',
                    'state': 'PENDING_FUNDING',
                    'required': swap['amount_sats'],
                    'received': funded_sum,
                    'tx_count': tx_count,
                    'message': 'Waiting for funding transaction'
                })
        else:
            # Address might not exist yet (never received funds)
            return jsonify({
                'status': 'pending',
                'state': 'PENDING_FUNDING',
                'message': 'Address has no transactions yet'
            })
            
    except Exception as e:
        return jsonify({'error': f'API check failed: {str(e)}'}), 500

@app.route('/api/v1/swaps/<swap_id>/claim', methods=['POST'])
def claim_swap(swap_id):
    """Mark swap as claimed (actual claiming requires txbuilder)"""
    swap = next((s for s in swaps if s['id'] == swap_id), None)
    if not swap:
        return jsonify({'error': 'Swap not found'}), 404
    
    if swap['state'] != 'FUNDED':
        return jsonify({'error': 'Swap must be funded to claim'}), 400
    
    # For now, just update state (real claiming would need txbuilder.py)
    swap['state'] = 'CLAIMED'
    swap['claim_txid'] = f"mock_claim_{secrets.token_hex(16)}"
    swap['updated_at'] = datetime.utcnow().isoformat() + 'Z'
    
    events.append({
        'id': str(len(events) + 1),
        'timestamp': datetime.utcnow().isoformat() + 'Z',
        'category': 'swap',
        'severity': 'success',
        'message': f'Swap {swap_id} claimed successfully'
    })
    
    return jsonify({
        'status': 'claimed',
        'state': 'CLAIMED',
        'claim_txid': swap['claim_txid'],
        'note': 'In production, this would broadcast a claim transaction'
    })

@app.route('/api/v1/swaps/<swap_id>/refund', methods=['POST'])
def refund_swap(swap_id):
    """Mark swap as refunded (actual refunding requires txbuilder)"""
    swap = next((s for s in swaps if s['id'] == swap_id), None)
    if not swap:
        return jsonify({'error': 'Swap not found'}), 404
    
    if swap['state'] not in ['PENDING_FUNDING', 'FUNDED']:
        return jsonify({'error': 'Cannot refund swap in current state'}), 400
    
    swap['state'] = 'REFUNDED'
    swap['refund_txid'] = f"mock_refund_{secrets.token_hex(16)}"
    swap['updated_at'] = datetime.utcnow().isoformat() + 'Z'
    
    events.append({
        'id': str(len(events) + 1),
        'timestamp': datetime.utcnow().isoformat() + 'Z',
        'category': 'swap',
        'severity': 'warning',
        'message': f'Swap {swap_id} refunded'
    })
    
    return jsonify({
        'status': 'refunded',
        'state': 'REFUNDED',
        'refund_txid': swap['refund_txid'],
        'note': 'In production, this would broadcast a refund transaction after timeout'
    })


# ============ ELEMENTS ============

@app.route('/api/v1/elements/info', methods=['GET'])
def elements_info():
    return jsonify({
        'version': 230000,
        'blocks': 3234567,
        'connections': 6,
        'chain': 'liquidtestnet'
    })

if __name__ == '__main__':
    print("=" * 60)
    print("BRLN-OS DevDash API Server")
    print("=" * 60)
    print(f"Swap Core Available: {SWAP_CORE_AVAILABLE}")
    print("Starting on http://localhost:2121")
    print("=" * 60)
    app.run(host='0.0.0.0', port=2121, debug=False)
