---
title: "XS Wallet - Documentação Técnica"
subtitle: "Carteira HD + Atomic Swaps - Aplicação Desktop"
author: "Equipe de Desenvolvimento XS Wallet"
date: "Janeiro de 2026"
version: "0.1.0"
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

**XS Wallet** é uma aplicação desktop self-custody que permite atomic swaps entre Bitcoin, Liquid e Lightning Network usando Taproot e a API Boltz como provedor de liquidez. O sistema mantém verdadeira auto-custódia enquanto fornece swaps cross-chain e cross-layer sem exigir que usuários operem sua própria infraestrutura de swap.

## Características Principais

- **Carteira HD**: Compatível com BIP39/32/84/85 com derivação determinística de chaves
- **Suporte Multi-Chain**: Bitcoin (on-chain), Liquid (sidechain), Lightning Network
- **Atomic Swaps**: Swaps Submarine, Reverse e Chain via Boltz API v2 (REST + WebSocket) com reconstrução local de scripts/taptree e enforcement de claim/refund via script-path
- **Integração Taproot**: Assinatura cooperativa MuSig2 com fallback script-path
- **Auto-Custódia**: Usuário controla todas as chaves privadas, criptografadas em repouso
- **Aplicação Desktop**: Baseada em Electron com nodes embarcados (bitcoind, elementsd, LND)

## Status do Projeto

- **Fase**: Fundação do storage/infra do engine completa; implementação do vault/adapters/engine pendente (15%)
- **Cronograma**: 5 semanas até MVP
- **Arquitetura**: Camada de banco de dados production-ready, gerenciador de nodes e decisões técnicas finalizadas

\newpage

# Arquitetura do Sistema

## Visão Geral de Alto Nível

```
┌─────────────────────────────────────────────────────────────┐
│                  Aplicação Desktop XS Wallet                 │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐         ┌──────────────────────────────┐  │
│  │   Electron   │         │  Processo Backend (Node.js)   │  │
│  │Processo Main │◄───────►│                               │  │
│  │              │   IPC   │  ┌────────────────────────┐   │  │
│  │  • Janela    │         │  │   Swap Engine          │   │  │
│  │  • Updater   │         │  │   • Máquina de Estados │   │  │
│  │  • Node Mgr  │         │  │   • CAS/Transações     │   │  │
│  └──────────────┘         │  │   • Watchdog/Recovery  │   │  │
│                            │  └────────────────────────┘   │  │
│  ┌──────────────┐         │                               │  │
│  │   React UI   │         │  ┌────────────────────────┐   │  │
│  │  (Renderer)  │◄───────►│  │  Banco de Dados (SQLite)  │  │
│  │              │   IPC   │  │   • Modo WAL           │   │  │
│  │  • Dashboard │         │  │   • CAS/Version        │   │  │
│  │  • Swap UI   │         │  │   • Event Log          │   │  │
│  │  • Settings  │         │  └────────────────────────┘   │  │
│  └──────────────┘         │                               │  │
│                            │  ┌────────────────────────┐   │  │
│                            │  │   Adaptadores          │   │  │
│                            │  │   • BTC (bitcoind)     │   │  │
│                            │  │   • Liquid (elementsd) │   │  │
│                            │  │   • LN (LND)           │   │  │
│                            │  └────────────────────────┘   │  │
│                            └──────────────────────────────┘  │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │        Nodes Embarcados (Download Verificável)        │   │
│  │  • bitcoind v26.0  • elementsd v23.2.1  • LND v0.17.3│   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │   API Boltz v2   │
                    │(Provedor de Swap)│
                    └──────────────────┘
```

## Detalhamento dos Componentes

### 1. Processo Main do Electron
- **Gerenciamento de Janelas**: Cria e gerencia janelas da aplicação
- **Auto-Updater**: Gerencia atualizações via electron-updater
- **Gerenciador de Nodes**: Gerenciamento de ciclo de vida dos nodes Bitcoin/Liquid/LND
- **Ponte IPC**: Comunicação segura entre renderer e backend

### 2. Processo Backend (Processo Node.js Separado)
- **Swap Engine**: Máquina de estados core para atomic swaps
- **Camada de Banco de Dados**: SQLite com modo WAL para acesso concorrente
- **Adaptadores de Chain**: Clientes RPC para bitcoind, elementsd e LND
- **Cliente Boltz**: Integração REST + WebSocket com API Boltz v2
- **Key Vault**: Implementação BIP39/32/84/85 com criptografia

