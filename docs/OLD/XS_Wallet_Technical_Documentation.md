---
title: "XS Wallet - Technical Documentation"
subtitle: "HD Wallet + Atomic Swaps Desktop Application"
author: "XS Wallet Development Team"
date: "January 2026"
version: "0.1.0"
documentclass: article
geometry: margin=1in
fontsize: 11pt
toc: true
toc-depth: 3
numbersections: true
colorlinks: true
---

\newpage

# Executive Summary

**XS Wallet** is a self-custody desktop application that enables atomic swaps between Bitcoin, Liquid, and Lightning Network using Taproot and the Boltz API as a liquidity provider. The system maintains true self-custody while providing seamless cross-chain and cross-layer swaps without requiring users to operate their own swap infrastructure.

## Key Features

- **HD Wallet**: BIP39/32/84/85 compliant with deterministic key derivation
- **Multi-Chain Support**: Bitcoin (on-chain), Liquid (sidechain), Lightning Network
- **Atomic Swaps**: Submarine, Reverse, and Chain swaps via Boltz API v2
- **Taproot Integration**: MuSig2 cooperative signing with script-path fallback
- **Self-Custody**: User controls all private keys, encrypted at rest
- **Desktop Application**: Electron-based with embedded nodes (bitcoind, elementsd, LND)

## Project Status

- **Phase**: Foundation Complete (15%)
- **Timeline**: 5 weeks to MVP
- **Architecture**: Production-ready database layer, node manager, and technical decisions finalized

\newpage

# System Architecture

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    XS Wallet Desktop App                     │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐         ┌──────────────────────────────┐  │
│  │   Electron   │         │    Backend Process (Node.js)  │  │
│  │ Main Process │◄───────►│                               │  │
│  │              │   IPC   │  ┌────────────────────────┐   │  │
│  │  • Window    │         │  │   Swap Engine          │   │  │
│  │  • Updater   │         │  │   • State Machine      │   │  │
│  │  • Node Mgr  │         │  │   • CAS/Transactions   │   │  │
│  └──────────────┘         │  │   • Watchdog/Recovery  │   │  │
│                            │  └────────────────────────┘   │  │
│  ┌──────────────┐         │                               │  │
│  │   React UI   │         │  ┌────────────────────────┐   │  │
│  │  (Renderer)  │◄───────►│  │   Database (SQLite)    │   │  │
│  │              │   IPC   │  │   • WAL Mode           │   │  │
│  │  • Dashboard │         │  │   • CAS/Version        │   │  │
│  │  • Swap UI   │         │  │   • Event Log          │   │  │
│  │  • Settings  │         │  └────────────────────────┘   │  │
│  └──────────────┘         │                               │  │
│                            │  ┌────────────────────────┐   │  │
│                            │  │   Adapters             │   │  │
│                            │  │   • BTC (bitcoind)     │   │  │
│                            │  │   • Liquid (elementsd) │   │  │
│                            │  │   • LN (LND)           │   │  │
│                            │  └────────────────────────┘   │  │
│                            └──────────────────────────────┘  │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │            Embedded Nodes (Verified Download)         │   │
│  │  • bitcoind v26.0  • elementsd v23.2.1  • LND v0.17.3│   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │   Boltz API v2   │
                    │  (Swap Provider) │
                    └──────────────────┘
