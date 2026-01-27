# DomniWallet - Wallet Service

Custodial wallet service (Go + PostgreSQL) per the DomniWallet spec.

## Requisitos
- PostgreSQL 15+
- (Opcional) bitcoind RPC para gerar endereços BTC P2TR
- (Opcional) elementsd RPC para gerar endereços Liquid

## Variáveis de ambiente
- `WALLET_DB_DSN` (obrigatório)
- `WALLET_HTTP_ADDR` (default `:8081`)
- `WALLET_INTERNAL_TOKEN` (opcional; protege endpoints internos)
- `BTC_RPC_URL`, `BTC_RPC_USER`, `BTC_RPC_PASS`
- `LIQUID_RPC_URL`, `LIQUID_RPC_USER`, `LIQUID_RPC_PASS`

## Como rodar
```bash
cd C:\Users\windows10\Downloads\DomniWallet\wallet-service
set WALLET_DB_DSN=postgres://domni:domni_regtest_pw@localhost:5432/domniwallet?sslmode=disable
set WALLET_HTTP_ADDR=:8081

go run ./cmd/walletd
```

## Endpoints (MVP)
- `POST /v1/accounts`
- `POST /v1/accounts/{uuid}/addresses`
- `GET /v1/accounts/{uuid}/balances`
- `GET /v1/accounts/{uuid}/transactions`
- `POST /v1/withdrawals`
- `GET /v1/withdrawals/{id}`

### Endpoints internos (watchers)
- `POST /v1/internal/utxos` (requer `X-Internal-Token` se configurado)
- `POST /v1/internal/accounts`

## Exemplos (curl)
Criar conta:
```bash
curl -X POST http://localhost:8081/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{"uuid":"11111111-1111-1111-1111-111111111111"}'
```

Gerar endereço:
```bash
curl -X POST http://localhost:8081/v1/accounts/11111111-1111-1111-1111-111111111111/addresses \
  -H "Content-Type: application/json" \
  -d '{"network":"btc","asset":"BTC"}'
```

Consultar saldo:
```bash
curl http://localhost:8081/v1/accounts/11111111-1111-1111-1111-111111111111/balances
```

Criar withdrawal:
```bash
curl -X POST http://localhost:8081/v1/withdrawals \
  -H "Content-Type: application/json" \
  -d '{"account_uuid":"11111111-1111-1111-1111-111111111111","network":"btc","asset":"BTC","amount":10000,"destination":"bc1p..."}'
```
