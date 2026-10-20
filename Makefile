KEEL_BUILD ?= dev
INSTALL_DIR ?= $(HOME)/.local/bin
BINARY := keel

.PHONY: build install test test-short clean vet lint demo docs containers smoke

# ============================================================
# Go CLI
# ============================================================

build:
	@mkdir -p dist
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(KEEL_BUILD)" -o dist/keel ./cmd/keel

install: build ## Build and symlink keel into INSTALL_DIR (default ~/.local/bin)
	@mkdir -p $(INSTALL_DIR)
	@if [ -L $(INSTALL_DIR)/$(BINARY) ] && [ "$$(readlink $(INSTALL_DIR)/$(BINARY))" = "$(abspath dist/$(BINARY))" ]; then \
		echo "$(INSTALL_DIR)/$(BINARY) → $(abspath dist/$(BINARY)) (up to date)"; \
	elif [ -w $(INSTALL_DIR) ]; then \
		ln -sf $(abspath dist/$(BINARY)) $(INSTALL_DIR)/$(BINARY); \
		echo "$(INSTALL_DIR)/$(BINARY) → $(abspath dist/$(BINARY))"; \
	else \
		sudo ln -sf $(abspath dist/$(BINARY)) $(INSTALL_DIR)/$(BINARY); \
		echo "$(INSTALL_DIR)/$(BINARY) → $(abspath dist/$(BINARY))"; \
	fi

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
# Containers (delegates to containers/Makefile)
# ============================================================

containers:
	$(MAKE) -C containers

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
# Temporary
# ============================================================
smoke:
	@./local/smoke-test.sh
