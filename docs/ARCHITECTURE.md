# XS Wallet - Arquitetura TS + Go

## Visão Geral

Arquitetura multi-plataforma com fronteira rígida:
- **Go Core** (`/core`): Autoritativo para swap, DB, criptografia, nodes
- **TS Shell** (`/apps/desktop`): UI Electron + React  
- **RN Shell** (`/apps/mobile`): UI React Native (futuro)

```
┌─────────────────────────────────────────────────────────────┐
│                        UI Layer                              │
│  ┌───────────────────┐     ┌─────────────────────────────┐  │
│  │  Electron + React │     │     React Native (futuro)   │  │
│  │  Desktop (Win/Mac)│     │     Mobile (iOS/Android)    │  │
│  └─────────┬─────────┘     └──────────────┬──────────────┘  │
│            │                              │                  │
│            │         gRPC/gomobile        │                  │
│            └──────────────┬───────────────┘                  │
└───────────────────────────┼──────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Go Core (xscore)                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐ │
│  │Swap Eng. │  │ Wallet   │  │ Adapters │  │ Boltz Client │ │
│  │State Mach│  │ BIP39/84 │  │ BTC/LQ/LN│  │ REST + WS    │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬───────┘ │
│       └─────────────┴─────────────┴───────────────┘         │
│                           │                                  │
│                    ┌──────┴──────┐                          │
│                    │  SQLite WAL │                          │
│                    │  (CAS/Events)│                          │
│                    └─────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

## Estrutura do Projeto

```
/
├── proto/                      # Contratos gRPC
│   ├── swap.proto              # SwapService (state machine)
│   ├── wallet.proto            # WalletService (vault, balances)
│   └── node.proto              # NodeService (bitcoind, elementsd, lnd)
│
├── core/                       # Go Core (autoritativo)
│   ├── cmd/xscore/main.go      # Entrypoint do daemon
│   ├── go.mod
│   └── internal/
│       ├── config/             # Configuração
│       ├── db/                 # SQLite + schema
│       ├── swap/               # State machine + CAS
│       ├── wallet/             # BIP39/32/84/85 + vault
│       ├── boltz/              # Cliente REST/WS
│       ├── adapters/           # bitcoind, elementsd, lnd
│       └── server/             # gRPC implementations
│
├── apps/
│   ├── desktop/                # Electron + React (TS)
│   │   ├── src/
│   │   │   ├── main/           # Electron main process
│   │   │   ├── renderer/       # React UI
│   │   │   └── grpc/           # gRPC client
│   │   └── package.json
│   │
│   └── mobile/                 # React Native (futuro)
│       ├── src/
│       └── package.json
│
├── scripts/                    # Build, CI, packaging
│   ├── build-core.sh
│   ├── gen-proto.sh
│   └── package-desktop.sh
│
└── docs/                       # Documentação técnica
```

## Princípios

### 1. Go Core é Autoritativo

O Go Core é a **única fonte de verdade** para:
- State machine do swap
- Operações idempotentes (swap_ops)
- CAS/version para concorrência
- Reservas de UTXO/LN
- Preimage e nonce hygiene
- Fee estimation e RBF/CPFP

**TS/RN NUNCA escrevem no SQLite diretamente.**

### 2. Comunicação via gRPC

```protobuf
service SwapService {
  rpc CreateSwap(CreateSwapRequest) returns (SwapSnapshot);
  rpc LockSwap(LockSwapRequest) returns (SwapSnapshot);
  rpc WatchSwap(WatchSwapRequest) returns (stream SwapEvent);
}
```

- Tipado (protobuf)
- Streaming para eventos em tempo real
- Autenticação local (token de sessão)

### 3. UI é Cliente Puro

UI pode:
- Chamar RPCs
- Exibir dados
- Solicitar ações

UI NÃO pode:
- Decidir estado
- Escrever no banco
- Validar scripts (só warnings)

## Quick Start

### 1. Gerar código do proto

```bash
# Instalar protoc + plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Gerar Go
protoc --go_out=core/proto --go-grpc_out=core/proto proto/*.proto
```

### 2. Build do Go Core

```bash
cd core
go mod tidy
go build -o xscore ./cmd/xscore
```

### 3. Rodar Go Core

```bash
./xscore --network=regtest --port=9735
```

### 4. Desktop (Electron)

```bash
cd apps/desktop
npm install
npm run dev
```

## Roadmap

### Fase 1: Core Backend (2 semanas)
- [x] Proto gRPC definido
- [x] Schema SQLite + CAS
- [x] State machine básica
- [ ] Key Vault (BIP39/32/84/85)
- [ ] Boltz Client (REST + WS)
- [ ] Chain Adapters

### Fase 2: Desktop Shell (1.5 semanas)
- [ ] Electron + React setup
- [ ] gRPC client
- [ ] Onboarding UI
- [ ] Swap UI

### Fase 3: Mobile (futuro)
- [ ] React Native setup
- [ ] gomobile bindings
- [ ] LND Neutrino mode

## Regras de Segurança

1. **Preimage**: `R = crypto.randomBytes(32)` - NUNCA derivar de privateKey
2. **MuSig2 Nonce**: Aux randomness OBRIGATÓRIO, session binding
3. **CAS**: Todas transições usam version check
4. **Idempotência**: Operações críticas em swap_ops
5. **IPC**: Whitelist de canais, validação Zod, rate limiting
