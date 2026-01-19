# Install MinGW-w64 for CGO support

Write-Host "Installing MinGW-w64 for Go CGO support..." -ForegroundColor Cyan

$mingwUrl = "https://github.com/niXman/mingw-builds-binaries/releases/download/14.2.0-rt_v12-rev0/x86_64-14.2.0-release-posix-seh-ucrt-rt_v12-rev0.7z"
$downloadPath = "$env:TEMP\mingw.7z"
$installPath = "C:\mingw64"

# Download
Write-Host "Downloading MinGW-w64..." -ForegroundColor Yellow
try {
    Invoke-WebRequest -Uri $mingwUrl -OutFile $downloadPath -UseBasicParsing
    Write-Host "[OK] Downloaded" -ForegroundColor Green
}
catch {
    Write-Host "[!] Download failed. Try manual download from:" -ForegroundColor Red
    Write-Host "    https://www.mingw-w64.org/downloads/" -ForegroundColor Yellow
    exit 1
}

# Extract (requires 7-Zip)
Write-Host "Extracting..." -ForegroundColor Yellow
if (Test-Path "C:\Program Files\7-Zip\7z.exe") {
    & "C:\Program Files\7-Zip\7z.exe" x $downloadPath -o"C:\" -y
    Write-Host "[OK] Extracted to $installPath" -ForegroundColor Green
}
else {
    Write-Host "[!] 7-Zip not found. Please install 7-Zip or extract manually" -ForegroundColor Red
    Write-Host "    Download 7-Zip: https://www.7-zip.org/" -ForegroundColor Yellow
    Write-Host "    Extract $downloadPath to C:\" -ForegroundColor Yellow
    exit 1
}

# Add to PATH
$mingwBin = "$installPath\bin"
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$mingwBin*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$mingwBin", "User")
    Write-Host "[OK] Added to PATH (permanent)" -ForegroundColor Green
}

# Add to current session
if ($env:Path -notlike "*$mingwBin*") {
    $env:Path += ";$mingwBin"
    Write-Host "[OK] Added to PATH (current session)" -ForegroundColor Green
}

# Verify
Write-Host "`nVerifying GCC installation..." -ForegroundColor Cyan
try {
    $gccVersion = & gcc --version 2>&1 | Select-Object -First 1
    Write-Host "[OK] $gccVersion" -ForegroundColor Green
}
catch {
    Write-Host "[!] GCC not found. Restart terminal and try again." -ForegroundColor Red
}

# Cleanup
Remove-Item $downloadPath -Force

Write-Host "`n==================================" -ForegroundColor Cyan
Write-Host "MinGW-w64 installed successfully!" -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor Cyan
Write-Host "1. Restart your terminal" -ForegroundColor White
Write-Host "2. cd core" -ForegroundColor White
Write-Host '3. $env:CGO_ENABLED=1' -ForegroundColor White
Write-Host "4. go build -o xscore.exe ./cmd/xscore" -ForegroundColor White
Write-Host "5. .\xscore.exe --network=regtest --port=18080" -ForegroundColor White