```

## Component Breakdown

### 1. Electron Main Process
- **Window Management**: Creates and manages application windows
- **Auto-Updater**: Handles application updates via electron-updater
- **Node Manager**: Lifecycle management of embedded Bitcoin/Liquid/LND nodes
- **IPC Bridge**: Secure communication between renderer and backend

### 2. Backend Process (Separate Node.js Process)
- **Swap Engine**: Core state machine for atomic swaps
- **Database Layer**: SQLite with WAL mode for concurrent access
- **Chain Adapters**: RPC clients for bitcoind, elementsd, and LND
- **Boltz Client**: REST + WebSocket integration with Boltz API v2
- **Key Vault**: BIP39/32/84/85 implementation with encryption

### 3. React Frontend (Renderer Process)
- **Onboarding**: Wallet creation and restoration flows
- **Dashboard**: Balance display and transaction history
- **Swap Interface**: Quote, confirmation, and status tracking
- **Settings**: Network selection, node status, backup management

### 4. Embedded Nodes
- **bitcoind**: Full Bitcoin node for on-chain operations
- **elementsd**: Liquid/Elements node for sidechain operations
- **LND**: Lightning Network daemon for payment channels
- **Strategy**: Verified download on first run (reduces installer size)

\newpage

# Database Architecture

## Schema Design (SQLite)

### Core Tables

#### 1. `swaps` - Authoritative Swap State
```sql
CREATE TABLE swaps (
  id                  TEXT PRIMARY KEY,
  kind                TEXT CHECK(kind IN ('submarine','reverse','chain')),
  state               TEXT CHECK(state IN ('open','locked',...)),
  version             INTEGER NOT NULL DEFAULT 0,  -- CAS/Optimistic Locking
  locked_intent       TEXT,  -- JSON: quote/fees snapshot
  swap_key_index      INTEGER NOT NULL,  -- Deterministic restore
  -- ... proofs, timeouts, MuSig2 context
);
```

**Key Features**:
- **Optimistic Locking**: `version` field for Compare-And-Swap (CAS)
- **Locked Intent**: Immutable snapshot of quote/fees/config
- **Deterministic Restore**: `swap_key_index` for mnemonic-based recovery

#### 2. `swap_events` - Audit Trail
```sql
CREATE TABLE swap_events (
  swap_id     TEXT REFERENCES swaps(id),
  seq         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_ms       INTEGER NOT NULL,
  source      TEXT NOT NULL,  -- 'boltz_ws' | 'node_poll' | 'engine'
  type        TEXT NOT NULL,
  payload     TEXT NOT NULL   -- JSON
);
```

**Purpose**: Complete replay/debug capability for swap lifecycle

#### 3. `swap_ops` - Idempotency Ledger
```sql
CREATE TABLE swap_ops (
  swap_id     TEXT REFERENCES swaps(id),
  op_key      TEXT NOT NULL,
  status      TEXT CHECK(status IN ('inflight','ok','fail')),
  heartbeat_at TEXT,  -- Stale operation detection
  PRIMARY KEY (swap_id, op_key)
);
```

**Purpose**: Prevents duplicate operations (e.g., double broadcast)

#### 4. `utxo_reservations` - Anti Double-Spend
```sql
CREATE TABLE utxo_reservations (
  chain       TEXT CHECK(chain IN ('btc','liquid')),
  txid        TEXT NOT NULL,
  vout        INTEGER NOT NULL CHECK(vout >= 0),
  swap_id     TEXT REFERENCES swaps(id),
  PRIMARY KEY (chain, txid, vout)
);
```

**Purpose**: Prevents same UTXO from being used in multiple swaps

#### 5. `ln_reservations` - Anti Payment Duplication
```sql
CREATE TABLE ln_reservations (
  payment_hash_hex TEXT PRIMARY KEY CHECK(length(payment_hash_hex)=64),
  swap_id          TEXT REFERENCES swaps(id),
  direction        TEXT CHECK(direction IN ('pay','receive'))
);
```

**Purpose**: Prevents duplicate Lightning payments

#### 6. `app_config` - Configuration Store
```sql
CREATE TABLE app_config (
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL,  -- JSON
  updated_at  TEXT NOT NULL
);
```

**Purpose**: User-configurable settings (network, provider, timeouts)

## Critical SQLite Pragmas

```sql
PRAGMA journal_mode = WAL;        -- Write-Ahead Logging (concurrency)
PRAGMA synchronous = NORMAL;      -- Balance safety/performance
PRAGMA busy_timeout = 5000;       -- Wait 5s on lock
PRAGMA foreign_keys = ON;         -- Enforce referential integrity
PRAGMA temp_store = MEMORY;       -- Temp tables in RAM
PRAGMA cache_size = -20000;       -- ~20MB cache
```

**Rationale**: These settings optimize SQLite for desktop use with concurrent reads/writes while maintaining data integrity.

\newpage

# Atomic Swap Flows

## Submarine Swap (Liquid → Lightning)

**Use Case**: User wants to convert L-BTC to Lightning capacity

```
┌──────┐                 ┌───────┐                ┌─────────┐
│ User │                 │ Boltz │                │ Liquid  │
└──┬───┘                 └───┬───┘                └────┬────┘
   │                         │                         │
   │ 1. Create LN invoice    │                         │
   ├────────────────────────►│                         │
   │                         │                         │
   │ 2. HTLC address + tree  │                         │
   │◄────────────────────────┤                         │
   │                         │                         │
   │ 3. Verify P2TR address  │                         │
   │    (rebuild swapTree)   │                         │
   │                         │                         │
   │ 4. Fund HTLC (L-BTC)    │                         │
   ├─────────────────────────┼────────────────────────►│
   │                         │                         │
   │                         │ 5. Pay LN invoice       │
   │                         │    (receives preimage R)│
   │                         │                         │
   │                         │ 6. Claim HTLC (reveal R)│
   │                         ├────────────────────────►│
   │                         │                         │
   │ 7. Success notification │                         │
   │◄────────────────────────┤                         │