### 3. Frontend React (Processo Renderer)
- **Onboarding**: Fluxos de criação e restauração de carteira
- **Dashboard**: Exibição de saldo e histórico de transações
- **Interface de Swap**: Quote, confirmação e rastreamento de status
- **Configurações**: Seleção de rede, status de nodes, gerenciamento de backup

### 4. Nodes Embarcados
- **bitcoind**: Node Bitcoin completo para operações on-chain
- **elementsd**: Node Liquid/Elements para operações sidechain
- **LND**: Daemon Lightning Network para canais de pagamento
- **Estratégia**: Download verificável no primeiro uso (reduz tamanho do instalador)
- **Lifecycle**: Node Manager cria datadirs isolados por rede (regtest/testnet/mainnet), executa healthchecks RPC/gRPC, e implementa restart com backoff exponencial em caso de falha
- **Compatibilidade**: Detecção de versão incompatível de chainstate/wallets ao atualizar nodes, com migração automática quando possível

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

**Características-Chave**:
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

**Propósito**: Capacidade completa de replay/debug para ciclo de vida do swap

#### 3. `swap_ops` - Ledger de Idempotência
```sql
CREATE TABLE swap_ops (
  swap_id     TEXT REFERENCES swaps(id),
  op_key      TEXT NOT NULL,
  status      TEXT CHECK(status IN ('inflight','ok','fail')),
  heartbeat_at TEXT,  -- Detecção de operação stale
  PRIMARY KEY (swap_id, op_key)
);
```

**Propósito**: Previne operações duplicadas (ex: double broadcast)

#### 4. `utxo_reservations` - Anti Double-Spend
```sql
CREATE TABLE utxo_reservations (
  chain       TEXT CHECK(chain IN ('btc','liquid')),
  txid        TEXT NOT NULL,
  vout        INTEGER NOT NULL CHECK(vout >= 0),
  swap_id     TEXT REFERENCES swaps(id),
  PRIMARY KEY (chain, txid, vout)
);
```

**Propósito**: Previne mesmo UTXO de ser usado em múltiplos swaps

#### 5. `ln_reservations` - Anti Duplicação de Pagamento
```sql
CREATE TABLE ln_reservations (
  payment_hash_hex TEXT PRIMARY KEY CHECK(length(payment_hash_hex)=64),
  swap_id          TEXT REFERENCES swaps(id),
  direction        TEXT CHECK(direction IN ('pay','receive'))
);
```

**Propósito**: Previne pagamentos Lightning duplicados

#### 6. `app_config` - Armazenamento de Configuração
```sql
CREATE TABLE app_config (
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL,  -- JSON
  updated_at  TEXT NOT NULL
);
```

**Propósito**: Configurações editáveis pelo usuário (rede, provider, timeouts)

## Pragmas Críticos do SQLite

```sql
PRAGMA journal_mode = WAL;        -- Write-Ahead Logging (concorrência)
PRAGMA synchronous = NORMAL;      -- Balanço segurança/performance
PRAGMA busy_timeout = 5000;       -- Espera 5s em lock
PRAGMA foreign_keys = ON;         -- Força integridade referencial
PRAGMA temp_store = MEMORY;       -- Tabelas temp em RAM
PRAGMA cache_size = -20000;       -- Cache ~20MB
```

**Justificativa**: Essas configurações otimizam SQLite para uso desktop com leituras/escritas concorrentes mantendo integridade de dados.

\newpage

# Fluxos de Atomic Swap

## Submarine Swap (Liquid → Lightning)

**Caso de Uso**: Usuário quer converter L-BTC para capacidade Lightning

```
┌──────┐                 ┌───────┐                ┌─────────┐
│Usuário│                │ Boltz │                │ Liquid  │
└──┬───┘                 └───┬───┘                └────┬────┘
   │                         │                         │
   │ 1. Criar invoice LN     │                         │
   ├────────────────────────►│                         │
   │                         │                         │
   │ 2. Endereço HTLC + tree │                         │
   │◄────────────────────────┤                         │
   │                         │                         │
   │ 3. Verificar end. P2TR  │                         │
   │    (rebuild swapTree)   │                         │
   │                         │                         │
   │ 4. Financiar HTLC(L-BTC)│                         │
   ├─────────────────────────┼────────────────────────►│
   │                         │                         │
   │                         │ 5. Pagar invoice LN     │
   │                         │    (recebe preimage R)  │
   │                         │                         │
   │                         │ 6. Claim HTLC (revela R)│
   │                         ├────────────────────────►│
   │                         │                         │
   │ 7. Notificação sucesso  │                         │
   │◄────────────────────────┤                         │
```

