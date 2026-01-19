# Download and install protoc (Protocol Buffers Compiler)

$protocVersion = "29.3"
$protocUrl = "https://github.com/protocolbuffers/protobuf/releases/download/v$protocVersion/protoc-$protocVersion-win64.zip"
$downloadPath = "$env:TEMP\protoc.zip"
$installPath = "$env:USERPROFILE\protoc"

Write-Host "Downloading protoc v$protocVersion..." -ForegroundColor Cyan

try {
    # Download
    Invoke-WebRequest -Uri $protocUrl -OutFile $downloadPath -UseBasicParsing
    Write-Host "[OK] Downloaded to $downloadPath" -ForegroundColor Green
    
    # Extract
    if (Test-Path $installPath) {
        Remove-Item $installPath -Recurse -Force
    }
    Expand-Archive -Path $downloadPath -DestinationPath $installPath -Force
    Write-Host "[OK] Extracted to $installPath" -ForegroundColor Green
    
    # Add to PATH
    $protocBin = "$installPath\bin"
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$protocBin*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$protocBin", "User")
        Write-Host "[OK] Added to PATH (permanent)" -ForegroundColor Green
    }
    
    # Add to current session
    if ($env:Path -notlike "*$protocBin*") {
        $env:Path += ";$protocBin"
        Write-Host "[OK] Added to PATH (current session)" -ForegroundColor Green
    }
    
    # Verify
    $version = & protoc --version 2>&1
    Write-Host "[OK] $version" -ForegroundColor Green
    
    # Cleanup
    Remove-Item $downloadPath -Force
    
    Write-Host "`nprotoc installed successfully!" -ForegroundColor Green
    Write-Host "You may need to restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
    
}
catch {
    Write-Host "[!] Installation failed: $_" -ForegroundColor Red
    exit 1
}
