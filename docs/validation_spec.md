# DomniWallet - Especificação de Validações e Erros

Validações detalhadas e códigos de erro por endpoint do Payout Service.

---

## Códigos de Erro

### Estrutura

```json
{
  "error": {
    "code": "PAYOUT_INVALID_ADDRESS",
    "message": "Endereço BTC inválido",
    "details": {
      "field": "address",
      "value": "bc1q...",
      "reason": "mixed_case_bech32"
    },
    "retryable": false
  }
}
```

### Códigos por Categoria

#### Validação (4xx)

| Código | HTTP | Descrição | Retryable |
|--------|-----:|-----------|-----------|
| `PAYOUT_INVALID_ADDRESS` | 400 | Endereço inválido | ❌ |
| `PAYOUT_MIXED_CASE_BECH32` | 400 | Bech32 com mixed-case | ❌ |
| `PAYOUT_AMOUNT_BELOW_MIN` | 400 | Valor abaixo do mínimo | ❌ |
| `PAYOUT_AMOUNT_ABOVE_MAX` | 400 | Valor acima do máximo | ❌ |
| `PAYOUT_DUPLICATE` | 409 | payment_id já processado | ❌ |
| `PAYOUT_LIMIT_EXCEEDED` | 429 | Limite diário excedido | ❌ |
| `PAYOUT_DESTINATION_COOLDOWN` | 429 | Destino alterado recentemente | ❌ |

#### Execução (5xx)

| Código | HTTP | Descrição | Retryable |
|--------|-----:|-----------|-----------|
| `PAYOUT_INSUFFICIENT_BALANCE` | 503 | Saldo insuficiente | ✅ |
| `PAYOUT_NETWORK_UNAVAILABLE` | 503 | Rede indisponível | ✅ |
| `PAYOUT_CIRCUIT_OPEN` | 503 | Circuit breaker aberto | ✅ |
| `PAYOUT_TX_REJECTED` | 502 | Tx rejeitada pelo node | ❌ |
| `PAYOUT_INTERNAL_ERROR` | 500 | Erro interno | ✅ |

---

## Validações por Endpoint

### POST /api/v1/payouts

#### Request

```json
{
  "payment_id": "uuid-v4",
  "network": "btc|liquid|tron",
  "destination": "address|invoice",
  "amount_sats": 100000,
  "priority": "fast|normal|slow",
  "metadata": {}
}
```

#### Validações

| Campo | Validação | Erro |
|-------|-----------|------|
| `payment_id` | UUID v4 válido | `PAYOUT_INVALID_PAYMENT_ID` |
| `payment_id` | Único por network | `PAYOUT_DUPLICATE` |
| `network` | Enum válido (btc|liquid|tron) | `PAYOUT_INVALID_NETWORK` |
| `destination` | Format por network | Ver abaixo |
| `amount_sats` | > 0 | `PAYOUT_INVALID_AMOUNT` |
| `amount_sats` | ≥ min por rede | `PAYOUT_AMOUNT_BELOW_MIN` |
| `amount_sats` | ≤ max por rede | `PAYOUT_AMOUNT_ABOVE_MAX` |
| `priority` | Enum válido | `PAYOUT_INVALID_PRIORITY` |

---

## Validação de Destino por Rede

### BTC Address

```go
func ValidateBTCDestination(addr string) []ValidationError {
    var errors []ValidationError
    
    // 1. Formato básico
    if len(addr) < 26 || len(addr) > 90 {
        errors = append(errors, ValidationError{
            Code: "PAYOUT_INVALID_ADDRESS",
            Reason: "invalid_length",
        })
        return errors
    }
    
    // 2. Mixed-case Bech32 (BIP-173)
    if strings.HasPrefix(strings.ToLower(addr), "bc1") {
        hasUpper := strings.ToLower(addr) != addr
        hasLower := strings.ToUpper(addr) != addr
        if hasUpper && hasLower {
            errors = append(errors, ValidationError{
                Code: "PAYOUT_MIXED_CASE_BECH32",
                Reason: "bech32_must_be_single_case",
            })
        }
    }
    
    // 3. Validação via bitcoind
    resp, err := rpc.Call("getaddressinfo", addr)
    if err != nil || !resp.IsValid {
        errors = append(errors, ValidationError{
            Code: "PAYOUT_INVALID_ADDRESS",
            Reason: "rpc_validation_failed",
        })
    }

    // 4. Exigir Taproot (witness v1) no MVP
    if !resp.IsWitness || resp.WitnessVersion != 1 {
        errors = append(errors, ValidationError{
            Code: "PAYOUT_INVALID_ADDRESS",
            Reason: "not_taproot",
        })
    }
    
    return errors
}
```