**Segurança**: Usuário verifica script HTLC e endereço P2TR antes de financiar

## Reverse Swap (Lightning → Liquid)

**Caso de Uso**: Usuário quer converter saldo Lightning para L-BTC

```
┌──────┐                 ┌───────┐                ┌─────────┐
│Usuário│                │ Boltz │                │ Liquid  │
└──┬───┘                 └───┬───┘                └────┬────┘
   │                         │                         │
   │ 1. Gerar R, H=SHA256(R) │                         │
   │                         │                         │
   │ 2. Solicitar reverse    │                         │
   ├────────────────────────►│                         │
   │    (enviar H)           │                         │
   │                         │                         │
   │ 3. Hold invoice + HTLC  │                         │
   │◄────────────────────────┤                         │
   │                         │                         │
   │ 4. Verificar hash inv.  │                         │
   │    (hash == H)          │                         │
   │                         │                         │
   │ 5. Pagar hold invoice   │                         │
   ├────────────────────────►│                         │
   │                         │                         │
   │                         │ 6. Financiar HTLC(L-BTC)│
   │                         ├────────────────────────►│
   │                         │                         │
   │ 7. Claim HTLC (revela R)│                         │
   ├─────────────────────────┼────────────────────────►│
   │                         │                         │
   │                         │ 8. Liquidar invoice (R) │
   │                         │◄────────────────────────┤
```

**Segurança**: Usuário gera preimage R, garantindo controle sobre claim

## Chain Swap (BTC ↔ Liquid)

**Caso de Uso**: Swap atômico entre mainchain Bitcoin e sidechain Liquid

```
┌──────┐                 ┌───────┐      ┌─────┐  ┌────────┐
│Usuário│                │ Boltz │      │ BTC │  │ Liquid │
└──┬───┘                 └───┬───┘      └──┬──┘  └───┬────┘
   │                         │              │         │
   │ 1. Gerar R, H=SHA256(R) │              │         │
   │                         │              │         │
   │ 2. Solicitar chain swap │              │         │
   ├────────────────────────►│              │         │
   │    (BTC→Liquid, enviar H)              │         │
   │                         │              │         │
   │ 3. Endereços HTLC(ambos)│              │         │
   │◄────────────────────────┤              │         │
   │                         │              │         │
   │ 4. Verificar ambos HTLCs│              │         │
   │    (scripts + timeouts) │              │         │
   │                         │              │         │
   │ 5. Financiar HTLC BTC   │              │         │
   ├─────────────────────────┼─────────────►│         │
   │                         │              │         │
   │                         │ 6. Financiar HTLC Liquid│
   │                         ├──────────────┼────────►│
   │                         │              │         │
   │ 7. Claim Liquid(revela R)              │         │
   ├─────────────────────────┼──────────────┼────────►│
   │                         │              │         │
   │                         │ 8. Claim BTC (usa R)   │
   │                         ├─────────────►│         │
```

**Segurança**: Timeouts escalonados (T_liquid < T_btc) para prevenir race conditions

\newpage

# Decisões Técnicas

## 1. SQLite com Modo WAL

**Decisão**: Usar SQLite ao invés de PostgreSQL para app desktop embarcado

**Justificativa**:
- Não requer servidor de banco de dados externo
- Banco de dados em arquivo único (fácil backup/restore)
- Modo WAL fornece excelente concorrência para uso desktop
- Confiabilidade comprovada em produção (navegadores, apps mobile)

**Implementação**:
- `journal_mode=WAL` para leituras concorrentes durante escritas
- `busy_timeout=5000ms` para lidar com contenção de lock
- Locking otimista via campo `version` para operações CAS

## 2. Separação de Processo Backend

**Decisão**: Executar backend como processo Node.js filho separado

**Justificativa**:
- **Isolamento de Crash**: Crash da UI não mata swap engine ou nodes
- **Gerenciamento de Recursos**: Melhor controle sobre alocação CPU/memória
- **IPC Limpo**: Fronteira de comunicação bem definida
- **Debugging**: Mais fácil debugar backend independentemente

