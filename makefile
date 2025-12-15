PROJECT_NAME := tex
PROJECT_ROOT := $(shell pwd)
REPO_PREFIX := ""

DOCKER_COMPOSE_FILE := $(PROJECT_ROOT)/tests/docker-compose.yaml

# ==============================================================================
# 💾 本地开发
# ==============================================================================
list-env:
	@find ./ -type f -name "*.go" \
	-exec grep -oP 'os.Getenv\(\K"[A-Z_]+"(?=\))' {} \; \
	| sort \
	| uniq \
	| awk '{gsub(/"/, "", $$0); print "ENV", $$0 "=\"\""}'

list-redis-key:
	@find ./ -type f -name "*.go" \
	-exec grep -oP 'REDIS_KEY_[A-Z0-9_]+\s*=\s*"[a-z_\-]+"' {} \; \
	| awk -F'"' '{gsub(/REDIS_KEY_/, "", $$1); print $$2}'

pb:
	@buf generate -v --path ./api

ut:
	@echo "work_dir=${PROJECT_ROOT}"
	@mkdir -p ${PROJECT_ROOT}/tmp
	@touch ${PROJECT_ROOT}/tmp/coverage.out
	@chmod +x ${PROJECT_ROOT}/tmp/coverage.out 
	@go test -v -count=1 -gcflags=all=-l -coverprofile=${PROJECT_ROOT}/tmp/coverage.out ./...
	@go tool cover -func=${PROJECT_ROOT}/tmp/coverage.out | grep total | awk '{print "Total Coverage: " $$3}'

test:
	@docker-compose -f $(DOCKER_COMPOSE_FILE) down  && sleep 2 && docker-compose -f $(DOCKER_COMPOSE_FILE) up -d && sleep 3
	@echo "Running integration tests" && go test -v -count=1 -gcflags=all=-l ./tests/...