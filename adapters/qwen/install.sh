#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Qwen Code — adapter install script
# ============================================================
# Called during container image build to install Qwen Code CLI
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

# Install Qwen Code CLI (npm package: @qwen-code/qwen-code)
if [ "${TOOL_VERSION}" = "latest" ]; then
    npm install -g @qwen-code/qwen-code
else
    npm install -g "@qwen-code/qwen-code@${TOOL_VERSION}"
fi

npm cache clean --force

echo "Qwen Code $(qwen --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
