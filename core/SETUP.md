# Go Core MVP - Setup Guide

## Pré-requisitos

O Go Core precisa das seguintes ferramentas instaladas:

### 1. Go (Golang)

**Download**: https://go.dev/dl/

Instale Go 1.22 ou superior:
```powershell
# Via winget
winget install GoLang.Go

# Ou baixe o instalador MSI do site oficial
```

Verifique a instalação:
```powershell
go version
# Deve mostrar: go version go1.22.x windows/amd64
```

### 2. Protocol Buffers Compiler (protoc)

**Download**: https://github.com/protocolbuffers/protobuf/releases

```powershell
# Via winget
winget install protocolbuffers.protoc

# Ou baixe o ZIP e adicione ao PATH
```

Verifique:
```powershell
protoc --version
# Deve mostrar: libprotoc 3.x.x
```

### 3. Plugins Go para protoc

Após instalar Go, instale os plugins:

```powershell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Certifique-se de que `%USERPROFILE%\go\bin` está no PATH.

---

## Build do Go Core

### 1. Gerar código proto

```powershell
cd core
protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative ../proto/*.proto
```

Isso cria:
- `proto/common.pb.go`
- `proto/swap.pb.go`, `proto/swap_grpc.pb.go`
- `proto/wallet.pb.go`, `proto/wallet_grpc.pb.go`
- `proto/node.pb.go`, `proto/node_grpc.pb.go`

### 2. Baixar dependências

```powershell
go mod tidy
```

### 3. Build

```powershell
go build -o xscore.exe ./cmd/xscore
```

### 4. Rodar

```powershell
./xscore.exe --network=regtest --port=9735
```

---

## Troubleshooting

### "protoc: command not found"
- Reinicie o terminal após instalar protoc
- Verifique se está no PATH: `$env:PATH -split ';' | Select-String protoc`

### "go: command not found"
- Reinicie o terminal após instalar Go
- Verifique: `$env:PATH -split ';' | Select-String Go`

### "protoc-gen-go: program not found"
- Certifique-se de que `%USERPROFILE%\go\bin` está no PATH
- Reinstale: `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`

### Import errors no código Go
- Execute `go mod tidy` para baixar dependências
- Verifique `go.mod` está correto

---

## Próximos Passos (após build)

1. **Regtest Setup**: `docker-compose -f test/regtest/docker-compose.yml up -d`
2. **E2E Tests**: `go test ./test/e2e/...`
3. **Manual Test**: Use grpcurl ou cliente gRPC para testar endpoints

---

## Estrutura de Arquivos Gerados

```
core/
├── proto/                    # Gerado pelo protoc
│   ├── common.pb.go
│   ├── swap.pb.go
│   ├── swap_grpc.pb.go
│   ├── wallet.pb.go
│   ├── wallet_grpc.pb.go
│   ├── node.pb.go
│   └── node_grpc.pb.go
├── xscore.exe               # Binário compilado
└── go.sum                   # Checksums de dependências
```
