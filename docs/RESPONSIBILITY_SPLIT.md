# XS Wallet - Responsibility Split (Agent)

Spec human doc: `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.

## Wallet (Go Core) responsibilities
- Gerar preimage R via CSPRNG.
- Verificar scripts/enderecos P2TR antes de funding.
- Verificar hash de invoice em reverse swap.
- Financiar HTLCs do usuario.
- Executar claim/refund (MuSig2 ou script-path).
- Persistir estado e operacoes idempotentes (CAS).

## Provider (boltz-backend) responsibilities
- Orquestrar swaps v2 (HTTP/WS).
- Detectar funding e executar pagamentos LN (submarine).
- Executar claims quando aplicavel (submarine key-path).
- Publicar status via WS.

## Security invariants
- Provider nao pode roubar fundos; no pior caso faz DoS.
- Eventos WS sao apenas triggers; validar on-chain/LN antes de agir.

## Current status
- Implementacao parcial (submarine only, sem MuSig2/claim/refund).
- Provider boltz-backend incluido no repo.
- Frontend opera com API Bridge HTTP e caminho Electron/IPC parcial.
- NodeService para lifecycle/download de nodes ainda esta em modo stub no core.
