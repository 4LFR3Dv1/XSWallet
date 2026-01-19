# XS Wallet - Decisões Técnicas (Desktop App)

Documento de decisões arquiteturais críticas para o Desktop App.

---

## 1. SQLite: Pragmas Críticos

**Decisão**: Usar SQLite com WAL mode para concorrência otimizada.

```sql
PRAGMA journal_mode = WAL;      -- Write-Ahead Logging
PRAGMA synchronous = NORMAL;    -- Balance safety/speed
PRAGMA busy_timeout = 5000;     -- Wait 5s on lock
PRAGMA foreign_keys = ON;       -- Enforce FK constraints
```

**Rationale**:
- WAL permite leituras concorrentes sem bloqueio
- `busy_timeout` evita "database is locked" errors
- `synchronous=NORMAL` é seguro com WAL

**Regra Operacional**:
- Todas as operações CAS em **transações curtas**
- **Single writer** (fila interna) para evitar deadlocks

---

## 2. No-Op CAS Deve Falhar

**Decisão**: `updateSwapCAS()` retorna `success: false` se nenhum campo válido foi fornecido.

**Rationale**:
- Evita bugs silenciosos quando key é filtrada pelo whitelist
- Desktop tem menos observabilidade que servidor
- Fail-fast é melhor que silent no-op

---

## 3. Backend em Processo Separado

**Decisão**: Arquitetura multi-processo.

```
Main Process (Electron)
  ├─ Window management
  ├─ Auto-updater
  ├─ Node manager
  └─ IPC bridge

Backend Process (Node.js child)
  ├─ Swap Engine
  ├─ Database
  ├─ Adapters
  └─ Boltz WebSocket
```

**Rationale**:
- UI crash não mata engine/nodes
- Crash isolado
- IPC mais limpo
- Melhor gestão de recursos

---

## 4. Embedded Nodes: Download Verificável

**Decisão**: Instalador leve + download verificável no primeiro run.

### Fluxo
1. Instalador **não** inclui binários (apenas app)
2. Primeiro run: download de GitHub Releases
3. Verificação: SHA256 checksum + assinatura (opcional)
4. Armazenamento: `appData/nodes/<version>/`

### Vantagens
- Instalador leve (~50MB vs ~500MB)
- Notarização mais rápida (macOS)
- Update de app ≠ update de nodes
- Versionamento independente

### Registry de Binários
```typescript
const NODE_BINARIES = {
  bitcoind: [
    {
      version: '26.0',
      platform: 'win32',
      downloadUrl: 'https://bitcoincore.org/...',
      checksum: 'sha256...',
    }
  ]
};
```

---

## 5. Boltz Regtest Environment

**Decisão**: Usar ambiente regtest oficial do Boltz para testes.

**Recursos**:
- Docker image: `boltz/regtest`
- Repo: `BoltzExchange/regtest`
- Documentação: api.docs.boltz.exchange/backend-development

**Regra**:
- WebSocket = gatilho (latência baixa)
- On-chain/LND = fonte de verdade (sempre revalidar)

---

## 6. Config Management

**Decisão**: Tabela `app_config` no SQLite (key-value JSON).

```sql
CREATE TABLE app_config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL, -- JSON
  updated_at TEXT NOT NULL
);
```

### Configurações
- Network (regtest/testnet/mainnet)
- Node paths
- Provider URL (Boltz)
- Timeouts
- KDF params
- Fee thresholds

### Snapshot em Swaps
- Cada swap carrega config snapshot em `locked_intent`
- Garante reprodutibilidade

---

## 7. Segurança do PIN + Vault

**Decisão**: PIN → Argon2id → AES-256-GCM + rate limiting.

### Implementação
```typescript
// KDF params (stored with ciphertext)
const params = {
  algorithm: 'argon2id',
  memory: 64 * 1024, // 64MB
  iterations: 3,
  parallelism: 1,
  saltLength: 16,
};

// Derive key from PIN
const key = argon2id(pin, salt, params);

// Encrypt seed
const ciphertext = aes256gcm.encrypt(seed, key, iv);
```

### Rate Limiting
- Backoff: 1s → 2s → 4s → 8s...
- Max attempts: 10 (depois bloqueia por 1h)

### Optional: OS Keychain
- Armazenar "device secret" no keychain
- Combinar com PIN para derivar key
- Defesa extra contra ataque offline

### SQLCipher
- MVP: vault encryption protege seed
- v1.1: SQLCipher (defesa em profundidade)

---

## 8. Auto-Update & Supply Chain

**Decisão**: Code signing + checksum verification + rollback.

### App Update
- electron-updater
- Code signing (Windows/macOS)
- Checksum verification
- Rollback automático em falha

### Node Update
- Versionamento separado
- Download verificável (SHA256 + GPG opcional)
- Rollback manual (via UI)

### Supply Chain
- App releases: GitHub Releases + code signing
- Node binaries: oficial sources (bitcoincore.org, github.com/ElementsProject, etc)
- Checksums em registry hardcoded

---

## Cronograma Ajustado

| Fase | Tempo | Nota |
|------|-------|------|
| Backend Core | 2 semanas | SQLite + adapters + engine |
| Frontend | 1.5 semanas | React + UI flows |
| Electron Integration | 1 semana | Multi-process + node manager |
| Packaging & Testing | 0.5 semana | Instaladores + E2E |
| **Total** | **5 semanas** | Com download verificável |

**Risco**: Se bundlar binários no instalador, adicionar +1 semana (notarização, infra).

---

## Próximos Arquivos a Criar

1. ✅ `db.sqlite.ts` - Database layer
2. ✅ `nodes/manager.ts` - Node lifecycle
3. [ ] `vault/index.ts` - Key vault (BIP39/32/84/85)
4. [ ] `vault/encryption.ts` - Argon2id + AES-GCM
5. [ ] `main.ts` - Electron main process
6. [ ] `preload.ts` - IPC bridge
7. [ ] `backend/server.ts` - Backend process
