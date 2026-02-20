#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/.runtime"
mkdir -p "${RUNTIME_DIR}"

kill_port_if_busy() {
  local port="$1"
  local pids
  pids="$(lsof -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -n "${pids}" ]]; then
    echo "Stopping listeners on :${port} (pid: ${pids})"
    kill ${pids} || true
  fi
}

kill_pid_file() {
  local pid_file="$1"
  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}" || true)"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      echo "Stopping process pid=${pid} from ${pid_file}"
      kill "${pid}" || true
    fi
    rm -f "${pid_file}"
  fi
}

kill_pid_file "${RUNTIME_DIR}/xscore.pid"
kill_pid_file "${RUNTIME_DIR}/api-bridge.pid"
kill_pid_file "${RUNTIME_DIR}/bitcoind.pid"

# Last safety net: clean common ports.
kill_port_if_busy 9735
kill_port_if_busy 3000
kill_port_if_busy 18332

echo "Runtime stopped (testnet)."
