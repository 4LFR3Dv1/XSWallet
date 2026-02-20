# XS Wallet - Project Status (Agent)

Canonical status vs spec: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.
Plan: `docs/PLANO_IMPLEMENTACAO_v0.2.md`.

## Current state (summary)
- Core: swap engine + vault + boltz client implemented (partial).
- Phase 1 core fixes concluida: schema unificado, preimage criptografada, lockout/backoff.
- Frontend: `frontend/` is the real app; HTTP via `api-bridge/` plus partial Electron IPC path.
- Wallet on-chain: BTC + Liquid (confidential) com enderecos derivados do vault + envio on-chain implementado.
- Electron/IPC: partial (`electron/main.ts`, `electron/preload.ts`, channel registry with allowlist + validation).
- Reverse/Chain swaps: quote/create + auto-lock + execucao backend ate `waiting_provider_broadcast` implementados.
- Swap events gRPC: `GetSwapEvents`, `WatchSwap` e `WatchAllSwaps` implementados no `SwapService` com polling incremental por `seq` e filtro de estado no stream global.
- MuSig2 partial signing no watcher: implementado (chain/reverse), com persistencia em `musig_*` e `locked_intent.musig`.
- Integracoes watcher em testnet (simulate trigger): chain e reverse PASS ate `waiting_provider_broadcast`.
- Integracoes watcher com modo real continuo (sem simulacao) prontas para execucao:
  - `XS_BOLTZ_CHAIN_EXEC_FULL_REAL=1` e `XS_BOLTZ_REVERSE_EXEC_FULL_REAL=1` estendem o teste ate estado terminal (`completed`/`refunding`).
- Pos-broadcast implementado no watcher:
  - sucesso terminal para `completed` por status finais de provider;
  - falha terminal entra em caminho de refund.
- Refund completion endurecido: `refunding -> completed` requer evidencia objetiva on-chain via `refund_txid` (tx observada em mempool/confirmada pelo adapter BTC).
- Fallback script-path consome `swapTree` persistido (`claim_details`/`lockup_details` em chain e `reverse_details` em reverse) para construir plano de refund.
- Fallback script-path BTC com `local_zero_builder` ativo como caminho preferencial (UTXO + leaf + control-block + destino canonico), mantendo provider hex apenas como fallback controlado.
- Frontend Swap Center atualizado com estados de execucao reais + timeline de eventos por swap (`/api/v1/swaps/:id/events`).
- Node adapters: elementsd implemented; LND base exists without full flow.
- NodeManager runtime: parcial funcional (`NodeService` com start/stop/restart/status/watch + supervisor + backoff/circuit-breaker + gate `manifest vs runtime`; download/verificação de binários ainda pendente).
- Maturidade geral: **pre-beta / MVP tecnico avancado** (usavel para desenvolvimento e validacao, ainda nao pronto para release de producao).

## Operational status (today)
- Core unit/integration tests: green.
- Watcher: green com novos testes de restart/idempotencia do caminho `local_zero_builder`.
- Core swap service tests: inclui cobertura de `GetSwapEvents` com transições gravadas (`TestGetSwapEventsReturnsRecordedTransitions`).
- Core E2E tests: dependem de `bitcoind/elementsd` ativos no ambiente de teste.
- Frontend build (`frontend/`): green.
- Root scripts (`package.json` do workspace): precisam alinhamento com a estrutura atual antes de uso como fonte unica de CI.

## Next focus
- Phase 2 (swap protocol) per `docs/PLANO_IMPLEMENTACAO_v0.2.md`.
