# XS Wallet - Architecture (Agent)

Spec human doc: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.
Agent guide: `docs/AGENTS.md`.

## Target architecture (spec)
- Electron Renderer (React) -> IPC -> Electron Main -> gRPC -> Go Core.
- Go Core: swap engine, SQLite WAL, adapters, boltz client (HTTP/WS).
- Provider: boltz-backend self-hosted.
- Nodes: LND (gRPC), elementsd (JSON-RPC), bitcoind optional.

## Current implementation
- Frontend uses HTTP via `api-bridge/` and has partial Electron IPC integration.
- Electron main/preload/IPC exist (`electron/main.ts`, `electron/preload.ts`, `electron/ipc/*`) with allowlist + validation + rate limit on mutations.
- Go Core exists with swap engine + vault + boltz client.
- Swap events gRPC in core are active (`GetSwapEvents`, `WatchSwap`, `WatchAllSwaps`) using incremental polling by sequence.
- elementsd adapter exists (Liquid JSON-RPC); LND adapter exists in base form (not complete for full LN flows).
- Node Manager lifecycle/binary management is not implemented in runtime (`core/internal/server/services.go` returns `Unimplemented` for key ops).

## Current maturity
- Stage: **pre-beta / advanced technical MVP**.
- Suitable for: local development, integration validation, and progressive hardening.
- Not yet suitable for: production release with full self-hosted node lifecycle and complete swap protocol coverage.

## Canonical entrypoints
- Core daemon: `core/cmd/xscore/main.go`.
- Frontend app: `frontend/src/app/App.tsx`.
- Electron main process: `electron/main.ts`.
- Electron IPC registry: `electron/ipc/registry.ts`.
- HTTP bridge: `api-bridge/server.js`.
- Protos: `proto/*.proto`.

## Migration target
Follow `docs/PLANO_IMPLEMENTACAO_v0.2.md`, Phase 4 for Electron/IPC.
