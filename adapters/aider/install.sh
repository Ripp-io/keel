#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Aider — adapter install script
# ============================================================
# Called during container image build to install Aider CLI
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "latest")
#
# SUPPLY-CHAIN NOTE: pip installs from PyPI are not
# reproducible without a lockfile. When TOOL_VERSION=latest, the installed
# version depends on PyPI state at build time. Pin TOOL_VERSION
# to a specific version for reproducible builds.
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-latest}"

# Aider requires Python — install if missing (node base image doesn't include it)
if ! command -v python3 &>/dev/null; then
    apt-get update && apt-get install -y --no-install-recommends python3 python3-pip python3-venv && rm -rf /var/lib/apt/lists/*
fi

# Install Aider CLI (PyPI package: aider-chat)
if [ "${TOOL_VERSION}" = "latest" ]; then
    python3 -m pip install --no-cache-dir --break-system-packages -U aider-chat
else
    python3 -m pip install --no-cache-dir --break-system-packages "aider-chat==${TOOL_VERSION}"
fi

# Verify installation
AIDER_VERSION=$(aider --version 2>/dev/null | head -1 || echo "${TOOL_VERSION}")

echo "Aider ${AIDER_VERSION} installed."
