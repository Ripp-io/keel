#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Claude Code — adapter install script
# ============================================================
# Called during container image build to install Claude Code
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "latest")
#
# SUPPLY-CHAIN NOTE: npm installs from the public registry are not
# reproducible without a lockfile. When TOOL_VERSION=latest, the installed
# version depends on the registry state at build time. Set CLAUDE_CODE_SHA256
# to verify the installed package, or pin TOOL_VERSION to a specific version.
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-latest}"

# Install Claude Code CLI
if [ "${TOOL_VERSION}" = "latest" ]; then
    npm install -g @anthropic-ai/claude-code
else
    npm install -g "@anthropic-ai/claude-code@${TOOL_VERSION}"
fi
# Optional checksum verification (opt-in via CLAUDE_CODE_SHA256 build arg)
if [ -n "${CLAUDE_CODE_SHA256:-}" ]; then
    INSTALLED_PKG="$(npm root -g)/@anthropic-ai/claude-code/package.json"
    if [ ! -f "${INSTALLED_PKG}" ]; then
        echo "ERROR: Cannot verify checksum — package.json not found at ${INSTALLED_PKG}" >&2
        exit 1
    fi
    ACTUAL_HASH=$(sha256sum "${INSTALLED_PKG}" | cut -d' ' -f1)
    if [ "${ACTUAL_HASH}" != "${CLAUDE_CODE_SHA256}" ]; then
        echo "ERROR: Claude Code package checksum mismatch" >&2
        echo "  Expected: ${CLAUDE_CODE_SHA256}" >&2
        echo "  Actual:   ${ACTUAL_HASH}" >&2
        exit 1
    fi
    echo "  ✓ Checksum verified"
fi

npm cache clean --force

echo "Claude Code $(claude --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
