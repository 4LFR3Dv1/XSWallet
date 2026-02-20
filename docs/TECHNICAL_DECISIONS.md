# XS Wallet - Technical Decisions (Agent)

Spec human doc: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.

## Decisions (target)
- SQLite WAL with strict pragmas.
- CAS on swaps with version.
- Vault encryption: Argon2id + AES-256-GCM.
- Electron multi-process with IPC (renderer isolated).
- boltz-backend self-hosted.
- LND/elementsd as primary RPCs.

## Current state
- WAL pragmas configured in `core/internal/db/db.go`.
- CAS implemented in `swap.Engine.Transition`.
- Vault uses Argon2id + AES-256-GCM with lockout/backoff (`core/internal/vault/lockout.go`).
- Electron/IPC implemented partially (`electron/main.ts`, `electron/preload.ts`, `electron/ipc/registry.ts`).
- elementsd adapter implemented (`core/internal/adapters/liquid/client.go`).
- LND adapter exists in base form (`core/internal/adapters/lnd/client.go`) but without full production flow in services.
- NodeService runtime lifecycle is still stubbed (`core/internal/server/services.go`).

## Agent rules
- Prefer Electron IPC contracts for new frontend-core integrations; keep HTTP bridge focused on dev/debug compatibility.
- Use `proto/*.proto` as source of truth.
- Keep schema canonical in `core/internal/db/db.go`.
