# DevDash - Especificação de Interface XS Wallet

## 1. Visão Geral

DevDash é a interface principal do XS Wallet, uma aplicação desktop self-custody para atomic swaps entre BTC, Liquid e Lightning.

### Princípios de Design

| Princípio | Descrição |
|-----------|-----------|
| **Zero Mock** | Todos os dados vêm do backend real |
| **State-Driven** | UI reflete estado autoritativo do xscore |
| **Fail-Safe** | Erros são explícitos, nunca silenciosos |
| **Progressive Disclosure** | Complexidade revelada conforme necessário |

---

## 2. Arquitetura de Telas

```
┌─────────────────────────────────────────────────────────┐
│                    App Shell                             │
├──────────┬──────────────────────────────────────────────┤
│          │                                               │
│ Sidebar  │              Content Area                     │
│          │                                               │
│ • Home   │  ┌─────────────────────────────────────────┐ │
│ • Swap   │  │                                         │ │
│ • Wallet │  │         Active Page                     │ │
│ • Network│  │                                         │ │
│ • Settings│ │                                         │ │
│          │  └─────────────────────────────────────────┘ │
├──────────┴──────────────────────────────────────────────┤
│                    Status Bar                            │
│  [Vault: 🔓 Unlocked] [BTC: 150 blocks] [gRPC: ✓]       │
└─────────────────────────────────────────────────────────┘
```

---

## 3. Fluxos de Usuário

### 3.1 Onboarding (Primeiro Uso)

```
Welcome Screen
     │
     ├── [Create Wallet] ──▶ Generate Mnemonic (24 words)
     │                              │
     │                              ▼
     │                       Confirm Words (verificar 3 random)
     │                              │
     │                              ▼
     │                       Set PIN (6 dígitos)
     │                              │
     │                              ▼
     │                       Success ──▶ Dashboard
     │
     └── [Restore Wallet] ──▶ Input Mnemonic (24 words)
                                    │
                                    ▼
                             Validate BIP39
                                    │
                                    ▼
                             Set PIN ──▶ Dashboard
```

### 3.2 Unlock Flow (Uso Normal)

```
App Launch
     │
     ▼
Vault Status Check (GetVaultStatus)
     │
     ├── NOT_INITIALIZED ──▶ Onboarding
     │
     ├── LOCKED ──▶ Unlock Screen
     │                   │
     │                   ▼
     │              PIN Input
     │                   │
     │                   ├── Valid ──▶ Dashboard
     │                   │
     │                   └── Invalid ──▶ Show error
     │                                   │
     │                                   ▼
     │                              Retry (max 10)
     │                                   │
     │                                   └── Lockout
     │
     └── UNLOCKED ──▶ Dashboard
```

### 3.3 Swap Flow (Submarine: BTC → LN)

```
Swap Center
     │
     ▼
Select Direction: [BTC] ──▶ [LN]
     │
     ▼
Enter Amount (sats)
     │
     ▼
[Get Quote] ──▶ QuoteSwap gRPC
     │
     ▼
Display Quote:
  • You send: 100,000 sats
  • You receive: ~99,500 sats  
  • Fee: 500 sats (0.5%)
  • Expires in: 10 minutes
     │
     ├── [Cancel]
     │
     └── [Accept] ──▶ AcceptQuote gRPC
                           │
                           ▼
                    State: OPEN
                           │
                           ▼
                    [Lock Quote]
                           │
                           ▼
                    State: LOCKED
                    Display HTLC Address + Amount
                           │
                           ▼
                    Wait for funding...
                           │
                           ▼
                    State: COMMIT_STARTED
                           │
                           ▼
                    State: WAITING
                           │
                           ▼
                    (Auto progression...)
                           │
                           ▼
                    State: COMPLETED ✓
```

---

## 4. Telas Detalhadas

### 4.1 Dashboard (Home)

```
┌────────────────────────────────────────────────────────┐
│  Welcome, User                              [Lock 🔒]  │
├────────────────────────────────────────────────────────┤
│                                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │   BTC    │  │  Liquid  │  │Lightning │             │
│  │ 0.00123  │  │ 0.00456  │  │ 50,000 ⚡│             │
│  │ $45.67   │  │ $167.89  │  │ sats     │             │
│  └──────────┘  └──────────┘  └──────────┘             │
│                                                        │
│  ═══════════════════════════════════════════           │
│                                                        │
│  Active Swaps (2)                                      │
│  ┌────────────────────────────────────────────────┐   │
│  │ #abc123  BTC→LN  100k sats  [AWAITING PAYMENT] │   │
│  │ #def456  LN→BTC  50k sats   [PROCESSING]       │   │
│  └────────────────────────────────────────────────┘   │
│                                                        │
│  Quick Actions                                         │
│  [+ New Swap]  [Receive]  [Send]                      │
│                                                        │
└────────────────────────────────────────────────────────┘
```

