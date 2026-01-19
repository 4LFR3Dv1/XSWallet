# Regtest Test Instructions

## Setup

1. **Start regtest environment**:
```powershell
cd test\regtest
docker compose up -d
```

This will:
- Start bitcoind in regtest mode
- Create a wallet
- Mine 101 blocks (coinbase maturity)

2. **Verify bitcoind is running**:
```powershell
docker exec xs-bitcoind-regtest bitcoin-cli -regtest -rpcuser=rpcuser -rpcpassword=rpcpass getblockchaininfo
```

## Running Tests

### E2E Submarine Test
```powershell
cd core
go test -v ./test/e2e -run TestSubmarineE2E
```

### Crash Recovery Test
```powershell
go test -v ./test/e2e -run TestCrashRecovery
```

### All Tests
```powershell
go test -v ./test/e2e/...
```

## Manual Testing

### 1. Start xscore
```powershell
.\xscore.exe --network=regtest --grpc-port=18080
```

### 2. Use grpcurl to test

**Initialize Vault**:
```powershell
grpcurl -plaintext -d '{"pin":"test1234","generate":{}}' localhost:18080 xswallet.WalletService/InitializeVault
```

**Unlock Vault**:
```powershell
grpcurl -plaintext -d '{"pin":"test1234"}' localhost:18080 xswallet.WalletService/UnlockVault
```

**Get New Address**:
```powershell
grpcurl -plaintext -H "authorization: Bearer <session_id>" -d '{"chain":"CHAIN_BTC"}' localhost:18080 xswallet.WalletService/GetNewAddress
```

## Cleanup

```powershell
docker compose down -v
```
