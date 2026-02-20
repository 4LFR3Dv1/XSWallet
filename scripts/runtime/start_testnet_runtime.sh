#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/.runtime"
mkdir -p "${RUNTIME_DIR}"

BITCOIND_RPC_USER="${BITCOIND_RPC_USER:-xsrpc}"
BITCOIND_RPC_PASSWORD="${BITCOIND_RPC_PASSWORD:-troque_essa_senha}"
BITCOIND_DATA_DIR="${BITCOIND_DATA_DIR:-${HOME}/.bitcoin}"
XSCORE_CONFIG="${XSCORE_CONFIG:-${ROOT_DIR}/core/config.local.testnet.pruned.json}"
GRPC_HOST="${GRPC_HOST:-127.0.0.1:9735}"
API_PORT="${API_PORT:-3000}"

require_cmd() {
  local c="$1"
  if ! command -v "${c}" >/dev/null 2>&1; then
    echo "Missing command: ${c}"
    exit 1
  fi
}

is_listening() {
  local port="$1"
  lsof -tiTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
}

start_bitcoind_if_needed() {
  if is_listening 18332; then
    echo "bitcoind already listening on :18332"
    return
  fi
  require_cmd bitcoind
  echo "Starting bitcoind testnet pruned..."
  bitcoind \
    -daemon \
    -testnet=1 \
    -server=1 \
    -prune=550 \
    -rpcbind=127.0.0.1 \
    -rpcallowip=127.0.0.1 \
    -rpcport=18332 \
    -rpcuser="${BITCOIND_RPC_USER}" \
    -rpcpassword="${BITCOIND_RPC_PASSWORD}" \
    -datadir="${BITCOIND_DATA_DIR}"
}

start_xscore() {
  if is_listening 9735; then
    echo "xscore already listening on :9735"
    return
  fi
  require_cmd go
  echo "Starting xscore (testnet)..."
  nohup go run ./cmd/xscore --network=testnet --port=9735 --config "${XSCORE_CONFIG}" \
    > "${RUNTIME_DIR}/xscore.log" 2>&1 &
  echo $! > "${RUNTIME_DIR}/xscore.pid"
}

start_api_bridge() {
  if is_listening "${API_PORT}"; then
    echo "api-bridge already listening on :${API_PORT}"
    return
  fi
  require_cmd npm
  echo "Starting api-bridge on :${API_PORT}..."
  (
    cd "${ROOT_DIR}/api-bridge"
    nohup env GRPC_HOST="${GRPC_HOST}" PORT="${API_PORT}" npm start \
      > "${RUNTIME_DIR}/api-bridge.log" 2>&1 &
    echo $! > "${RUNTIME_DIR}/api-bridge.pid"
  )
}

echo "Booting testnet runtime (single instance)..."
"${ROOT_DIR}/scripts/runtime/stop_testnet_runtime.sh" || true
start_bitcoind_if_needed
sleep 1
start_xscore
sleep 2
start_api_bridge

echo "Started. Health checks:"
echo "  curl -sf http://127.0.0.1:${API_PORT}/health"
echo "  curl -sf http://127.0.0.1:${API_PORT}/ready"
echo "Logs:"
echo "  tail -f ${RUNTIME_DIR}/xscore.log"
echo "  tail -f ${RUNTIME_DIR}/api-bridge.log"
