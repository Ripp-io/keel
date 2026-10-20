#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Mistral Vibe — adapter install script
# ============================================================
# Called during container image build to install Mistral Vibe CLI
# and set tool-specific environment variables.
#
# Build args available:
#   TOOL_VERSION — version to install (default: "latest")
#
# SUPPLY-CHAIN NOTE: uv tool installs from PyPI are not reproducible
# without a lockfile. When TOOL_VERSION=latest, the installed version
# depends on PyPI state at build time. Pin TOOL_VERSION to a specific
# version for reproducible builds.
#
# EARLY-STAGE WARNING: Mistral Vibe (released Jan 2026) is early-stage.
# Install method may change. Binary name and flags are based on docs
# at https://docs.mistral.ai/mistral-vibe/introduction as of Oct 2026.
# ============================================================

TOOL_VERSION="${TOOL_VERSION:-latest}"

# Install uv (manages Python 3.12 and the Rust-compiled vibe package)
if ! command -v uv &>/dev/null; then
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="${HOME}/.local/bin:${PATH}"
fi

# Install Mistral Vibe via uv tool (handles Python 3.12 and Rust build)
if [ "${TOOL_VERSION}" = "latest" ]; then
    uv tool install mistral-vibe --python 3.12
else
    uv tool install "mistral-vibe==${TOOL_VERSION}" --python 3.12
fi

# uv installs to ~/.local/bin — copy to /usr/local/bin for keel user access
if [ -f "${HOME}/.local/bin/vibe" ]; then
    cp "${HOME}/.local/bin/vibe" /usr/local/bin/vibe
    chmod 755 /usr/local/bin/vibe
fi

# Make uv's managed Python accessible to keel user (vibe binary links to it)
chmod a+x /root /root/.local /root/.local/share /root/.local/share/uv
chmod -R a+rX /root/.local/share/uv/python/ /root/.local/share/uv/tools/

INSTALLED_VERSION=$(vibe --version 2>/dev/null || echo "${TOOL_VERSION}")

echo "Mistral Vibe ${INSTALLED_VERSION} installed."
