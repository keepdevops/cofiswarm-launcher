#!/usr/bin/env bash
set -euo pipefail
PORT="${CONFIGURE_TEST_PORT:-18017}"
BIN="$(cd "$(dirname "$0")/../.." && pwd)/bin/cofiswarm-configure"
"$BIN" -listen ":${PORT}" &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT
for _ in $(seq 1 20); do
  curl -sf "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1 && break
  sleep 0.2
done
curl -sf "http://127.0.0.1:${PORT}/api/configure/status" | grep -q '"active"'
echo "ok: configure API"