**Implementação**:
- Processo main spawna backend no startup
- IPC via canais nativos do Electron ou localhost loopback
- Backend continua rodando mesmo se UI reiniciar

## 3. Downloads Verificáveis de Binários

**Decisão**: Baixar binários de nodes no primeiro uso ao invés de bundlar no instalador

**Justificativa**:
- **Tamanho do Instalador**: ~50MB vs ~500MB (redução de 10x)
- **Velocidade de Notarização**: Notarização macOS mais rápida
- **Updates Independentes**: App e nodes podem atualizar separadamente
- **Segurança**: Verificação de checksum SHA256 + assinaturas GPG opcionais

**Implementação**:
```typescript
const NODE_BINARIES = {
  bitcoind: {
    version: '26.0',
    downloadUrl: 'https://bitcoincore.org/...',
    checksum: 'sha256...'
  }
};
```

**Raiz de Confiança**: Os checksums são obtidos de um manifest assinado; a aplicação valida a assinatura com chave pública fixa (pinned) embutida no código. Sem servidor central de custódia/execução de swaps; apenas endpoints públicos de distribuição (GitHub Releases) para updates e binários verificados.

## 4. Integração com API Boltz

**Decisão**: Usar Boltz como provedor de swap ao invés de operar infraestrutura própria

**Justificativa**:
- **MVP Mais Rápido**: Não precisa gerenciar liquidez ou hold invoices
- **Segurança Atômica**: Scripts HTLC garantem atomicidade independente do provedor
- **Verificação**: "Don't trust, verify" - rebuild scripts localmente
- **Fallback**: Script-path permite claim/refund mesmo se Boltz offline

**Trade-offs**:
- **Dependência**: Depende de disponibilidade Boltz (mitigado por fallback paths)
- **Privacidade**: Provedor vê metadados do swap (aceitável para MVP)
- **Taxas**: Taxas do provedor (competitivas com custos self-hosted)

## 5. Taproot + MuSig2

**Decisão**: Usar swaps Taproot com assinatura cooperativa MuSig2

**Justificativa**:
- **Eficiência**: Key-path spends são menores e mais baratos
- **Privacidade**: Spends cooperativos parecem pagamentos regulares
- **Fallback**: Script-path disponível quando cooperação falha
- **Future-Proof**: Taproot é o upgrade mais recente do Bitcoin

**Implementação**:
- Happy path: Assinatura agregada MuSig2 com ordem determinística (pubkey provider primeiro, depois user)
- Persistência de sessão: Nonces e contexto MuSig2 persistidos em `musig_*` fields para recovery após restart
- Fallback: Script-path com condições HTLC quando cooperação falha
- Geração determinística de nonce para segurança

## 6. Criptografia Baseada em PIN

**Decisão**: Derivação de chave Argon2id a partir de PIN + criptografia AES-256-GCM

**Justificativa**:
- **User-Friendly**: PIN é mais fácil que senha para app desktop
- **Seguro**: Argon2id é memory-hard (resistente a ataques GPU)
- **Rate-Limited**: Backoff previne brute force
- **Melhoria Opcional**: OS keychain para device secret (v1.1)

**Parâmetros**:
```typescript
{
  algorithm: 'argon2id',
  memory: 65536,      // 64MB
  iterations: 3,
  parallelism: 1,
  saltLength: 16
}
```

**Proteção Contra Ataque Offline**: Rate limiting funciona em runtime, mas não protege contra ataque offline ao arquivo de banco de dados. Para endurecer, a chave final é derivada de PIN + device secret armazenado no keychain do OS (Keychain/DPAPI/SecretService quando disponível). MVP usa apenas PIN + Argon2id; device secret é implementado em v1.1.

## 7. Chaves de Swap Determinísticas (BIP85)

**Decisão**: Usar child seed BIP85 para derivação de chaves de swap

**Justificativa**:
- **Restore**: Pode recuperar swaps pendentes apenas do mnemônico
- **Separação**: Chaves de swap isoladas das chaves da carteira
- **Preimages Determinísticas**: `R = SHA256(privKey(index))`

**Implementação**:
```
Master Seed (BIP39)
  └─ BIP85 Child Seed (subtree de swap)
      └─ m/0/0, m/0/1, ... (chaves de swap por índice)
          └─ SHA256(privKey) = preimage R
```