```

**Security**: User verifies HTLC script and P2TR address before funding

## Reverse Swap (Lightning → Liquid)

**Use Case**: User wants to convert Lightning balance to L-BTC

```
┌──────┐                 ┌───────┐                ┌─────────┐
│ User │                 │ Boltz │                │ Liquid  │
└──┬───┘                 └───┬───┘                └────┬────┘
   │                         │                         │
   │ 1. Generate R, H=SHA256(R)                        │
   │                         │                         │
   │ 2. Request reverse swap │                         │
   ├────────────────────────►│                         │
   │    (send H)             │                         │
   │                         │                         │
   │ 3. Hold invoice + HTLC  │                         │
   │◄────────────────────────┤                         │
   │                         │                         │
   │ 4. Verify invoice hash  │                         │
   │    (hash == H)          │                         │
   │                         │                         │
   │ 5. Pay hold invoice     │                         │
   ├────────────────────────►│                         │
   │                         │                         │
   │                         │ 6. Fund HTLC (L-BTC)    │
   │                         ├────────────────────────►│
   │                         │                         │
   │ 7. Claim HTLC (reveal R)│                         │
   ├─────────────────────────┼────────────────────────►│
   │                         │                         │
   │                         │ 8. Settle invoice (R)   │
   │                         │◄────────────────────────┤
```

**Security**: User generates preimage R, ensuring control over claim

## Chain Swap (BTC ↔ Liquid)

**Use Case**: Atomic swap between Bitcoin mainchain and Liquid sidechain

```
┌──────┐                 ┌───────┐      ┌─────┐  ┌────────┐
│ User │                 │ Boltz │      │ BTC │  │ Liquid │
└──┬───┘                 └───┬───┘      └──┬──┘  └───┬────┘
   │                         │              │         │
   │ 1. Generate R, H=SHA256(R)             │         │
   │                         │              │         │
   │ 2. Request chain swap   │              │         │
   ├────────────────────────►│              │         │
   │    (BTC→Liquid, send H) │              │         │
   │                         │              │         │
   │ 3. HTLC addresses (both)│              │         │
   │◄────────────────────────┤              │         │
   │                         │              │         │
   │ 4. Verify both HTLCs    │              │         │
   │    (scripts + timeouts) │              │         │
   │                         │              │         │
   │ 5. Fund BTC HTLC        │              │         │
   ├─────────────────────────┼─────────────►│         │
   │                         │              │         │
   │                         │ 6. Fund Liquid HTLC    │
   │                         ├──────────────┼────────►│
   │                         │              │         │
   │ 7. Claim Liquid (reveal R)             │         │
   ├─────────────────────────┼──────────────┼────────►│
   │                         │              │         │
   │                         │ 8. Claim BTC (use R)   │
   │                         ├─────────────►│         │
