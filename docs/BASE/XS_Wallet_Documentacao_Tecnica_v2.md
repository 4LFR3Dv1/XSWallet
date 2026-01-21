---
title: "XS Wallet - Documentação Técnica v2"
subtitle: "Carteira HD + Atomic Swaps - Aplicação Desktop"
author: "Equipe de Desenvolvimento XS Wallet"
date: "20 de Janeiro de 2026"
version: "0.2.0"
documentclass: article
geometry: margin=1in
fontsize: 11pt
toc: true
toc-depth: 3
numbersections: true
colorlinks: true
lang: pt-BR
---

\newpage

# Sumário Executivo

**XS Wallet** é uma aplicação desktop self-custody que permite atomic swaps entre Bitcoin, Liquid e Lightning Network usando Taproot e **boltz-backend (self-hosted)** como swap orchestrator com **nossa liquidez**. O sistema mantém verdadeira auto-custódia enquanto fornece swaps cross-chain e cross-layer.

## Características Principais

- **Go Core (`xscore`)**: Backend autoritativo em Go com comunicação gRPC
- **Carteira HD**: Compatível com BIP39/32/84/85 com derivação determinística de chaves
- **Suporte Multi-Chain**: Bitcoin (on-chain), Liquid (sidechain), Lightning Network
- **Atomic Swaps**: Swaps Submarine, Reverse e Chain via boltz-backend (compatível API v2)
- **Integração Taproot**: Assinatura cooperativa MuSig2 com fallback script-path
- **Auto-Custódia**: Usuário controla todas as chaves privadas, criptografadas em repouso

## Status do Projeto

| Componente | Status |
|------------|--------|
| Go Core (xscore) | ✅ Implementado |
| Boltz Client HTTP | ✅ Production-Ready |
| WebSocket Client | ✅ Production-Ready |
| Status Normalization | ✅ Implementado |
| Swap Engine | ✅ Base implementada |
| LND Adapter | 🔄 Próximo Sprint |
| Liquid Adapter | 🔄 Próximo Sprint |
| Frontend Electron | 📋 Backlog |

**Progresso**: ~40% do MVP

\newpage

# Arquitetura do Sistema

## Visão Geral de Alto Nível

```
┌─────────────────────────────────────────────────────────────┐
│                     XS Wallet Desktop                        │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐         ┌──────────────────────────────┐  │
│  │   Electron   │         │     Go Core (xscore)         │  │
│  │  React UI    │◄──gRPC──►│                              │  │
│  │              │         │  ┌────────────────────────┐   │  │
│  │  • Dashboard │         │  │   Swap Engine          │   │  │
│  │  • Swap UI   │         │  │   • State Machine      │   │  │
│  │  • Settings  │         │  │   • CAS/Idempotência   │   │  │
│  └──────────────┘         │  │   • Crash Recovery     │   │  │
│                            │  └────────────────────────┘   │  │
│                            │                               │  │
│                            │  ┌────────────────────────┐   │  │
│                            │  │   Adapters (2 RPC)     │   │  │
│                            │  │   • LND (gRPC)         │   │  │
│                            │  │   • elementsd (JSON)   │   │  │
│                            │  └────────────────────────┘   │  │
│                            │                               │  │
│                            │  ┌────────────────────────┐   │  │
│                            │  │   Boltz Client         │   │  │
│                            │  │   • HTTP + Retry       │   │  │
│                            │  │   • WebSocket          │   │  │
│                            │  │   • Status Normalization│  │  │
│                            │  └────────────────────────┘   │  │
│                            └──────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │   boltz-backend      │
                    │   (self-hosted)      │
                    │   REST compatível v2 │
                    │   Nossa liquidez     │
                    └──────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
         ┌────────┐      ┌──────────┐     ┌─────────┐
         │ LND    │      │elementsd │     │bitcoind │
         │(gRPC)  │      │(JSON-RPC)│     │(runtime)│
         └────────┘      └──────────┘     └─────────┘
```

## Detalhamento dos Componentes

### 1. Go Core (`xscore`)

O Go Core é o **backend autoritativo** da aplicação, implementado em Go para máxima performance e segurança.

**Responsabilidades:**
- Swap Engine com máquina de estados
- State machine autoritativa com CAS (Compare-And-Swap)
- Gestão de chaves criptográficas
- Comunicação com nodes via adapters
- Exposição de API via gRPC