### Liquid Address

```go
func ValidateLiquidDestination(addr string) []ValidationError {
    var errors []ValidationError
    
    // 1. Validação via elementsd
    resp, err := rpc.Call("validateaddress", addr)
    if err != nil || !resp.IsValid {
        errors = append(errors, ValidationError{
            Code: "PAYOUT_INVALID_ADDRESS",
            Reason: "elements_validation_failed",
        })
    }
    
    // 2. Guardar confidential_key para auditoria
    if resp.ConfidentialKey != "" {
        // Store for audit
    }
    
    return errors
}
```

### Tron Address (TRC20)

```go
func ValidateTronDestination(addr string) []ValidationError {
    var errors []ValidationError

    // 1. Formato base58check (prefixo T)
    if len(addr) < 34 || addr[0] != 'T' {
        errors = append(errors, ValidationError{
            Code: "PAYOUT_INVALID_ADDRESS",
            Reason: "invalid_format",
        })
        return errors
    }

    // 2. Validação via node (wallet/validateaddress)
    resp, err := rpc.Call("validateaddress", addr)
    if err != nil || !resp.IsValid {
        errors = append(errors, ValidationError{
            Code: "PAYOUT_INVALID_ADDRESS",
            Reason: "tron_validation_failed",
        })
    }

    return errors
}
```

### Lightning Invoice (futuro)

Lightning está fora do MVP. Esta validação fica como referência para versão futura.

---

## Validações de Negócio

### Idempotência

```sql
-- Check + Lock atômico
WITH existing AS (
    SELECT id, status 
    FROM payouts 
    WHERE payment_id = $1 AND network = $2
    FOR UPDATE NOWAIT
)
SELECT 
    CASE 
        WHEN EXISTS (SELECT 1 FROM existing) THEN 'DUPLICATE'
        ELSE 'OK'
    END as result,
    existing.status as existing_status
FROM existing;
```

| Cenário | Resposta |
|---------|----------|
| Não existe | Continuar |
| Existe + COMPLETED | 409 `PAYOUT_DUPLICATE` |
| Existe + FAILED_FINAL | 409 `PAYOUT_DUPLICATE` |
| Existe + outro status | 409 `PAYOUT_IN_PROGRESS` |

### Limites

```go
func ValidateLimits(userID string, amount int64, network string) error {

// Cooldown de destino (anti-fraude)
func ValidateDestinationCooldown(userID, destination string) error {
    // Se destino foi alterado recentemente, bloquear
    return nil
}

    // 1. Limite por payout
    limits := GetNetworkLimits(network)
    if amount < limits.MinSats {
        return ErrAmountBelowMin
    }
    if amount > limits.MaxSats {
        return ErrAmountAboveMax
    }
    
    // 2. Limite diário por usuário
    dailyTotal := GetUserDailyTotal(userID)
    userLimit := GetUserDailyLimit(userID)
    if dailyTotal + amount > userLimit {
        return ErrDailyLimitExceeded
    }
    
    return nil
}
```

---

## Response Codes

### Sucesso

| HTTP | Cenário |
|-----:|---------|
| 201 | Payout criado, aguardando execução |
| 200 | Status consultado com sucesso |

### Erro

| HTTP | Categoria |
|-----:|-----------|
| 400 | Validação de input |
| 409 | Conflito (duplicado) |
| 429 | Rate limit ou limite de valor |
| 500 | Erro interno |
| 502 | Erro de rede (node) |
| 503 | Serviço indisponível (circuit open, saldo) |

---

## Retry Guidance

Na response de erro, incluir header:

```http
X-Retry-After: 30
X-Retryable: true
```

| Código | Retry After |
|--------|-------------|
| 503 Circuit Open | 60s |
| 503 Insufficient Balance | 300s |
| 429 Rate Limit | Header value |
