#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Kiro adapter — runtime functions only
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

TOOL_NAME="Kiro"
TOOL_COMMAND="kiro-cli"
TOOL_HOME_DIR=".kiro"
TOOL_SESSION_NAME="kiro"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (Kiro API key authentication)
# ============================================================

adapter_configure_backend() {
    # Kiro headless mode requires KIRO_API_KEY
    # Available for Kiro Pro, Pro+, and Power subscribers
    _KIRO_AUTH_CONFIGURED=0

    if [ -n "${KIRO_API_KEY:-}" ]; then
        _KIRO_AUTH_CONFIGURED=1
        export KIRO_API_KEY
    fi

    # Note: Interactive mode may use browser-based SSO,
    # but headless strictly requires API key
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Create ~/.kiro directory if it doesn't exist
    mkdir -p "${TOOL_HOME}"

    # Kiro CLI configuration is minimal
    # Most settings are handled via environment variables
    # Create a marker file to indicate container setup
    local config_file="${TOOL_HOME}/.keel-configured"

    cat > "${config_file}" <<EOF
# Kiro configured by Keel
# Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Container setup complete
EOF
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Kiro CLI"

    if [ "${_KIRO_AUTH_CONFIGURED:-0}" -eq 1 ]; then
        keel_status_kv "Auth" "API Key (configured)"
    else
        keel_status_kv "Auth" "not configured"
    fi

    if [ -n "${KIRO_MODEL:-}" ]; then
        keel_status_kv "Model" "${KIRO_MODEL}"
    fi
}

adapter_print_warnings() {
    if [ "${_KIRO_AUTH_CONFIGURED:-0}" -eq 0 ]; then
        echo "WARNING: No Kiro API key configured."
        echo "  Headless mode requires KIRO_API_KEY environment variable."
        echo "  Set with: -e KIRO_API_KEY=your-api-key"
        echo "  API keys are available for Kiro Pro, Pro+, and Power subscribers."
        echo "  Interactive mode may work with browser-based SSO."
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

    local MODEL="${KIRO_MODEL:-}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "----------------------"

    # Check API key for headless mode
    if [ -z "${KIRO_API_KEY:-}" ]; then
        echo "ERROR: Headless mode requires KIRO_API_KEY environment variable"
        exit 1
    fi

    # Build args for headless execution
    local ARGS=("--no-interactive")

    if [ -n "${MODEL}" ]; then
        ARGS+=("--model" "${MODEL}")
    fi

    # Add output format if specified and supported
    if [ -n "${OUTPUT_FORMAT}" ]; then
        ARGS+=("--format" "${OUTPUT_FORMAT}")
    fi

    # Prompt is passed as positional argument
    ARGS+=("${PROMPT}")

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