**Arquivos Implementados:**
```
core/
├── cmd/xscore/main.go       # Entrypoint do daemon
├── internal/
│   ├── boltz/               # Boltz Client (Production-Ready)
│   │   ├── errors.go        # Erros tipados
│   │   ├── types.go         # Structs API v2
│   │   ├── client.go        # HTTP + retry/backoff
│   │   ├── ws.go            # WebSocket single-writer
│   │   ├── status.go        # Status normalization
│   │   └── boltz.go         # Provider interface
│   ├── swap/                # Swap Engine
│   │   └── engine.go        # State machine + CAS
│   ├── provider/            # Interfaces abstração
│   └── db/                  # Camada de banco de dados
├── proto/                   # Definições gRPC
└── test/                    # Testes E2E
```

### 2. Interfaces RPC (2 Primárias)

| Interface | Protocolo | Uso |
|-----------|-----------|-----|
| **LND** | gRPC | Lightning (invoices, payments, channels) |
| **elementsd** | JSON-RPC | Liquid (addresses, transactions, blinding) |
| bitcoind | - | Dependência do runtime (não RPC primário) |

### 3. Boltz Client (Production-Ready)

**Features implementadas:**
- HTTP Client com exponential backoff
- Rate limit handling (`Retry-After` header)
- Resource leak prevention (body fechado em closure)
- WebSocket com padrão Single-Writer
- Gap Recovery automático via REST
- Status normalization table-driven por tipo de swap
- Tratamento de tipos polimórficos (minerFees, timeouts)

### 4. boltz-backend (Self-Hosted)

> **Mudança Arquitetural**: Usamos boltz-backend self-hosted com nossa liquidez, não a API pública da Boltz.

**Características:**
- REST API compatível com Boltz v2
- WebSocket para status updates
- Orquestração de nodes
- Liquidez controlada internamente

**Modelo de Confiança:**
- Provider operacional = boltz-backend interno
- Riscos = infraestrutura + liquidez própria
- Verificação = scripts HTLC reconstruídos localmente

\newpage

# Arquitetura do Banco de Dados

## Design do Schema (SQLite)

### Tabelas Core

#### 1. `swaps` - Estado Autoritativo do Swap
```sql
CREATE TABLE swaps (
  id                  TEXT PRIMARY KEY,
  kind                TEXT CHECK(kind IN ('submarine','reverse','chain')),
  state               TEXT CHECK(state IN ('open','locked',...)),
  version             INTEGER NOT NULL DEFAULT 0,  -- CAS/Locking Otimista
  locked_intent       TEXT,  -- JSON: snapshot de quote/fees
  swap_key_index      INTEGER NOT NULL,  -- Restore determinístico
  -- ... proofs, timeouts, contexto MuSig2
);
```

**Características-Chave:**
- **Locking Otimista**: Campo `version` para Compare-And-Swap (CAS)
- **Locked Intent**: Snapshot imutável de quote/fees/config
- **Restore Determinístico**: `swap_key_index` para recuperação via mnemônico

#### 2. `swap_events` - Trilha de Auditoria
```sql
CREATE TABLE swap_events (
  swap_id     TEXT REFERENCES swaps(id),
  seq         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_ms       INTEGER NOT NULL,
  source      TEXT NOT NULL,  -- 'boltz_ws' | 'node_poll' | 'engine'
  type        TEXT NOT NULL,
  payload     TEXT NOT NULL   -- JSON
);
```

#### 3. `swap_ops` - Ledger de Idempotência
```sql
CREATE TABLE swap_ops (
  swap_id     TEXT REFERENCES swaps(id),
  op_key      TEXT NOT NULL,
  status      TEXT CHECK(status IN ('inflight','ok','fail')),
  PRIMARY KEY (swap_id, op_key)
);
```

#### 4. `utxo_reservations` - Anti Double-Spend
```sql
CREATE TABLE utxo_reservations (
  chain       TEXT CHECK(chain IN ('btc','liquid')),
  txid        TEXT NOT NULL,
  vout        INTEGER NOT NULL,
  swap_id     TEXT REFERENCES swaps(id),
  PRIMARY KEY (chain, txid, vout)
);
```

## Pragmas Críticos do SQLite

```sql
PRAGMA journal_mode = WAL;        -- Write-Ahead Logging
PRAGMA synchronous = NORMAL;      -- Balanço segurança/performance
PRAGMA busy_timeout = 5000;       -- Espera 5s em lock
PRAGMA foreign_keys = ON;         -- Integridade referencial
```

\newpage

# Fluxos de Atomic Swap

## Submarine Swap (Liquid → Lightning)

