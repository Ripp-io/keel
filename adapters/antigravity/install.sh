#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Antigravity CLI — adapter install script
# ============================================================
# Called during container image build to install Antigravity CLI
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "latest")
#
# SUPPLY-CHAIN NOTE: The official Antigravity installer fetches the
# latest stable release unless otherwise specified. When TOOL_VERSION=latest,
# the installed version depends on what Google releases. Pin TOOL_VERSION
# to a specific version for reproducible builds (if supported by installer).
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-latest}"

# Install Antigravity CLI using official installer
if [ "${TOOL_VERSION}" = "latest" ]; then
    curl -fsSL https://antigravity.google/cli/install.sh | bash
else
    ANTIGRAVITY_VERSION="${TOOL_VERSION}" curl -fsSL https://antigravity.google/cli/install.sh | bash
fi

# The installer puts the binary in ~/.local/bin/ which is root-owned.
# Copy to /usr/local/bin so it's accessible to the keel user at runtime.
if [ -f "${HOME}/.local/bin/agy" ]; then
    cp "${HOME}/.local/bin/agy" /usr/local/bin/agy
    chmod 755 /usr/local/bin/agy
fi

echo "Antigravity CLI $(agy --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