```

**Security**: Timeouts are staggered (T_liquid < T_btc) to prevent race conditions

\newpage

# Technical Decisions

## 1. SQLite with WAL Mode

**Decision**: Use SQLite instead of PostgreSQL for embedded desktop app

**Rationale**:
- No external database server required
- Single-file database (easy backup/restore)
- WAL mode provides excellent concurrency for desktop use
- Proven reliability in production applications (browsers, mobile apps)

**Implementation**:
- `journal_mode=WAL` for concurrent reads during writes
- `busy_timeout=5000ms` to handle lock contention
- Optimistic locking via `version` field for CAS operations

## 2. Backend Process Separation

**Decision**: Run backend as separate Node.js child process

**Rationale**:
- **Crash Isolation**: UI crash doesn't kill swap engine or nodes
- **Resource Management**: Better control over CPU/memory allocation
- **Clean IPC**: Well-defined communication boundary
- **Debugging**: Easier to debug backend independently

**Implementation**:
- Main process spawns backend on startup
- IPC via Electron's native channels or localhost loopback
- Backend continues running even if UI restarts

## 3. Verified Binary Downloads

**Decision**: Download node binaries on first run instead of bundling in installer

**Rationale**:
- **Installer Size**: ~50MB vs ~500MB (10x reduction)
- **Notarization Speed**: Faster macOS notarization
- **Independent Updates**: App and nodes can update separately
- **Security**: SHA256 checksum verification + optional GPG signatures

**Implementation**:
```typescript
const NODE_BINARIES = {
  bitcoind: {
    version: '26.0',
    downloadUrl: 'https://bitcoincore.org/...',
    checksum: 'sha256...'
  }
};
```

## 4. Boltz API Integration

**Decision**: Use Boltz as swap provider instead of operating own infrastructure

**Rationale**:
- **Faster MVP**: No need to manage liquidity or hold invoices
- **Atomic Security**: HTLC scripts ensure atomicity regardless of provider
- **Verification**: "Don't trust, verify" - rebuild scripts locally
- **Fallback**: Script-path allows claim/refund even if Boltz offline

**Trade-offs**:
- **Dependency**: Relies on Boltz availability (mitigated by fallback paths)
- **Privacy**: Provider sees swap metadata (acceptable for MVP)
- **Fees**: Provider fees (competitive with self-hosted costs)

## 5. Taproot + MuSig2

**Decision**: Use Taproot swaps with MuSig2 cooperative signing

**Rationale**:
- **Efficiency**: Key-path spends are smaller and cheaper
- **Privacy**: Cooperative spends look like regular payments
- **Fallback**: Script-path available when cooperation fails
- **Future-Proof**: Taproot is Bitcoin's latest upgrade

**Implementation**:
- Happy path: MuSig2 aggregated signature (Boltz pubkey first)
- Fallback: Script-path with HTLC conditions
- Deterministic nonce generation for safety

## 6. PIN-Based Encryption

**Decision**: Argon2id key derivation from PIN + AES-256-GCM encryption

**Rationale**:
- **User-Friendly**: PIN is easier than password for desktop app
- **Secure**: Argon2id is memory-hard (resistant to GPU attacks)
- **Rate-Limited**: Backoff prevents brute force
- **Optional Enhancement**: OS keychain for device secret (v1.1)

**Parameters**:
```typescript
{
  algorithm: 'argon2id',
  memory: 65536,      // 64MB
  iterations: 3,
  parallelism: 1,
  saltLength: 16
}
```

## 7. Deterministic Swap Keys (BIP85)

**Decision**: Use BIP85 child seed for swap key derivation

**Rationale**:
- **Restore**: Can recover pending swaps from mnemonic alone
- **Separation**: Swap keys isolated from wallet keys
- **Deterministic Preimages**: `R = SHA256(privKey(index))`

**Implementation**:
```
Master Seed (BIP39)
  └─ BIP85 Child Seed (swap subtree)
      └─ m/0/0, m/0/1, ... (swap keys by index)
          └─ SHA256(privKey) = preimage R
