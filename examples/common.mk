# ============================================================
# Shared macros and targets for keel example Makefiles
# ============================================================
#
# Include from example Makefiles with:
#   include $(KEEL_ROOT)/examples/common.mk
#
# Required variables (set before include):
#   CONTAINER_NAME — container name for exec target
#   KEEL_ROOT      — path to repository root
#
# Optional variables:
#   KEEL_TMUX      — "1" to exec into tmux (default: 1)
# ============================================================

# Quiet build: capture output, show only on failure.
# Usage: $(call quiet_build,<label>,<command>)
define quiet_build
	@printf "Building %-30s" "$(1)..."
	@if $(2) > /tmp/build-$(1).log 2>&1; then \
		echo " ✓"; \
	else \
		echo " ✗"; \
		echo ""; \
		cat /tmp/build-$(1).log; \
		exit 1; \
	fi
endef

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
