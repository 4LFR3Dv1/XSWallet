# Responsibility Split: Wallet vs boltz-backend

> **Status**: ✅ Documentado durante Spike de Integração (2026-01-20)
> **API Version**: Boltz API v2 (REST + WebSocket)
> **Mainnet API**: https://api.boltz.exchange
> **WebSocket**: wss://api.boltz.exchange/v2/ws

---

## Visão Geral

Este documento mapeia as responsabilidades entre o **XS Wallet (xscore)** e o **boltz-backend** (API pública ou self-hosted).

Todos os swaps usam **Taproot** (P2TR) com **MuSig2** para key path spends cooperativos.

---

## Submarine Swap (Chain → LN)

| Etapa | Responsável | Endpoint/Ação | Notas |
|-------|-------------|---------------|-------|
| 1. Criar swap | **Wallet** | `POST /swap/submarine` | Envia: `from`, `to`, `invoice`, `refundPublicKey` |
| 2. Receber HTLC address | **boltz-backend** | Response | Retorna: `id`, `address`, `bip21`, `expectedAmount`, `timeout` |
| 3. **Verificar endereço P2TR** | **Wallet** | Local | Rebuild swapTree com chaves públicas |
| 4. Financiar HTLC (L-BTC/BTC) | **Wallet** | Broadcast tx | Assina e broadcast para `address` |
| 5. Detectar funding | **boltz-backend** | WebSocket | Status: `transaction.mempool` → `transaction.confirmed` |
| 6. Pagar invoice LN | **boltz-backend** | Automático | Status: `invoice.pending` → `invoice.paid` |
| 7. Claim cooperativo (MuSig2) | **Wallet** | `GET/POST /swap/submarine/{id}/claim` | Opcional: Wallet fornece partial signature |
| 8. Claim final | **boltz-backend** | Broadcast claim tx | Status: `transaction.claimed` (final) |

### Preimage
- **Nasce**: Invoice do usuário (fornecida a Boltz)
- **Revelada por**: Boltz (ao pagar invoice)
- **Exposta para wallet?**: ✅ Sim, no status após `invoice.paid`

### State Machine
```
swap.created → transaction.mempool → transaction.confirmed 
            → invoice.pending → invoice.paid → transaction.claim.pending 
            → transaction.claimed (✅ SUCCESS)

Errors: invoice.failedToPay, transaction.lockupFailed, swap.expired
```

### Refund (se falhar)
- **Cooperativo**: `POST /swap/submarine/{id}/refund` - Imediato
- **Script path**: Após timeout, wallet broadcast refund tx

---

## Reverse Swap (LN → Chain)

| Etapa | Responsável | Endpoint/Ação | Notas |
|-------|-------------|---------------|-------|
| 1. Gerar preimage (32 bytes) | **Wallet** | Local | `crypto.randomBytes(32)` |
| 2. Calcular hash | **Wallet** | Local | `H = SHA256(preimage)` |
| 3. Criar reverse swap | **Wallet** | `POST /swap/reverse` | Envia: `from`, `to`, `preimageHash`, `claimPublicKey` |
| 4. Receber hold invoice | **boltz-backend** | Response | Retorna: `id`, `invoice`, `lockupAddress`, `timeout` |
| 5. **Verificar hash na invoice** | **Wallet** | Local | `invoice.paymentHash == H` |
| 6. Pagar hold invoice | **Wallet** | LN node | Pagamento fica pending (hold) |
| 7. Boltz financia HTLC | **boltz-backend** | Broadcast lockup tx | Status: `transaction.mempool` → `transaction.confirmed` |
| 8. **Claim HTLC (revela R)** | **Wallet** | `POST /swap/reverse/{id}/claim` | Envia: preimage + partial signature |
| 9. Boltz liquida invoice | **boltz-backend** | Automático | Usa preimage revelada. Status: `invoice.settled` (final) |