```

## 8. Configuration Management

**Decision**: Store config in SQLite `app_config` table, snapshot in swaps

**Rationale**:
- **No Hardcoding**: All settings user-configurable
- **Reproducibility**: Each swap carries config snapshot in `locked_intent`
- **Auditability**: Can trace exact parameters used for any swap

**Default Config**:
```json
{
  "network": "regtest",
  "provider_url": "https://api.boltz.exchange",
  "kdf_params": {"algorithm":"argon2id",...}
}
```

\newpage

# Implementation Roadmap

## Phase 1: Backend Core (2 weeks)

### Week 1
- [x] Database schema (SQLite)
- [x] DB layer with CAS/transactions
- [x] Node manager with verified downloads
- [ ] Key Vault (BIP39/32/84/85)
- [ ] Vault encryption (Argon2id + AES-GCM)
- [ ] BTC Adapter (bitcoind RPC)

### Week 2
- [ ] Liquid Adapter (elementsd RPC)
- [ ] LN Adapter (LND gRPC)
- [ ] Boltz Client (REST + WebSocket)
- [ ] MuSig2 implementation
- [ ] Swap Engine (state machine)
- [ ] Watchdog + recovery

## Phase 2: Frontend (1.5 weeks)

### Week 3
- [ ] React + Vite setup
- [ ] Onboarding flow (create/restore wallet)
- [ ] Dashboard (balances + history)
- [ ] Swap interface (quote + confirmation)

### Week 4 (first half)
- [ ] Status tracking (WebSocket updates)
- [ ] Settings (network, nodes, backup)
- [ ] Error handling + UX polish

## Phase 3: Electron Integration (1 week)

### Week 4 (second half)
- [ ] Main process setup
- [ ] Backend child process
- [ ] IPC handlers (secure)
- [ ] Node lifecycle integration

### Week 5 (first half)
- [ ] Auto-updater
- [ ] Deep links (xs-wallet://)
- [ ] Logging + diagnostics

## Phase 4: Packaging & Testing (0.5 week)

### Week 5 (second half)
- [ ] electron-builder config
- [ ] Installers (Windows, macOS, Linux)
- [ ] Code signing
- [ ] Integration tests (Boltz regtest)
- [ ] E2E tests (Electron)

## Post-MVP Roadmap

### v1.1 (Q2 2026)
- Hardware wallet support (Ledger/Trezor)
- SQLCipher database encryption
- Multi-wallet support
- Mobile app (React Native)

### v1.2 (Q3 2026)
- Coinjoin integration
- Payjoin support
- LNURL

### v2.0 (Q4 2026)
- RGB assets
- Taproot Assets
- DLC support

\newpage

# Security Considerations

## Threat Model

### In-Scope Threats
1. **Seed Theft**: Attacker gains access to encrypted database
2. **PIN Brute Force**: Attacker attempts to guess PIN
3. **Swap Manipulation**: Attacker tries to steal funds during swap
4. **Supply Chain**: Malicious node binaries or app updates

### Out-of-Scope (Future Work)
1. **Physical Access**: Attacker with physical device access
2. **OS Compromise**: Malware with root/admin privileges
3. **Network Attacks**: Man-in-the-middle (mitigated by TLS)

## Mitigations

### 1. Seed Protection
- **Encryption**: Argon2id + AES-256-GCM
- **Memory**: Zeroize sensitive data after use
- **Storage**: Encrypted at rest, never in plaintext

### 2. PIN Security
- **Rate Limiting**: Exponential backoff (1s → 2s → 4s → 8s...)
- **Max Attempts**: Lock after 10 failed attempts (1 hour cooldown)
- **KDF**: Argon2id (64MB memory, 3 iterations)

### 3. Swap Atomicity
- **HTLC Scripts**: Cryptographic guarantees via hash locks
- **Verification**: Rebuild scripts locally, never trust provider
- **Timeouts**: Staggered to prevent race conditions
- **Fallback**: Script-path claim/refund if cooperation fails

### 4. Supply Chain
- **App Updates**: Code signing (Windows/macOS)
- **Node Binaries**: SHA256 checksum verification
- **Manifest**: Signed manifest for binary registry
- **Rollback**: Automatic rollback on update failure

## Audit Recommendations

### Pre-Launch
1. **Code Review**: Third-party security audit
2. **Penetration Testing**: Simulated attacks on swap flows
3. **Cryptography Review**: Verify key derivation and encryption

### Ongoing
1. **Dependency Scanning**: Automated CVE detection
2. **Bug Bounty**: Community-driven security testing
3. **Incident Response**: Documented procedures for vulnerabilities

\newpage

# Testing Strategy

## Unit Tests

### Key Vault
- BIP39 mnemonic generation/validation
- BIP32 key derivation (all paths)
- BIP85 child seed derivation
- Argon2id KDF correctness
- AES-GCM encryption/decryption

### Swap Engine
- State machine transitions
- CAS/version conflict handling
- Idempotent operation execution
- Stale operation reclaim
- Event logging

### Database Layer
- Transaction atomicity
- Optimistic locking (CAS)
- UTXO/LN reservation uniqueness
- Query correctness

## Integration Tests (Regtest)

### Environment
```yaml
services:
  bitcoind:
    image: bitcoin/bitcoin:26.0
    command: -regtest -server -rpcuser=test -rpcpassword=test
  
  elementsd:
    image: elementsd:23.2.1
    command: -chain=elementsregtest
  
  lnd:
    image: lightninglabs/lnd:v0.17.3
    command: --bitcoin.regtest --bitcoin.node=bitcoind
  
  boltz:
    image: boltz/regtest
