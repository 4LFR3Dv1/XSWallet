param(
  [string]$WalletBaseUrl = "http://localhost:8081",
  [string]$PayoutBaseUrl = "http://localhost:8090",
  [string]$Network = "btc",
  [string]$Asset = "BTC",
  [int]$AmountSats = 10000,
  [string]$InternalToken = $env:WALLET_INTERNAL_TOKEN,
  [string]$BtcRpcUser = $(if ($env:BITCOIN_RPC_USER) { $env:BITCOIN_RPC_USER } elseif ($env:BTC_RPC_USER) { $env:BTC_RPC_USER } else { "domni" }),
  [string]$BtcRpcPass = $(if ($env:BITCOIN_RPC_PASSWORD) { $env:BITCOIN_RPC_PASSWORD } elseif ($env:BTC_RPC_PASS) { $env:BTC_RPC_PASS } else { "domni_regtest_pw" }),
  [string]$BtcWallet = $(if ($env:BTC_RPC_WALLET) { $env:BTC_RPC_WALLET } else { "domniwallet" }),
  [switch]$Verify
)

$ErrorActionPreference = "Stop"

function LogLine {
  param([string]$Message)
  $ts = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss")
  $line = "$ts $Message"
  Add-Content -Path $LogFile -Value $line
  Write-Host $line
}

function InvokeJson {
  param(
    [string]$Method,
    [string]$Url,
    [object]$Body = $null,
    [hashtable]$Headers = $null
  )
  $params = @{ Method = $Method; Uri = $Url }
  if ($Headers) { $params.Headers = $Headers }
  if ($Body -ne $null) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Depth 6)
  }
  return Invoke-RestMethod @params
}

function DockerBitcoinCli {
  param([string[]]$CmdArgs)
  $walletArg = ""
  if ($BtcWallet) { $walletArg = "-rpcwallet=$BtcWallet" }
  $cmd = @("exec", "-i", "domni-bitcoind-regtest", "bitcoin-cli", "-regtest", "-rpcuser=$BtcRpcUser", "-rpcpassword=$BtcRpcPass")
  if ($walletArg) { $cmd += $walletArg }
  $cmd += $CmdArgs
  $out = & docker @cmd
  return $out
}

function DockerBitcoinCliNamed {
  param([string[]]$CmdArgs)
  $walletArg = ""
  if ($BtcWallet) { $walletArg = "-rpcwallet=$BtcWallet" }
  $cmd = @("exec", "-i", "domni-bitcoind-regtest", "bitcoin-cli", "-regtest", "-rpcuser=$BtcRpcUser", "-rpcpassword=$BtcRpcPass")
  if ($walletArg) { $cmd += $walletArg }
  $cmd += "-named"
  $cmd += $CmdArgs
  $out = & docker @cmd
  return $out
}

$LogDir = "C:\Users\windows10\Downloads\DomniWallet\payout-service\logs"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
$LogFile = Join-Path $LogDir "payout_service_e2e.log"
$ReportStamp = (Get-Date).ToString("yyyyMMdd_HHmmss")
$ReportFile = Join-Path $LogDir "payout_service_e2e_report_$ReportStamp.html"

$destValueSat = $null
$feeSat = $null
$sumInputsSat = $null
$sumOutputsSat = $null
$changeRows = @()
$destScriptOk = $false
$changeScriptOk = $false
$balanceAvailable = $null

LogLine "Starting payout-service e2e test"
LogLine "WalletBaseUrl=$WalletBaseUrl"
LogLine "PayoutBaseUrl=$PayoutBaseUrl"
LogLine "Network=$Network Asset=$Asset AmountSats=$AmountSats"
LogLine "LogFile=$LogFile"

$accountUuid = [guid]::NewGuid().ToString()
LogLine "AccountUuid=$accountUuid"

$headers = @{}
if ($InternalToken) {
  $headers["X-Internal-Token"] = $InternalToken
}

# 1) Create account
$accResp = InvokeJson -Method Post -Url "$WalletBaseUrl/v1/accounts" -Body @{ uuid = $accountUuid }
LogLine "CreateAccount response=$(ConvertTo-Json $accResp -Compress)"

# 2) Create address (wallet-managed)
$addrResp = InvokeJson -Method Post -Url "$WalletBaseUrl/v1/accounts/$accountUuid/addresses" -Body @{ network = $Network; asset = $Asset }
$depositAddr = $addrResp.address
LogLine "Deposit address=$depositAddr"

# 3) Mine 101 blocks to make coinbase spendable
LogLine "Mining 101 blocks to fund deposit address"
$null = DockerBitcoinCliNamed @("generatetoaddress", "nblocks=101", "address=$depositAddr")

# 4) Fetch UTXO from bitcoind
$utxosJson = DockerBitcoinCli @("listunspent", "1", "9999999")
$utxos = $utxosJson | ConvertFrom-Json
$utxosForAddr = @($utxos | Where-Object { $_.address -eq $depositAddr })
if (!$utxosForAddr -or $utxosForAddr.Count -eq 0) {
  throw "No UTXO found for address $depositAddr"
}
$utxo = $utxosForAddr[0]
$amountSat = [int64]([math]::Round($utxo.amount * 1e8))
LogLine ("UTXO txid={0} vout={1} amount_btc={2}" -f $utxo.txid, $utxo.vout, $utxo.amount)