```
┌──────┐                 ┌───────────────┐        ┌─────────┐
│Wallet│                 │boltz-backend  │        │ Liquid  │
└──┬───┘                 └───────┬───────┘        └────┬────┘
   │                             │                     │
   │ 1. Criar invoice LN         │                     │
   ├────────────────────────────►│                     │
   │                             │                     │
   │ 2. Endereço HTLC + tree     │                     │
   │◄────────────────────────────┤                     │
   │                             │                     │
   │ 3. VERIFICAR end. P2TR      │                     │
   │    (rebuild swapTree)       │                     │
   │                             │                     │
   │ 4. Financiar HTLC(L-BTC)    │                     │
   ├─────────────────────────────┼────────────────────►│
   │                             │                     │
   │                             │ 5. Pagar invoice LN │
   │                             │    (recebe preimage)│
   │                             │                     │
   │                             │ 6. Claim HTLC       │
   │                             ├────────────────────►│
   │                             │                     │
   │ 7. Notificação sucesso      │                     │
   │◄────────────────────────────┤                     │
```

## Reverse Swap (Lightning → Liquid)

```
┌──────┐                 ┌───────────────┐        ┌─────────┐
│Wallet│                 │boltz-backend  │        │ Liquid  │
└──┬───┘                 └───────┬───────┘        └────┬────┘
   │                             │                     │
   │ 1. Gerar R, H=SHA256(R)     │                     │
   │                             │                     │
   │ 2. Solicitar reverse        │                     │
   ├────────────────────────────►│                     │
   │    (enviar H)               │                     │
   │                             │                     │
   │ 3. Hold invoice + HTLC      │                     │
   │◄────────────────────────────┤                     │
   │                             │                     │
   │ 4. VERIFICAR hash invoice   │                     │
   │    (hash == H)              │                     │
   │                             │                     │
   │ 5. Pagar hold invoice       │                     │
   ├────────────────────────────►│                     │
   │                             │                     │
   │                             │ 6. Financiar HTLC   │
   │                             ├────────────────────►│
   │                             │                     │
   │ 7. CLAIM HTLC (revela R)    │                     │
   ├─────────────────────────────┼────────────────────►│
   │                             │                     │
   │                             │ 8. Liquidar invoice │
   │                             │◄────────────────────│
```

\newpage

# Boltz Client - Implementação Production-Ready

## Visão Geral

O Boltz Client foi implementado com padrões de produção para garantir confiabilidade e resiliência.

## HTTP Client com Retry/Backoff

```go
// doRequest com retry, backoff, e CLOSURE para fechar body
func (c *Client) doRequest(ctx context.Context, ...) error {
    var lastErr error
    backoff := initialBackoff

    for attempt := 0; attempt <= maxRetries; attempt++ {
        // Backoff exponencial
        if attempt > 0 {
            time.Sleep(backoff)
            backoff = min(backoff*2, maxBackoff)
        }

        // Executa em closure para fechar body corretamente
        respBody, statusCode, err := func() ([]byte, int, error) {
            resp, err := c.httpClient.Do(req)
            if err != nil { return nil, 0, err }
            defer resp.Body.Close() // Fechado aqui, não no loop!
            return io.ReadAll(resp.Body)
        }()

        // Retry em 5xx ou 429
        if statusCode >= 500 || statusCode == 429 {
            lastErr = fmt.Errorf("server error %d", statusCode)
            continue
        }
        // ...
    }
}
```

## WebSocket com Single-Writer

```go
type WSClient struct {
    writeMu    sync.Mutex // Single writer pattern
    conn       *websocket.Conn
    // ...
}

// Todos os writes passam pelo mutex
func (ws *WSClient) sendSubscribe(ids []string) error {
    ws.writeMu.Lock()
    defer ws.writeMu.Unlock()
    return conn.WriteJSON(msg)
}
```

## Status Normalization Table-Driven

```go
// Normalização por tipo de swap
func Normalize(status string, kind swap.Kind) StatusMapping {
    switch kind {
    case swap.KindSubmarine:
        return NormalizeSubmarine(status)
    case swap.KindReverse:
        return NormalizeReverse(status)
    case swap.KindChain:
        return NormalizeChain(status)
    }
}

// Submarine: trigger correto para assinatura
case StatusTxClaimPending:
    return StatusMapping{
        State:   swap.StateSigningMusig2Partial,
        Action:  ActionSign,
        Trigger: "boltz:claim_pending",
    }
```

## MinerFees Polimórfico

```go
// Lida com number OU object
type MinerFeesAny struct {
    Flat   *int64      // Quando é número
    Detail *MinerFees  // Quando é objeto
}

func (m *MinerFeesAny) UnmarshalJSON(b []byte) error {
    var n int64
    if err := json.Unmarshal(b, &n); err == nil {
        m.Flat = &n
        return nil
    }
    var obj MinerFees
    if err := json.Unmarshal(b, &obj); err != nil {
        return err
    }
    m.Detail = &obj
    return nil
}
```

\newpage

# Decisões Técnicas

## 1. Go Core ao invés de Node.js

**Decisão**: Implementar backend em Go com comunicação gRPC

**Justificativa:**
- Performance superior para operações criptográficas
- Cross-platform nativo (Desktop + Mobile via gRPC)
- Melhor gerenciamento de concorrência
- Type safety em tempo de compilação

