# Proto gRPC - Changelog de Hardening

## ✅ 9 Correções Críticas Aplicadas

### 1. 🔴 CRÍTICO: Removido `musig_secnonce` do SwapSnapshot
**Problema**: Segredo criptográfico atravessando fronteira gRPC  
**Solução**: Removido completamente. Apenas `musig_pubnonce` (público) é exposto.

```diff
message SwapSnapshot {
-  bytes musig_secnonce = 31;       // ❌ NUNCA expor
+  bytes musig_pubnonce = 31;       // ✅ Public nonce (ok)
+  // CRÍTICO: musig_secnonce NUNCA atravessa fronteira
}
```

### 2. ✅ Adicionado `common.proto`
**Problema**: Imports circulares e duplicação de tipos  
**Solução**: Tipos compartilhados em `common.proto`

**Conteúdo**:
- `Chain` (BTC, Liquid, LN)
- `Network` (mainnet, testnet, regtest)
- `FeePolicy`
- `PlatformCapabilities` (desktop vs mobile)

### 3. ✅ Quote Flow com `QuoteSwap`
**Problema**: Sem passo de quote antes de criar swap  
**Solução**: Novo RPC `QuoteSwap` retorna fees, timeouts, addresses

```protobuf
service SwapService {
  rpc QuoteSwap(QuoteSwapRequest) returns (SwapQuote);  // Novo!
  rpc CreateSwap(CreateSwapRequest) returns (SwapSnapshot);
}
```

**Flow**:
1. `QuoteSwap` → obtém `SwapQuote` com `quote_id`
2. Usuário aceita/recusa
3. `CreateSwap(quote_id)` → cria swap com quote

### 4. ✅ `oneof params` por tipo de swap
**Problema**: Campos inválidos misturados (ex: `invoice` em chain swap)  
**Solução**: Params específicos por tipo

```protobuf
message QuoteSwapRequest {
  SwapKind kind = 1;
  oneof params {
    SubmarineQuoteParams submarine = 10;
    ReverseQuoteParams reverse = 11;
    ChainQuoteParams chain = 12;
  }
}

message SubmarineQuoteParams { string invoice = 1; }
message ReverseQuoteParams { string payout_address = 1; }
message ChainQuoteParams { string payout_address = 1; }
```

### 5. ✅ Timestamps com `google.protobuf.Timestamp`
**Problema**: Strings causam bugs de parsing cross-platform  
**Solução**: Tipo nativo `Timestamp`

```diff
message SwapSnapshot {
-  string created_at = 60;
+  google.protobuf.Timestamp created_at = 60;
}
```

### 6. ✅ Streaming Resumable
**Problema**: UI crash perde eventos  
**Solução**: `from_seq` para resumir streaming

```protobuf
message WatchSwapRequest {
  string swap_id = 1;
  uint64 from_seq = 2;  // Novo! Começar desta sequência
}
```

**Gap fill**: `GetSwapEvents(after_seq)` + `WatchSwap(from_seq)`

### 7. ✅ `BackupStatus` no WalletService
**Problema**: Swaps com preimage R no DB sem aviso de backup  
**Solução**: Novo RPC `GetBackupStatus`

```protobuf
message BackupStatus {
  bool has_pending_swaps_requiring_backup = 1;
  int32 pending_swap_count = 2;
  google.protobuf.Timestamp last_backup_at = 3;
  string warning_message = 5;  // "You have 2 pending swaps. Backup required!"
}
```

### 8. ✅ Session ID no unlock
**Problema**: Sem autenticação de sessão  
**Solução**: `session_id` retornado no unlock

```protobuf
message UnlockVaultResponse {
  bool success = 1;
  string session_id = 2;  // Novo! Válido até lock
}
```

**Desktop**: usar como metadata gRPC  
**Mobile**: passar em cada call gomobile

### 9. ✅ `PlatformCapabilities`
**Problema**: Desktop e mobile têm capacidades diferentes  
**Solução**: RPC `GetPlatformCapabilities`

```protobuf
message PlatformCapabilities {
  bool can_spawn_nodes = 1;        // Desktop: true, Mobile: false
  bool can_download_binaries = 2;  // Desktop: true, Mobile: false
  bool has_embedded_neutrino = 3;  // Mobile: true, Desktop: false
  string platform = 4;             // "desktop", "ios", "android"
}
```

## 📊 Resumo de Mudanças

| Arquivo | Status | Mudanças |
|---------|--------|----------|
| `common.proto` | ✅ Novo | Chain, Network, FeePolicy, PlatformCapabilities |
| `swap.proto` | ✅ Atualizado | QuoteSwap, oneof params, from_seq, Timestamp, removido secnonce |
| `wallet.proto` | ✅ Atualizado | session_id, BackupStatus, Timestamp |
| `node.proto` | ✅ Atualizado | PlatformCapabilities, Timestamp |

## 🔄 Próximos Passos

```bash
# 1. Gerar código Go
cd core
protoc --go_out=. --go-grpc_out=. ../proto/*.proto

# 2. Atualizar implementações
# - swap_service.go: implementar QuoteSwap
# - wallet_service.go: implementar BackupStatus, session_id
# - node_service.go: implementar GetPlatformCapabilities

# 3. Atualizar schema SQLite
# - Adicionar quote_id em swaps
# - Adicionar last_backup_at em vault
```

## 🎯 Benefícios

1. **Segurança**: Segredos nunca atravessam fronteira
2. **Mobile-ready**: PlatformCapabilities + gomobile
3. **Crash recovery**: Streaming resumable
4. **UX**: Quote flow + backup warnings
5. **Type-safe**: oneof params evita erros
6. **Cross-platform**: Timestamp nativo
