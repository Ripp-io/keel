#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# OpenCode V2 — adapter install script
# ============================================================
# Called during container image build to install OpenCode CLI
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

# Install OpenCode V2 CLI (npm package: @opencode/cli)
if [ "${TOOL_VERSION}" = "latest" ]; then
    npm install -g @opencode/cli
else
    npm install -g "@opencode/cli@${TOOL_VERSION}"
fi

npm cache clean --force

echo "OpenCode $(opencode --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
