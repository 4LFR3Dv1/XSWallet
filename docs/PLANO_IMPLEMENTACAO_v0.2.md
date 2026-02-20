# XS Wallet - Plano de Implementacao Final
Spec: v0.2.0 (HTML) | Estimativa: 10 semanas | Decisoes: todas confirmadas

## Checkpoint de execucao (codigo atual)
- Fonte canonica do estado atual: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.
- Fase 1 (Core Fixes): concluida.
- Fase 2 (Swap Protocol): parcial avancada (submarine com auto-lock + reconcile; chain/reverse com execucao ate `waiting_provider_broadcast`; pendente claim/refund final e fallback script-path).
- Fase 3 (Adapters/Infra): parcial (elementsd implementado; LND parcial; Node Manager runtime pendente).
- Fase 4 (Electron): parcial (main/preload/registry ativos; migracao completa do frontend para IPC pendente).
- Fase 5 (Tx Management): pendente.
- Fase 6 (Packaging): pendente.

> Este documento descreve o plano alvo. Para status de implementacao, considerar sempre o checkpoint acima e o arquivo canonico de status.

## Decisoes confirmadas
- Electron/IPC: sim, migrar (opcao A).
- Node Manager infra: criar (GitHub Releases + manifest assinado).
- Spec versao: v0.2.0 (HTML).

## Dependency graph
- Phase 1 (Core) > Phase 2 (Swaps) > Phase 5 (Tx)
- Phase 1 (Core) > Phase 3 (Adapters) > Phase 4 (Electron) > Phase 6 (Packaging)

## Phase 1: Core Fixes (Semana 1-2)
1.1 QuoteSwap Kind/Chain Mapping
- Arquivo: `core/internal/server/swap_service.go`
- Problema: hardcode BTC/LN e `req.Kind.String()` nao casa com provider.
- Fix: criar helpers `protoKindToProvider()` e `protoChainToProvider()`.

1.2 Schema Unificado
- Acao: consolidar em `core/internal/db/db.go`.
- Adicionar colunas do spec (ex: `encrypted_preimage`, `from_asset`, `boltz_status`).
- Remover `database/schema.sqlite.sql`.
- Criar migracao para DBs existentes.

1.3 Preimage Criptografada
- Arquivo: `core/internal/vault/vault.go`
- Novo: `EncryptPreimage()` e `DecryptPreimage()`.
- Alterar: `core/internal/swap/engine.go` para salvar encrypted, nao plaintext.

1.4 PIN Lockout Persistente
- Novo: `core/internal/vault/lockout.go`
- Schema: tabela `vault_lockout`.

## Phase 2: Swap Protocol (Semana 3-4)
2.1 Submarine Auto-Lock
- Alterar `CreateFromQuote()` para chamar `Lock()` automaticamente.

2.2 Reverse Swap
- Novo: `core/internal/swap/reverse.go`.

2.3 Chain Swap
- Novo: `core/internal/swap/chain.go`.

2.4 P2TR + MuSig2
- Novo: `core/internal/swap/taproot.go`.
- Novo: `core/internal/musig2/session.go`.

2.5 Watcher + WS
- Integrar `boltz.Normalize()` existente.
- Handler para WS updates.

## Phase 3: Node Adapters + Infra (Semana 5-6)
3.1 LND gRPC Adapter
- Novo: `core/internal/adapters/lnd/client.go`.

3.2 elementsd JSON-RPC
- Novo: `core/internal/adapters/liquid/client.go`.

3.3 Node Manager Infra (CRIAR)
- GitHub Releases setup para binarios.
- Manifest assinado (Ed25519).
- Novo: `core/internal/nodemanager/manager.go`.

## Phase 4: Electron Migration (Semana 7-8)
4.1 Electron Main
- Novo: `electron/main.ts`.
- Hardening: `contextIsolation`, `sandbox`, `nodeIntegration=false`.

4.2 IPC Bridge
- Novo: `electron/preload.ts`.
- Whitelist + Zod validation.

4.3 Frontend Migration
- Alterar `frontend/src/services/api.ts` para IPC.
- Ajustar Vite config para Electron.

4.4 Deprecar api-bridge
- Manter apenas para debug local.

## Phase 5: Tx Management (Semana 9)
5.1 RBF/CPFP
- Novo: `core/internal/tx/bumper.go`.
- Integrar com LND WalletKit + elementsd.

## Phase 6: Packaging (Semana 10)
6.1 Installers
- Windows: Inno Setup.
- macOS: DMG + notarizacao.

6.2 Code Signing
- Windows: Authenticode.
- macOS: Apple Developer ID.

6.3 E2E Tests
- Submarine regtest.
- Reverse regtest.
- Chain regtest.
- Crash recovery.

## Timeline
Semana | Entregas
1-2 | QuoteSwap fix, schema, preimage encrypted, lockout
3-4 | Auto-Lock, Reverse, Chain, MuSig2, Watcher+WS
5-6 | LND adapter, elementsd adapter, Node Manager + GitHub infra
7-8 | Electron main, IPC, frontend migration
9 | RBF/CPFP
10 | Installers, code signing, E2E
