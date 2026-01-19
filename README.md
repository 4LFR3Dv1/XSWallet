# XS Wallet Core

## 🚀 Overview
**XS Wallet** is a high-performance, self-custodial **Atomic Swap** wallet engine written in Go.
It acts as the authoritative backend for cross-platform clients (Electron Desktop & React Native Mobile), ensuring consistent logic, security, and state management across all devices.

## 🏗 Architecture
The project follows a strict **Clean Architecture**:

- **Go Core (`xscore`)**: The brain. Handles state machines, cryptography, database transactions, and P2P networking. It runs as a sidecar process.
- **Frontend Agnostic**: Communicates with UIs via **gRPC**, allowing interchangeable frontends.
- **Data Integrity**: Uses **SQLite in WAL mode** with **Optimistic Locking (CAS)** to guarantee state consistency even during crashes or concurrent user actions.
- **Security**: Sensitive keys are encrypted at rest using **Argon2id** and **AES-256-GCM**.

## ⚡ Current Status: MVP Phase 1 Completed
We have successfully implemented the **Boltz Client v2** integration with production-grade reliability standards:

### ✅ Production Features Implemented
*   **Resilient Networking**: HTTP Client with exponential backoff, rate limit handling (`Retry-After`), and resource leak prevention.
*   **Real-time Updates**: WebSocket client with **Single-Writer** pattern (handling concurrency safely) and automatic **Gap Recovery** (fetching missed states via REST upon reconnection).
*   **Correctness**: Strict adherence to the Boltz Life Cycle for **Submarine**, **Reverse**, and **Chain** swaps.
*   **Polymorphic Handling**: Robust handling of complex Boltz types (e.g., polymorphic `minerFees` and `timeouts`).

### 🔜 Next Steps (In Progress)
1.  **Idempotency Engine**: Implementing `INSERT OR IGNORE` patterns to guarantee safe retries for funding and claiming operations.
2.  **LND/Liquid Adapters**: Connecting the swap logic to actual chains for execution.
3.  **End-to-End Testing**: Validating the full cycle on Regtest.

## 🛠 Tech Stack
- **Language**: Go 1.25+
- **Communication**: gRPC (Protobuf)
- **Database**: SQLite3 (modern, embedded)
- **Swap Provider**: Boltz API v2
- **Testing**: Docker-based Regtest (Bitcoind + Elementsd)

## 🏃‍♂️ How to Run

### Prerequisites
- Go 1.25+
- Docker (for local Regtest environment)
- Git

### Build Core
```bash
cd core
go build ./cmd/xscore
```

### Run Tests
The project includes a comprehensive test suite (Unit + E2E):

```bash
cd core
# Run Unit Tests (including new Boltz client tests)
go test -v ./internal/boltz/...

# Start Regtest Environment
docker compose -f test/regtest/docker-compose.yml up -d

# Run Integration Tests
go test -v ./test/e2e/...
```

## 📂 Project Structure
```
core/
├── cmd/xscore/       # Daemon entrypoint
├── internal/
│   ├── boltz/        # Boltz API Client (Production Ready)
│   ├── swap/         # Swap State Machine & Engine
│   ├── provider/     # Provider Interfaces
│   └── db/           # Database Layer
├── proto/            # gRPC Definitions
└── test/             # E2E & Regtest tooling
```
