#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Claude Code adapter — runtime functions only
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

TOOL_NAME="Claude Code"
TOOL_COMMAND="claude"
TOOL_HOME_DIR=".claude"
TOOL_SESSION_NAME="claude"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"

# ============================================================
# Backend configuration (Vertex AI vs direct Anthropic API)
# ============================================================

adapter_configure_backend() {
    local cred_file="${GOOGLE_APPLICATION_CREDENTIALS:-/secrets/gcp-sa.json}"

    # Fill project ID from env or credential JSON
    if [ -z "${ANTHROPIC_VERTEX_PROJECT_ID:-}" ] && [ -n "${GOOGLE_CLOUD_PROJECT:-}" ]; then
        export ANTHROPIC_VERTEX_PROJECT_ID="${GOOGLE_CLOUD_PROJECT}"
    fi
    if [ -z "${ANTHROPIC_VERTEX_PROJECT_ID:-}" ] && [ -f "${cred_file}" ] && command -v jq &>/dev/null; then
        local pid
        pid=$(jq -r '.project_id // .quota_project_id // empty' "${cred_file}" 2>/dev/null || true)
        if [ -n "${pid}" ] && [ "${pid}" != "null" ]; then
            export ANTHROPIC_VERTEX_PROJECT_ID="${pid}"
        fi
    fi

    if [ -n "${CLAUDE_CODE_USE_VERTEX+x}" ] && [ -n "${CLAUDE_CODE_USE_VERTEX}" ]; then
        export CLAUDE_CODE_USE_VERTEX
        return 0
    fi
    if [ -n "${ANTHROPIC_VERTEX_PROJECT_ID:-}" ]; then
        export CLAUDE_CODE_USE_VERTEX=1
        return 0
    fi
    if [ -f "${cred_file}" ]; then
        export CLAUDE_CODE_USE_VERTEX=1
        return 0
    fi
    export CLAUDE_CODE_USE_VERTEX=0
}

# ============================================================
# First-run onboarding bypass
# ============================================================

adapter_write_onboarding() {
    local config_file="${HOME}/.claude.json"
    local theme="${KEEL_THEME:-${CLAUDE_THEME:-dark}}"

    local claude_version
    claude_version=$(claude --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
    claude_version="${claude_version:-0.0.0}"

    if command -v jq &>/dev/null; then
        jq -n \
            --arg theme "${theme}" \
            --arg version "${claude_version}" \
            '{
                numStartups: 0,
                hasCompletedOnboarding: true,
                lastOnboardingVersion: $version,
                lastReleaseNotesSeen: $version,
                projects: {
                    "/keel/workspace": {
                        allowedTools: [],
                        hasTrustDialogAccepted: true,
                        projectOnboardingSeenCount: 1
                    }
                }
            }' > "${config_file}"
    else
        cat > "${config_file}" <<EOF
{
  "numStartups": 0,
  "hasCompletedOnboarding": true,
  "lastOnboardingVersion": "${claude_version}",
  "lastReleaseNotesSeen": "${claude_version}",
  "projects": {
    "/keel/workspace": {
      "allowedTools": [],
      "hasTrustDialogAccepted": true,
      "projectOnboardingSeenCount": 1
    }
  }
}
EOF
    fi
}

# ============================================================
# Status display hooks
# ============================================================

adapter_print_backend_status() {
    if [ "${CLAUDE_CODE_USE_VERTEX:-0}" = "1" ]; then
        keel_status_kv "Backend" "Vertex AI"
        keel_status_kv "Region" "${CLOUD_ML_REGION:-not set}"
        keel_status_kv "Project" "${ANTHROPIC_VERTEX_PROJECT_ID:-not set}"
    else
        keel_status_kv "Backend" "non-Vertex (ANTHROPIC_API_KEY)"
    fi
    keel_status_kv "Model" "${ANTHROPIC_DEFAULT_SONNET_MODEL:-default}"
}

adapter_print_warnings() {
    if [ "${CLAUDE_CODE_USE_VERTEX:-0}" = "1" ]; then
        local cred_file="${GOOGLE_APPLICATION_CREDENTIALS:-/secrets/gcp-sa.json}"
        if [ ! -f "${cred_file}" ]; then
            echo "WARNING: GCP credentials not found at ${cred_file}"
            echo "  Mount with: -v /path/to/sa-key.json:${cred_file}:ro,Z"
            echo ""
        fi

        if [ -z "${ANTHROPIC_VERTEX_PROJECT_ID:-}" ]; then
            echo "WARNING: ANTHROPIC_VERTEX_PROJECT_ID is not set (Vertex mode)"
            echo "  Pass with: -e ANTHROPIC_VERTEX_PROJECT_ID=my-project"
            echo ""
        fi
    else
        if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
            echo "NOTE: Direct API mode — set ANTHROPIC_API_KEY (or use OAuth where supported),"
            echo "  or pass -e ANTHROPIC_VERTEX_PROJECT_ID=... for Vertex without CLAUDE_CODE_USE_VERTEX."
            echo ""
        fi
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
    # passed verbatim to the agent CLI via -p. This is a prompt injection
    # surface — the agent CLI is the trust boundary, not this script. Do not
    # add shell interpolation of PROMPT content in subsequent commands.
    local PROMPT="${*:-Review this codebase.}"

    local MAX_TURNS="${MAX_TURNS:-15}"
    local MAX_BUDGET="${MAX_BUDGET_USD:-10.00}"
    local OUTPUT_FORMAT="${OUTPUT_FORMAT:-stream-json}"
    local ALLOWED_TOOLS="${ALLOWED_TOOLS:-Read,Bash(grep:*),Bash(find:*),Bash(wc:*),Bash(git:*)}"

    echo "--- ${TOOL_NAME} Runner ---"
    echo "Prompt:  ${PROMPT}"
    adapter_print_backend_status
    echo "Budget:  \$${MAX_BUDGET}"
    echo "----------------------"

    IFS=',' read -ra TOOLS <<< "${ALLOWED_TOOLS}"
    local TOOL_ARGS=()
    for tool in "${TOOLS[@]}"; do
        TOOL_ARGS+=("--allowedTools" "${tool}")
    done

    exec "${TOOL_COMMAND}" \
        -p "${PROMPT}" \
        --bare \
        "${TOOL_ARGS[@]}" \
        --output-format "${OUTPUT_FORMAT}" \
        --max-turns "${MAX_TURNS}" \
        --max-budget-usd "${MAX_BUDGET}"
}
