# DomniWallet

Stack custodial para **Wallet + Payout Service** (BTC/Liquid/Tron) com integração regtest e testes e2e.

## Estrutura
- `wallet-service`: API de contas, endereços, saldo, withdrawals.
- `payout-service`: state machine + executor BTC (regtest).
- `gateway`: fluxo PIX/Depix (mock + testes).
- `reuse/xs_wallet`: adapters e infra regtest.
- `docs`: especificações, runbook e checklists.

## Quickstart (regtest)
```powershell
cd C:\Users\windows10\Downloads\DomniWallet\reuse\xs_wallet\infra\regtest
docker compose up -d
```

## Wallet Service
```powershell
# criar conta
curl -X POST http://localhost:8081/v1/accounts `
  -H "Content-Type: application/json" `
  -d '{"uuid":"11111111-1111-1111-1111-111111111111"}'
```

## Payout Service (BTC regtest)
```powershell
# e2e completo (cria conta, UTXO, withdrawal, payout, confirma on-chain)
powershell -ExecutionPolicy Bypass -File C:\Users\windows10\Downloads\DomniWallet\payout-service\scripts\payout_service_e2e.ps1 -Verify
```

Relatório do e2e:
- `payout-service/logs/payout_service_e2e_report_*.html`

## Testes
```powershell
# payout-service unit tests
cd C:\Users\windows10\Downloads\DomniWallet\payout-service
go test ./...

# adapter BTC (taproot + broadcast real)
cd C:\Users\windows10\Downloads\DomniWallet\reuse\xs_wallet
go test -v -tags=integration ./adapters/bitcoin -run TestBuildTaprootFundingTx_Regtest
```

## Docs
- Especificação técnica: `docs/DomniWallet_Especificacao_Tecnica.html`
- Checklist: `docs/DomniWallet_Checklist_Conformidade.html`
- Runbook: `docs/runbook.md`

## Status atual
- BTC executor + e2e regtest: **OK**
- Liquid/Tron executors: **pendente**
- Watchers: **pendente**
- Observabilidade/Runbook operacional: **parcial**