```

### Test Cases
1. **Submarine Swap**: Liquid → LN (full flow)
2. **Reverse Swap**: LN → Liquid (full flow)
3. **Chain Swap**: BTC → Liquid (full flow)
4. **Refund Path**: Timeout + cooperative refund
5. **Script-Path Fallback**: Boltz offline scenario
6. **Recovery**: Kill process mid-swap, verify resume

## E2E Tests (Electron)

### User Flows
1. **First Launch**: Create wallet, backup mnemonic
2. **Restore**: Recover wallet from mnemonic
3. **Swap**: Complete submarine swap end-to-end
4. **Settings**: Change network, view node status
5. **Update**: Trigger auto-update, verify rollback

### Tools
- **Spectron**: Electron testing framework
- **Playwright**: Browser automation (for renderer)
- **Mock Services**: Simulated Boltz API responses

## Manual Testing (Testnet)

### Pre-Release Checklist
- [ ] Create wallet on testnet
- [ ] Fund with testnet BTC/L-BTC
- [ ] Execute all swap types
- [ ] Test refund scenarios
- [ ] Verify recovery from mnemonic
- [ ] Test on all platforms (Win/Mac/Linux)

\newpage

# Performance Considerations

## Database Optimization

### SQLite Tuning
- **WAL Mode**: Allows concurrent reads during writes
- **Cache Size**: 20MB cache for frequently accessed data
- **Temp Store**: In-memory temporary tables
- **Busy Timeout**: 5s wait prevents lock errors

### Query Optimization
- **Indexes**: Partial indexes for active swaps
- **Prepared Statements**: Reuse compiled queries
- **Transaction Batching**: Group related operations

## Node Management

### Resource Limits
- **bitcoind**: ~2GB RAM, ~500GB disk (mainnet)
- **elementsd**: ~1GB RAM, ~50GB disk
- **LND**: ~500MB RAM, ~10GB disk

### Optimization
- **Pruning**: Enable pruned mode for Bitcoin (reduces to ~10GB)
- **Neutrino**: Consider Neutrino mode for LND (light client)
- **Lazy Start**: Start nodes on-demand, not at app launch

## Frontend Performance

### React Optimization
- **Code Splitting**: Lazy load routes
- **Memoization**: React.memo for expensive components
- **Virtual Lists**: For transaction history (react-window)

### IPC Optimization
- **Batching**: Group multiple IPC calls
- **Caching**: Cache frequently accessed data
- **Streaming**: Use streams for large data transfers

\newpage

# Deployment & Distribution

## Build Process

### Development
```bash
npm run dev              # Start dev server (hot reload)
npm run dev:main         # Compile main process
npm run dev:renderer     # Start Vite dev server
```

### Production
```bash
npm run build            # Build main + renderer
npm run package          # Create installers
```

## Installers

### Windows
- **Format**: NSIS installer + portable exe
- **Size**: ~50MB (without embedded nodes)
- **Signing**: Authenticode certificate
- **Auto-Update**: Via electron-updater

### macOS
- **Format**: DMG + ZIP
- **Size**: ~50MB
- **Signing**: Apple Developer ID
- **Notarization**: Required for Gatekeeper
- **Auto-Update**: Via electron-updater

### Linux
- **Format**: AppImage + deb
- **Size**: ~50MB
- **Signing**: GPG signature
- **Auto-Update**: Via electron-updater

## Release Process

### CI/CD (GitHub Actions)
```yaml
1. Build (all platforms)
2. Test (unit + integration)
3. Sign (code signing)
4. Upload (GitHub Releases)
5. Publish (auto-update server)
```

### Versioning
- **Semantic Versioning**: MAJOR.MINOR.PATCH
- **Changelog**: Auto-generated from conventional commits
- **Release Notes**: Manually curated highlights

\newpage

# Appendix A: Database Schema (Complete)

```sql
-- swaps: Authoritative swap state
CREATE TABLE swaps (
  id TEXT PRIMARY KEY,
  kind TEXT CHECK(kind IN ('submarine','reverse','chain')),
  env TEXT CHECK(env IN ('regtest','testnet','mainnet')),
  version INTEGER NOT NULL DEFAULT 0,
  state TEXT CHECK(state IN ('open','locked',...)),
  locked_intent TEXT,  -- JSON
  swap_key_index INTEGER NOT NULL,
  preimage_hash_hex TEXT CHECK(length(preimage_hash_hex)=64),
  -- ... 40+ additional fields
);

