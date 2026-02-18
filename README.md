# XS Wallet

**Self-Custody Desktop Wallet** - Atomic Swaps entre BTC, Liquid e Lightning Network

[![Status](https://img.shields.io/badge/Status-Em%20Desenvolvimento-yellow)]()
[![Spec](https://img.shields.io/badge/Spec-v0.2.0-blue)]()
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)]()

## 📋 Visão Geral

XS Wallet é uma aplicação desktop self-custody com carteira on-chain (BTC + Liquid confidential) e atomic swaps usando Taproot e boltz-backend (self-hosted). Projetada com princípios de **Zero Trust** e **Crash Recovery**.

### Status Atual

| Componente | Status |
|------------|--------|
| Go Core (xscore) | ✅ Implementado |
| Wallet On-chain BTC (BIP84) | ✅ Implementado |
| Wallet On-chain Liquid (confidential) | ✅ Implementado |
| Boltz HTTP/WS Client | ✅ Production-Ready |
| Status Normalization | ✅ Implementado |
| Swap Engine (CAS) | ✅ Base Implementada |
| Vault (Argon2id + AES-GCM) | ✅ Implementado |
| Submarine Swap | ⚠️ Parcial (falta Auto-Lock) |
| Reverse/Chain Swap | ⏳ Pendente |
| MuSig2 | ⏳ Pendente |
| LND Adapter | ⚠️ Base implementada (sem fluxo completo) |
| elementsd Adapter | ✅ Implementado (JSON-RPC) |
| Electron/IPC | ⏳ Pendente |
| Node Manager | ⏳ Pendente |

## 🏗️ Arquitetura

```
┌─────────────────────────────────────────────────┐
│              XS Wallet Desktop                  │
├──────────────────┬──────────────────────────────┤
│   Frontend UI    │     Go Core (xscore)         │
│  ├─ React + Vite │     ├─ Wallet On-chain       │
│  ├─ API Bridge   │     ├─ Swap Engine           │
│  └─ HTTP (temp)  │     ├─ Vault + SQLite WAL    │
│                  │     └─ gRPC Server           │
├──────────────────┴──────────────────────────────┤
│              RPC Adapters                        │
│  ├─ bitcoind     ├─ elementsd                   │
│  └─ LND          └─ boltz-backend               │
└─────────────────────────────────────────────────┘
```

Frontend atualmente comunica via HTTP (`api-bridge/`) como ponte temporária para o gRPC do `xscore`. A arquitetura alvo continua Electron + IPC.

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

### 0. Nodes (regtest)
```bash
docker compose -f test/regtest/docker-compose.yml up -d
```
Para mainnet/testnet, rode `bitcoind` e `elementsd` e ajuste as credenciais no config JSON do `xscore`.

### 0.1 Configuração (mainnet/testnet)
Exemplo mínimo de `config.json` para on-chain (BTC + Liquid). Ajuste paths/credenciais conforme seu ambiente:
```json
{
  "network": "mainnet",
  "data_dir": "C:\\\\Users\\\\seu-usuario\\\\.xs-wallet",
  "db_path": "C:\\\\Users\\\\seu-usuario\\\\.xs-wallet\\\\xs-wallet.db",
  "bitcoind": {
    "enabled": true,
    "host": "127.0.0.1",
    "port": 8332,
    "user": "rpcuser",
    "password": "rpcpass"
  },
  "elementsd": {
    "enabled": true,
    "host": "127.0.0.1",
    "port": 7041,
    "user": "elements",
    "password": "elements_dev_pass_2026"
  },
  "lnd": {
    "enabled": false
  }
}
```
Para testnet, ajuste `network` para `"testnet"` e a porta do `bitcoind` para `18332`.
Se o JSON tiver `network`, ele sobrescreve o valor passado em `--network`.

### 1. Go Core
```bash
cd core
go build ./cmd/xscore
./xscore.exe --network=regtest --port=9735
```
Para mainnet/testnet com config JSON:
```bash
./xscore.exe --config C:\path\config.json --port=9735
```
Se preferir sem JSON, use apenas `--network` e ajuste o `data_dir` com `--datadir`.

### 2. API Bridge (dev only)
```bash
cd api-bridge
npm install && npm start
```
Ajuste `GRPC_HOST` se o `xscore` estiver em outro host/porta.

### 3. Frontend
```bash
cd frontend
npm install && npm run dev
```

### 4. Teste on-chain via frontend (mainnet/testnet)
1. Suba `bitcoind`, `elementsd`, `xscore`, `api-bridge` e `frontend`.
2. Crie a carteira, salve o mnemônico e destrave com o PIN.
3. Gere um endereço BTC e envie uma pequena quantia; aguarde confirmações.
4. Gere um endereço Liquid (confidential) e envie L-BTC; aguarde confirmações.
5. Verifique saldo e UTXOs na tela.
6. Use o formulário de envio para testar `Send` em BTC e em Liquid; confira o TXID retornado.

## 📅 Roadmap (10 semanas)

| Fase | Semana | Entregas |
|------|--------|----------|
| 1. Core Fixes | 1-2 | QuoteSwap fix, schema unificado, preimage encrypted |
| 2. Swap Protocol | 3-4 | Auto-Lock, Reverse, Chain, MuSig2 |
| 3. Node Manager + LND | 5-6 | Node Manager, LND flows completos |
| 4. Electron | 7-8 | Main process, IPC, frontend migration |
| 5. Tx Management | 9 | RBF/CPFP |
| 6. Packaging | 10 | Installers, code signing, E2E |

## 📚 Documentação

- **Especificação Técnica**: [`docs/XS_Wallet_Especificacao_Tecnica_v2.html`](docs/XS_Wallet_Especificacao_Tecnica_v2.html)
- **Status vs Spec**: [`docs/STATUS_ESPECIFICACAO_XS_WALLET.md`](docs/STATUS_ESPECIFICACAO_XS_WALLET.md)
- **Plano de Implementação**: [`docs/PLANO_IMPLEMENTACAO_v0.2.md`](docs/PLANO_IMPLEMENTACAO_v0.2.md)
- **Checklist E2E Swap Pipeline**: [`docs/XS_Wallet_Checklist_E2E_Swap.html`](docs/XS_Wallet_Checklist_E2E_Swap.html)

## ⚠️ Notas Importantes

> **Backup Obrigatório**: O mnemônico restaura apenas chaves. Swaps pendentes (reverse/chain) requerem backup do DB criptografado - a preimage R não é derivável do mnemônico.

## 🔧 Tecnologias

- **Backend**: Go 1.21+, gRPC, SQLite WAL
- **Criptografia**: Argon2id (64MB/3iter), AES-256-GCM
- **Nodes**: Bitcoin Core, Elements (JSON-RPC), LND (gRPC)
- **Frontend**: React, Vite, Electron (em migração)
- **Swap Provider**: boltz-backend (self-hosted)

---

*XS Wallet v0.2.0 - Janeiro 2026*  
*Self-Custody • Zero Trust • Don't Trust, Verify*
