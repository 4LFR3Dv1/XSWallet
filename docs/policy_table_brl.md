# DomniWallet - Policy Table v1 (BRL)

Thresholds em BRL com SLOs operacionais. Taxa de conversão de referência: **1 BTC = R$ 600.000** (ajustar conforme mercado).

---

## Faixas de Valor

| Faixa | BRL | Sats | Classificação |
|-------|----:|-----:|---------------|
| **Pequeno** | < R$ 600 | < 100k | Baixo risco |
| **Médio** | R$ 600 – R$ 6.000 | 100k – 1M | Médio risco |
| **Grande** | > R$ 6.000 | > 1M | Alto risco |

---

## Lightning Network (Futuro)

Lightning está fora do MVP atual. Esta seção será publicada em versão futura.

---

## BTC On-Chain

### Confirmações por Valor

| Faixa BRL | Confirmações | Tempo Estimado |
|-----------|-------------:|----------------|
| < R$ 600 | 1 | ~10 min |
| R$ 600 – R$ 6.000 | 3 | ~30 min |
| > R$ 6.000 | 6 | ~60 min |

### Timelock CSV (Recuperação BTC)

Aplicável apenas ao script-path de recuperação do usuário. O key-path do serviço não é afetado.
Tier definido pelo valor do saque (BRL).

| Faixa BRL | CSV (blocos) | Tempo Estimado |
|-----------|-------------:|----------------|
| < R$ 600 | 144 | ~1 dia |
| R$ 600 – R$ 6.000 | 432 | ~3 dias |
| > R$ 6.000 | 1008 | ~1 semana |

### Fee Tiers

| Tier | Target (blocos) | Uso | Limite BRL |
|------|----------------:|-----|------------|
| **fast** | 2 | Urgente | até R$ 30/tx |
| **normal** | 6 | Padrão | até R$ 15/tx |
| **slow** | 12 | Consolidação | até R$ 5/tx |

### SLOs BTC

| Métrica | Target | Janela |
|---------|--------|--------|
| Taxa de broadcast | ≥ 99.5% | 24h |
| Tempo até 1 conf (p50) | ≤ 15 min | 24h |
| Tempo até 1 conf (p99) | ≤ 60 min | 24h |
| Tx stuck (> 24h) | ≤ 0.1% | Semanal |

---

## Liquid

### Confirmações

| Operação | Confirmações | Tempo Estimado |
|----------|-------------:|----------------|
| Depix recebido | 2 | ~2 min |
| Payout enviado | 2 | ~2 min |

### SLOs Liquid

| Métrica | Target | Janela |
|---------|--------|--------|
| Taxa de sucesso | ≥ 99.5% | 24h |
| Latência (p99) | ≤ 5 min | 24h |

---
## Tron (USDC TRC20)

### Confirmações

| Operação | Confirmações | Tempo Estimado |
|----------|-------------:|----------------|
| Depósito | 19 | ~1 min |
| Payout enviado | 19 | ~1 min |

### Observações
- Manter saldo mínimo de TRX para taxas (energy/bandwidth)
- Ajustar confirmações conforme risco e monitoramento

---


## Hot Wallet Limits

| Rede | min | target | max | Em BRL |
|------|----:|-------:|----:|-------:|
| **BTC** | 0.05 | 0.2 | 0.5 | R$ 30k – R$ 300k |
| **L-BTC** | 0.02 | 0.1 | 0.3 | R$ 12k – R$ 180k |

### Alertas de Saldo

| Condição | Ação |
|----------|------|
| `balance < min` por 5 min | Alert: Warning |
| `balance < min * 0.5` | Alert: Critical + SMS |
| `balance > max` | Alert: Info (considerar sweep) |

---

## Limites Operacionais

### Por Payout

| Rede | Mínimo | Máximo |
|------|-------:|-------:|
| BTC | R$ 60 (~10k sat) | R$ 300.000 (~50M sat) |
| Liquid | R$ 6 (~1k sat) | R$ 300.000 (~50M sat) |
| Tron (USDC) | R$ 6 (~1k sat eq.) | R$ 300.000 (~50M sat eq.) |

### Cooldown de Mudança de Destino (Anti-fraude)

| Tier | Cooldown mínimo | Observação |
|------|-----------------:|------------|
| Pequeno | 24h | Bloquear saques para destino recém-alterado |
| Médio | 24h | Bloquear saques para destino recém-alterado |
| Grande | 72h | Bloquear saques para destino recém-alterado |

### Por Usuário (Diário)

| Tier | Limite Diário |
|------|-------------:|
| Básico | R$ 10.000 |
| Verificado | R$ 50.000 |
| Premium | R$ 200.000 |

---

## Retry e Fallback

Para BTC/Liquid, aplicar backoff e usar RBF/CPFP quando necessário.
Lightning fica fora do MVP.

---

## Atualização da Tabela

- Revisar taxas de conversão: **semanal**
- Revisar fee limits: **quando mempool > 100 sat/vB por 24h**
- Revisar SLOs: **mensal** baseado em métricas reais