-- swap_events: Audit trail
CREATE TABLE swap_events (
  swap_id TEXT REFERENCES swaps(id),
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_ms INTEGER NOT NULL,
  source TEXT NOT NULL,
  type TEXT NOT NULL,
  payload TEXT NOT NULL
);

-- swap_ops: Idempotency ledger
CREATE TABLE swap_ops (
  swap_id TEXT REFERENCES swaps(id),
  op_key TEXT NOT NULL,
  status TEXT CHECK(status IN ('inflight','ok','fail')),
  heartbeat_at TEXT,
  PRIMARY KEY (swap_id, op_key)
);

-- utxo_reservations: Anti double-spend
CREATE TABLE utxo_reservations (
  chain TEXT CHECK(chain IN ('btc','liquid')),
  txid TEXT NOT NULL,
  vout INTEGER NOT NULL CHECK(vout >= 0),
  swap_id TEXT REFERENCES swaps(id),
  amount_sat TEXT CHECK(amount_sat GLOB '[0-9]*'),
  PRIMARY KEY (chain, txid, vout)
);

-- ln_reservations: Anti payment duplication
CREATE TABLE ln_reservations (
  payment_hash_hex TEXT PRIMARY KEY CHECK(length(payment_hash_hex)=64),
  swap_id TEXT REFERENCES swaps(id),
  direction TEXT CHECK(direction IN ('pay','receive')),
  amount_msat TEXT CHECK(amount_msat GLOB '[0-9]*')
);

