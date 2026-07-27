#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Startup status banner
# ============================================================
#
# Two-column banner: KEEL art + version on the left,
# title + backend details + config status on the right.
#
# Requires: CONFIG_LAYERS and USER_CONFIG variables (from config.sh)
#           _GIT_IDENTITY_MISSING variable (from git.sh)
#           adapter_print_backend_status() and adapter_print_warnings()
#
# Provides:
#   keel_status_kv — format a "Label: value" line
#   print_status   — print the full two-column banner
# ============================================================

# Column widths
_KEEL_LEFT_W=34
_KEEL_LABEL_W=12

# Format a label: value line.
keel_status_kv() {
    printf '%-*s %s\n' "${_KEEL_LABEL_W}" "${1}:" "${2}"
}

# Two-column row with Unicode-safe padding.
_keel_row() {
    local left="$1"
    local right="${2:-}"
    local display_w=${#left}
    local pad=$(( _KEEL_LEFT_W - display_w ))
    if [ "${pad}" -lt 0 ]; then pad=0; fi
    printf '%s%*s%s\n' "${left}" "${pad}" '' "${right}"
}

# Horizontal rule for the right column.
_keel_rule() {
    local i
    for ((i = 0; i < 30; i++)); do printf '─'; done
}

print_status() {
    local title="${1:-Container}"

    # --- Gather right-column data ---

    local backend_lines=""
    if declare -f adapter_print_backend_status >/dev/null 2>&1; then
        backend_lines=$(adapter_print_backend_status)
    fi

    # Config layers from compiled output (single jq call)
    local layers=""
    local layers_json
    layers_json=$(keel query layers 2>/dev/null) || true
    if [ -n "${layers_json}" ] && [ "${layers_json}" != "null" ]; then
        layers=$(echo "${layers_json}" | jq -r '[.[] | .name // (.path | split("/") | last)] | join(" → ")')
    fi
    if [ -d "${USER_CONFIG:-}" ]; then
        if [ -n "${layers}" ]; then
            layers="${layers} → user"
        else
            layers="user"
        fi
    fi
    [ -z "${layers}" ] && layers="none"

    # SSH
    local ssh_dir="${HOME}/.ssh"
    local ssh_keys
    ssh_keys=$(find "${ssh_dir}" -name '*.pub' 2>/dev/null | wc -l)
    local ssh_status
    if [ "${ssh_keys}" -gt 0 ]; then
        ssh_status="${ssh_keys} public keys"
    else
        ssh_status="not mounted"
    fi

    # Git
    local git_name git_email git_status
    git_name=$(git config --global user.name 2>/dev/null || true)
    git_email=$(git config --global user.email 2>/dev/null || true)
    if [ -n "${git_name}" ] && [ -n "${git_email}" ]; then
        git_status="${git_name} <${git_email}>"
    else
        git_status="not configured"
    fi

    # --- Print two-column layout ---

    local title_line="${title}"
    local proj_ver="${PROJECT_VERSION:-${KEEL_VERSION:-}}"
    [ -n "${proj_ver}" ] && title_line="${title} · ${proj_ver}"
    _keel_row "" "${title_line}"
    _keel_row "" "$(_keel_rule)"

    # Right-side lines
    local right_lines=()
    if [ -n "${backend_lines}" ]; then
        while IFS= read -r line; do
            right_lines+=("${line}")
        done <<< "${backend_lines}"
    fi
    right_lines+=("$(keel_status_kv "Layers" "${layers}")")
    right_lines+=("$(keel_status_kv "SSH" "${ssh_status}")")
    right_lines+=("$(keel_status_kv "Git" "${git_status}")")

    # Art lines with dynamic version padding
    local ver="${KEEL_BUILD:-}"
    local ver_prefix="  ${ver}"
    local ver_len=${#ver_prefix}
    local art_col=13
    local ver_pad=$(( art_col - ver_len ))
    if [ "${ver_pad}" -lt 1 ]; then ver_pad=1; fi
    local ver_line
    ver_line=$(printf '%s%*s▜█████▘' "${ver_prefix}" "${ver_pad}" '')

    local art_lines=(
        "  ~~~▝███████████████████▘~~~"
        "                 ███"
        "  KEEL       ▟██████"
        "${ver_line}"
        "               ▐█▘"
    )

    # Interleave: max of art or right-side rows
    local art_count=${#art_lines[@]}
    local right_count=${#right_lines[@]}
    local max_count=$(( art_count > right_count ? art_count : right_count ))
    local i
    for ((i = 0; i < max_count; i++)); do
        local left=""
        local right=""
        if [ "${i}" -lt "${art_count}" ]; then
            left="${art_lines[$i]}"
        fi
        if [ "${i}" -lt "${right_count}" ]; then
            right="${right_lines[$i]}"
        fi
        _keel_row "${left}" "${right}"
    done

    printf '\n'

    # --- Warnings ---

    if declare -f adapter_print_warnings >/dev/null 2>&1; then adapter_print_warnings; fi

    if [ "${ssh_keys}" -eq 0 ]; then
        echo "NOTE: No SSH keys mounted. Git clone over SSH will fail."
        echo "  Mount with: -v ~/.ssh:${HOME}/.ssh:ro,Z"
        printf '\n'
    fi

    if [ "${_GIT_IDENTITY_MISSING:-1}" -eq 1 ]; then
        echo "NOTE: Git user identity not configured. Commits will fail."
        echo "  Pass with: -e GIT_USER_NAME=\"Your Name\" -e GIT_USER_EMAIL=\"you@example.com\""
        printf '\n'
    fi
}