# 5) Upsert UTXO into wallet-service (watcher simulation)
$upsertBody = @{
  network       = $Network
  asset         = $Asset
  txid          = $utxo.txid
  vout          = [int64]$utxo.vout
  address       = $depositAddr
  amount        = $amountSat
  confirmations = [int]$utxo.confirmations
  confirmed_at  = (Get-Date).ToUniversalTime().ToString("o")
  block_hash    = ""
  block_height  = 0
}
$upsertResp = InvokeJson -Method Post -Url "$WalletBaseUrl/v1/internal/utxos" -Body $upsertBody -Headers $headers
LogLine "UpsertUTXO response=$(ConvertTo-Json $upsertResp -Compress)"

# 6) Create withdrawal
$destAddr = DockerBitcoinCli @("getnewaddress", "payout", "bech32m")
$withdrawResp = InvokeJson -Method Post -Url "$WalletBaseUrl/v1/withdrawals" -Body @{
  account_uuid = $accountUuid
  network      = $Network
  asset        = $Asset
  amount       = $AmountSats
  destination  = $destAddr.Trim()
}
LogLine "CreateWithdrawal response=$(ConvertTo-Json $withdrawResp -Compress)"
$withdrawalId = $withdrawResp.id

# 7) Create payout
$paymentId = [guid]::NewGuid().ToString()
$payoutResp = InvokeJson -Method Post -Url "$PayoutBaseUrl/v1/payouts" -Body @{
  payment_id    = $paymentId
  withdrawal_id = $withdrawalId
  network       = $Network
  asset         = $Asset
  amount_sats   = $AmountSats
  destination   = $destAddr.Trim()
  priority      = "normal"
}
LogLine "CreatePayout response=$(ConvertTo-Json $payoutResp -Compress)"
$payoutId = $payoutResp.id

# 8) Poll status, mine 1 block once it is confirming
$confirmed = $false
for ($i = 0; $i -lt 15; $i++) {
  Start-Sleep -Seconds 2
  $status = InvokeJson -Method Get -Url "$PayoutBaseUrl/v1/payouts/$payoutId"
  LogLine "Payout status=$(ConvertTo-Json $status -Compress)"
  if ($status.status -eq "CONFIRMING" -and -not $confirmed) {
    $mineAddr = DockerBitcoinCli @("getnewaddress", "miner", "bech32m")
    LogLine "Mining 1 block to confirm tx"
    $null = DockerBitcoinCliNamed @("generatetoaddress", "nblocks=1", "address=$($mineAddr.Trim())")
    $confirmed = $true
  }
  if ($status.status -eq "COMPLETED") {
    break
  }
  if ($status.status -eq "FAILED_FINAL") {
    throw "Payout failed final"
  }
}

# 9) Final status
$final = InvokeJson -Method Get -Url "$PayoutBaseUrl/v1/payouts/$payoutId"
LogLine "Final status=$(ConvertTo-Json $final -Compress)"

