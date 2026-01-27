# DomniWallet - Payout Service

Serviço de execução de payouts (state machine + executors).

## Requisitos
- PostgreSQL 15+
- Bitcoin Core RPC (para executor BTC)

## Variáveis de ambiente
- `PAYOUT_DB_DSN` (obrigatório)
- `PAYOUT_HTTP_ADDR` (default `:8090`)
- `WALLET_SERVICE_URL` (opcional; para atualizar status de withdrawals)
- `WALLET_INTERNAL_TOKEN` (opcional)
- `BTC_RPC_URL`, `BTC_RPC_USER`, `BTC_RPC_PASS`, `BTC_NETWORK`
- `BTC_RPC_WALLET` (wallet usada para assinar)
- `PAYOUT_BTC_CONFIRMATIONS` (default `1`)
- `PAYOUT_FEE_FALLBACK_SATVB` (default `10`)
- `PAYOUT_WORKER_INTERVAL` (default `2s`)
- `PAYOUT_MAX_ATTEMPTS` (default `3`)
- `PAYOUT_CIRCUIT_FAILURE_THRESHOLD` (default `5`)
- `PAYOUT_CIRCUIT_OPEN_DURATION` (default `60s`)

## Como rodar
```bash
cd C:\Users\windows10\Downloads\DomniWallet\payout-service
set PAYOUT_DB_DSN=postgres://domni:domni_regtest_pw@localhost:5432/domniwallet?sslmode=disable
set PAYOUT_HTTP_ADDR=:8090
set WALLET_SERVICE_URL=http://localhost:8081
set BTC_RPC_URL=http://localhost:18443
set BTC_RPC_USER=domni
set BTC_RPC_PASS=domni_regtest_pw
set BTC_NETWORK=regtest

go run ./cmd/payoutd
```

## Endpoints (MVP)
- `POST /v1/payouts`
- `GET /v1/payouts/{id}`
- `POST /v1/payouts/{id}/retry`

## Formato de erro (padrão)
```json
{
  "error": {
    "code": "PAYOUT_INVALID_ADDRESS",
    "message": "invalid address length",
    "details": {
      "field": "destination",
      "reason": "invalid_length",
      "value": "bc1..."
    },
    "retryable": false
  }
}
```

## Exemplo (curl)
```bash
curl -X POST http://localhost:8090/v1/payouts \
  -H "Content-Type: application/json" \
  -d '{"payment_id":"11111111-1111-1111-1111-111111111111","withdrawal_id":1,"network":"btc","asset":"BTC","amount_sats":10000,"destination":"bc1p...","priority":"normal"}'
```
