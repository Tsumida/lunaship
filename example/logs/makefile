PROJECT_ROOT := $(shell cd $(dir $(lastword $(MAKEFILE_LIST)))/../.. && pwd)
IMAGE_NAME := lunaship-logs-demo:latest
IMAGE_TAR_DIR := $(PROJECT_ROOT)/tmp
IMAGE_TAR := $(IMAGE_TAR_DIR)/lunaship-logs-demo.tar
REMOTE_TARGET ?= tsumida@192.168.0.120
REMOTE_DIR := /home/tsumida/image_cache
REMOTE_TAR := $(REMOTE_DIR)/$(notdir $(IMAGE_TAR))
SSH_PORT ?= 22
SCP_FLAGS ?= -O
SSH_FLAGS ?=
SUDO ?= sudo -n
KUBECTL := kubectl
NAMESPACE := test
TOOL_IMAGE ?= busybox:1.37.0
IMPORT ?= 0
LOCAL_COMPOSE := $(PROJECT_ROOT)/example/logs/docker-compose.yaml

pb:
	@buf generate -v

build:
	@mkdir -p $(IMAGE_TAR_DIR)
	@docker build -t $(IMAGE_NAME) -f $(PROJECT_ROOT)/example/logs/Dockerfile $(PROJECT_ROOT)
	@docker save $(IMAGE_NAME) -o $(IMAGE_TAR)
	@echo "saved image to $(IMAGE_TAR)"

upload:
	@test -f $(IMAGE_TAR) || (echo "missing $(IMAGE_TAR); run 'make build' first" && exit 1)
	@scp $(SCP_FLAGS) -q -P $(SSH_PORT) $(IMAGE_TAR) $(REMOTE_TARGET):$(REMOTE_TAR)
	@echo "uploaded image to $(REMOTE_TARGET):$(REMOTE_TAR)"

import: upload
	@ssh $(SSH_FLAGS) -p $(SSH_PORT) $(REMOTE_TARGET) "$(SUDO) k3s ctr images import $(REMOTE_TAR)"
	@echo "imported image on $(REMOTE_TARGET)"

mysql-up:
	@docker-compose -f $(LOCAL_COMPOSE) up -d mysql
	@echo "mysql is starting from $(LOCAL_COMPOSE)"

mysql-down:
	@docker-compose -f $(LOCAL_COMPOSE) stop mysql
	@echo "mysql stopped"

redis-up:
	@docker-compose -f $(LOCAL_COMPOSE) up -d redis
	@echo "redis is starting from $(LOCAL_COMPOSE)"

redis-down:
	@docker-compose -f $(LOCAL_COMPOSE) stop redis
	@echo "redis stopped"

demo-logs:
	@$(MAKE) -f $(lastword $(MAKEFILE_LIST)) mysql-up
	@$(MAKE) -f $(lastword $(MAKEFILE_LIST)) redis-up
ifeq ($(IMPORT),1)
	@$(MAKE) import
else
	@echo "IMPORT=0: skipping image import"
endif
	@$(KUBECTL) apply -f $(PROJECT_ROOT)/example/logs/deploy/namespace.yaml
	@$(KUBECTL) -n $(NAMESPACE) apply -f $(PROJECT_ROOT)/example/logs/deploy/jaeger-traefik.yaml
	@$(KUBECTL) -n $(NAMESPACE) apply -f $(PROJECT_ROOT)/example/logs/deploy/logs-demo.yaml
	@$(KUBECTL) -n $(NAMESPACE) rollout restart deployment/logs-demo-a
	@$(KUBECTL) -n $(NAMESPACE) rollout restart deployment/logs-demo-b
	@$(KUBECTL) -n $(NAMESPACE) rollout status deployment/logs-demo-a
	@$(KUBECTL) -n $(NAMESPACE) rollout status deployment/logs-demo-b
	@POD_A_IP=$$($(KUBECTL) -n $(NAMESPACE) get pod -l app=logs-demo,role=a -o jsonpath='{.items[0].status.podIP}'); \
	POD_B_IP=$$($(KUBECTL) -n $(NAMESPACE) get pod -l app=logs-demo,role=b -o jsonpath='{.items[0].status.podIP}'); \
	if [ -z "$$POD_A_IP" ] || [ -z "$$POD_B_IP" ]; then echo "missing logs-demo pod IPs"; exit 1; fi; \
	JSON_OK=$$(printf '{"target":"%s:8080","endpoint":"/logs.v1.DummyService/Ping","request_json":"{}"}' "$$POD_B_IP"); \
	$(KUBECTL) -n $(NAMESPACE) run curl-logs-demo --image=$(TOOL_IMAGE) --restart=Never --rm -i -- \
		sh -c "wget -qO- --header \"Content-Type: application/json\" --header \"Connect-Protocol-Version: 1\" --post-data '$$JSON_OK' http://$$POD_A_IP:8080/logs.v1.DummyService/Transfer"; \
	JSON_ERR=$$(printf '{"target":"logs-demo-bad:8080","endpoint":"/logs.v1.DummyService/Ping","request_json":"{}"}'); \
	$(KUBECTL) -n $(NAMESPACE) run curl-logs-demo-error --image=$(TOOL_IMAGE) --restart=Never --rm -i -- \
		sh -c "wget -qO- --header \"Content-Type: application/json\" --header \"Connect-Protocol-Version: 1\" --post-data '$$JSON_ERR' http://$$POD_A_IP:8080/logs.v1.DummyService/Transfer || true"; \
	JSON_DB=$$(printf '{"limit":5,"offset":0}'); \
	$(KUBECTL) -n $(NAMESPACE) run curl-logs-demo-mysql --image=$(TOOL_IMAGE) --restart=Never --rm -i -- \
		sh -c "wget -qO- --header \"Content-Type: application/json\" --header \"Connect-Protocol-Version: 1\" --post-data '$$JSON_DB' http://$$POD_A_IP:8080/logs.v1.DummyService/GetSpot"
