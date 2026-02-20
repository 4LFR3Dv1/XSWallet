# XS Wallet - IPC Contract Matrix v1

Status: draft inicial para migração `HTTP -> IPC -> gRPC`.

Fallback HTTP no renderer só é permitido com `VITE_DEBUG_HTTP=1`.

Legenda de status de implementacao:
- `ativo`: channel implementado no `electron/ipc/registry.ts`.
- `parcial`: channel existe, mas depende de endpoint provisório/bridge ou comportamento simplificado.
- `pendente`: channel mapeado, mas handler retorna `UNIMPLEMENTED`.

| HTTP atual (`api-bridge`) | IPC channel | gRPC alvo | Access | Mode | Requires unlocked | Status |
|---|---|---|---|---|---|---|
| `GET /api/v1/system/health` | `app.ping` | n/a (main process) | `renderer_safe` | `read` | `no` | `ativo` |
| `GET /api/v1/wallet/status` | `wallet.getStatus` | `WalletService/GetVaultStatus` | `renderer_safe` | `read` | `no` | `ativo` |
| `POST /api/v1/wallet/unlock` | `wallet.unlock` | `WalletService/UnlockVault` | `renderer_safe` | `mutate` | `no` | `ativo` |
| `POST /api/v1/wallet/lock` | `wallet.lock` | `WalletService/LockVault` | `privileged` | `mutate` | `no` | `ativo` |
| `GET /api/v1/wallet/balances` | `wallet.getBalances` | `WalletService/GetAllBalances` | `privileged` | `read` | `yes` | `ativo` |
| `POST /api/v1/swaps` | `swap.create` | `SwapService/QuoteSwap + CreateSwap` | `privileged` | `mutate` | `yes` | `ativo` |
| `GET /api/v1/swaps` | `swap.list` | `SwapService/ListSwaps` | `privileged` | `read` | `yes` | `ativo` |
| `GET /api/v1/swaps/:id` | `swap.get` | `SwapService/GetSwap` | `privileged` | `read` | `yes` | `ativo` |
| n/a (event `swap.watchAll.update` + polling no main process) | `swap.watchAll` | `SwapService/WatchAllSwaps` | `privileged` | `read` | `yes` | `parcial` |
| `GET /api/v1/system/status` (provisório) | `nodes.list` | `NodeService/GetAllNodeStatuses` (alvo) | `privileged` | `read` | `yes` | `parcial` |
| `POST /api/v1/nodes/:node_type/start` | `nodes.start` | `NodeService/StartNode` | `privileged` | `mutate` | `yes` | `parcial` |
| `POST /api/v1/nodes/:node_type/stop` | `nodes.stop` | `NodeService/StopNode` | `privileged` | `mutate` | `yes` | `parcial` |
| `POST /api/v1/nodes/:node_type/restart` | `nodes.restart` | `NodeService/RestartNode` | `privileged` | `mutate` | `yes` | `parcial` |
| n/a | `nodes.watchLogs` | `NodeService/WatchLogs` | `privileged` | `read` | `yes` | `pendente` |
| n/a | `binaries.ensureInstalled` | `NodeService/DownloadBinary + VerifyBinary` | `privileged` | `mutate` | `yes` | `pendente` |

## Canais proibidos no renderer (fora whitelist)

- Qualquer channel fora do registry `electron/ipc/registry.ts`.
- Acesso direto a `ipcRenderer`, `fs`, `child_process`, `net`, `process`, `require`.

## Error contract

Formato padrão de erro IPC:

```json
{
  "ok": false,
  "error": {
    "code": "INVALID_ARGUMENT|UNAUTHENTICATED|RESOURCE_EXHAUSTED|UNIMPLEMENTED|INTERNAL",
    "message": "human readable",
    "details": {},
    "traceId": "uuid"
  }
}
```
