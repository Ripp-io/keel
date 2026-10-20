#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Qwen Code adapter — runtime functions only
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

TOOL_NAME="Qwen Code"
TOOL_COMMAND="qwen"
TOOL_HOME_DIR=".qwen"
TOOL_SESSION_NAME="qwen"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (multi-provider)
# ============================================================

adapter_configure_backend() {
    _QWEN_PROVIDERS=""
    _QWEN_AUTH_MISSING=1

    # Alibaba DashScope (Qwen's native API)
    if [ -n "${DASHSCOPE_API_KEY:-}" ]; then
        _QWEN_PROVIDERS="${_QWEN_PROVIDERS}DashScope "
        _QWEN_AUTH_MISSING=0
    fi

    # Anthropic Claude
    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        _QWEN_PROVIDERS="${_QWEN_PROVIDERS}Anthropic "
        _QWEN_AUTH_MISSING=0
    fi

    # OpenAI (or OpenAI-compatible endpoints)
    if [ -n "${OPENAI_API_KEY:-}" ]; then
        _QWEN_PROVIDERS="${_QWEN_PROVIDERS}OpenAI "
        _QWEN_AUTH_MISSING=0
    fi

    # Google Gemini
    if [ -n "${GEMINI_API_KEY:-}" ]; then
        _QWEN_PROVIDERS="${_QWEN_PROVIDERS}Gemini "
        _QWEN_AUTH_MISSING=0
    fi

    # Local models (Ollama, vLLM)
    if [ -n "${OLLAMA_HOST:-}" ] || [ -n "${VLLM_ENDPOINT:-}" ]; then
        _QWEN_PROVIDERS="${_QWEN_PROVIDERS}Local "
        _QWEN_AUTH_MISSING=0
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    # Qwen Code does not require onboarding bypass.
    # Configuration is driven by environment variables and config files.
    # OAuth was discontinued April 2026 — API key auth is now required.
    :
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Qwen Code"
    if [ -n "${_QWEN_PROVIDERS}" ]; then
        keel_status_kv "Providers" "${_QWEN_PROVIDERS% }"
    else
        keel_status_kv "Providers" "none configured"
    fi
    keel_status_kv "Model" "${QWEN_MODEL:-default}"
}

adapter_print_warnings() {
    if [ "${_QWEN_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No AI provider configured. Qwen Code requires at least one."
        echo "  Set one or more:"
        echo "    DASHSCOPE_API_KEY (Alibaba DashScope)"
        echo "    OPENAI_API_KEY (OpenAI or compatible endpoint)"
        echo "    ANTHROPIC_API_KEY (Claude)"
        echo "    GEMINI_API_KEY (Google Gemini)"
        echo "  Or for local models: OLLAMA_HOST or VLLM_ENDPOINT"
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

    local MODEL="${QWEN_MODEL:-}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "----------------------"

    local ARGS=("-p" "${PROMPT}")

    if [ -n "${MODEL}" ]; then
        ARGS+=("--model" "${MODEL}")
    fi

    if [ -n "${OUTPUT_FORMAT}" ]; then
        ARGS+=("--output-format" "${OUTPUT_FORMAT}")
    fi

    exec "${TOOL_COMMAND}" "${ARGS[@]}"
}
