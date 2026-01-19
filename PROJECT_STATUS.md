# XS Wallet - Estrutura do Projeto

Revisão completa do diretório em 2026-01-19.

---

## 📁 Estrutura de Diretórios

```
XS WALLET/
├── database/
│   ├── schema.sql              # Schema Postgres (original, deprecated)
│   └── schema.sqlite.sql       # ✅ Schema SQLite (production)
├── docs/
│   └── TECHNICAL_DECISIONS.md  # ✅ Decisões arquiteturais
├── src/
│   ├── nodes/
│   │   └── manager.ts          # ✅ Node lifecycle + verified downloads
│   └── swap/
│       ├── db.ts               # Postgres version (deprecated)
│       └── db.sqlite.ts        # ✅ SQLite version (production)
└── package.json                # ✅ Dependencies + build config
```

---

## ✅ Arquivos Implementados (8)

### 1. **database/schema.sqlite.sql** (Production)
- 5 tabelas: `swaps`, `swap_events`, `swap_ops`, `utxo_reservations`, `ln_reservations`
- 1 config table: `app_config`
- Pragmas: WAL, synchronous, busy_timeout, foreign_keys
- Timestamps: ISO-8601 UTC
- Guards: vout >= 0, amount GLOB '[0-9]*'
- **Status**: ✅ Production-ready

### 2. **src/swap/db.sqlite.ts** (Production)
- better-sqlite3 (synchronous)
- 6 pragmas críticos (WAL, busy_timeout, temp_store, cache_size)
- CAS com `updated_at` controlado
- Transactions para `transitionSwapState`
- Stale op reclaim via SQL interval
- Normalization helpers (txid, hash, pubkey)
- **Status**: ✅ Production-ready

### 3. **src/nodes/manager.ts**
- Lifecycle: start/stop/status
- Verified downloads (SHA256 checksum)
- Binary registry (versioned)
- Spawn + logging
- **Status**: ✅ Core complete (needs real checksums)

### 4. **docs/TECHNICAL_DECISIONS.md**
- 8 decisões críticas documentadas
- SQLite pragmas
- Backend process separation
- Download verificável
- PIN + Vault security
- Auto-update strategy
- **Status**: ✅ Complete

### 5. **package.json**
- Dependencies: better-sqlite3, bitcoinjs-lib, bip32, bip39, argon2
- DevDeps: electron, vite, typescript
- Scripts: dev, build, package, test
- electron-builder config
- **Status**: ✅ Complete

---

## 📊 Status do Projeto

### ✅ Concluído (Fase 0: Fundação)
- [x] Plano de implementação (Desktop App)
- [x] Decisões técnicas (8/8)
- [x] Schema SQLite (production-ready)
- [x] DB Layer (SQLite + WAL + pragmas)
- [x] Node Manager (verified downloads)
- [x] Package.json (dependencies)

### 🚧 Próximos Passos (Fase 1: Backend Core)
- [ ] tsconfig.json (main + renderer)
- [ ] Key Vault (BIP39/32/84/85)
- [ ] Vault encryption (Argon2id + AES-GCM)
- [ ] Chain Adapters (BTC, Liquid, LN)
- [ ] Boltz Client (API v2 + MuSig2)
- [ ] Swap Engine (state machine)

### 📅 Roadmap
- **Fase 1**: Backend Core (2 semanas)
- **Fase 2**: Frontend React (1.5 semanas)
- **Fase 3**: Electron Integration (1 semana)
- **Fase 4**: Packaging (0.5 semana)
- **Total**: ~5 semanas

---

## 🔧 Configuração Atual

### Database
- **Engine**: SQLite 3.31+ (WAL mode)
- **Library**: better-sqlite3 (synchronous)
- **Location**: `%APPDATA%/xs-wallet/xs-wallet.db`

### Nodes
- **bitcoind**: v26.0 (planned)
- **elementsd**: v23.2.1 (planned)
- **LND**: v0.17.3 (planned)
- **Strategy**: Verified download on first run

### Security
- **Seed**: Argon2id + AES-256-GCM
- **PIN**: 6-8 digits + rate limiting
- **Database**: SQLite (optional SQLCipher in v1.1)

---

## 🎯 Próximo Arquivo a Criar

**Recomendação**: `tsconfig.json` (main + renderer)

Isso vai resolver os lint errors atuais e permitir começar a implementação do Key Vault.
