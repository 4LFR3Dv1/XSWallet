# XS Wallet - Project Status (Agent)

Canonical status vs spec: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.
Plan: `docs/PLANO_IMPLEMENTACAO_v0.2.md`.

## Current state (summary)
- Core: swap engine + vault + boltz client implemented (partial).
- Phase 1 core fixes concluida: schema unificado, preimage criptografada, lockout/backoff.
- Frontend: `frontend/` is the real app; still HTTP-based via `api-bridge/`.
- Wallet on-chain: BTC + Liquid (confidential) com enderecos derivados do vault + envio on-chain implementado.
- Electron/IPC: not implemented.
- Reverse/Chain swaps, MuSig2, claim/refund: not implemented.
- Node adapters (LND/elementsd) + Node Manager: not implemented.

## Next focus
- Phase 2 (swap protocol) per `docs/PLANO_IMPLEMENTACAO_v0.2.md`.
