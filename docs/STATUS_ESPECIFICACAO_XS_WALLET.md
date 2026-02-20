# XS Wallet - Spec v0.2.0 vs Estado Atual (Agent)

Spec humano: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.
Guia agente: `docs/AGENTS.md`.

## Atualizacao rapida (2026-02-20)
- O core expõe streams de swaps implementados (`GetSwapEvents`, `WatchSwap`, `WatchAllSwaps`) e o frontend já consome o stream global com fallback de polling.
- O fluxo de execução watcher para `chain` e `reverse` já assina MuSig2 parcial real e atinge `waiting_provider_broadcast` em integração testnet.
- O caminho pós-broadcast está endurecido: sucesso terminal por evidência de provider e caminho de refund separado.
- `refunding -> completed` depende de evidência objetiva on-chain (`refund_txid` observado), reduzindo dependência de status textual.
- NodeService está com health em níveis (`UP/READY/DEGRADED`), supervisor com backoff/circuit-breaker e gate `manifest vs runtime`.
- Projeto permanece em pre-beta: forte para validação técnica/operacional, ainda sem fechamento de release produção.

## 1. Arquitetura (spec)
- Frontend Electron + IPC -> Go Core (gRPC).
- Go Core: swap engine, DB SQLite WAL, adapters, boltz client.
- boltz-backend self-hosted como provider.
- Nodes primarios: LND (gRPC) e elementsd (JSON-RPC).

Estado atual:
- Frontend real: `frontend/` (React + Vite) com caminho HTTP e integração Electron/IPC parcial.
- Ponte HTTP existe via `api-bridge/` (temporaria).
- Go Core existe; adapter elementsd (Liquid JSON-RPC) implementado e LND parcial.
- boltz-backend existe no repo.
- Wallet on-chain BTC + Liquid (confidential) implementada via seed do vault (BIP84 + SLIP-0077).
- Nivel de maturidade: **pre-beta / MVP tecnico avancado**.

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
- Submarine parcial (quote/create com auto-lock/commit).
- Reverse/Chain com create+lock e execucao backend avancada ate `waiting_provider_broadcast`.
- MuSig2 parcial real plugada no watcher para chain/reverse (derivacao + nonce/session + envio de partial).
- Reconciliacao usa status real do provider com idempotencia por `swap_ops`.
- Pos-broadcast endurecido:
  - sucesso terminal: `waiting_provider_broadcast -> completed` com evidencias de provider (`transaction.claimed` / `invoice.settled` / `swap.completed`);
  - falha terminal: `waiting_provider_broadcast -> refunding` via caminho fallback.
- RefundSeen endurecido: conclusao de refund depende de evidencia objetiva on-chain via `refund_txid` (mempool/confirmada), nao apenas status textual de provider.
- Watcher ignora transicao terminal dirigida por provider quando swap ja esta em `refunding`; fechamento depende da evidencia do adapter BTC.
- Fallback script-path agora consome dados persistidos de `locked_intent`:
  - chain: `lockup_details.swapTree` + `claim_details.swapTree` + `blindingKey`;
  - reverse: `reverse_details.swapTree` + `refundPublicKey`.
- Fallback script-path BTC: builder local `local_zero_builder` ativo (UTXO + leaf + control-block + destino canonico derivado da chave refund), com provider hex como contingencia.
- Streaming gRPC de eventos implementado no core:
  - `GetSwapEvents` consulta eventos reais.
  - `WatchSwap` faz stream incremental por `seq`.
  - `WatchAllSwaps` faz stream global com `filter_states`.
  - Cobertura em teste inclui fluxo de transicoes e details JSON em eventos.
- Integracao watcher (testnet):
  - `TestChainExecutionToWaitingProviderBroadcastIntegration`: PASS (modo simulate trigger).
  - `TestReverseExecutionToWaitingProviderBroadcastIntegration`: PASS (modo simulate trigger).
  - modo `FULL_REAL` sem simulacao implementado em ambos testes para seguir ate estado terminal (`completed`/`refunding`) com polling continuo.

## 4. Criptografia (spec)
- Preimage R CSPRNG e armazenada criptografada.
- Vault: Argon2id + AES-256-GCM.
- Nonce safety MuSig2 + fallback script-path.

Estado atual:
- Vault Argon2id + AES-256-GCM implementado.
- Preimage salva criptografada no DB (`encrypted_preimage`).
- Lockout/backoff implementado (temporario e permanente).
- MuSig2 parcial implementada no watcher; artefatos persistidos em colunas `musig_*` e `locked_intent.musig`.
- API expõe lockout/attempts via `GetVaultStatus` e `UnlockVault`.

## 5. Node manager (spec)
- Download/verificacao de binarios (manifest assinado).
- Lifecycle e health checks.

Estado atual:
- NodeService com lifecycle real parcial: `start/stop/restart/status/watch`.
- Health em 3 niveis (UP/READY/DEGRADED) com reason codes operacionais.
- Supervisor de processo com restart/backoff/circuit-breaker.
- Validacao `manifest vs runtime` no boot (gate para bloquear READY em divergencia critica).
- Manifest canônico publicado:
  - `core/config/nodemanager.manifest.json`
  - `core/config/nodemanager.manifest.schema.json`
- Gate automatizado de release para manifest/schema:
  - `go test ./internal/server -run TestCanonicalNodeManagerManifestSchema -count=1`

## 6. Frontend (spec)
- Electron hardening (contextIsolation, sandbox, nodeIntegration=false).
- IPC whitelist + validacao (Zod).

Estado atual:
- Frontend Vite/React com Electron em progresso (`electron/main.ts`, `electron/preload.ts`, `electron/ipc/*`).
- IPC com allowlist/validação e hardening base em Electron; cobertura de canais ainda parcial.
- UI inclui fluxo basico de envio on-chain (BTC/Liquid) via API Bridge.
- Swap Center atualizado para estados reais de execucao (`waiting_claim_details`, `signing_musig2_partial`, `sent_partial_to_provider`, `waiting_provider_broadcast`, `refund_*`).
- Timeline de eventos por swap adicionada no frontend (trigger, transicao e `details_json`) via endpoint `/api/v1/swaps/:id/events`.
- Stream frontend endurecido:
  - watchdog de atividade (ready/ping/swap_event) com reconexao se stream ficar estagnado;
  - fallback polling controlado para stream global;
  - headers SSE `X-Accel-Buffering: no` no bridge para reduzir buffering intermediario.

## 7. API contracts (spec)
- boltz-backend v2 endpoints.
- IPC interno com canais definidos no spec.

Estado atual:
- Protos gRPC existem.
- HTTP `/api/v1` via `api-bridge/` (nao previsto no spec).

## Gaps chave
- Migrar para Electron/IPC.
- Fechar fluxo de claim/refund final com broadcast de refund real em todos os caminhos e persistencia consistente de artefatos.
- Completar adapter/flows de LND + Node Manager.
- Endurecer streaming de swaps/events (latencia, backpressure e escalabilidade) no runtime de producao.
- Alinhar scripts de build/lint/test do root com a estrutura atual do monorepo para CI consistente.