**Dados necessários:**
- `GET /wallet/balances` → BTC, Liquid, LN
- `GET /swaps?state=active` → Lista de swaps ativos
- `GET /vault/status` → Estado do vault

---

### 4.2 Swap Center

```
┌────────────────────────────────────────────────────────┐
│  Atomic Swap                                           │
├────────────────────────────────────────────────────────┤
│                                                        │
│  From                              To                  │
│  ┌──────────────┐    ⇄    ┌──────────────┐            │
│  │ [BTC ▼]      │         │ [Lightning ▼]│            │
│  │              │         │              │            │
│  │ 100,000 sats │         │ ~99,500 sats │            │
│  └──────────────┘         └──────────────┘            │
│                                                        │
│  Quote Details                                         │
│  ├─ Service Fee: 0.5% (500 sats)                      │
│  ├─ Network Fee: ~200 sats                            │
│  ├─ Rate: 1:0.995                                     │
│  └─ Expires: 9:45 remaining                           │
│                                                        │
│  [Cancel]                        [Accept Quote →]      │
│                                                        │
├────────────────────────────────────────────────────────┤
│  Swap History                                          │
│  ┌────────────────────────────────────────────────┐   │
│  │ ID        Type    Amount     State      Date   │   │
│  │ abc123    BTC→LN  100k       COMPLETED  Today  │   │
│  │ def456    LN→BTC  50k        REFUNDED   Jan 19 │   │
│  └────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────┘
```

---

### 4.3 State Machine (14 Estados)

| Estado | Badge | Cor | Ícone | Ações |
|--------|-------|-----|-------|-------|
| OPEN | Created | Gray | ○ | Lock, Cancel |
| LOCKED | Awaiting Payment | Yellow | ⏳ | Show Address, Cancel |
| COMMIT_STARTED | Broadcasting | Yellow | 📡 | - |
| WAITING | Processing | Blue | ⏳ | - |
| WAITING_CLAIM_DETAILS | Claiming | Green | ⚡ | - |
| SIGNING_MUSIG2_PARTIAL | Signing | Green | ✍️ | - |
| SENT_PARTIAL_TO_PROVIDER | Finalizing | Green | 📤 | - |
| WAITING_PROVIDER_BROADCAST | Confirming | Green | 📻 | - |
| REFUND_COOP_WAITING | Refunding | Orange | 🔄 | - |
| FALLBACK_SCRIPT_READY | Refund Ready | Orange | ⚠️ | Refund |
| REFUNDING | Refunding | Orange | 🔄 | - |
| COMPLETED | Success | Green | ✓ | View Details |
| FAILED | Failed | Red | ✗ | View Error |
| CANCELED | Canceled | Gray | ⊘ | - |

---

### 4.4 Wallet Page

```
┌────────────────────────────────────────────────────────┐
│  Wallet                                                │
├────────────────────────────────────────────────────────┤
│                                                        │
│  [BTC]  [Liquid]  [Lightning]                         │
│  ═══════════════════════════                          │
│                                                        │
│  Balance: 0.00123456 BTC ($45.67)                     │
│                                                        │
│  Addresses                              [+ New]        │
│  ┌────────────────────────────────────────────────┐   │
│  │ bc1q...abc   m/84'/0'/0'/0/0   0.00100000 BTC │   │
│  │ bc1q...def   m/84'/0'/0'/0/1   0.00023456 BTC │   │
│  └────────────────────────────────────────────────┘   │
│                                                        │
│  UTXOs (2)                                             │
│  ┌────────────────────────────────────────────────┐   │
│  │ txid:0  100,000 sats  ✓ Confirmed (6 blocks)  │   │
│  │ txid:1   23,456 sats  ⏳ Pending               │   │
│  └────────────────────────────────────────────────┘   │
│                                                        │
│  [Send]                                   [Receive]    │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

### 4.5 Network Status

```
┌────────────────────────────────────────────────────────┐
│  Network Status                                        │
├────────────────────────────────────────────────────────┤
│                                                        │
│  Services                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ XSCore   │  │ Bitcoin  │  │ Boltz    │            │
│  │ ✓ Online │  │ ✓ Synced │  │ ✓ Ready  │            │
│  │ Port 9735│  │ Blk 850k │  │ API OK   │            │
│  └──────────┘  └──────────┘  └──────────┘            │
│                                                        │
│  Bitcoin Network                                       │
│  ├─ Chain: regtest                                    │
│  ├─ Block Height: 850,123                             │
│  ├─ Difficulty: 1.0                                   │
│  └─ Mempool: 5 txs                                    │
│                                                        │
│  Fee Estimates                                         │
│  ├─ Fast (1 block): 10 sat/vB                         │
│  ├─ Medium (6 blocks): 5 sat/vB                       │
│  └─ Slow (24 blocks): 1 sat/vB                        │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

