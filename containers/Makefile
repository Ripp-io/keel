IMAGE_NAME := keel
AGENT ?= claude
TAG ?= latest
TOOL_VERSION ?= latest
KEEL_BUILD ?= dev
CONTAINER_NAME ?= $(IMAGE_NAME)-$(AGENT)-$(notdir $(abspath ..))
SA_KEY ?= $(HOME)/.config/gcloud/application_default_credentials.json
PROJECT_ID ?= $(ANTHROPIC_VERTEX_PROJECT_ID)
REGION ?= us-east5
WORKSPACE ?= $(abspath ..)
USER_CONFIG ?= $(HOME)/.config/keel
SSH_DIR ?= $(HOME)/.ssh

.PHONY: all base dev runner update run run-with-config exec clean

all: base dev runner

base:
	podman build -t $(IMAGE_NAME)-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TOOL_VERSION=$(TOOL_VERSION) \
		--build-arg KEEL_BUILD=$(KEEL_BUILD) \
		-f Containerfile ..

dev:
	podman build -t $(IMAGE_NAME)-dev-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TAG=$(TAG) \
		-f dev/Containerfile ..

runner:
	podman build -t $(IMAGE_NAME)-runner-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TAG=$(TAG) \
		-f runner/Containerfile ..

# Rebuild with latest tool version (busts cache)
update:
	podman build --no-cache -t $(IMAGE_NAME)-$(AGENT):$(TAG) \
		--build-arg AGENT=$(AGENT) \
		--build-arg TOOL_VERSION=latest \
		--build-arg KEEL_BUILD=$(KEEL_BUILD) \
		-f Containerfile ..

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

clean:
	-podman rmi $(IMAGE_NAME)-$(AGENT):$(TAG) 2>/dev/null
	-podman rmi $(IMAGE_NAME)-dev-$(AGENT):$(TAG) 2>/dev/null
	-podman rmi $(IMAGE_NAME)-runner-$(AGENT):$(TAG) 2>/dev/null
