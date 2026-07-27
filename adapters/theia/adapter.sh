#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Theia IDE adapter — runtime functions only
# ============================================================
#
# Config assembly is handled by `keel compile`. This adapter
# provides runtime-only hooks: backend configuration, onboarding,
# status display, and launch functions.
#
# Theia is a web-based IDE. Do not build keel-runner-theia.
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

TOOL_NAME="Theia IDE"
TOOL_COMMAND="theia"
TOOL_HOME_DIR=".theia-ide"
TOOL_SESSION_NAME="theia"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

THEIA_PORT="${THEIA_PORT:-3000}"
# Default to localhost. Set THEIA_HOST=0.0.0.0 to expose the IDE on
# the network, but note that Theia has no built-in authentication.
THEIA_HOST="${THEIA_HOST:-127.0.0.1}"

# ============================================================
# Backend configuration (multi-provider AI)
# ============================================================

adapter_configure_backend() {
    _THEIA_AI_CONFIGURED=0
    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then _THEIA_AI_CONFIGURED=1; fi
    if [ -n "${OPENAI_API_KEY:-}" ]; then _THEIA_AI_CONFIGURED=1; fi
    if [ -n "${GOOGLE_API_KEY:-}" ]; then _THEIA_AI_CONFIGURED=1; fi
    if [ -n "${OLLAMA_HOST:-}" ]; then _THEIA_AI_CONFIGURED=1; fi
}

# ============================================================
# First-run onboarding
# ============================================================

adapter_write_onboarding() {
    mkdir -p "${TOOL_HOME}"
    local settings_file="${TOOL_HOME}/settings.json"
    if [ ! -f "${settings_file}" ]; then
        cat > "${settings_file}" <<'EOF'
{
  "ai-features.AiEnable": true,
  "workbench.startupEditor": "none",
  "telemetry.telemetryLevel": "off",
  "terminal.integrated.defaultProfile.linux": "bash"
}
EOF
    fi
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Theia IDE"
    keel_status_kv "Port" "${THEIA_PORT}"
    local providers=""
    [ -n "${ANTHROPIC_API_KEY:-}" ] && providers="${providers}Anthropic "
    [ -n "${OPENAI_API_KEY:-}" ]   && providers="${providers}OpenAI "
    [ -n "${GOOGLE_API_KEY:-}" ]   && providers="${providers}Google "
    [ -n "${OLLAMA_HOST:-}" ]      && providers="${providers}Ollama "
    if [ -n "${providers}" ]; then
        keel_status_kv "Providers" "${providers% }"
    else
        keel_status_kv "Providers" "none configured"
    fi
}

adapter_print_warnings() {
    if [ "${_THEIA_AI_CONFIGURED:-0}" -eq 0 ]; then
        echo "NOTE: No AI provider configured. AI chat features will be unavailable."
        echo "  Set one or more: ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY, OLLAMA_HOST"
        echo ""
    fi
}

# ============================================================
# Launch functions
# ============================================================

adapter_launch() {
    echo "Starting Theia IDE on http://${THEIA_HOST}:${THEIA_PORT}"
    echo ""
    export SHELL="${SHELL:-/bin/bash}"
    export USE_LOCAL_GIT=true
    exec "${TOOL_COMMAND}" /keel/workspace \
        --hostname="${THEIA_HOST}" \
        --port="${THEIA_PORT}" \
        "$@"
}

adapter_launch_tmux() {
    export SHELL="${SHELL:-/bin/bash}"
    export USE_LOCAL_GIT=true
    keel_launch_tmux "${TOOL_SESSION_NAME}" \
        "${TOOL_COMMAND}" /keel/workspace \
        "--hostname=${THEIA_HOST}" "--port=${THEIA_PORT}" "$@"
}

adapter_launch_headless() {
    echo "ERROR: Theia IDE does not support headless mode." >&2
    echo "  Theia is a browser-native IDE — it serves a web UI." >&2
    echo "  Use a CLI-based adapter (e.g. claude, cursor) for headless/runner tasks." >&2
    exit 1
}
