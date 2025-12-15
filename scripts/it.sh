#!/usr/bin/env bash
set -Eeuo pipefail

#######################################
# Config
#######################################
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.ci.yml}"
PROJECT_NAME="ci-itest"
WAIT_TIMEOUT=30

#######################################
# Utils
#######################################
log() {
  echo "[$(date '+%H:%M:%S')] $*"
}

cleanup() {
  log "Cleaning up docker compose"
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans
}

trap cleanup EXIT INT TERM

#######################################
# Start services
#######################################
log "Starting docker compose services"
docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --quiet-pull

docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" ps

#######################################
# Wait for healthy
#######################################
log "Waiting for services to be healthy"

start_ts=$(date +%s)
while true; do
  unhealthy=$(
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" ps \
      --format json | jq -r '.[] | select(.Health!="healthy") | .Name'
  )

  if [[ -z "$unhealthy" ]]; then
    log "All services are healthy"
    break
  fi

  if (( $(date +%s) - start_ts > WAIT_TIMEOUT )); then
    log "Timeout waiting for services:"
    echo "$unhealthy"
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" logs
    exit 1
  fi

  sleep 1
done

#######################################
# Run tests
#######################################
log "Running Go integration tests"
go test -v -count=1 -gcflags=all=-l ./tests/...

log "Integration tests finished successfully"