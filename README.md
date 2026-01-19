# XS Wallet Core

## 🚀 Visão Geral
**XS Wallet** é uma engine de carteira **Atomic Swap** de alta performance e auto-custodial, escrita em Go.
Atua como o backend autoritativo para clientes multiplataforma (Desktop Electron & Mobile React Native), garantindo lógica consistente, segurança e gestão de estado em todos os dispositivos.

## 🏗 Arquitetura
O projeto segue estritamente a **Clean Architecture**:

- **Go Core (`xscore`)**: O cérebro. Gerencia máquinas de estado, criptografia, transações de banco de dados e rede P2P. Executa como um processo *sidecar*.
- **Agnóstico de Frontend**: Comunica-se com UIs via **gRPC**, permitindo frontends intercambiáveis.
- **Integridade de Dados**: Utiliza **SQLite em modo WAL** com **Optimistic Locking (CAS)** para garantir consistência de estado mesmo durante falhas ou ações simultâneas de usuários.
- **Segurança**: Chaves sensíveis são criptografadas em repouso usando **Argon2id** e **AES-256-GCM**.

## ⚡ Status Atual: MVP Fase 1 Concluído
Implementamos com sucesso a integração do **Boltz Client v2** com padrões de confiabilidade de produção:

### ✅ Features de Produção Implementadas
*   **Rede Resiliente**: Cliente HTTP com *exponential backoff*, tratamento de *rate limit* (`Retry-After`) e prevenção de vazamento de recursos.
*   **Atualizações em Tempo Real**: Cliente WebSocket com padrão **Single-Writer** (tratamento seguro de concorrência) e **Gap Recovery** automático (busca estados perdidos via REST após reconexão).
*   **Corretude**: Adesão estrita ao Ciclo de Vida do Boltz para swaps **Submarine**, **Reverse**, e **Chain**.
*   **Tratamento Polimórfico**: Manipulação robusta de tipos complexos do Boltz (ex: `minerFees` e `timeouts` polimórficos).

### 🔜 Próximos Passos (Em Progresso)
1.  **Engine de Idempotência**: Implementação de padrões `INSERT OR IGNORE` para garantir retries seguros em operações de funding e claim.
2.  **LND/Liquid Adapters**: Conexão da lógica de swap com as chains reais para execução.
3.  **Testes End-to-End**: Validação do ciclo completo em ambiente Regtest.

## 🛠 Tech Stack
- **Linguagem**: Go 1.25+
- **Comunicação**: gRPC (Protobuf)
- **Banco de Dados**: SQLite3 (moderno, embarcado)
- **Provedor de Swap**: Boltz API v2
- **Testes**: Regtest baseado em Docker (Bitcoind + Elementsd)

## 🏃‍♂️ Como Executar

### Pré-requisitos
- Go 1.25+
- Docker (para ambiente local Regtest)
- Git

### Build do Core
```bash
cd core
go build ./cmd/xscore
```

### Rodar Testes
O projeto inclui uma suíte de testes abrangente (Unitários + E2E):

```bash
cd core
# Rodar Testes Unitários (incluindo novos testes do cliente Boltz)
go test -v ./internal/boltz/...

# Iniciar Ambiente Regtest
docker compose -f test/regtest/docker-compose.yml up -d

# Rodar Testes de Integração
go test -v ./test/e2e/...
```

## 📂 Estrutura do Projeto
```
core/
├── cmd/xscore/       # Entrypoint do Daemon
├── internal/
│   ├── boltz/        # Cliente API Boltz (Pronto para Produção)
│   ├── swap/         # Máquina de Estado & Engine de Swap
│   ├── provider/     # Interfaces de Provider
│   └── db/           # Camada de Banco de Dados
├── proto/            # Definições gRPC
└── test/             # Ferramentas E2E & Regtest
```