## 8. Gerenciamento de Configuração

**Decisão**: Armazenar config na tabela SQLite `app_config`, snapshot em swaps

**Justificativa**:
- **Sem Hardcoding**: Todas configurações editáveis pelo usuário
- **Reprodutibilidade**: Cada swap carrega snapshot de config em `locked_intent`
- **Auditabilidade**: Pode rastrear parâmetros exatos usados em qualquer swap

**Config Padrão**:
```json
{
  "network": "regtest",
  "provider_url": "https://api.boltz.exchange",
  "kdf_params": {"algorithm":"argon2id",...}
}
```

\newpage

# Roadmap de Implementação

## Fase 1: Backend Core (2 semanas)

### Semana 1
- [x] Schema do banco de dados (SQLite)
- [x] Camada DB com CAS/transações
- [x] Gerenciador de nodes com downloads verificáveis
- [ ] Key Vault (BIP39/32/84/85)
- [ ] Criptografia do Vault (Argon2id + AES-GCM)
- [ ] Adaptador BTC (bitcoind RPC)

### Semana 2
- [ ] Adaptador Liquid (elementsd RPC)
- [ ] Adaptador LN (LND gRPC)
- [ ] Cliente Boltz (REST + WebSocket)
- [ ] Implementação MuSig2
- [ ] Swap Engine (máquina de estados)
- [ ] Watchdog + recovery

## Fase 2: Frontend (1.5 semanas)

### Semana 3
- [ ] Setup React + Vite
- [ ] Fluxo de onboarding (criar/restaurar carteira)
- [ ] Dashboard (saldos + histórico)
- [ ] Interface de swap (quote + confirmação)

### Semana 4 (primeira metade)
- [ ] Rastreamento de status (updates WebSocket)
- [ ] Configurações (rede, nodes, backup)
- [ ] Tratamento de erros + polish UX

## Fase 3: Integração Electron (1 semana)

### Semana 4 (segunda metade)
- [ ] Setup do processo main
- [ ] Processo filho backend
- [ ] Handlers IPC (seguro)
- [ ] Integração de ciclo de vida dos nodes

