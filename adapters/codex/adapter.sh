#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Codex CLI adapter — runtime functions only
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

TOOL_NAME="Codex CLI"
TOOL_COMMAND="codex"
TOOL_HOME_DIR=".codex"
TOOL_SESSION_NAME="codex"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (OpenAI API)
# ============================================================

adapter_configure_backend() {
    # Codex CLI uses OPENAI_API_KEY for authentication
    # CODEX_API_KEY can also be used for headless exec mode
    _CODEX_AUTH_MISSING=1

    if [ -n "${OPENAI_API_KEY:-}" ]; then
        _CODEX_AUTH_MISSING=0
    fi
    if [ -n "${CODEX_API_KEY:-}" ]; then
        _CODEX_AUTH_MISSING=0
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Codex CLI configuration lives in ~/.codex directory
    # Create minimal config to bypass first-run setup
    mkdir -p "${TOOL_HOME}"

    # If OPENAI_API_KEY is available, use it for login
    if [ -n "${OPENAI_API_KEY:-}" ]; then
        # Attempt API key login (non-interactive)
        echo "${OPENAI_API_KEY}" | "${TOOL_COMMAND}" login --with-api-key 2>/dev/null || true
    fi
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "OpenAI"
    if [ -n "${OPENAI_API_KEY:-}" ] || [ -n "${CODEX_API_KEY:-}" ]; then
        keel_status_kv "Auth" "API key configured"
    else
        keel_status_kv "Auth" "not configured"
    fi
}

adapter_print_warnings() {
    if [ "${_CODEX_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No OpenAI API key configured. Codex CLI requires authentication."
        echo "  Set OPENAI_API_KEY or CODEX_API_KEY environment variable."
        echo "  For interactive login, run: codex login --device-auth"
        echo ""
    fi
}

# ============================================================
# Launch functions
# ============================================================

adapter_launch() {
    exec "${TOOL_COMMAND}" "$@"
}

adapter_launch_tmux() {
    keel_launch_tmux "${TOOL_SESSION_NAME}" "${TOOL_COMMAND}" "$@"
}

adapter_launch_headless() {
    # SECURITY: PROMPT is constructed from user-controlled CMD args ($@) and
    # passed verbatim to the agent CLI via exec. This is a prompt injection
    # surface — the agent CLI is the trust boundary, not this script.
    local PROMPT="${*:-Review this codebase.}"

    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-json}"
    local API_KEY="${CODEX_API_KEY:-${OPENAI_API_KEY:-}}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "Format:  ${OUTPUT_FORMAT}"
    echo "----------------------"

    local ARGS=("exec")

    if [ -n "${OUTPUT_FORMAT}" ] && [ "${OUTPUT_FORMAT}" != "text" ]; then
        ARGS+=("--${OUTPUT_FORMAT}")
    fi

    ARGS+=("${PROMPT}")

    # Set CODEX_API_KEY for exec mode if available
    if [ -n "${API_KEY}" ]; then
        export CODEX_API_KEY="${API_KEY}"
    fi

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
