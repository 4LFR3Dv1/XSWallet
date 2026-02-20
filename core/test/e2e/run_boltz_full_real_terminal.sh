#!/usr/bin/env bash
set -euo pipefail

# Runs chain + reverse watcher integrations in full-real mode (no simulation)
# until terminal states, using Boltz testnet.
#
# Required env:
#   XS_BOLTZ_CHAIN_EXEC_INTEGRATION=1
#   XS_BOLTZ_CHAIN_EXEC_FULL_REAL=1
#   XS_BOLTZ_REVERSE_EXEC_INTEGRATION=1
#   XS_BOLTZ_REVERSE_EXEC_FULL_REAL=1
#
# Optional:
#   XS_BOLTZ_API_URL (default: https://api.testnet.boltz.exchange)
#   XS_BOLTZ_WS_URL
#   XS_BOLTZ_CHAIN_EXEC_WAIT_SECONDS (default: 1800)
#   XS_BOLTZ_REVERSE_EXEC_WAIT_SECONDS (default: 1800)
#   XS_BOLTZ_CHAIN_EXEC_POLL_SECONDS (default: 10)
#   XS_BOLTZ_REVERSE_EXEC_POLL_SECONDS (default: 10)
#   XS_BOLTZ_CHAIN_EXEC_EXPECT_TERMINAL (completed|refunding, default: completed)
#   XS_BOLTZ_REVERSE_EXEC_EXPECT_TERMINAL (completed|refunding, default: completed)

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

export XS_BOLTZ_API_URL="${XS_BOLTZ_API_URL:-https://api.testnet.boltz.exchange}"
export XS_BOLTZ_CHAIN_EXEC_WAIT_SECONDS="${XS_BOLTZ_CHAIN_EXEC_WAIT_SECONDS:-1800}"
export XS_BOLTZ_REVERSE_EXEC_WAIT_SECONDS="${XS_BOLTZ_REVERSE_EXEC_WAIT_SECONDS:-1800}"
export XS_BOLTZ_CHAIN_EXEC_POLL_SECONDS="${XS_BOLTZ_CHAIN_EXEC_POLL_SECONDS:-10}"
export XS_BOLTZ_REVERSE_EXEC_POLL_SECONDS="${XS_BOLTZ_REVERSE_EXEC_POLL_SECONDS:-10}"
export XS_BOLTZ_CHAIN_EXEC_EXPECT_TERMINAL="${XS_BOLTZ_CHAIN_EXEC_EXPECT_TERMINAL:-completed}"
export XS_BOLTZ_REVERSE_EXEC_EXPECT_TERMINAL="${XS_BOLTZ_REVERSE_EXEC_EXPECT_TERMINAL:-completed}"
export XS_BOLTZ_CHAIN_EXEC_INTEGRATION=1
export XS_BOLTZ_CHAIN_EXEC_FULL_REAL=1
export XS_BOLTZ_REVERSE_EXEC_INTEGRATION=1
export XS_BOLTZ_REVERSE_EXEC_FULL_REAL=1

# Ensure simulation flags are clean in full-real mode.
unset XS_BOLTZ_CHAIN_EXEC_SIMULATE_TRIGGER XS_BOLTZ_CHAIN_EXEC_SIMULATE_TERMINAL
unset XS_BOLTZ_REVERSE_EXEC_SIMULATE_TRIGGER XS_BOLTZ_REVERSE_EXEC_SIMULATE_TERMINAL

echo "[1/2] Chain full-real terminal integration"
go test ./internal/watcher -run TestChainExecutionToWaitingProviderBroadcastIntegration -v -count=1

echo "[2/2] Reverse full-real terminal integration"
go test ./internal/watcher -run TestReverseExecutionToWaitingProviderBroadcastIntegration -v -count=1

echo "OK: chain + reverse full-real terminal integrations completed."
