#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="${ROOT}/tmp/logs-server"

mkdir -p "${ROOT}/tmp"
go build -o "${BIN}" "${ROOT}/example/logs"

LOG_DIR_A="${ROOT}/tmp/logs-a"
LOG_DIR_B="${ROOT}/tmp/logs-b"
mkdir -p "${LOG_DIR_A}" "${LOG_DIR_B}"

BIND_ADDR=":8080" LOG_FILE="${LOG_DIR_A}/log.log" ERR_FILE="${LOG_DIR_A}/err.log" "${BIN}" &
PID_A=$!

BIND_ADDR=":8081" LOG_FILE="${LOG_DIR_B}/log.log" ERR_FILE="${LOG_DIR_B}/err.log" "${BIN}" &
PID_B=$!

echo "service A: http://127.0.0.1:8080 (pid ${PID_A}) log: ${LOG_DIR_A}/log.log"
echo "service B: http://127.0.0.1:8081 (pid ${PID_B}) log: ${LOG_DIR_B}/log.log"
echo "Press Ctrl+C to stop both services."

trap 'kill ${PID_A} ${PID_B}' EXIT INT TERM
wait
