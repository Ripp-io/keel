#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Mistral Vibe adapter — runtime functions only
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
#
# EARLY-STAGE WARNING: Mistral Vibe (released Jan 2026) is early-stage.
# CLI flags and behavior may change. Based on docs at
# https://docs.mistral.ai/mistral-vibe/introduction as of Oct 2026.
# ============================================================

# Source lib utilities
source /usr/local/lib/config.sh
source /usr/local/lib/git.sh
source /usr/local/lib/launch.sh
source /usr/local/lib/status.sh

TOOL_NAME="Mistral Vibe"
TOOL_COMMAND="vibe"
TOOL_HOME_DIR=".vibe"
TOOL_SESSION_NAME="vibe"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration
# ============================================================

adapter_configure_backend() {
    _VIBE_AUTH_MISSING=1

    if [ -n "${MISTRAL_API_KEY:-}" ]; then
        _VIBE_AUTH_MISSING=0
    fi

    # Check for existing config with saved key
    if [ -f "${TOOL_HOME}/config.toml" ]; then
        _VIBE_AUTH_MISSING=0
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Mistral Vibe setup wizard runs on first launch if no config exists.
    # Bypass by ensuring MISTRAL_API_KEY is set — the CLI will use it
    # without prompting if the env var is present.
    #
    # If MISTRAL_API_KEY is set, create a minimal config to prevent
    # the setup wizard from running.

    if [ -n "${MISTRAL_API_KEY:-}" ]; then
        mkdir -p "${TOOL_HOME}"

        # Create minimal config if none exists
        # Vibe will auto-populate the rest on first run
        if [ ! -f "${TOOL_HOME}/config.toml" ]; then
            cat > "${TOOL_HOME}/config.toml" <<EOF
# Mistral Vibe configuration
# API key provided via MISTRAL_API_KEY environment variable
EOF
        fi
    fi
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Mistral Vibe"

    if [ "${_VIBE_AUTH_MISSING:-1}" -eq 0 ]; then
        keel_status_kv "Auth" "configured"
    else
        keel_status_kv "Auth" "not configured"
    fi

    keel_status_kv "Model" "${VIBE_MODEL:-default}"
}

adapter_print_warnings() {
    if [ "${_VIBE_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No API key configured. Mistral Vibe requires authentication."
        echo "  Set MISTRAL_API_KEY to your Mistral API key."
        echo "  Get a key from: https://chat.mistral.ai/code/extensions"
        echo ""
    fi

    echo "NOTE: Mistral Vibe is early-stage (released Jan 2026)."
    echo "  Behavior and CLI flags may change in future releases."
    echo ""
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
    # passed verbatim to the agent CLI via --prompt. This is a prompt injection
    # surface — the agent CLI is the trust boundary, not this script.
    local PROMPT="${*:-Review this codebase.}"

    local MODEL="${VIBE_MODEL:-}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-json}"
    local MAX_TURNS="${VIBE_MAX_TURNS:-10}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "----------------------"

    local ARGS=("--prompt" "${PROMPT}")

    # Set output format for headless mode
    # Options: text, json, streaming
    ARGS+=("--output" "${OUTPUT_FORMAT}")

    # Limit turns to prevent runaway execution
    if [ -n "${MAX_TURNS}" ]; then
        ARGS+=("--max-turns" "${MAX_TURNS}")
    fi

    # Set model if specified
    if [ -n "${MODEL}" ]; then
        ARGS+=("--model" "${MODEL}")
    fi

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
