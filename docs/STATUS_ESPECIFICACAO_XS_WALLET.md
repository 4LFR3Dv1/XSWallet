# XS Wallet - Spec v0.2.0 vs Estado Atual (Agent)

Spec humano: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.
Guia agente: `docs/AGENTS.md`.

## 1. Arquitetura (spec)
- Frontend Electron + IPC -> Go Core (gRPC).
- Go Core: swap engine, DB SQLite WAL, adapters, boltz client.
- boltz-backend self-hosted como provider.
- Nodes primarios: LND (gRPC) e elementsd (JSON-RPC).

Estado atual:
- Frontend real: `frontend/` (React + Vite), sem Electron/IPC.
- Ponte HTTP existe via `api-bridge/` (temporaria).
- Go Core existe, mas adapters LND/elementsd nao existem.
- boltz-backend existe no repo.
- Wallet on-chain BTC + Liquid (confidential) implementada via seed do vault (BIP84 + SLIP-0077).

## 2. Database (spec)
- SQLite WAL + pragmas do spec.
- CAS com `version`.
- Tabelas: `swaps`, `swap_events`, `swap_ops`, `utxo_reservations`, `ln_reservations`, `app_config`.

Estado atual:
- Pragmas configurados em `core/internal/db/db.go`.
- CAS existe em `swap.Engine.Transition`.
- Schema unificado em `core/internal/db/db.go` (arquivo duplicado removido).
- Colunas adicionadas em `swaps`: `encrypted_preimage`, `boltz_status`, `boltz_raw`, `from_asset`, `to_asset`.
- Tabela `vault_lockout` criada com migracao automatica para DBs existentes.
- Tabelas `wallet_addresses` e `wallet_transactions` adicionadas para enderecos/tx on-chain.

## 3. Swaps (spec)
- Submarine, Reverse e Chain completos.
- Verificacoes P2TR e timeouts antes de lock/funding.
- WS events como trigger, validacao on-chain/LN como verdade.
- Maquina de estados completa (14 estados).

Estado atual:
- Submarine parcial (quote/create/lock/commit).
- Reverse/Chain nao implementados.
- MuSig2 e claim/refund nao implementados.
- Reconciliacao nao usa status real do provider.

## 4. Criptografia (spec)
- Preimage R CSPRNG e armazenada criptografada.
- Vault: Argon2id + AES-256-GCM.
- Nonce safety MuSig2 + fallback script-path.

Estado atual:
- Vault Argon2id + AES-256-GCM implementado.
- Preimage salva criptografada no DB (`encrypted_preimage`).
- Lockout/backoff implementado (temporario e permanente).
- MuSig2 nao implementado.
- API expõe lockout/attempts via `GetVaultStatus` e `UnlockVault`.

## 5. Node manager (spec)
- Download/verificacao de binarios (manifest assinado).
- Lifecycle e health checks.

Estado atual:
- NodeService e stub.
- Nao ha Node Manager nem infra de manifest.

## 6. Frontend (spec)
- Electron hardening (contextIsolation, sandbox, nodeIntegration=false).
- IPC whitelist + validacao (Zod).

Estado atual:
- Frontend Vite/React sem Electron.
- Sem IPC security.
- UI inclui fluxo basico de envio on-chain (BTC/Liquid) via API Bridge.

## 7. API contracts (spec)
- boltz-backend v2 endpoints.
- IPC interno com canais definidos no spec.

Estado atual:
- Protos gRPC existem.
- HTTP `/api/v1` via `api-bridge/` (nao previsto no spec).

## Gaps chave
- Migrar para Electron/IPC.
- Implementar reverse/chain swaps + MuSig2 + claim/refund.
- Adapters LND/elementsd + Node Manager.
