#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Cursor Agent CLI — install script
# ============================================================
# Downloads the official Cursor CLI installer, runs it in an
# isolated staging directory, then relocates the self-contained
# tool directory to /opt/cursor-agent/ for image hygiene.
#
# Cursor installer layout:
#   .local/bin/<name>  →  symlink into versions dir
#   .local/share/cursor-agent/versions/<ver>/
#     ├── cursor-agent   (shell wrapper — uses realpath to find peers)
#     ├── node           (bundled Node.js binary)
#     └── index.js       (CLI entry point)
#
# The wrapper resolves its own realpath to locate node and
# index.js, so all three must live in the same directory.
# ============================================================

INSTALL_DIR="/opt/cursor-agent"
STAGING=$(mktemp -d)
trap 'rm -rf "${STAGING}"' EXIT

echo "Installing Cursor Agent CLI..."

# Download and run the official installer.
#
# SUPPLY-CHAIN NOTE: CURSOR_INSTALLER_SHA256 build arg is REQUIRED.
# The installer is downloaded from cursor.com and verified against this
# hash before execution. To get the current hash:
#   curl -fsSL https://cursor.com/install | sha256sum
curl -fsSL https://cursor.com/install -o "${STAGING}/install.sh"
[ -s "${STAGING}/install.sh" ] || { echo "ERROR: Downloaded installer is empty" >&2; exit 1; }

# Optional checksum verification (opt-in via build arg)
EXPECTED_HASH="${CURSOR_INSTALLER_SHA256:?ERROR: CURSOR_INSTALLER_SHA256 build arg must be set for supply-chain safety. See adapters/cursor/install.sh for details.}"
if [ -n "${EXPECTED_HASH}" ]; then
    ACTUAL_HASH=$(sha256sum "${STAGING}/install.sh" | cut -d' ' -f1)
    if [ "${ACTUAL_HASH}" != "${EXPECTED_HASH}" ]; then
        echo "ERROR: Cursor installer checksum mismatch" >&2
        echo "  Expected: ${EXPECTED_HASH}" >&2
        echo "  Actual:   ${ACTUAL_HASH}" >&2
        exit 1
    fi
fi

HOME="${STAGING}" bash "${STAGING}/install.sh"

# Find the installed binary (name varies across Cursor CLI versions)
BIN=""
for name in agent cursor-agent; do
    [ -f "${STAGING}/.local/bin/${name}" ] && BIN="${name}" && break
done
[ -z "${BIN}" ] && { echo "ERROR: No Cursor CLI binary found after install" >&2; exit 1; }

# Resolve the symlink to the self-contained tool directory,
# then relocate it to a stable image path.
TOOL_DIR=$(dirname "$(readlink -f "${STAGING}/.local/bin/${BIN}")")
mkdir -p "${INSTALL_DIR}"
cp -a "${TOOL_DIR}/." "${INSTALL_DIR}/"

# Symlink to PATH (both names for adapter compatibility)
WRAPPER="${INSTALL_DIR}/cursor-agent"
[ -f "${WRAPPER}" ] || WRAPPER="${INSTALL_DIR}/agent"
ln -sf "${WRAPPER}" /usr/local/bin/agent
ln -sf "${WRAPPER}" /usr/local/bin/cursor-agent

echo "Cursor Agent CLI $("${WRAPPER}" --version 2>/dev/null || echo 'installed')."
