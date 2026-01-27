param(
  [string]$BaseUrl = "http://localhost:8081",
  [string]$AccountUuid = "",
  [string]$LogFile = "C:\Users\windows10\Downloads\DomniWallet\wallet-service\logs\wallet_service_test.log",
  [string]$InternalToken = $env:WALLET_INTERNAL_TOKEN,
  [switch]$Verify,
  [int64]$WithdrawAmount = 10000,
  [string]$WithdrawDestination = "bcrt1pqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"
)

$ErrorActionPreference = "Stop"

if (-not $AccountUuid) {
  $AccountUuid = [guid]::NewGuid().ToString()
}

$logDir = Split-Path -Parent $LogFile
if (-not (Test-Path $logDir)) {
  New-Item -ItemType Directory -Force -Path $logDir | Out-Null
}

function Write-Log {
  param([string]$Message)
  $line = "$(Get-Date -Format s) $Message"
  Write-Host $line
  Add-Content -Path $LogFile -Value $line
}

function To-JsonLine {
  param($Obj)
  return ($Obj | ConvertTo-Json -Depth 8 -Compress)
}

Write-Log "Starting wallet-service test"
Write-Log "BaseUrl=$BaseUrl"
Write-Log "AccountUuid=$AccountUuid"
Write-Log "LogFile=$LogFile"
Write-Log "Verify=$Verify"
Write-Log "WithdrawAmount=$WithdrawAmount"
Write-Log "WithdrawDestination=$WithdrawDestination"

# 1) Create account
$accBody = @{ uuid = $AccountUuid } | ConvertTo-Json
$accResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/accounts" -ContentType "application/json" -Body $accBody
Write-Log "CreateAccount response=$(To-JsonLine $accResp)"

# 2) Create BTC address
$addrBody = @{ network = "btc"; asset = "BTC" } | ConvertTo-Json
$addrResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/accounts/$AccountUuid/addresses" -ContentType "application/json" -Body $addrBody
Write-Log "CreateAddress response=$(To-JsonLine $addrResp)"

# 3) Get balances (pre)
$balResp = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/accounts/$AccountUuid/balances"
Write-Log "Balances pre=$(To-JsonLine $balResp)"

# 4) simulate watcher UTXO ingest
$headers = @{}
if ($InternalToken) {
  $headers["X-Internal-Token"] = $InternalToken
}

$utxoAmount = 50000
try {
  $txid = -join ((0..63) | ForEach-Object { '{0:x}' -f (Get-Random -Max 16) })
  $utxoBody = @{
    network = "btc"
    asset = "BTC"
    txid = $txid
    vout = 0
    address = $addrResp.address
    amount = $utxoAmount
    confirmations = 1
    confirmed_at = (Get-Date).ToUniversalTime().ToString("o")
    block_hash = ""
    block_height = 100
  } | ConvertTo-Json

  $utxoResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/internal/utxos" -Headers $headers -ContentType "application/json" -Body $utxoBody
  Write-Log "UpsertUTXO response=$(To-JsonLine $utxoResp)"
} catch {
  Write-Log "UpsertUTXO failed: $($_.Exception.Message)"
}

# 5) Get balances (post)
$balResp2 = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/accounts/$AccountUuid/balances"
Write-Log "Balances post=$(To-JsonLine $balResp2)"

# 6) Get transactions
$txsResp = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/accounts/$AccountUuid/transactions?limit=20"
Write-Log "Transactions=$(To-JsonLine $txsResp)"

# 7) Create withdrawal
$wBody = @{ account_uuid = $AccountUuid; network = "btc"; asset = "BTC"; amount = $WithdrawAmount; destination = $WithdrawDestination } | ConvertTo-Json
$wResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/withdrawals" -ContentType "application/json" -Body $wBody
Write-Log "CreateWithdrawal response=$(To-JsonLine $wResp)"

# 8) Get withdrawal by id
$wId = $wResp.id
if ($wId) {
  $wGet = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/withdrawals/$wId"
  Write-Log "GetWithdrawal response=$(To-JsonLine $wGet)"
}

# 9) Balances after withdrawal (reserved should increase)
$balResp3 = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/accounts/$AccountUuid/balances"
Write-Log "Balances after withdrawal=$(To-JsonLine $balResp3)"

# 10) Update withdrawal status to EXECUTING and COMPLETED (internal)
if ($wId) {
  $statusHeaders = $headers
  $execBody = @{ id = $wId; status = "EXECUTING" } | ConvertTo-Json
  $execResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/internal/withdrawals/status" -Headers $statusHeaders -ContentType "application/json" -Body $execBody
  Write-Log "UpdateWithdrawal EXECUTING response=$(To-JsonLine $execResp)"

  $txid2 = "tx-" + (-join ((0..15) | ForEach-Object { '{0:x}' -f (Get-Random -Max 16) }))
  $compBody = @{ id = $wId; status = "COMPLETED"; txid = $txid2 } | ConvertTo-Json
  $compResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/internal/withdrawals/status" -Headers $statusHeaders -ContentType "application/json" -Body $compBody
  Write-Log "UpdateWithdrawal COMPLETED response=$(To-JsonLine $compResp)"

  $balResp4 = Invoke-RestMethod -Method Get -Uri "$BaseUrl/v1/accounts/$AccountUuid/balances"
  Write-Log "Balances after completion=$(To-JsonLine $balResp4)"
}

# 11) Verify balances against UTXOs
if ($Verify) {
  $txs = $txsResp.transactions
  $sum = 0
  foreach ($t in $txs) {
    if ($t.network -eq "btc" -and $t.asset -eq "BTC") {
      $sum += [int64]$t.amount
    }
  }

  $btcBal = 0
  foreach ($b in $balResp2.balances) {
    if ($b.network -eq "btc" -and $b.asset -eq "BTC") {
      $btcBal = [int64]$b.available
    }
  }

  if ($btcBal -ne $sum) {
    Write-Log "VERIFY FAIL: balance=$btcBal sum_txs=$sum"
    throw "Verify failed: balance does not match tx sum"
  } else {
    Write-Log "VERIFY OK: balance=$btcBal sum_txs=$sum"
  }
}

Write-Log "Done"
