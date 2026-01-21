# XS Wallet - Architecture (Agent)

Spec human doc: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.
Agent guide: `docs/AGENTS.md`.

## Target architecture (spec)
- Electron Renderer (React) -> IPC -> Electron Main -> gRPC -> Go Core.
- Go Core: swap engine, SQLite WAL, adapters, boltz client (HTTP/WS).
- Provider: boltz-backend self-hosted.
- Nodes: LND (gRPC), elementsd (JSON-RPC), bitcoind optional.

## Current implementation
- Frontend uses HTTP via `api-bridge/` (temporary).
- Electron main/preload/IPC not implemented.
- Go Core exists with swap engine + vault + boltz client.
- Adapters LND/elementsd and Node Manager not implemented.

## Canonical entrypoints
- Core daemon: `core/cmd/xscore/main.go`.
- Frontend app: `frontend/src/app/App.tsx`.
- Protos: `proto/*.proto`.

## Migration target
Follow `docs/PLANO_IMPLEMENTACAO_v0.2.md`, Phase 4 for Electron/IPC.
