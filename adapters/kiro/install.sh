#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Kiro — adapter install script
# ============================================================
# Called during container image build to install Kiro CLI
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "latest")
#
# SUPPLY-CHAIN NOTE: The Kiro installer downloads from AWS CDN.
# When TOOL_VERSION=latest, the installed version depends on the
# CDN state at build time. Pin TOOL_VERSION to a specific version
# for reproducible builds (if version-specific URLs become available).
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-latest}"

# Kiro installer requires unzip — install if missing
if ! command -v unzip &>/dev/null; then
    apt-get update && apt-get install -y --no-install-recommends unzip && rm -rf /var/lib/apt/lists/*
fi

# Kiro CLI official installer
# Downloads and installs to ~/.local/bin/kiro-cli
if [ "${TOOL_VERSION}" = "latest" ]; then
    curl -fsSL https://cli.kiro.dev/install | bash
else
    # Version-specific install not documented yet
    # Fall back to latest installer
    echo "WARNING: Version-specific install not supported, using latest"
    curl -fsSL https://cli.kiro.dev/install | bash
fi

# Ensure binary is in system PATH
if [ -f "${HOME}/.local/bin/kiro-cli" ]; then
    cp "${HOME}/.local/bin/kiro-cli" /usr/local/bin/kiro-cli
    chmod 755 /usr/local/bin/kiro-cli
fi

KIRO_VERSION=$(kiro-cli --version 2>/dev/null | head -1 || echo "${TOOL_VERSION}")
echo "Kiro ${KIRO_VERSION} installed."
