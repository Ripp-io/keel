#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Cursor Agent CLI adapter — runtime functions only
# ============================================================
#
# Config assembly is handled by `keel compile`. This adapter
# provides runtime-only hooks: backend configuration, onboarding,
# status display, and launch functions.
#
# Adapter interface (7 functions):
#   adapter_configure_backend
#   adapter_write_onboarding
#   adapter_print_backend_status
#   adapter_print_warnings
#   adapter_launch
#   adapter_launch_tmux
#   adapter_launch_headless
# ============================================================

# Source lib utilities
source /usr/local/lib/config.sh
source /usr/local/lib/git.sh
source /usr/local/lib/launch.sh
source /usr/local/lib/status.sh

TOOL_NAME="Cursor Agent"
TOOL_COMMAND="agent"
TOOL_HOME_DIR=".cursor"
TOOL_SESSION_NAME="cursor"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (API key)
# ============================================================

adapter_configure_backend() {
    if [ -n "${CURSOR_API_KEY:-}" ]; then
        _CURSOR_AUTH_MISSING=0
    else
        _CURSOR_AUTH_MISSING=1
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # The Cursor CLI is self-contained at /opt/cursor-agent/ (node, index.js,
    # and all chunks). No external share directory or onboarding bypass needed.
    :
}

# ============================================================
# Fail-fast authentication guard
# ============================================================

_cursor_require_auth() {
    if [ "${_CURSOR_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "ERROR: CURSOR_API_KEY is required but not set." >&2
        echo "  Generate an API key at https://cursor.com/settings (Integrations > API Keys)" >&2
        echo "  Pass with: -e CURSOR_API_KEY=your_key_here" >&2
        exit 1
    fi
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Cursor Agent"
    if [ -n "${CURSOR_API_KEY:-}" ]; then
        keel_status_kv "Auth" "API key configured"
    else
        keel_status_kv "Auth" "not configured"
    fi
    keel_status_kv "Model" "${CURSOR_MODEL:-auto}"
}

adapter_print_warnings() {
    if [ "${_CURSOR_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: CURSOR_API_KEY is not set."
        echo "  Generate an API key at https://cursor.com/settings (Integrations > API Keys)"
        echo "  Pass with: -e CURSOR_API_KEY=your_key_here"
        echo ""
    fi
}

# ============================================================
# Launch functions
# ============================================================

adapter_launch() {
    _cursor_require_auth
    exec "${TOOL_COMMAND}" "$@"
}

adapter_launch_tmux() {
    _cursor_require_auth
    keel_launch_tmux "${TOOL_SESSION_NAME}" "${TOOL_COMMAND}" "$@"
}

adapter_launch_headless() {
    _cursor_require_auth
    local PROMPT="${*:-Review this codebase.}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-stream-json}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "----------------------"

    exec "${TOOL_COMMAND}" \
        -p "${PROMPT}" \
        --force \
        --output-format "${OUTPUT_FORMAT}"
}
