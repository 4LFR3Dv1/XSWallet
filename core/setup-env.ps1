# XS Wallet Core - Environment Setup Script
# Run this script to configure PATH for Go and protoc

Write-Host "XS Wallet Core - Environment Setup" -ForegroundColor Cyan
Write-Host "==================================`n" -ForegroundColor Cyan

# Check if Go is installed
$goPath = "C:\Program Files\Go\bin"
if (Test-Path $goPath) {
    Write-Host "[OK] Go found at: $goPath" -ForegroundColor Green
    
    # Add to current session PATH
    if ($env:Path -notlike "*$goPath*") {
        $env:Path += ";$goPath"
        Write-Host "[+] Added Go to PATH (current session)" -ForegroundColor Yellow
    }
    
    # Add to user PATH permanently
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$goPath*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$goPath", "User")
        Write-Host "[+] Added Go to PATH (permanent)" -ForegroundColor Green
    }
} else {
    Write-Host "[!] Go not found. Install from: https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

# Check if GOPATH/bin exists and add to PATH
$goUserBin = "$env:USERPROFILE\go\bin"
if (-not (Test-Path $goUserBin)) {
    New-Item -ItemType Directory -Path $goUserBin -Force | Out-Null
    Write-Host "[+] Created $goUserBin" -ForegroundColor Yellow
}

if ($env:Path -notlike "*$goUserBin*") {
    $env:Path += ";$goUserBin"
    Write-Host "[+] Added $goUserBin to PATH (current session)" -ForegroundColor Yellow
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$goUserBin*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$goUserBin", "User")
    Write-Host "[+] Added $goUserBin to PATH (permanent)" -ForegroundColor Green
}

# Verify Go works
Write-Host "`nVerifying Go installation..." -ForegroundColor Cyan
try {
    $goVersion = & go version 2>&1
    Write-Host "[OK] $goVersion" -ForegroundColor Green
} catch {
    Write-Host "[!] Go command failed. You may need to restart your terminal." -ForegroundColor Red
}

# Install protoc-gen-go plugins
Write-Host "`nInstalling protoc plugins..." -ForegroundColor Cyan
try {
    Write-Host "Installing protoc-gen-go..." -ForegroundColor Yellow
    & go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    Write-Host "[OK] protoc-gen-go installed" -ForegroundColor Green
    
    Write-Host "Installing protoc-gen-go-grpc..." -ForegroundColor Yellow
    & go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    Write-Host "[OK] protoc-gen-go-grpc installed" -ForegroundColor Green
} catch {
    Write-Host "[!] Failed to install plugins: $_" -ForegroundColor Red
}

# Check for protoc
Write-Host "`nChecking for protoc..." -ForegroundColor Cyan
try {
    $protocVersion = & protoc --version 2>&1
    Write-Host "[OK] $protocVersion" -ForegroundColor Green
} catch {
    Write-Host "[!] protoc not found" -ForegroundColor Red
    Write-Host "Download from: https://github.com/protocolbuffers/protobuf/releases" -ForegroundColor Yellow
    Write-Host "Extract and add to PATH, or use chocolatey: choco install protoc" -ForegroundColor Yellow
}

Write-Host "`n==================================`n" -ForegroundColor Cyan
Write-Host "Setup complete! You may need to restart your terminal." -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor Cyan
Write-Host "1. Close and reopen your terminal" -ForegroundColor White
Write-Host "2. cd core" -ForegroundColor White
Write-Host "3. Run: .\build.ps1" -ForegroundColor White
