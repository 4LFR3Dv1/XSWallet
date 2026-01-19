# Docker Troubleshooting

## Erro: 500 Internal Server Error

**Causa**: Docker Desktop instalado mas engine Linux ainda não inicializou completamente.

**Solução**:

1. **Aguardar 1-2 minutos** para Docker Desktop iniciar completamente
2. Verificar status no ícone do Docker Desktop (bandeja do sistema)
3. Quando o ícone ficar verde/estável, tentar novamente

## Verificar Docker

```powershell
# Testar conexão
docker info

# Se funcionar, você verá informações do Docker
# Se falhar, aguardar mais tempo
```

## Alternativa: Restart Docker Desktop

```powershell
# Fechar Docker Desktop
Stop-Process -Name "Docker Desktop" -Force

# Aguardar 10 segundos
Start-Sleep -Seconds 10

# Iniciar novamente
Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"

# Aguardar 1-2 minutos
Start-Sleep -Seconds 120

# Testar
docker info
```

## Quando Docker estiver pronto

```powershell
cd test\regtest
docker compose up -d

# Verificar containers
docker compose ps

# Ver logs
docker compose logs -f
```