### Preimage
- **Nasce**: **Wallet** (etapa 1) - CRÍTICO: nunca derivar de chave privada!
- **Exposta para boltz?**: ✅ Sim, no claim (etapa 8)

### State Machine
```
swap.created → minerfee.paid (opcional) → transaction.mempool 
            → transaction.confirmed → invoice.settled (✅ SUCCESS)

Errors: invoice.expired, transaction.failed, transaction.refunded, swap.expired
```

### Refund
- **Não necessário pelo wallet**: Se swap falhar, LN funds voltam automaticamente

---

## Chain Swap (Chain → Chain)

| Etapa | Responsável | Endpoint/Ação | Notas |
|-------|-------------|---------------|-------|
| 1. Gerar preimage | **Wallet** | Local | 32 bytes random |
| 2. Calcular hash | **Wallet** | Local | H = SHA256(preimage) |
| 3. Criar chain swap | **Wallet** | `POST /swap/chain` | Envia: `from`, `to`, `preimageHash`, `claimPublicKey`, `refundPublicKey` |
| 4. Receber HTLCs (ambos) | **boltz-backend** | Response | Retorna: `lockupDetails` (user), `claimDetails` (server) |
| 5. **Verificar ambos HTLCs** | **Wallet** | Local | Scripts + timeouts + valores |
| 6. Financiar HTLC user | **Wallet** | Broadcast tx | Para `lockupDetails.lockupAddress` |
| 7. Boltz financia HTLC | **boltz-backend** | Broadcast | Status: `transaction.server.mempool` → `transaction.server.confirmed` |
| 8. **Claim (revela R)** | **Wallet** | `GET/POST /swap/chain/{id}/claim` | Fornece partial sig para claim de Boltz + preimage |
| 9. Boltz claim user funds | **boltz-backend** | Broadcast | Status: `transaction.claimed` (final) |

### Preimage
- **Nasce**: **Wallet** (etapa 1)
- **Exposta para boltz?**: ✅ Sim, no claim (etapa 8)

### State Machine
```
swap.created → transaction.mempool → transaction.confirmed 
            → transaction.server.mempool → transaction.server.confirmed 
            → transaction.claimed (✅ SUCCESS)

Errors: transaction.lockupFailed (renegociável!), swap.expired
```

### Refund
- **Cooperativo**: `POST /swap/chain/{id}/refund`
- **Script path**: Após timeout

### Renegociação (Novo!)
Se `transaction.lockupFailed` por over/underpayment:
1. `GET /swap/chain/{id}/quote` - Novo quote baseado no valor real
2. `POST /swap/chain/{id}/quote` - Aceitar novo quote

---

## Resumo de Responsabilidades

| Componente | Wallet | boltz-backend |
|------------|--------|---------------|
| Geração de preimage (reverse/chain) | ✅ | |
| Verificação de scripts HTLC (P2TR) | ✅ | |
| Verificação de hash em invoices | ✅ | |
| Funding de HTLCs | ✅ | |
| Claim de HTLCs (MuSig2 partial sig) | ✅ (reverse/chain) | ✅ (submarine) |
| Refund de HTLCs | ✅ (submarine/chain) | ✅ (reverse) |
| Pagamento LN | ✅ (reverse) | ✅ (submarine) |
| WebSocket de status | Consome | Produz |
| Liquidez | | ✅ |
| Broadcast de claim tx (key path) | | ✅ (submarine) / ✅ (reverse/chain - wallet) |

---

## Perguntas Respondidas no Spike

### 1. ✅ boltz-backend expõe preimage no status do submarine?
**Sim.** Após `invoice.paid`, o status inclui a preimage revelada pelo pagamento LN.

### 2. ✅ boltz-backend faz verificação de funding ou só detecta?
**Detecta valor.** Verifica se valor enviado == `expectedAmount`. Se diferente: `transaction.lockupFailed`.