## 2. boltz-backend Self-Hosted

**Decisão**: Usar boltz-backend com nossa liquidez ao invés da API pública

**Justificativa:**
- Controle total sobre liquidez
- Redução de dependências externas
- Menor risco operacional
- Compatibilidade com API v2

## 3. Duas Interfaces RPC Primárias

**Decisão**: LND (gRPC) + elementsd (JSON-RPC), bitcoind como runtime

**Justificativa:**
- LND já faz manejo on-chain e Lightning
- Simplifica modelo de integração
- Menos pontos de falha
- Menor superfície de ataque

## 4. SQLite com Modo WAL

**Decisão**: SQLite como banco de dados embarcado

**Justificativa:**
- Zero configuração
- Arquivo único (fácil backup)
- WAL para concorrência
- Confiabilidade comprovada

## 5. Taproot + MuSig2

**Decisão**: Swaps Taproot com assinatura cooperativa MuSig2

**Justificativa:**
- Key-path spends são menores e mais baratos
- Privacidade melhorada
- Fallback script-path disponível
- Future-proof

\newpage

# Stack Tecnológico

## Tecnologias Core

| Componente | Tecnologia | Versão |
|------------|------------|--------|
| Backend | **Go** | 1.25+ |
| Comunicação | **gRPC** | 1.60+ |
| Frontend | React | 18 |
| Desktop | Electron | 28+ |
| Banco de Dados | SQLite | 3.31+ |
| Build Tool | Vite | 5 |

## Bibliotecas Go

| Biblioteca | Propósito |
|------------|-----------|
| google.golang.org/grpc | Comunicação gRPC |
| github.com/mattn/go-sqlite3 | Banco de dados |
| github.com/gorilla/websocket | WebSocket client |
| github.com/btcsuite/btcd | Bitcoin primitives |
| golang.org/x/crypto | Argon2id |

\newpage

# Roadmap de Implementação

## Sprint Atual: Spike boltz-backend

**Objetivo**: Mapear "Responsibility Split: Wallet vs boltz-backend"

- [ ] Setup boltz-backend docker (regtest)
- [ ] Conectar LND (gRPC)
- [ ] Conectar elementsd (JSON-RPC)
- [ ] Testar 3 tipos de swap
- [ ] Documentar responsibility split

## Fase 1: Boltz Client ✅ CONCLUÍDA

- [x] HTTP client com retry/backoff
- [x] WebSocket single-writer + gap-recovery
- [x] Status normalization table-driven
- [x] MinerFees polimórfico
- [x] Testes unitários

## Fase 2: Adapters (após Spike)

- [ ] LND Adapter (gRPC) - Interface Primária
- [ ] Liquid Adapter (JSON-RPC) - Interface Primária
- [ ] Bitcoin Adapter (runtime monitoring)

## Fase 3: Wiring e E2E

- [ ] Conectar xscore → boltz-backend
- [ ] E2E tests em regtest
- [ ] Crash recovery tests

## Fase 4: Frontend

- [ ] Migrar UI do BRLN devdash
- [ ] Integrar Electron
- [ ] Trocar API REST por gRPC

## Roadmap Pós-MVP

### v1.1 (Q2 2026)
- Hardware wallet (Ledger/Trezor)
- SQLCipher encryption
- Multi-wallet

### v1.2 (Q3 2026)
- Coinjoin integration
- Payjoin support
- LNURL

### v2.0 (Q4 2026)
- **EVM Module** (MetaMask + LI.FI)
- RGB Assets
- Taproot Assets

\newpage

# Considerações de Segurança

## Modelo de Ameaças

### Ameaças no Escopo
1. **Roubo de Seed**: Atacante ganha acesso ao banco criptografado
2. **Brute Force de PIN**: Atacante tenta adivinhar PIN
3. **Manipulação de Swap**: Atacante tenta roubar fundos durante swap
4. **Supply Chain**: Binários maliciosos

## Mitigações

### 1. Proteção de Seed
- Argon2id + AES-256-GCM
- Zeroize dados sensíveis após uso
- Criptografado em repouso

### 2. Segurança de PIN
- Rate limiting exponencial
- Máximo 10 tentativas
- Argon2id (64MB, 3 iterações)

### 3. Atomicidade de Swap
- Scripts HTLC com verificação local
- Timeouts escalonados
- Fallback script-path

### 4. State Machine Autoritativa
- CAS (Compare-And-Swap) otimista
- Idempotência via swap_ops
- Reservas de UTXO e LN
- Crash recovery automático

---

**Versão do Documento**: 0.2.0
**Última Atualização**: 20 de Janeiro de 2026
**Status**: Fase 1 Concluída (~40%)
