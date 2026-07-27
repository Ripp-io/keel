#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Repo manifest cloning and workspace init (init.d/ scripts)
# ============================================================
#
# Source from dev/IDE entrypoints after the adapter.
#
# Expected variables:
#   CONFIG_LAYERS — glob pattern for layer directories (set by image ENV)
#   AUTO_CLONE    — "1" clones missing repos without prompting
#   REINIT        — "1" re-runs init.d/ even when already initialized
#   TOOL_COMMAND  — used to capture tool version in state
# ============================================================

# Keel workspace state directory and state file.
KEEL_DIR="${KEEL_DIR:-/keel/workspace/.keel}"
KEEL_STATE="${KEEL_DIR}/state.yml"

# --- URL validation ---

# Validate that a git URL uses a safe scheme. Rejects ext:: and other
# arbitrary remote helpers that could execute commands.
validate_git_url() {
    local url="$1"
    case "${url}" in
        https://*|http://*|git://*|ssh://*)
            # Verify URL has a host component after the scheme
            local host
            host=$(echo "${url}" | sed -E 's|^[a-z]+://||' | cut -d'/' -f1 | cut -d':' -f1)
            if [ -z "${host}" ] || [ "${host}" = "${url}" ]; then
                echo "ERROR: Git URL has no host: ${url}" >&2
                return 1
            fi
            ;;
        git@*)
            # Verify git@ has a host component (git@host:path format)
            local host
            host=$(echo "${url}" | sed 's/^git@//' | cut -d':' -f1)
            if [ -z "${host}" ]; then
                echo "ERROR: Git URL has no host: ${url}" >&2
                return 1
            fi
            ;;
        *)
            echo "ERROR: Unsupported git URL scheme: ${url}" >&2
            echo "  Only https://, http://, ssh://, git://, and git@ URLs are allowed" >&2
            return 1
            ;;
    esac
}

# --- Repo manifest ---

# Check and clone repos from compiled config via keel query.
init_repos() {
    local repos_json
    repos_json=$(keel query repos 2>/dev/null) || return 0
    [ "${repos_json}" = "null" ] && return 0
    [ "${repos_json}" = "{}" ] && return 0

    local missing=()
    local present=()

    # Single jq call to extract name, path, and required status for all repos
    while IFS=$'\t' read -r name path required; do
        if [ -d "/keel/workspace/${path}/.git" ]; then
            present+=("${name}")
        else
            local marker="optional"
            [ "${required}" = "true" ] && marker="required"
            missing+=("${name}|${path}|${marker}")
        fi
    done < <(echo "${repos_json}" | jq -r 'to_entries[] | [.key, .value.path, (if .value.required == false then "false" else "true" end)] | @tsv')

    # Display repos: ● present, ○ missing, * = required
    echo "Repositories:"
    local display=""
    for name in ${present[@]+"${present[@]}"}; do
        if [ -n "${display}" ]; then display="${display}  "; fi
        display="${display}● ${name}"
    done
    for entry in ${missing[@]+"${missing[@]}"}; do
        local name path marker
        IFS='|' read -r name path marker _ <<< "${entry}"
        if [ -n "${display}" ]; then display="${display}  "; fi
        if [ "${marker}" = "required" ]; then
            display="${display}○ ${name}*"
        else
            display="${display}○ ${name}"
        fi
    done
    echo "  ${display}"
    echo ""

    if [ ${#missing[@]} -eq 0 ]; then
        return
    fi

    if [ "${AUTO_CLONE:-0}" = "1" ]; then
        clone_repos "${missing[@]}"
    elif [ -t 0 ]; then
        prompt_clone "${missing[@]}"
    else
        echo "Missing repos detected. Run with AUTO_CLONE=1 to clone automatically."
        echo ""
    fi
}

# Interactive clone menu.
prompt_clone() {
    local entries=("$@")

    read -r -n 1 -p "Clone missing repos? [a]ll  [r]equired  [s]elect  [n]o: " choice
    echo ""

    case "${choice}" in
        a|A)
            clone_repos "${entries[@]}"
            ;;
        r|R)
            local required_only=()
            for entry in "${entries[@]}"; do
                local marker
                IFS='|' read -r _ _ marker _ <<< "${entry}"
                [ "${marker}" = "required" ] && required_only+=("${entry}")
            done
            if [ ${#required_only[@]} -gt 0 ]; then
                clone_repos "${required_only[@]}"
            else
                echo "No required repos missing."
            fi
            ;;
        s|S)
            for entry in "${entries[@]}"; do
                local name
                IFS='|' read -r name _ _ _ <<< "${entry}"
                read -r -n 1 -p "  Clone ${name}? [y/N]: " yn
                echo ""
                if [[ "${yn}" =~ ^[Yy]$ ]]; then
                    clone_single "${entry}"
                fi
            done
            ;;
        *)
            echo "Skipping repo initialization."
            ;;
    esac
    echo ""
}

# Clone each entry.
clone_repos() {
    local entries=("$@")
    for entry in "${entries[@]}"; do
        clone_single "${entry}"
    done
}

