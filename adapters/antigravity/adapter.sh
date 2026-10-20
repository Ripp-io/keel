#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Antigravity CLI adapter — runtime functions only
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

TOOL_NAME="Antigravity CLI"
TOOL_COMMAND="agy"
TOOL_HOME_DIR=".gemini"
TOOL_SESSION_NAME="antigravity"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (Gemini API vs Google Cloud)
# ============================================================

adapter_configure_backend() {
    _ANTIGRAVITY_AUTH_METHOD=""
    _ANTIGRAVITY_AUTH_MISSING=1

    # Primary: GEMINI_API_KEY (recommended for headless/container use)
    if [ -n "${GEMINI_API_KEY:-}" ]; then
        _ANTIGRAVITY_AUTH_METHOD="Gemini API Key"
        _ANTIGRAVITY_AUTH_MISSING=0
        return 0
    fi

    # Secondary: Google Cloud Application Default Credentials
    local cred_file="${GOOGLE_APPLICATION_CREDENTIALS:-/secrets/gcp-sa.json}"
    if [ -f "${cred_file}" ] || [ -n "${GOOGLE_CLOUD_PROJECT:-}" ]; then
        _ANTIGRAVITY_AUTH_METHOD="Google Cloud ADC"
        _ANTIGRAVITY_AUTH_MISSING=0
        return 0
    fi

    # Tertiary: Cached credentials (requires prior interactive login)
    if [ -d "${TOOL_HOME}" ] && [ -f "${TOOL_HOME}/credentials" ]; then
        _ANTIGRAVITY_AUTH_METHOD="Cached credentials"
        _ANTIGRAVITY_AUTH_MISSING=0
        return 0
    fi

    # No auth configured
    _ANTIGRAVITY_AUTH_METHOD="none"
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Antigravity CLI may cache auth state in .gemini directory
    # For headless use with GEMINI_API_KEY, no onboarding bypass needed
    # as the CLI will use the API key directly
    mkdir -p "${TOOL_HOME}"
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Google Antigravity"
    keel_status_kv "Auth" "${_ANTIGRAVITY_AUTH_METHOD:-not configured}"

    if [ -n "${GEMINI_MODEL:-}" ]; then
        keel_status_kv "Model" "${GEMINI_MODEL}"
    else
        keel_status_kv "Model" "default"
    fi
}

adapter_print_warnings() {
    if [ "${_ANTIGRAVITY_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No authentication configured for Antigravity CLI."
        echo "  For headless/container use, set GEMINI_API_KEY:"
        echo "    -e GEMINI_API_KEY=your-api-key"
        echo "  For Google Cloud, mount credentials:"
        echo "    -v /path/to/sa-key.json:/secrets/gcp-sa.json:ro,Z"
        echo "    -e GOOGLE_CLOUD_PROJECT=your-project"
        echo "  For interactive use, run 'agy' once to authenticate."
        echo ""
    fi

    if [ "${_ANTIGRAVITY_AUTH_METHOD:-}" = "Cached credentials" ]; then
        echo "NOTE: Using cached credentials. These may expire in containerized environments."
        echo "  For reliable headless operation, use GEMINI_API_KEY instead."
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
    # passed verbatim to the agent CLI. This is a prompt injection surface —
    # the agent CLI is the trust boundary, not this script.
    local PROMPT="${*:-Review this codebase.}"

    local MODEL="${GEMINI_MODEL:-}"
    local MAX_TURNS="${MAX_TURNS:-15}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-json}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "Max turns: ${MAX_TURNS}"
    echo "----------------------"

    local ARGS=("--print" "${PROMPT}")

    if [ -n "${MODEL}" ]; then
        ARGS+=("--model" "${MODEL}")
    fi

    ARGS+=("--output-format" "${OUTPUT_FORMAT}")

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
