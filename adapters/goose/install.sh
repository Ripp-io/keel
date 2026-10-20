#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Goose — adapter install script
# ============================================================
# Called during container image build to install Goose CLI
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "stable")
#
# SUPPLY-CHAIN NOTE: The download script fetches the latest stable
# binary from GitHub releases. When TOOL_VERSION=stable, the installed
# version depends on the latest release at build time. Pin TOOL_VERSION
# to a specific version tag for reproducible builds.
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-stable}"

# Goose installer uses bzip2 for extraction — install if missing
if ! command -v bzip2 &>/dev/null; then
    apt-get update && apt-get install -y --no-install-recommends bzip2 && rm -rf /var/lib/apt/lists/*
fi

# Install Goose CLI via official installer
# CONFIGURE=false skips interactive provider setup
curl -fsSL https://github.com/block/goose/releases/latest/download/download_cli.sh | CONFIGURE=false bash

# The installer puts the binary in ~/.local/bin/ which is root-owned.
# Copy to /usr/local/bin so it's accessible to the keel user at runtime.
if [ -f "${HOME}/.local/bin/goose" ]; then
    cp "${HOME}/.local/bin/goose" /usr/local/bin/goose
    chmod 755 /usr/local/bin/goose
fi

# Verify installation
if command -v goose &>/dev/null; then
    echo "Goose $(goose --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
else
    echo "ERROR: Goose installation failed" >&2
    exit 1
fi
