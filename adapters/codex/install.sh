#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Codex CLI — adapter install script
# ============================================================
# Called during container image build to install Codex CLI
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "latest")
#
# SUPPLY-CHAIN NOTE: npm installs from the public registry are not
# reproducible without a lockfile. When TOOL_VERSION=latest, the installed
# version depends on the registry state at build time. Pin TOOL_VERSION
# to a specific version for reproducible builds.
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-latest}"

# Install Codex CLI (npm package: @openai/codex)
if [ "${TOOL_VERSION}" = "latest" ]; then
    npm install -g @openai/codex
else
    npm install -g "@openai/codex@${TOOL_VERSION}"
fi

npm cache clean --force

echo "Codex CLI $(codex --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
