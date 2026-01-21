# XS Wallet - Agent Guide (Spec v0.2.0)

Fonte de verdade (humano): `docs/XS_Wallet_Especificacao_Tecnica_v2.html`.
Este guia e para agentes: arquitetura, estado atual, fluxos de trabalho e limites.

## Escopo e prioridades
- UI oficial: `frontend/` (React + Vite). `devdash/` esta fora do escopo.
- Arquitetura alvo: Electron + IPC + gRPC (conforme spec).
- Estado atual: frontend ainda usa HTTP (`api-bridge/`) como ponte temporaria.

## Mapa do repo (componentes)
- Go Core: `core/` (xscore, gRPC, swap engine, vault, DB).
- Frontend: `frontend/` (app real).
- API Bridge (temporario/debug): `api-bridge/`.
- boltz-backend (self-hosted provider): `boltz-backend/`.
- Protos: `proto/` (fonte), `core/proto/` (gerados).
- Tests/regtest: `test/`.

## Estado atual resumido
- Swap engine e vault existem (parcial). Referencias: `core/internal/swap/`, `core/internal/vault/`.
- Cliente Boltz HTTP/WS implementado. Referencias: `core/internal/boltz/`.
- Reverse/Chain swaps, MuSig2 e claim/refund nao implementados.
- Electron/IPC nao implementado.
- Node Manager e adapters LND/elementsd nao implementados.
- DB schema duplicado: fonte canonica deve ser `core/internal/db/db.go`.

## Fluxos de trabalho (agente)
### Build e run (core)
```
cd core
go build ./cmd/xscore
.\xscore.exe --network=regtest --port=9735
```

### Frontend (atual via HTTP)
```
cd frontend
npm install
npm run dev
```

### API Bridge (debug)
```
cd api-bridge
npm install
npm start
```

### Regtest
```
docker compose -f test/regtest/docker-compose.yml up -d
```

## Invariantes do spec (nao violar)
- Preimage R gerada via CSPRNG, nunca derivar de chave privada.
- locked_intent deve existir para swaps ativos (state != open/failed/canceled).
- CAS via version para transicoes de estado.
- Eventos WS sao apenas triggers; validar on-chain/LN antes de agir.

## Contratos e limites
- Protos gRPC: `proto/*.proto` sao a fonte.
- Schema canonico: `core/internal/db/db.go`.
- API HTTP (`api-bridge/`) e temporaria e deve ser removida na migracao Electron/IPC.
- `docs/XS_Wallet_Especificacao_Tecnica_v2.html` e o unico documento humano.

## Documentos de trabalho
- Plano: `docs/PLANO_IMPLEMENTACAO_v0.2.md`.
- Status vs spec: `docs/STATUS_ESPECIFICACAO_XS_WALLET.md`.

## Boas praticas para agentes
- Sempre alinhar mudancas ao HTML do spec.
- Atualizar `docs/STATUS_ESPECIFICACAO_XS_WALLET.md` quando mudar estado real.
- Evitar criar novos contratos paralelos (HTTP vs IPC).
