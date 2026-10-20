#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Kilo Code adapter — runtime functions only
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

TOOL_NAME="Kilo Code"
TOOL_COMMAND="kilo"
TOOL_HOME_DIR=".kilo"
TOOL_SESSION_NAME="kilo"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (multi-provider)
# ============================================================

adapter_configure_backend() {
    _KILO_PROVIDERS=""
    _KILO_AUTH_MISSING=1

    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        _KILO_PROVIDERS="${_KILO_PROVIDERS}Anthropic "
        _KILO_AUTH_MISSING=0
    fi
    if [ -n "${OPENAI_API_KEY:-}" ]; then
        _KILO_PROVIDERS="${_KILO_PROVIDERS}OpenAI "
        _KILO_AUTH_MISSING=0
    fi
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        _KILO_PROVIDERS="${_KILO_PROVIDERS}Copilot "
        _KILO_AUTH_MISSING=0
    fi

    # Google Vertex AI
    local cred_file="${GOOGLE_APPLICATION_CREDENTIALS:-/secrets/gcp-sa.json}"
    if [ -n "${GOOGLE_VERTEX_PROJECT:-}" ] || [ -f "${cred_file}" ]; then
        _KILO_PROVIDERS="${_KILO_PROVIDERS}Vertex "
        _KILO_AUTH_MISSING=0
    fi

    # Local models (Ollama, LM Studio, etc.)
    if [ -n "${OLLAMA_HOST:-}" ] || [ -n "${LOCAL_ENDPOINT:-}" ]; then
        _KILO_PROVIDERS="${_KILO_PROVIDERS}Local "
        _KILO_AUTH_MISSING=0
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Kilo Code does not require an onboarding bypass.
    # Configuration is driven by environment variables and
    # kilo.json (if produced by keel compile).
    :
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Kilo Code"
    if [ -n "${_KILO_PROVIDERS}" ]; then
        keel_status_kv "Providers" "${_KILO_PROVIDERS% }"
    else
        keel_status_kv "Providers" "none configured"
    fi
    keel_status_kv "Model" "${KILO_MODEL:-default}"
}

adapter_print_warnings() {
    if [ "${_KILO_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No AI provider configured. Kilo Code requires at least one."
        echo "  Set one or more: ANTHROPIC_API_KEY, OPENAI_API_KEY, GITHUB_TOKEN"
        echo "  Or for Vertex: GOOGLE_VERTEX_PROJECT + credentials mount"
        echo "  Or for local: OLLAMA_HOST or LOCAL_ENDPOINT"
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
    # passed verbatim to the agent CLI via run. This is a prompt injection
    # surface — the agent CLI is the trust boundary, not this script.
    local PROMPT="${*:-Review this codebase.}"

    local MODEL="${KILO_MODEL:-}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "----------------------"

    local ARGS=("run" "--auto")

    if [ -n "${MODEL}" ]; then
        ARGS+=("--model" "${MODEL}")
    fi

    if [ -n "${OUTPUT_FORMAT}" ]; then
        ARGS+=("-f" "${OUTPUT_FORMAT}")
    fi

    ARGS+=("${PROMPT}")

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
