#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Cline adapter — runtime functions only
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

TOOL_NAME="Cline"
TOOL_COMMAND="cline"
TOOL_HOME_DIR=".cline"
TOOL_SESSION_NAME="cline"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (multi-provider)
# ============================================================

adapter_configure_backend() {
    _CLINE_PROVIDERS=""
    _CLINE_AUTH_MISSING=1

    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        _CLINE_PROVIDERS="${_CLINE_PROVIDERS}Anthropic "
        _CLINE_AUTH_MISSING=0
    fi
    if [ -n "${OPENAI_API_KEY:-}" ]; then
        _CLINE_PROVIDERS="${_CLINE_PROVIDERS}OpenAI "
        _CLINE_AUTH_MISSING=0
    fi
    if [ -n "${GOOGLE_API_KEY:-}" ]; then
        _CLINE_PROVIDERS="${_CLINE_PROVIDERS}Google "
        _CLINE_AUTH_MISSING=0
    fi

    # Google Vertex AI
    local cred_file="${GOOGLE_APPLICATION_CREDENTIALS:-/secrets/gcp-sa.json}"
    if [ -n "${GOOGLE_VERTEX_PROJECT:-}" ] || [ -f "${cred_file}" ]; then
        _CLINE_PROVIDERS="${_CLINE_PROVIDERS}Vertex "
        _CLINE_AUTH_MISSING=0
    fi

    # Local models (Ollama, LM Studio, etc.)
    if [ -n "${OLLAMA_HOST:-}" ] || [ -n "${LM_STUDIO_HOST:-}" ]; then
        _CLINE_PROVIDERS="${_CLINE_PROVIDERS}Local "
        _CLINE_AUTH_MISSING=0
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Cline CLI is configured via environment variables and
    # .cline/ directory structure. No onboarding bypass needed.
    :
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Cline"
    if [ -n "${_CLINE_PROVIDERS}" ]; then
        keel_status_kv "Providers" "${_CLINE_PROVIDERS% }"
    else
        keel_status_kv "Providers" "none configured"
    fi
    keel_status_kv "Model" "${CLINE_MODEL:-default}"
}

adapter_print_warnings() {
    if [ "${_CLINE_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No AI provider configured. Cline requires at least one."
        echo "  Set one or more: ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY"
        echo "  Or for Vertex: GOOGLE_VERTEX_PROJECT + credentials mount"
        echo "  Or for local: OLLAMA_HOST or LM_STUDIO_HOST"
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

    local MODEL="${CLINE_MODEL:-}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-json}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "----------------------"

    local ARGS=("--json")

    if [ -n "${MODEL}" ]; then
        ARGS+=("--model" "${MODEL}")
    fi

    # Cline CLI accepts prompt as final argument
    ARGS+=("${PROMPT}")

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