# 10) Verify on-chain outputs
if ($final.txid) {
  $rawTxJson = DockerBitcoinCli @("getrawtransaction", $final.txid, "true")
  $rawTx = $rawTxJson | ConvertFrom-Json
  $destVout = @($rawTx.vout | Where-Object { $_.scriptPubKey.address -eq $destAddr -or ($_.scriptPubKey.addresses -contains $destAddr) })
  if (!$destVout -or $destVout.Count -eq 0) {
    throw "Destination output not found in tx $($final.txid)"
  }
  $destValueSat = [int64]([math]::Round($destVout[0].value * 1e8))
  if ($destValueSat -ne $AmountSats) {
    throw "Destination amount mismatch: got $destValueSat want $AmountSats"
  }
  LogLine "Tx output OK: destination=$destAddr amount_sats=$destValueSat"

  $destInfoJson = DockerBitcoinCli @("getaddressinfo", $destAddr)
  $destInfo = $destInfoJson | ConvertFrom-Json
  if ($destInfo.scriptPubKey) {
    $expectedDest = $destInfo.scriptPubKey.ToString().ToLower()
    $gotDest = $destVout[0].scriptPubKey.hex
    if ($gotDest) {
      $gotDestHex = $gotDest.ToString().ToLower()
      if ($gotDestHex -ne $expectedDest) {
        throw ("Destination scriptPubKey mismatch: got {0} want {1}" -f $gotDestHex, $expectedDest)
      }
      $destScriptOk = $true
      LogLine "Tx destination scriptPubKey OK: address=$destAddr"
    }
  }

  $sumOutputsSat = 0
  foreach ($vout in $rawTx.vout) {
    $sumOutputsSat += [int64]([math]::Round($vout.value * 1e8))
  }
  $sumInputsSat = 0
  foreach ($vin in $rawTx.vin) {
    $prevJson = DockerBitcoinCli @("getrawtransaction", $vin.txid, "true")
    $prevTx = $prevJson | ConvertFrom-Json
    $prevVout = $prevTx.vout | Where-Object { $_.n -eq $vin.vout } | Select-Object -First 1
    if ($prevVout) {
      $sumInputsSat += [int64]([math]::Round($prevVout.value * 1e8))
    }
  }
  $feeSat = $sumInputsSat - $sumOutputsSat
  LogLine "Tx fee (multi input) fee_sats=$feeSat sum_outputs_sats=$sumOutputsSat sum_inputs_sats=$sumInputsSat"

  $changeVouts = @($rawTx.vout | Where-Object { $_.scriptPubKey.address -ne $destAddr -and -not ($_.scriptPubKey.addresses -contains $destAddr) })
  if ($changeVouts.Count -gt 0) {
    foreach ($cv in $changeVouts) {
      $cvAddr = $cv.scriptPubKey.address
      if (-not $cvAddr -and $cv.scriptPubKey.addresses) {
        $cvAddr = $cv.scriptPubKey.addresses[0]
      }
      $cvValueSat = [int64]([math]::Round($cv.value * 1e8))
      LogLine "Tx change output: address=$cvAddr amount_sats=$cvValueSat"
      if ($cvAddr) {
        $changeRows += [pscustomobject]@{ address = $cvAddr; amount_sats = $cvValueSat }
      }
      if ($cvAddr) {
        $addrInfoJson = DockerBitcoinCli @("getaddressinfo", $cvAddr)
        $addrInfo = $addrInfoJson | ConvertFrom-Json
        if ($addrInfo.scriptPubKey) {
          $expected = $addrInfo.scriptPubKey.ToString().ToLower()
          $got = $cv.scriptPubKey.hex
          if ($got) {
            $gotHex = $got.ToString().ToLower()
            if ($gotHex -ne $expected) {
              throw ("Change scriptPubKey mismatch for {0}: got {1} want {2}" -f $cvAddr, $gotHex, $expected)
            }
            $changeScriptOk = $true
            LogLine "Tx change scriptPubKey OK: address=$cvAddr"
          }
        }
      }
    }
  } else {
    LogLine "Tx change output: none (fee absorbed)"
  }
}

if ($Verify) {
  $balances = InvokeJson -Method Get -Url "$WalletBaseUrl/v1/accounts/$accountUuid/balances"
  LogLine "Balances=$(ConvertTo-Json $balances -Compress)"
  if ($balances.balances -and $balances.balances.Count -gt 0) {
    $balanceAvailable = $balances.balances[0].available
  }
}

@"
<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>DomniWallet - Payout E2E Report</title>
  <style>
    body { font-family: Arial, sans-serif; margin: 24px; color: #111; }
    h1 { font-size: 20px; border-bottom: 2px solid #F7931A; padding-bottom: 6px; }
    table { border-collapse: collapse; width: 100%; margin: 12px 0; }
    th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
    th { background: #f3f3f3; }
    code { background: #f5f5f5; padding: 2px 4px; border-radius: 3px; }
  </style>
</head>
<body>
  <h1>Payout E2E Report</h1>
  <table>
    <tr><th>Data</th><td>$(Get-Date -Format "yyyy-MM-dd HH:mm:ss")</td></tr>
    <tr><th>Account UUID</th><td>$accountUuid</td></tr>
    <tr><th>Withdrawal ID</th><td>$withdrawalId</td></tr>
    <tr><th>Payout ID</th><td>$payoutId</td></tr>
    <tr><th>Payment ID</th><td>$paymentId</td></tr>
    <tr><th>TXID</th><td>$($final.txid)</td></tr>
    <tr><th>Destination</th><td>$destAddr</td></tr>
    <tr><th>Amount (sats)</th><td>$AmountSats</td></tr>
    <tr><th>Fee (sats)</th><td>$feeSat</td></tr>
    <tr><th>Sum Inputs (sats)</th><td>$sumInputsSat</td></tr>
    <tr><th>Sum Outputs (sats)</th><td>$sumOutputsSat</td></tr>
    <tr><th>Dest scriptPubKey OK</th><td>$destScriptOk</td></tr>
    <tr><th>Change scriptPubKey OK</th><td>$changeScriptOk</td></tr>
    <tr><th>Balance available (sats)</th><td>$balanceAvailable</td></tr>
  </table>

  <h2>Change Outputs</h2>
  <table>
    <tr><th>Address</th><th>Amount (sats)</th></tr>
    $(if ($changeRows.Count -gt 0) { ($changeRows | ForEach-Object { "<tr><td>$($_.address)</td><td>$($_.amount_sats)</td></tr>" }) -join "`n" } else { "<tr><td colspan='2'>none</td></tr>" })
  </table>
</body>
</html>
"@ | Set-Content -Path $ReportFile -Encoding UTF8

LogLine "Report generated: $ReportFile"

LogLine "Done"
