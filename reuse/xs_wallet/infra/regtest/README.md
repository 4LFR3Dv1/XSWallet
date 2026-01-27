# Regtest Test Environment

## Complete Setup (with Boltz Backend)

### 1. Start all services
```powershell
cd test\regtest
docker compose up -d
```

This starts:
- **bitcoind** - Bitcoin Core in regtest mode (port 18443)
- **CLN** - Core Lightning node (port 9736)
- **boltz** - Boltz Backend (port 9001) ← Swap provider!

Wait ~60 seconds for all services to initialize.

### 2. Verify services are running
```powershell
# Check bitcoind
docker exec xs-bitcoind-regtest bitcoin-cli -regtest -rpcuser=rpcuser -rpcpassword=rpcpass getblockchaininfo

# Check Boltz
curl http://localhost:9001/version
```

### 3. Start xscore
```powershell
cd core
.\xscore.exe --network=regtest
```

You should see:
```
Using Boltz provider at http://127.0.0.1:9001
```

## Quick Setup (Mock Only - No Docker)

If you don't need real swaps, just run xscore directly:
```powershell
.\xscore.exe --network=regtest
```
It will use the mock provider for testing.

## Running E2E Tests

### Submarine Swap Test
```powershell
cd core
go test -v ./test/e2e -run TestSubmarineE2E
```

### All Tests
```powershell
go test -v ./test/e2e/...
```

## Manual Testing with grpcurl

**Initialize Vault**:
```powershell
grpcurl -plaintext -d '{"pin":"123456","generate":{}}' localhost:9735 xswallet.WalletService/InitializeVault
```

**Unlock Vault**:
```powershell
grpcurl -plaintext -d '{"pin":"123456"}' localhost:9735 xswallet.WalletService/UnlockVault
```

**Get New Address**:
```powershell
grpcurl -plaintext -H "authorization: Bearer <session_id>" -d '{"chain":"CHAIN_BTC"}' localhost:9735 xswallet.WalletService/GetNewAddress
```

## Cleanup

```powershell
docker compose down -v
```

## Troubleshooting

See [DOCKER_TROUBLESHOOTING.md](./DOCKER_TROUBLESHOOTING.md)

