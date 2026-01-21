# XS Wallet

**Self-Custody Desktop Wallet** - Atomic Swaps entre BTC, Liquid e Lightning Network

[![Status](https://img.shields.io/badge/Status-Em%20Desenvolvimento-yellow)]()
[![Spec](https://img.shields.io/badge/Spec-v0.2.0-blue)]()
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)]()

## 📋 Visão Geral

XS Wallet é uma aplicação desktop self-custody que permite atomic swaps usando Taproot e boltz-backend (self-hosted). Projetada com princípios de **Zero Trust** e **Crash Recovery**.

### Status Atual

| Componente | Status |
|------------|--------|
| Go Core (xscore) | ✅ Implementado |
| Boltz HTTP/WS Client | ✅ Production-Ready |
| Status Normalization | ✅ Implementado |
| Swap Engine (CAS) | ✅ Base Implementada |
| Vault (Argon2id + AES-GCM) | ✅ Implementado |
| Submarine Swap | ⚠️ Parcial (falta Auto-Lock) |
| Reverse/Chain Swap | ⏳ Pendente |
| MuSig2 | ⏳ Pendente |
| LND/elementsd Adapters | ⏳ Pendente |
| Electron/IPC | ⏳ Pendente |

## 🏗️ Arquitetura

```
┌─────────────────────────────────────────────────┐
│              XS Wallet Desktop                  │
├──────────────────┬──────────────────────────────┤
│  Electron (IPC)  │     Go Core (xscore)         │
│  ├─ React UI     │     ├─ Swap Engine           │
│  └─ Preload      │     ├─ SQLite WAL            │
│                  │     ├─ Boltz Client          │
│                  │     └─ gRPC Server           │
├──────────────────┴──────────────────────────────┤
│              RPC Adapters                        │
│  ├─ LND (gRPC)   └─ elementsd (JSON-RPC)        │
└─────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────┐
│  boltz-backend          │
│  (self-hosted)          │
└─────────────────────────┘
```

## 📁 Estrutura do Projeto

```
XS WALLET/
├── core/                  # Go Core (xscore)
│   ├── cmd/xscore/        # Entry point
│   ├── internal/
│   │   ├── boltz/         # HTTP/WS client + status.go
│   │   ├── swap/          # Engine, orchestrators
│   │   ├── vault/         # Argon2id + AES-GCM
│   │   └── db/            # SQLite WAL
│   └── proto/             # Generated gRPC
├── frontend/              # React + Vite
├── api-bridge/            # HTTP bridge (dev/debug)
├── boltz-backend/         # Swap orchestrator
├── proto/                 # Proto definitions
├── docs/                  # Especificação técnica
└── test/                  # Regtest configs
```

## 🚀 Quick Start

### 1. Go Core
```bash
cd core
go build ./cmd/xscore
./xscore.exe --network=regtest --port=9735
```

### 2. API Bridge (dev only)
```bash
cd api-bridge
npm install && npm start
```

### 3. Frontend
```bash
cd frontend
npm install && npm run dev
```

## 📅 Roadmap (10 semanas)

| Fase | Semana | Entregas |
|------|--------|----------|
| 1. Core Fixes | 1-2 | QuoteSwap fix, schema unificado, preimage encrypted |
| 2. Swap Protocol | 3-4 | Auto-Lock, Reverse, Chain, MuSig2 |
| 3. Node Adapters | 5-6 | LND, elementsd, Node Manager |
| 4. Electron | 7-8 | Main process, IPC, frontend migration |
| 5. Tx Management | 9 | RBF/CPFP |
| 6. Packaging | 10 | Installers, code signing, E2E |

## 📚 Documentação

- **Especificação Técnica**: [`docs/XS_Wallet_Especificacao_Tecnica_v2.html`](docs/XS_Wallet_Especificacao_Tecnica_v2.html)
- **Status vs Spec**: [`docs/STATUS_ESPECIFICACAO_XS_WALLET.md`](docs/STATUS_ESPECIFICACAO_XS_WALLET.md)
- **Plano de Implementação**: [`docs/PLANO_IMPLEMENTACAO_v0.2.md`](docs/PLANO_IMPLEMENTACAO_v0.2.md)

## ⚠️ Notas Importantes

> **Backup Obrigatório**: O mnemônico restaura apenas chaves. Swaps pendentes (reverse/chain) requerem backup do DB criptografado - a preimage R não é derivável do mnemônico.

## 🔧 Tecnologias

- **Backend**: Go 1.21+, gRPC, SQLite WAL
- **Criptografia**: Argon2id (64MB/3iter), AES-256-GCM
- **Frontend**: React, Vite, Electron (em migração)
- **Swap Provider**: boltz-backend (self-hosted)

---

*XS Wallet v0.2.0 - Janeiro 2026*  
*Self-Custody • Zero Trust • Don't Trust, Verify*
