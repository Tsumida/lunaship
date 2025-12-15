#!/usr/bin/env bash
set -Eeuo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.ci.yml}"
PROJECT_NAME="ci-itest"
WAIT_TIMEOUT=30
SERVICE_NAME="redis"

log() {
  echo "[$(date '+%H:%M:%S')] $*"
}

cleanup() {
  log "Cleaning up docker compose"
  # 使用 -v 确保移除匿名卷，--remove-orphans 确保清理所有未跟踪的容器
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans
}

# 确保在脚本退出、中断或终止时执行清理
trap cleanup EXIT INT TERM

# 启动
log "Starting docker compose services"
docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --quiet-pull

# 列出服务状态，便于调试
docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" ps

log "Waiting for services to be healthy"

# 获取容器ID
REDIS_CONTAINER_ID=$(docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" ps -q "$SERVICE_NAME")

if [ -z "$REDIS_CONTAINER_ID" ]; then
    log "Error: Container ID for service '$SERVICE_NAME' not found."
    exit 1
fi

start_ts=$(date +%s)
while true; do
    # 检查 Redis 容器的健康状态
    HEALTH_STATUS=$(docker inspect --format='{{.State.Health.Status}}' "$REDIS_CONTAINER_ID" 2>/dev/null || echo "not running")

    if [[ "$HEALTH_STATUS" == "healthy" ]]; then
        log "Service '$SERVICE_NAME' is healthy."
        break
    fi
    
    log "Current status of $SERVICE_NAME: $HEALTH_STATUS"

    if (( $(date +%s) - start_ts > WAIT_TIMEOUT )); then
        log "Timeout waiting for service '$SERVICE_NAME' to become healthy."
        docker logs "$REDIS_CONTAINER_ID"
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