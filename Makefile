IMAGE_NAME := keel
AGENT ?= claude
TAG ?= latest
TOOL_VERSION ?= latest
KEEL_BUILD ?= dev
CONTAINER_NAME ?= $(IMAGE_NAME)-$(AGENT)-$(notdir $(CURDIR))
SA_KEY ?= $(HOME)/.config/gcloud/application_default_credentials.json
PROJECT_ID ?= $(ANTHROPIC_VERTEX_PROJECT_ID)
REGION ?= us-east5
WORKSPACE ?= $(PWD)
USER_CONFIG ?= $(HOME)/.config/keel
SSH_DIR ?= $(HOME)/.ssh

.PHONY: build-cli install build-dev build-runner update run run-with-config exec clean test vet lint demo docs

# ============================================================
# Go CLI
# ============================================================

build-cli:
	@mkdir -p dist
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(KEEL_BUILD)" -o dist/keel ./cmd/keel

INSTALL_DIR ?= $(HOME)/.local/bin
BINARY := keel

install: build-cli ## Build and symlink keel into INSTALL_DIR (default ~/.local/bin)
	@mkdir -p $(INSTALL_DIR)
	@ln -sf $(abspath dist/$(BINARY)) $(INSTALL_DIR)/$(BINARY) && \
		echo "$(INSTALL_DIR)/$(BINARY) → $(abspath dist/$(BINARY))"

test:
	@mkdir -p dist
	go test ./... -coverprofile=dist/coverage.out

test-short:
	go test ./... -short

clean:
	rm -rf dist
	go clean

vet:
	go vet ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

# ============================================================
# Demo
# ============================================================

demo:
	$(MAKE) -C examples/demo run

# ============================================================
# Documentation
# ============================================================

DOCS_PORT ?= 1313

docs:
	@echo "Starting docs at http://localhost:$(DOCS_PORT)/keel/"
	@open "http://localhost:$(DOCS_PORT)/keel/" 2>/dev/null || xdg-open "http://localhost:$(DOCS_PORT)/keel/" 2>/dev/null || true
	hugo server -s docs --port $(DOCS_PORT) --cleanDestinationDir

# ============================================================
# Container images
# ============================================================

build:
	podman build -t $(IMAGE_NAME)-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TOOL_VERSION=$(TOOL_VERSION) \
		--build-arg KEEL_BUILD=$(KEEL_BUILD) \
		-f containers/Containerfile .

build-dev:
	podman build -t $(IMAGE_NAME)-dev-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TAG=$(TAG) \
		-f containers/dev/Containerfile .

build-runner:
	podman build -t $(IMAGE_NAME)-runner-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TAG=$(TAG) \
		-f containers/runner/Containerfile .

# Rebuild with latest tool version (busts cache)
update:
	podman build --no-cache -t $(IMAGE_NAME)-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TOOL_VERSION=latest \
		--build-arg KEEL_BUILD=$(KEEL_BUILD) \
		-f containers/Containerfile .

# ============================================================
# Run targets
# ============================================================

run:
	podman run -it --rm \
		--name $(CONTAINER_NAME) \
		-v $(SA_KEY):/secrets/gcp-sa.json:ro,Z \
		-v $(WORKSPACE):/keel/workspace:Z \
		-e ANTHROPIC_VERTEX_PROJECT_ID=$(PROJECT_ID) \
		-e CLOUD_ML_REGION=$(REGION) \
		$(IMAGE_NAME)-$(AGENT):$(TAG)

run-with-config:
	podman run -it --rm \
		--name $(CONTAINER_NAME) \
		-v $(SA_KEY):/secrets/gcp-sa.json:ro,Z \
		-v $(USER_CONFIG):/mnt/user-config:ro,Z \
		-v $(HOME)/.gitconfig:/home/keel/.gitconfig:ro,Z \
		-v $(SSH_DIR):/home/keel/.ssh:ro,Z \
		-v $(WORKSPACE):/keel/workspace:Z \
		-e ANTHROPIC_VERTEX_PROJECT_ID=$(PROJECT_ID) \
		-e CLOUD_ML_REGION=$(REGION) \
		$(IMAGE_NAME)-$(AGENT):$(TAG)

# Open a shell in the running container
KEEL_TMUX ?= 1
exec:
	@CONTAINER=$$(podman ps -q --filter name=$(CONTAINER_NAME) | head -1); \
	if [ -z "$$CONTAINER" ]; then \
		echo "No running $(CONTAINER_NAME) container found."; \
		exit 1; \
	fi; \
	if [ "$(KEEL_TMUX)" = "1" ]; then \
		podman exec -it $$CONTAINER tmux new-session -s shell; \
	else \
		podman exec -it $$CONTAINER bash; \
	fi


# ============================================================
# Temporary
# ============================================================
smoke:
	@./local/smoke-test.sh
