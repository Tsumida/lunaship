#!/usr/bin/env bash
set -euo pipefail

curl -sS -X POST \
	-H "Content-Type: application/json" \
	-H "Connect-Protocol-Version: 1" \
	--data '{"target":"127.0.0.1:8081","endpoint":"/logs.v1.DummyService/Ping","request_json":"{}"}' \
	http://127.0.0.1:8080/logs.v1.DummyService/Transfer
