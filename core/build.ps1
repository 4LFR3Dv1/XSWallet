# XS Wallet Core - Build Script
# Generates proto files and builds the Go binary

Write-Host "XS Wallet Core - Build" -ForegroundColor Cyan
Write-Host "=====================`n" -ForegroundColor Cyan

# Check Go
try {
    $goVersion = & go version
    Write-Host "[OK] $goVersion" -ForegroundColor Green
} catch {
    Write-Host "[!] Go not found. Run setup-env.ps1 first" -ForegroundColor Red
    exit 1
}

# Check protoc
try {
    $protocVersion = & protoc --version
    Write-Host "[OK] $protocVersion" -ForegroundColor Green
} catch {
    Write-Host "[!] protoc not found" -ForegroundColor Red
    Write-Host "Download from: https://github.com/protocolbuffers/protobuf/releases" -ForegroundColor Yellow
    exit 1
}

# Generate proto files
Write-Host "`nGenerating proto files..." -ForegroundColor Cyan
$protoFiles = Get-ChildItem -Path "..\proto\*.proto"
foreach ($file in $protoFiles) {
    Write-Host "  - $($file.Name)" -ForegroundColor Gray
}

try {
    & protoc `
        --go_out=. `
        --go-grpc_out=. `
        --go_opt=paths=source_relative `
        --go-grpc_opt=paths=source_relative `
        ..\proto\*.proto
    
    Write-Host "[OK] Proto files generated" -ForegroundColor Green
} catch {
    Write-Host "[!] Proto generation failed: $_" -ForegroundColor Red
    exit 1
}

# Download dependencies
Write-Host "`nDownloading dependencies..." -ForegroundColor Cyan
try {
    & go mod tidy
    Write-Host "[OK] Dependencies downloaded" -ForegroundColor Green
} catch {
    Write-Host "[!] go mod tidy failed: $_" -ForegroundColor Red
    exit 1
}

# Build
Write-Host "`nBuilding xscore..." -ForegroundColor Cyan
try {
    & go build -o xscore.exe .\cmd\xscore
    Write-Host "[OK] Build successful: xscore.exe" -ForegroundColor Green
} catch {
    Write-Host "[!] Build failed: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n=====================`n" -ForegroundColor Cyan
Write-Host "Build complete!" -ForegroundColor Green
Write-Host "`nRun with: .\xscore.exe --network=regtest --port=9735" -ForegroundColor Cyan
