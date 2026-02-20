# XS Wallet

Carteira desktop self-custody com execução de atomic swaps entre BTC, Liquid e Lightning.

[![Status](https://img.shields.io/badge/Status-Pre--Beta-yellow)](docs/STATUS_ESPECIFICACAO_XS_WALLET.md)
[![Spec](https://img.shields.io/badge/Spec-v0.2.0-blue)](docs/XS_Wallet_Especificacao_Tecnica_v2.html)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](core/go.mod)

## Resumo

O projeto está em **pre-beta / MVP técnico avançado**: núcleo de carteira e swaps com backend funcional para desenvolvimento, validação operacional e integração com provider real (Boltz testnet), ainda com hardening final pendente para produção.

Principais capacidades atuais:
- Carteira on-chain BTC + Liquid (confidential).
- Vault com Argon2id + AES-256-GCM.
- Engine de swaps com máquina de estados, CAS e idempotência.
- Fluxos `submarine`, `reverse` e `chain` com execução watcher até caminhos avançados de claim/refund.
- Integrações reais com Boltz testnet (incluindo testes de execução).
- NodeService com lifecycle real parcial (`start/stop/restart/status/watch`) + supervisor/backoff/circuit-breaker.
- Gate de manifest canônico (`manifest vs runtime`) para NodeManager.

## Estado Atual (objetivo)

| Área | Estado |
|---|---|
| Core Go (`xscore`) | Funcional |
| Wallet BTC/Liquid | Funcional |
| Swap Engine + Reconcile | Funcional |
| MuSig2 claim path (watcher) | Funcional para fluxo atual |
| Refund script-path fallback | Funcional com builder local + fallback |
| gRPC swap streams/events | Funcional |
| Frontend Swap Center | Parcial avançado, alinhado a estados reais |
| NodeService | Parcial real (download/verificação de binários ainda pendente) |
| LND fluxo operacional completo | Pendente |
| Release produção | Pendente (hardening final) |

Fonte canônica de status: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.

## Arquitetura

```text
Frontend (React/Vite + Electron em migração)
        |
        | HTTP (api-bridge, caminho atual) / IPC (alvo)
        v
Go Core (xscore)
  - Wallet + Vault
  - Swap Engine + Watcher
  - gRPC Services (Swap/Wallet/Node)
  - SQLite WAL
        |
        +--> bitcoind (JSON-RPC)
        +--> elementsd (JSON-RPC)
        +--> lnd (gRPC, parcial)
        +--> boltz backend/API (HTTP/WS)
```

## Estrutura do Repositório

```text
core/              backend Go (xscore)
frontend/          app React/Vite
electron/          main/preload/ipc (migração)
api-bridge/        ponte HTTP para frontend atual
proto/             contratos proto fonte
core/proto/        código gRPC gerado
docs/              especificação, status e planos
.github/workflows/ CI e release gates
```

## Pré-requisitos

- Go 1.24+
- Node.js 20+
- npm
- Docker (para ambiente regtest)
- (Opcional mainnet/testnet) `bitcoind`, `elementsd`, `lnd` instalados no host

## Execução Local

### 1) Subir ambiente regtest

```bash
docker compose -f test/regtest/docker-compose.yml up -d
```

### 2) Rodar core

```bash
cd core
go build ./cmd/xscore
go run ./cmd/xscore --network=regtest --port=9735
```

Com config explícita:

```bash
go run ./cmd/xscore --config ./config.local.mainnet.pruned.json --network=mainnet --port=9735
```

### 3) Rodar API bridge

```bash
cd api-bridge
npm install
npm start
```

### 4) Rodar frontend

```bash
cd frontend
npm install
npm run dev
```

## Configuração NodeManager (manifest canônico)

Arquivos canônicos versionados:
- `core/config/nodemanager.manifest.json`
- `core/config/nodemanager.manifest.schema.json`

Variáveis suportadas pelo `NodeService`:
- `XS_NODEMANAGER_MANIFEST_PATH` (path do manifest)
- `XS_NODEMANAGER_MANIFEST_REQUIRED` (`1|true|yes|on` para tornar obrigatório)

Com `required` ativo, divergência crítica de manifest bloqueia estado operacional READY e `StartNode`.

## Testes

### Backend core essencial

```bash
cd core
go test ./internal/server -count=1
go test ./internal/watcher -count=1
```

### Gate de manifest canônico

```bash
cd core
go test ./internal/server -run TestCanonicalNodeManagerManifestSchema -count=1 -v
```

### Integrações reais com Boltz testnet (opcional)

Exemplos:

```bash
cd core
XS_BOLTZ_CHAIN_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateChainIntegration -v
XS_BOLTZ_REVERSE_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateReverseIntegration -v
```

Para execução watcher full-flow, use os testes em `core/internal/watcher/*execution_integration_test.go` com as variáveis `XS_BOLTZ_*` correspondentes.

## CI e Release Gates

Workflows ativos:
- `/.github/workflows/ci-core.yml`
  - roda `go test ./internal/server -count=1`
  - roda `go test ./internal/watcher -count=1`
- `/.github/workflows/release-gate-manifest.yml`
  - valida manifest canônico por schema/teste (`TestCanonicalNodeManagerManifestSchema`)

Esses gates são bloqueadores para evolução de release backend.

## Segurança e Operação

- Vault: preimage nunca em plaintext persistente (criptografia em repouso).
- RPC host policy: NodeService exige host local/loopback para operação segura.
- Credenciais LND (tls/macaroon): validação de existência e permissões restritas.
- Refund terminal: fechamento por evidência objetiva on-chain (`refund_txid` observado), não apenas status textual do provider.

## Documentação Importante

- Especificação técnica: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`
- Status vs implementação real: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`
- Status executivo do projeto: `PROJECT_STATUS.md`
- Pré-requisitos NodeManager real: `docs/XS_Wallet_PreRequisitos_NodeManager_Real.html`
- Checklist E2E swap: `docs/XS_Wallet_Checklist_E2E_Swap.html`

## Próximos Focos

- Fechar execução E2E real terminal contínua sem simulação para chain e reverse.
- Hardening final de NodeManager/LND para produção.
- Concluir migração frontend para caminho principal Electron/IPC.
- Fechar checklist final de release com runbook operacional.

## Nota de Backup

O mnemônico restaura chaves, mas recuperação operacional de swaps em andamento depende também do estado persistido (DB/vault/artefatos de execução). Planeje backup consistente de dados locais.

## Comandos Rápidos de Release

```bash
# 1) Core gates
cd core
go test ./internal/server -count=1
go test ./internal/watcher -count=1

# 2) Gate de manifest canônico (schema + exemplo)
go test ./internal/server -run TestCanonicalNodeManagerManifestSchema -count=1 -v

# 3) (Opcional) integração real Boltz testnet
XS_BOLTZ_CHAIN_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateChainIntegration -v
XS_BOLTZ_REVERSE_INTEGRATION=1 XS_BOLTZ_API_URL=https://api.testnet.boltz.exchange go test ./internal/boltz -run TestCreateReverseIntegration -v
```