### 4.6 Settings

```
┌────────────────────────────────────────────────────────┐
│  Settings                                              │
├────────────────────────────────────────────────────────┤
│                                                        │
│  Security                                              │
│  ├─ [Lock Wallet Now]                                 │
│  ├─ Auto-lock after: [5 minutes ▼]                    │
│  └─ Change PIN: [Change...]                           │
│                                                        │
│  Backup                                                │
│  ├─ [View Recovery Phrase]  ⚠️ Requires PIN           │
│  └─ Last backup verified: Never                       │
│                                                        │
│  Network                                               │
│  ├─ Network: [Regtest ▼]                              │
│  └─ API Bridge: localhost:3000                        │
│                                                        │
│  About                                                 │
│  ├─ Version: 0.1.0                                    │
│  └─ XSCore: v0.2.0                                    │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

## 5. API Contracts

### 5.1 Vault

```typescript
// GET /api/v1/vault/status
Response: {
  state: 'not_initialized' | 'locked' | 'unlocked' | 'locked_out',
  failed_attempts?: number
}

// POST /api/v1/vault/init
Request: {
  action: 'generate' | 'import',
  mnemonic?: string,
  pin: string
}
Response: {
  success: boolean,
  mnemonic?: string,
  session_id: string
}

// POST /api/v1/vault/unlock
Request: { pin: string }
Response: {
  success: boolean,
  session_id?: string,
  error_message?: string
}

// POST /api/v1/vault/lock
Response: { success: boolean }
```

### 5.2 Wallet

```typescript
// GET /api/v1/wallet/balances
Response: {
  btc: { confirmed: number, unconfirmed: number },
  liquid: { confirmed: number, unconfirmed: number },
  ln: { balance: number }
}

// POST /api/v1/wallet/address
Request: { chain: 'btc' | 'liquid', label?: string }
Response: {
  address: string,
  derivation_path: string
}
```

### 5.3 Swap

```typescript
// POST /api/v1/swap/quote
Request: {
  kind: 'submarine' | 'reverse' | 'chain',
  from_chain: 'btc' | 'liquid' | 'ln',
  to_chain: 'btc' | 'liquid' | 'ln',
  amount_sats: number
}
Response: {
  quote_id: string,
  from_amount: number,
  to_amount: number,
  fee_sats: number,
  expires_at: string
}

// POST /api/v1/swap/accept
Request: { quote_id: string }
Response: {
  swap_id: string,
  state: string,
  htlc_address?: string
}

// GET /api/v1/swaps
// GET /api/v1/swaps/:id
```

### 5.4 Bitcoin

```typescript
// GET /api/v1/bitcoin/info
Response: {
  chain: string,
  blocks: number,
  synced: boolean
}

// GET /api/v1/bitcoin/fees
Response: {
  fast: number,
  medium: number,
  slow: number
}
```

---

## 6. Design System

### Cores

| Token | Hex | Uso |
|-------|-----|-----|
| `--bg-primary` | #000000 | Background principal |
| `--bg-secondary` | #111111 | Cards |
| `--border` | #222222 | Bordas |
| `--text-primary` | #ffffff | Texto principal |
| `--text-secondary` | #666666 | Texto secundário |
| `--accent-btc` | #F7931A | Bitcoin |
| `--accent-ln` | #FFD700 | Lightning |
| `--accent-liquid` | #00B4D8 | Liquid |
| `--success` | #10B981 | Sucesso |
| `--warning` | #F59E0B | Aviso |
| `--error` | #EF4444 | Erro |

### Tipografia

| Token | Font | Size |
|-------|------|------|
| Heading | Inter | 24px bold |
| Body | Inter | 14px |
| Mono | JetBrains Mono | 13px |

---

## 7. Segurança UX

| Ação | Confirmação |
|------|-------------|
| Lock wallet | Imediato |
| View mnemonic | Requer PIN |
| Accept swap | Mostrar resumo |
| Cancel swap | Confirmar |
| Reset app | Digitar "RESET" |
