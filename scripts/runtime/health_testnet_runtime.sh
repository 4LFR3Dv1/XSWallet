#!/usr/bin/env bash
set -euo pipefail

API_PORT="${API_PORT:-3000}"
RPC_USER="${BITCOIND_RPC_USER:-xsrpc}"
RPC_PASSWORD="${BITCOIND_RPC_PASSWORD:-troque_essa_senha}"

echo "== Bridge health =="
curl -sf "http://127.0.0.1:${API_PORT}/health" && echo

echo "== Bridge readiness =="
curl -sf "http://127.0.0.1:${API_PORT}/ready" && echo

echo "== Bitcoin RPC =="
curl -sf --user "${RPC_USER}:${RPC_PASSWORD}" \
  --data-binary '{"jsonrpc":"1.0","id":"ping","method":"getblockchaininfo","params":[]}' \
  -H 'content-type:text/plain;' \
  http://127.0.0.1:18332/ && echo

echo "== Swaps endpoint =="
curl -sf "http://127.0.0.1:${API_PORT}/api/v1/swaps?access_token=dev" && echo