-- app_config: Configuration store
CREATE TABLE app_config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

\newpage

# Appendix B: Technology Stack

## Core Technologies

| Component | Technology | Version | Purpose |
|-----------|-----------|---------|---------|
| Desktop Framework | Electron | 28+ | Cross-platform desktop app |
| Frontend | React | 18 | UI framework |
| Build Tool | Vite | 5 | Fast dev server + bundler |
| Language | TypeScript | 5.3 | Type-safe development |
| Database | SQLite | 3.31+ | Embedded database |
| DB Library | better-sqlite3 | 9.2 | Synchronous SQLite bindings |
| Styling | TailwindCSS | 3 | Utility-first CSS |
| State | Zustand | 4 | Lightweight state management |

## Bitcoin Libraries

| Library | Version | Purpose |
|---------|---------|---------|
| bitcoinjs-lib | 6.1 | Bitcoin transaction building |
| bip32 | 4.0 | HD key derivation |
| bip39 | 3.1 | Mnemonic generation |
| bolt11 | 1.4 | Lightning invoice parsing |

## Cryptography

| Library | Version | Purpose |
|---------|---------|---------|
| argon2 | 0.31 | Password hashing (KDF) |
| crypto (Node.js) | Built-in | AES-GCM encryption |

## Node Communication

| Library | Version | Purpose |
|---------|---------|---------|
| @grpc/grpc-js | 1.9 | LND gRPC client |
| axios | 1.6 | HTTP client (Boltz API) |
| ws | 8.16 | WebSocket client |

## Development Tools

| Tool | Version | Purpose |
|------|---------|---------|
| electron-builder | 24.9 | Packaging + installers |
| electron-updater | 6.1 | Auto-update |
| vitest | 1.1 | Unit testing |
| eslint | 8.56 | Linting |

\newpage

# Appendix C: Glossary

**Atomic Swap**: A cryptographic protocol that enables exchange of assets across different blockchains without requiring trust in a third party.

**BIP32**: Bitcoin Improvement Proposal 32 - Hierarchical Deterministic Wallets. Defines how to derive multiple keys from a single seed.

**BIP39**: Bitcoin Improvement Proposal 39 - Mnemonic code for generating deterministic keys. Defines the 12/24 word backup phrase.

**BIP84**: Bitcoin Improvement Proposal 84 - Derivation scheme for P2WPKH (native SegWit) addresses.

**BIP85**: Bitcoin Improvement Proposal 85 - Deterministic entropy from BIP32 keychains. Used for deriving child seeds.

**CAS (Compare-And-Swap)**: An atomic operation that updates a value only if it matches an expected value. Used for optimistic locking.

**HTLC (Hash Time-Locked Contract)**: A smart contract that requires the recipient to acknowledge payment before a deadline by generating cryptographic proof of payment, or forfeit the ability to claim the payment.

**MuSig2**: A multi-signature scheme for Schnorr signatures that allows multiple parties to cooperatively sign a transaction.

**P2TR (Pay-to-Taproot)**: A Bitcoin output type that uses Taproot, enabling more efficient and private transactions.

**P2WSH (Pay-to-Witness-Script-Hash)**: A SegWit output type that commits to a script via its hash.

**Preimage**: In the context of HTLCs, the preimage is the secret value R whose hash H is used in the contract. Revealing R allows claiming the locked funds.

**Submarine Swap**: A swap from on-chain to Lightning Network. User locks funds on-chain, receives Lightning payment.

**Reverse Swap**: A swap from Lightning Network to on-chain. User pays Lightning invoice, receives on-chain funds.

**Chain Swap**: A swap between two different blockchains (e.g., Bitcoin ↔ Liquid).

**WAL (Write-Ahead Logging)**: A SQLite journaling mode that improves concurrency by allowing reads during writes.

---

**Document Version**: 0.1.0  
**Last Updated**: January 19, 2026  
**Status**: Foundation Complete (15%)
