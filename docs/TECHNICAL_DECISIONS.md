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
- Vault uses Argon2id + AES-256-GCM, but no lockout/backoff.
- Electron/IPC not implemented.
- LND/elementsd adapters not implemented.

## Agent rules
- Do not introduce new HTTP contracts for frontend.
- Use `proto/*.proto` as source of truth.
- Keep schema canonical in `core/internal/db/db.go`.
