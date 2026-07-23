# XS Wallet

Desktop wallet self-custody com execução de swaps entre BTC, Liquid e Lightning, baseada em arquitetura modular (`frontend + electron + api-bridge + core`).

> **Public status — 2026-07-22:** this repository is a pre-beta engineering build, not a public release. The current frontend uses the working name **Domini** while the repository and technical documents use **XS Wallet**. Treat the product identity as unresolved until the owner selects a canonical name. Security properties below describe implemented controls; they are not an external audit.

[![Status](https://img.shields.io/badge/Status-Pre--Beta%20Technical-yellow)](docs/STATUS_ESPECIFICACAO_XS_WALLET.md)
[![Spec](https://img.shields.io/badge/Spec-v0.2.0-blue)](docs/XS_Wallet_Especificacao_Tecnica_v2.html)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](core/go.mod)

## 1) Executive Overview

O projeto está em **pré-beta técnico operacional**.

Estado validado em **2026-02-26**:
- Caminho crítico de wallet/sessão/swap em **IPC-first**.
- Contrato de sessão consolidado entre Frontend, Electron, Bridge e Core.
- Fluxo wallet BTC on-chain validado ponta a ponta com transação real confirmada em testnet.

Evidência operacional on-chain:
- `txid`: `ec05410c116e30c7dd21ada4417fb48ad6f56242d5d4847b60772bab464a343b`
- bloco: `000000002c0756d28eb8d1a7727dc2520912f3f16cf70541571f8ce471078d17`
- altura: `4842408`

## 2) Implementation Status

| Domínio | Estado | Observação |
|---|---|---|
| Core Go (`xscore`) | Funcional | Serviços wallet/swap/node ativos em testnet |
| Wallet BTC/Liquid | Funcional | Derivação HD, UTXO, send on-chain |
| Vault + PIN/session | Funcional | Argon2id + AES-256-GCM, sessão com invalidação explícita |
| Transporte crítico FE→BE | Funcional | IPC-first; HTTP apenas em debug explícito |
| Swap engine + watcher | Funcional (parcial avançado) | Fluxos principais ativos; hardening final em andamento |
| NodeManager lifecycle | Parcial real | Download/verificação final ainda em evolução |
| LND operação completa | Parcial | Dependente de setup/segredos e fluxo final |
| Release produção | Pendente | Gates finais de hardening e operação |

Fonte canônica de status: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.

## 3) System Architecture

```text
Frontend (React/Vite)
   |
   | IPC (padrão para operações críticas)
   v
Electron Main (IPC Registry + Session Contract)
   |
   | HTTP local (bridge interno)
   v
API Bridge (REST local -> gRPC)
   |
   v
Core Go (xscore)
  - WalletService (vault, addresses, utxos, send)
  - SwapService (quotes, swaps, events, watcher)
  - NodeService (status/lifecycle)
  - SQLite WAL
   |
   +--> bitcoind (JSON-RPC)
   +--> elementsd (JSON-RPC)
   +--> lnd (gRPC)
   +--> boltz backend/API (HTTP/WS)
```

## 4) Security Model

- **Self-custody**: seed local sob vault criptografado.
- **Vault**: Argon2id (KDF) + AES-256-GCM (at-rest).
- **Session contract**: unlock gera sessão, 401/unauth limpa sessão em toda a cadeia.
- **IPC-first policy**: operações críticas não dependem de HTTP em produção.
- **Guard rails**: export de chave privada desabilitado por padrão.

## 5) Repository Structure

```text
core/              backend Go (xscore)
frontend/          app React/Vite
electron/          main/preload/ipc
api-bridge/        ponte REST local -> gRPC
proto/             contratos proto fonte
core/proto/        código gRPC gerado
docs/              especificação, status, planos e relatórios
.github/workflows/ CI e release gates
test/regtest/      laboratório regtest
```

## 6) Environment & Prerequisites

Requisitos:
- Go `1.24+`
- Node.js `20+`
- npm
- `bitcoind` testnet no host (ou equivalente)
- opcional: `elementsd`, `lnd`

Ambientes suportados:
- `dev` (rápido)
- `testnet` (validação real)
- `regtest` (laboratório local)
- `prod-like` (hardening/release)

## 7) Quickstart

### 7.1 Runtime testnet único (recomendado)

```bash
cd XSWallet
scripts/runtime/start_testnet_runtime.sh
scripts/runtime/health_testnet_runtime.sh
```

Parar runtime:

```bash
cd XSWallet
scripts/runtime/stop_testnet_runtime.sh
```

Logs:
- `XSWallet/.runtime/xscore.log`
- `XSWallet/.runtime/api-bridge.log`

### 7.2 Fluxo manual (alternativo)

```bash
cd core
go run ./cmd/xscore --config ./config.local.testnet.pruned.json --network=testnet --port=9735
```

```bash
cd api-bridge
npm install
GRPC_HOST=127.0.0.1:9735 npm start
```

```bash
cd frontend
npm install
npm run dev
```

### 7.3 Regtest (laboratório)

```bash
docker compose -f test/regtest/docker-compose.yml up -d
cd core
go run ./cmd/xscore --network=regtest --port=9735
```

## 8) Configuration Matrix (essencial)

| Variável | Camada | Obrigatória | Observação |
|---|---|---|---|
| `GRPC_HOST` | api-bridge | Sim | Endpoint do `xscore` |
| `VITE_DEBUG_HTTP` | frontend | Não | Permite HTTP fallback apenas em debug |
| `ELECTRON_API_URL` | electron | Sim (desktop) | URL local da bridge |
| `ELECTRON_API_AUTH_BEARER` | electron | Condicional | Sem fallback implícito em produção |
| `XS_NODEMANAGER_MANIFEST_PATH` | core | Condicional | Path do manifest canônico |
| `XS_NODEMANAGER_MANIFEST_REQUIRED` | core | Condicional | `1|true|yes|on` para tornar obrigatório |

## 9) Reliability & Failure Modes

Cenários conhecidos:

1. `scantxoutset` lento/conflitante em nó pruned/testnet.
2. Scan exclusivo já em execução (`Scan already in progress`).
3. Timeout intermitente em descoberta de UTXO.

Mitigações já no código:
- cache de scan em background,
- fallback para respostas rápidas em leitura,
- recuperação adicional no fluxo de envio quando necessário,
- reconciliação de sessão em erros de autenticação.

## 10) Testing & Quality Gates

### 10.1 Core backend

```bash
cd core
go test ./internal/server -count=1
go test ./internal/watcher -count=1
```

### 10.2 Manifest gate

```bash
cd core
go test ./internal/server -run TestCanonicalNodeManagerManifestSchema -count=1 -v
```

### 10.3 Boltz integration (opcional)

```bash
cd core
XS_BOLTZ_CHAIN_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateChainIntegration -v
XS_BOLTZ_REVERSE_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateReverseIntegration -v
```

## 11) CI / Release Process

Workflows ativos:
- `.github/workflows/ci-core.yml`
- `.github/workflows/release-gate-manifest.yml`

Gates mínimos de release backend:
1. testes `server` e `watcher` verdes,
2. schema/manifest canônico validado,
3. sem regressão de contrato crítico IPC/session.

## 12) Observability

- Logs estruturados por request no bridge (latência, status, requestId).
- Logs de gRPC no core para trilhas críticas.
- Endpoints de saúde para diagnóstico operacional.

## 13) Documentation Index

- Status canônico: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`
- Especificação técnica: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`
- Relatório FE-BE: `docs/RELATORIO_CONFORMIDADE_INTEGRACAO_FE_BE.html`
- Estado geral atingido: `docs/XS_Wallet_Estado_Operacional_Atual.html`
- Pré-beta plano executável: `docs/XS_Wallet_PreBeta_Plano_Executavel_v1.1.html`
- Roadmap hardening/restore/e2e/release: `docs/XS_Wallet_Roadmap_PreBeta_Hardening_Restore_E2E_Release.html`

## 14) Known Limitations

- Performance de scan depende do estado do nó BTC.
- Algumas trilhas de NodeManager/LND ainda em consolidação.
- A release pública depende do fechamento dos gates finais de pré-beta.

## 15) Contributing & Change Discipline

- Não quebrar contrato crítico de sessão/transporte sem atualizar docs e testes.
- Toda alteração em fluxo wallet/swap deve incluir evidência de validação.
- Mudanças de estado de produto devem refletir em `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.

## 16) Operational Disclaimer

XS Wallet é software de self-custody. Uso em produção exige hardening final, runbook operacional e procedimentos de backup/recuperação validados.

