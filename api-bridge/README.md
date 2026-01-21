# XS Wallet API Bridge

Production-ready HTTP → gRPC bridge with best practices.

## Features

✅ **Request ID tracking** - Every request gets unique ID  
✅ **Structured logging** - JSON logs with latency, status, errors  
✅ **Health checks** - `/health` + `/ready` endpoints  
✅ **Error handling** - Proper HTTP status codes from gRPC errors  
✅ **Deadlines** - 5s list, 15s create, 30s commit  
✅ **Schema normalization** - gRPC → Flask format  
✅ **State machine** - `/check` advances swap through states  

## Quick Start

```bash
# Install dependencies
npm install

# Start bridge (connects to xscore on :9735)
npm start

# Dev mode with auto-reload
npm run dev
```

## Environment Variables

```bash
GRPC_HOST=localhost:9735  # xscore gRPC server
PORT=3000                 # Bridge HTTP port
LOG_LEVEL=info           # info, warn, error
NODE_ENV=development     # development, production
```

## API Endpoints

### Health

**GET /health**
```json
{ "status": "ok", "service": "xs-wallet-bridge" }
```

**GET /ready**
```json
{ "status": "ready", "grpc_host": "localhost:9735" }
```

### Swaps

**GET /api/v1/swaps** - List all swaps
```bash
curl http://localhost:3000/api/v1/swaps
```

**POST /api/v1/swaps** - Create swap
```bash
curl -X POST http://localhost:3000/api/v1/swaps \
  -H "Content-Type: application/json" \
  -d '{
    "from_chain": "L-BTC",
    "to_chain": "LN",
    "amount_sats": 100000
  }'
```

**POST /api/v1/swaps/:id/check** - Advance state
```bash
curl -X POST http://localhost:3000/api/v1/swaps/abc123/check
```

**POST /api/v1/swaps/:id/claim** - Claim (NOT IMPLEMENTED)  
**POST /api/v1/swaps/:id/refund** - Refund (NOT IMPLEMENTED)

### Wallet

**POST /api/v1/wallet/unlock** - Unlock vault
```bash
curl -X POST http://localhost:3000/api/v1/wallet/unlock \
  -H "Content-Type: application/json" \
  -d '{ "pin": "123456" }'
```

## State Machine Flow

```
POST /swaps → STATE_OPEN

POST /swaps/:id/check → STATE_LOCKED
POST /swaps/:id/check → STATE_COMMIT_STARTED (funding tx broadcast)
POST /swaps/:id/check → STATE_WAITING (reconcile)

[Boltz completes] → STATE_COMPLETED
```

## Error Handling

All errors return:
```json
{
  "error": "Human readable message",
  "code": "GRPC_CODE",
  "details": {},
  "request_id": "uuid"
}
```

HTTP status codes mapped from gRPC:
- `400` - INVALID_ARGUMENT
- `404` - NOT_FOUND
- `409` - ALREADY_EXISTS
- `503` - UNAVAILABLE
- `504` - DEADLINE_EXCEEDED

## Logging

JSON structured logs:
```json
{
  "timestamp": "2026-01-20T14:00:00.000Z",
  "level": "info",
  "message": "POST /api/v1/swaps",
  "requestId": "uuid",
  "status": 201,
  "latency_ms": 234
}
```

## Security Notes

- Runs on `localhost` by default
- No authentication (local use only)
- Never logs sensitive data (seeds, preimages, session IDs)
- For production, add:
  - Bearer token auth
  - HTTPS
  - Rate limiting

## Testing

```bash
# Check health
curl http://localhost:3000/health

# Check gRPC connection
curl http://localhost:3000/ready

# List swaps
curl http://localhost:3000/api/v1/swaps
```

## Known Limitations

- **Claim**: Not implemented (MuSig2 crypto missing in core)
- **Refund**: Not implemented (logic missing in core)
- Swaps work end-to-end, but Boltz handles claims

## Architecture

```
React DevDash (:5173)
  ↓ HTTP
API Bridge (:3000)
  ↓ gRPC
xscore (:9735)
  ↓ gRPC
Boltz Backend (:9001)
```
