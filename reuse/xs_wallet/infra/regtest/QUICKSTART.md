# XS Wallet - Docker Stack Quick Start

## Subir a Stack

```powershell
cd test\regtest
docker compose up -d
```

## Criar Wallet LND (Primeira Vez)

Após os containers subirem, crie o wallet do LND:

```powershell
docker exec -it xs-lnd lncli --network=regtest create
```

Siga os prompts:
1. Digite a senha do wallet (mínimo 8 caracteres)
2. Confirme a senha
3. Escolha `n` para não usar seed existente
4. **IMPORTANTE**: Anote o seed de 24 palavras exibido

## Verificar Status

```powershell
# Ver logs do LND
docker compose logs -f lnd

# Ver info do nó
docker exec xs-lnd lncli --network=regtest getinfo

# Ver status do bitcoind
docker exec xs-bitcoind bitcoin-cli -regtest -rpcuser=xswallet -rpcpassword=xswallet_dev_pass_2026 getblockchaininfo
```

## Parar a Stack

```powershell
docker compose down
```

## Limpar Tudo (Reset Completo)

```powershell
docker compose down -v
```