### Semana 5 (primeira metade)
- [ ] Auto-updater
- [ ] Deep links (xs-wallet://)
- [ ] Logging + diagnósticos

## Fase 4: Packaging & Testes (0.5 semana)

### Semana 5 (segunda metade)
- [ ] Config electron-builder
- [ ] Instaladores (Windows, macOS, Linux)
- [ ] Code signing
- [ ] Testes de integração (Boltz regtest)
- [ ] Testes E2E (Electron)

## Roadmap Pós-MVP

### v1.1 (Q2 2026)
- Suporte a hardware wallet (Ledger/Trezor)
- Criptografia de banco de dados SQLCipher
- Suporte multi-wallet
- App mobile (React Native)

### v1.2 (Q3 2026)
- Integração Coinjoin
- Suporte Payjoin
- LNURL

### v2.0 (Q4 2026)
- Assets RGB
- Taproot Assets
- Suporte DLC

\newpage

# Considerações de Segurança

## Modelo de Ameaças

### Ameaças no Escopo
1. **Roubo de Seed**: Atacante ganha acesso ao banco de dados criptografado
2. **Brute Force de PIN**: Atacante tenta adivinhar PIN
3. **Manipulação de Swap**: Atacante tenta roubar fundos durante swap
4. **Supply Chain**: Binários maliciosos de nodes ou atualizações de app

### Fora do Escopo (Trabalho Futuro)
1. **Acesso Físico**: Atacante com acesso físico ao dispositivo
2. **Comprometimento de OS**: Malware com privilégios root/admin
3. **Ataques de Rede**: Man-in-the-middle (mitigado por TLS)

## Mitigações

### 1. Proteção de Seed
- **Criptografia**: Argon2id + AES-256-GCM
- **Memória**: Zeroize dados sensíveis após uso
- **Armazenamento**: Criptografado em repouso, nunca em plaintext

### 2. Segurança de PIN
- **Rate Limiting**: Backoff exponencial (1s → 2s → 4s → 8s...)
- **Tentativas Máximas**: Bloqueia após 10 tentativas falhas (cooldown 1 hora)
- **KDF**: Argon2id (64MB memória, 3 iterações)

### 3. Atomicidade de Swap
- **Scripts HTLC**: Garantias criptográficas via hash locks
- **Verificação**: Rebuild scripts localmente, nunca confiar no provedor
- **Timeouts**: Escalonados para prevenir race conditions
- **Fallback**: Script-path claim/refund se cooperação falhar

### 4. Supply Chain
- **Atualizações de App**: Code signing (Windows/macOS)
- **Binários de Nodes**: Verificação de checksum SHA256
- **Manifest**: Manifest assinado para registry de binários
- **Rollback**: Rollback automático em falha de atualização

\newpage

# Stack Tecnológico

## Tecnologias Core

| Componente | Tecnologia | Versão | Propósito |
|-----------|-----------|---------|---------|
| Framework Desktop | Electron | 28+ | App desktop cross-platform |
| Frontend | React | 18 | Framework UI |
| Build Tool | Vite | 5 | Dev server rápido + bundler |
| Linguagem | TypeScript | 5.3 | Desenvolvimento type-safe |
| Banco de Dados | SQLite | 3.31+ | Banco embarcado |
| Biblioteca DB | better-sqlite3 | 9.2 | Bindings SQLite síncronos |
| Estilização | TailwindCSS | 3 | CSS utility-first |
| Estado | Zustand | 4 | Gerenciamento de estado leve |

## Bibliotecas Bitcoin

| Biblioteca | Versão | Propósito |
|---------|---------|---------|
| bitcoinjs-lib + libs Taproot | 6.1 | Construção de transações Bitcoin + Taproot/MuSig2 |
| bip32 | 4.0 | Derivação de chaves HD |
| bip39 | 3.1 | Geração de mnemônico |
| bolt11 | 1.4 | Parsing de invoice Lightning |

**Nota sobre Swaps LN↔BTC**: O MVP suporta Submarine (Liquid→LN) e Reverse (LN→Liquid). Swaps diretos LN↔BTC (sem Liquid) são suportados via Submarine/Reverse na chain BTC se o par estiver disponível na API Boltz v2. Chain Swap (BTC↔Liquid) é implementado como swap atômico cross-chain.

## Criptografia

| Biblioteca | Versão | Propósito |
|---------|---------|---------|
| argon2 | 0.31 | Password hashing (KDF) |
| crypto (Node.js) | Built-in | Criptografia AES-GCM |

## Comunicação com Nodes

| Biblioteca | Versão | Propósito |
|---------|---------|---------|
| @grpc/grpc-js | 1.9 | Cliente gRPC LND |
| axios | 1.6 | Cliente HTTP (API Boltz) |
| ws | 8.16 | Cliente WebSocket |

\newpage

# Status Atual do Projeto

## Arquivos Implementados

### 1. Database Layer
- **`database/schema.sqlite.sql`**: Schema completo production-ready
  - 6 tabelas (swaps, events, ops, utxo_res, ln_res, config)
  - Pragmas otimizados (WAL, busy_timeout, cache)
  - Guards de integridade (CHECK constraints)
  - Timestamps ISO-8601 UTC

- **`src/swap/db.sqlite.ts`**: Camada de acesso a dados
  - better-sqlite3 (síncrono)
  - CAS com updated_at controlado
  - Transações para transitionSwapState
  - Stale op reclaim via SQL interval
  - Helpers de normalização (txid, hash, pubkey)

### 2. Node Management
- **`src/nodes/manager.ts`**: Gerenciador de ciclo de vida
  - Start/stop/status de nodes
  - Downloads verificáveis (SHA256 checksum)
  - Registry de binários versionado
  - Spawn + logging

### 3. Documentação
- **`docs/TECHNICAL_DECISIONS.md`**: 8 decisões críticas
- **`docs/XS_Wallet_Documentacao_Tecnica_PT.md`**: Documentação completa
- **`PROJECT_STATUS.md`**: Status e estrutura do projeto

### 4. Configuração
- **`package.json`**: Dependencies + scripts + electron-builder config

## Próximos Passos

1. **tsconfig.json** (main + renderer)
2. **Key Vault** (BIP39/32/84/85 + encryption)
3. **Chain Adapters** (BTC, Liquid, LN)
4. **Boltz Client** (API v2 + MuSig2)
5. **Swap Engine** (state machine)

## Progresso

**15% Completo** - Fundação sólida estabelecida

---

**Versão do Documento**: 0.1.0  
**Última Atualização**: 19 de Janeiro de 2026  
**Status**: Fundação Completa (15%)