# Clone a single repo via keel query.
clone_single() {
    local entry="$1"
    local name path _
    IFS='|' read -r name path _ <<< "${entry}"

    local repo_json url branch
    repo_json=$(keel query repo "${name}") || {
        echo "WARNING: Could not query repo '${name}', skipping" >&2
        return 1
    }
    if [ -z "${repo_json}" ] || [ "${repo_json}" = "null" ]; then
        echo "WARNING: No config found for repo '${name}', skipping" >&2
        return 1
    fi
    url=$(echo "${repo_json}" | jq -r '.url')
    branch=$(echo "${repo_json}" | jq -r '.branch // "main"')

    if ! validate_git_url "${url}"; then
        echo "  ✗ ${name} — rejected (unsafe URL scheme)" >&2
        return 1
    fi

    # Reject absolute paths
    case "${path}" in
        /*)
            echo "ERROR: Repo path must be relative: ${path}" >&2
            return 1
            ;;
    esac

    # Validate path doesn't escape workspace via traversal or symlinks
    local workspace="/keel/workspace"
    local resolved
    resolved=$(realpath -m --relative-to="${workspace}" "${workspace}/${path}")
    case "${resolved}" in
        ../*)
            echo "ERROR: Repo path escapes workspace after resolution: ${path} → ${resolved}" >&2
            return 1
            ;;
    esac

    echo "  Cloning ${name} (${branch}) → /keel/workspace/${path} ..."
    if git clone -q --branch "${branch}" --single-branch "${url}" "/keel/workspace/${path}"; then
        echo "  ✓ ${name}"
    else
        echo "  ✗ ${name} — clone failed (check SSH keys or repo access)" >&2
    fi
}

# --- Workspace init ---

# Returns init.d directory path from compiled config, or empty string.
find_init_dir() {
    keel query init-dir 2>/dev/null || echo ""
}

# True if state.yml exists and marks initialized: true.
is_initialized() {
    [ -f "${KEEL_STATE}" ] && grep -q '^initialized: true' "${KEEL_STATE}" 2>/dev/null
}

# Write state.yml with init status and metadata.
write_initialized_marker() {
    local status="${1:-true}"
    mkdir -p "${KEEL_DIR}"

    cat > "${KEEL_STATE}" <<EOF
# Keel workspace state — written by workspace init
initialized: ${status}
timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)
keel_build: ${KEEL_BUILD:-dev}
tool: ${TOOL_NAME:-unknown}
tool_version: $(${TOOL_COMMAND:-echo} --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo 'unknown')
EOF
}

# Run sorted *.sh in init_dir/ unless already initialized (unless REINIT=1).
#
# SECURITY NOTE: Init scripts from config layers execute with the full
# container environment, including any mounted secrets (API keys, credentials).
# Only mount config layers from trusted sources. In non-interactive mode with
# REINIT=1, scripts run without user confirmation.
init_workspace() {
    local init_dir
    init_dir=$(find_init_dir)

    if [ -z "${init_dir}" ]; then
        return
    fi

    local _ws_scripts
    mapfile -t _ws_scripts < <(find "${init_dir}" -maxdepth 1 -name '*.sh' -type f 2>/dev/null | sort)

    if [ "${#_ws_scripts[@]}" -eq 0 ]; then
        return
    fi

    if is_initialized && [ "${REINIT:-0}" != "1" ]; then
        echo "Workspace already initialized ($(grep '^timestamp:' "${KEEL_STATE}" 2>/dev/null | cut -d' ' -f2 || echo 'unknown'))"
        echo "  Run with REINIT=1 to re-initialize."
        echo ""
        return
    fi

    if [ -t 0 ] && [ "${REINIT:-0}" != "1" ]; then
        echo "Workspace initialization scripts available:"
        local script
        for script in "${_ws_scripts[@]}"; do
            echo "  • $(basename "${script}")"
        done
        echo ""
        read -r -n 1 -p "Run workspace initialization? [y/N]: " yn
        echo ""
        if [[ ! "${yn}" =~ ^[Yy]$ ]]; then
            echo "Skipping workspace initialization."
            write_initialized_marker "false"
            echo ""
            return
        fi
        echo ""
    fi

    local failed=0
    local script script_name ec
    for script in "${_ws_scripts[@]}"; do
        script_name=$(basename "${script}")
        echo "  Running ${script_name} ..."
        if bash "${script}"; then
            echo "  ✓ ${script_name}"
        else
            ec=$?
            echo "  ✗ ${script_name} — failed (exit code ${ec})"
            failed=1
        fi
    done
    echo ""

    if [ "${failed}" -eq 0 ]; then
        write_initialized_marker "true"
        echo "Workspace initialized successfully."
    else
        write_initialized_marker "false"
        echo "Workspace initialization completed with errors."
    fi
    echo ""

    # Pause so the user can review init output before the agent launches
    if [ -t 0 ]; then
        read -r -n 1 -s -p "Press any key to continue..."
        echo ""
    fi
}
