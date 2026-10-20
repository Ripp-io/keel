#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Goose adapter — runtime functions only
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

TOOL_NAME="Goose"
TOOL_COMMAND="goose"
TOOL_HOME_DIR=".config/goose"
TOOL_SESSION_NAME="goose"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (multi-provider)
# ============================================================

adapter_configure_backend() {
    _GOOSE_PROVIDERS=""
    _GOOSE_AUTH_MISSING=1

    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}Anthropic "
        _GOOSE_AUTH_MISSING=0
    fi
    if [ -n "${OPENAI_API_KEY:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}OpenAI "
        _GOOSE_AUTH_MISSING=0
    fi
    if [ -n "${GOOGLE_API_KEY:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}Google "
        _GOOSE_AUTH_MISSING=0
    fi

    # Azure OpenAI
    if [ -n "${AZURE_OPENAI_API_KEY:-}" ] || [ -n "${AZURE_OPENAI_ENDPOINT:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}Azure "
        _GOOSE_AUTH_MISSING=0
    fi

    # AWS Bedrock
    if [ -n "${AWS_ACCESS_KEY_ID:-}" ] || [ -n "${AWS_PROFILE:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}Bedrock "
        _GOOSE_AUTH_MISSING=0
    fi

    # OpenRouter
    if [ -n "${OPENROUTER_API_KEY:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}OpenRouter "
        _GOOSE_AUTH_MISSING=0
    fi

    # Local models (Ollama, etc.)
    if [ -n "${OLLAMA_HOST:-}" ] || [ -n "${LOCAL_ENDPOINT:-}" ]; then
        _GOOSE_PROVIDERS="${_GOOSE_PROVIDERS}Local "
        _GOOSE_AUTH_MISSING=0
    fi
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    mkdir -p "${TOOL_HOME}"

    # Create minimal config.yaml to bypass initial setup
    local config_file="${TOOL_HOME}/config.yaml"
    cat > "${config_file}" <<'EOF'
# Goose configuration (container managed)
provider: ${GOOSE_PROVIDER:-anthropic}
model: ${GOOSE_MODEL:-claude-sonnet-4-20250514}
mode: ${GOOSE_MODE:-auto}
EOF

    # Create secrets.yaml for API keys (used in headless/container environments)
    # Goose reads API keys from system keyring or secrets.yaml
    local secrets_file="${TOOL_HOME}/secrets.yaml"
    cat > "${secrets_file}" <<'EOF'
# API keys for Goose providers
# In containers, secrets.yaml is preferred over system keyring
EOF

    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        echo "anthropic: ${ANTHROPIC_API_KEY}" >> "${secrets_file}"
    fi
    if [ -n "${OPENAI_API_KEY:-}" ]; then
        echo "openai: ${OPENAI_API_KEY}" >> "${secrets_file}"
    fi
    if [ -n "${GOOGLE_API_KEY:-}" ]; then
        echo "google: ${GOOGLE_API_KEY}" >> "${secrets_file}"
    fi
    if [ -n "${AZURE_OPENAI_API_KEY:-}" ]; then
        echo "azure: ${AZURE_OPENAI_API_KEY}" >> "${secrets_file}"
    fi
    if [ -n "${OPENROUTER_API_KEY:-}" ]; then
        echo "openrouter: ${OPENROUTER_API_KEY}" >> "${secrets_file}"
    fi

    chmod 600 "${secrets_file}"
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    keel_status_kv "Backend" "Goose"
    if [ -n "${_GOOSE_PROVIDERS}" ]; then
        keel_status_kv "Providers" "${_GOOSE_PROVIDERS% }"
    else
        keel_status_kv "Providers" "none configured"
    fi
    keel_status_kv "Provider" "${GOOSE_PROVIDER:-default}"
    keel_status_kv "Model" "${GOOSE_MODEL:-default}"
}

adapter_print_warnings() {
    if [ "${_GOOSE_AUTH_MISSING:-1}" -eq 1 ]; then
        echo "WARNING: No AI provider configured. Goose requires at least one."
        echo "  Set one or more: ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY"
        echo "  Or for Azure: AZURE_OPENAI_API_KEY + AZURE_OPENAI_ENDPOINT"
        echo "  Or for Bedrock: AWS_ACCESS_KEY_ID (or AWS_PROFILE)"
        echo "  Or for OpenRouter: OPENROUTER_API_KEY"
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
    # passed verbatim to the agent CLI via run -t. This is a prompt injection
    # surface — the agent CLI is the trust boundary, not this script.
    local PROMPT="${*:-Review this codebase.}"

    local MAX_TURNS="${MAX_TURNS:-50}"
    local CONTEXT_STRATEGY="${GOOSE_CONTEXT_STRATEGY:-summarize}"
    local DISABLE_SESSION_NAMING="${GOOSE_DISABLE_SESSION_NAMING:-true}"
    local CLI_MIN_PRIORITY="${GOOSE_CLI_MIN_PRIORITY:-0.2}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "Max turns: ${MAX_TURNS}"
    echo "----------------------"

    # Export Goose environment variables for headless operation
    export GOOSE_MODE="${GOOSE_MODE:-auto}"
    export GOOSE_CONTEXT_STRATEGY="${CONTEXT_STRATEGY}"
    export GOOSE_MAX_TURNS="${MAX_TURNS}"
    export GOOSE_DISABLE_SESSION_NAMING="${DISABLE_SESSION_NAMING}"
    export GOOSE_CLI_MIN_PRIORITY="${CLI_MIN_PRIORITY}"

    # Run in headless mode with --no-session for one-off tasks
    exec "${TOOL_COMMAND}" run --no-session -t "${PROMPT}"
}