### 3. ✅ Cooperative claim (MuSig2) é feito por quem?
- **Submarine**: Wallet fornece partial sig, Boltz agrega e broadcast
- **Reverse/Chain**: Wallet obtém partial sig de Boltz, agrega e broadcast

### 4. ✅ Em caso de timeout, quem detecta e dispara refund?
- **Submarine**: Wallet deve monitorar e broadcast refund após timeout
- **Reverse**: Boltz auto-refunds seus próprios fundos
- **Chain**: Wallet deve monitorar e broadcast refund após timeout

### 5. ✅ boltz-backend tem retry automático para LN payments?
**Não explícito na docs.** Se falhar: `invoice.failedToPay` e swap termina.

### 6. ✅ Qual formato de WebSocket events do boltz-backend?
```json
// Subscribe
{ "op": "subscribe", "channel": "swap.update", "args": ["swap-id-1", "swap-id-2"] }

// Confirmation
{ "event": "subscribe", "channel": "swap.update", "args": ["swap-id-1", "swap-id-2"] }

// Update
{ "event": "update", "channel": "swap.update", "args": [{ "id": "swap-id-1", "status": "transaction.confirmed" }] }

// Unsubscribe
{ "op": "unsubscribe", "channel": "swap.update", "args": ["swap-id-1"] }
```

---

## API Endpoints Principais

### Info
| Endpoint | Método | Descrição |
|----------|--------|-----------|
| `/version` | GET | Versão do backend |
| `/swap/submarine` | GET | Pares disponíveis para Submarine |
| `/swap/reverse` | GET | Pares disponíveis para Reverse |
| `/swap/chain` | GET | Pares disponíveis para Chain |

### Submarine Swap
| Endpoint | Método | Descrição |
|----------|--------|-----------|
| `/swap/submarine` | POST | Criar swap |
| `/swap/submarine/{id}` | GET | Status do swap |
| `/swap/submarine/{id}/claim` | GET/POST | Claim cooperativo (MuSig2) |
| `/swap/submarine/{id}/refund` | POST | Refund cooperativo |

### Reverse Swap
| Endpoint | Método | Descrição |
|----------|--------|-----------|
| `/swap/reverse` | POST | Criar swap |
| `/swap/reverse/{id}` | GET | Status do swap |
| `/swap/reverse/{id}/claim` | POST | Claim cooperativo (MuSig2) |

### Chain Swap
| Endpoint | Método | Descrição |
|----------|--------|-----------|
| `/swap/chain` | POST | Criar swap |
| `/swap/chain/{id}` | GET | Status do swap |
| `/swap/chain/{id}/claim` | GET/POST | Claim cooperativo (MuSig2) |
| `/swap/chain/{id}/refund` | POST | Refund cooperativo |
| `/swap/chain/{id}/quote` | GET/POST | Renegociação (se lockupFailed) |

---

## Notas de Implementação

### MuSig2 (Taproot Key Path)
- Boltz usa BIP-327 (MuSig2) para assinaturas agregadas
- Boltz public key sempre vem PRIMEIRO na agregação
- `SIGHASH_DEFAULT` para partial signatures
- boltz-core tem exemplos: https://github.com/BoltzExchange/boltz-core

### 0-conf
- Liquid: Aceito até `maximalZeroConf` sats
- Bitcoin: Geralmente requer 1 confirmação

### Timeouts
- Invoice expira em 50% do tempo de swap
- Swap expira baseado em block height

### Rate Limits
- Não documentados oficialmente
- Recomendado: backoff exponencial em erros

---

## Próximos Passos (Implementação)

1. **BoltzProvider** (Go): Implementar interface `provider.Provider` com API real
2. **WebSocket Client**: Goroutine com reconexão automática
3. **MuSig2**: Usar go-secp256k1 ou similar para partial signatures
4. **Testes**: Usar mainnet API com valores mínimos (1000 sats para Liquid)
