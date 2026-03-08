#!/usr/bin/env bash
set -euo pipefail

TARGET_ADDR="${TARGET_ADDR:-127.0.0.1:8081}"
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
METRICS_URL="${METRICS_URL:-http://127.0.0.1:9090/metrics}"

curl -sS -X POST \
	-H "Content-Type: application/json" \
	-H "Connect-Protocol-Version: 1" \
	--data "{\"target_addr\":\"${TARGET_ADDR}\",\"endpoint\":\"/logs.v1.DummyService/Ping\",\"request_json\":\"{}\"}" \
	"${BASE_URL}/logs.v1.DummyService/Transfer"
echo

curl -sS -X POST \
	-H "Content-Type: application/json" \
	-H "Connect-Protocol-Version: 1" \
	--data '{"limit":5,"offset":0}' \
	"${BASE_URL}/logs.v1.DummyService/GetSpot"
echo

curl -sS "${METRICS_URL}" | grep -E '^(go_|rpc_server_|lunaship_)' | head -n 30
