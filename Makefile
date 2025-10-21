# Makefile for building and pushing service Docker images
# Usage:
#   make build-all        # build all service images
#   make push-all         # push all service images (after docker login)
#   make build-<service>  # build a single service (e.g. make build-task-service)
#   make push-<service>   # push a single service image
#   make gen-mongo-uri    # prints PowerShell command to generate base64 MongoDB URI

REGISTRY ?= shimulmahmud
TAG ?= latest
DOCKER ?= docker

SERVICES := api-gateway task-service user-service notification-service analytics-service

.PHONY: all help build-all push-all clean login gen-mongo-uri $(SERVICES)

all: build-all

help:
	@echo "Makefile targets:"
	@echo "  build-all       Build all service images"
	@echo "  push-all        Push all service images to $(REGISTRY)"
	@echo "  build-<service> Build a single service image (e.g. make build-task-service)"
	@echo "  push-<service>  Push a single service image"
	@echo "  gen-mongo-uri   Print PowerShell command to generate base64 MongoDB URI"

build-all: $(addprefix build-,$(SERVICES))

push-all: $(addprefix push-,$(SERVICES))

login:
	@$(DOCKER) login

gen-mongo-uri:
	@echo "PowerShell command to generate base64 MongoDB URI (run in pwsh):"
	@echo "$username = 'admin' # replace with your user"
	@echo "$password = 'password123' # replace with your password"
	@echo "$uri = \"mongodb://$username:$password@mongodb:27017/todo_app?authSource=admin\""
	@echo "[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($uri))"

clean:
	@echo "No-op: remove local images manually if desired"

# Per-service build/push rules (pattern rules)
build-%:
	@echo "Building $* -> $(REGISTRY)/$*:$(TAG)"
	@$(DOCKER) build -t $(REGISTRY)/$*:$(TAG) -f $*/Dockerfile .

push-%:
	@echo "Pushing $* -> $(REGISTRY)/$*:$(TAG)"
	@$(DOCKER) push $(REGISTRY)/$*:$(TAG)
