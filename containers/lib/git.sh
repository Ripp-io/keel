#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Git identity configuration
# ============================================================
#
# Sets global git user.name and user.email from env vars or
# mounted .gitconfig. Sourced by adapters.
#
# Provides:
#   configure_git_identity — set git user from env/mount
#
# Sets:
#   _GIT_IDENTITY_MISSING — 1 if identity not configured
# ============================================================

configure_git_identity() {
    local has_name has_email
    has_name=$(git config --global user.name 2>/dev/null || true)
    has_email=$(git config --global user.email 2>/dev/null || true)

    # Try mounted .gitconfig
    if [ -z "${has_name}" ] || [ -z "${has_email}" ]; then
        if [ -f "${HOME}/.gitconfig" ]; then
            local mounted_name mounted_email
            mounted_name=$(git config -f "${HOME}/.gitconfig" user.name 2>/dev/null || true)
            mounted_email=$(git config -f "${HOME}/.gitconfig" user.email 2>/dev/null || true)
            [ -z "${has_name}" ] && [ -n "${mounted_name}" ] && git config --global user.name "${mounted_name}" && has_name="${mounted_name}"
            [ -z "${has_email}" ] && [ -n "${mounted_email}" ] && git config --global user.email "${mounted_email}" && has_email="${mounted_email}"
        fi
    fi

    # Env vars override
    if [ -n "${GIT_USER_NAME:-}" ]; then
        git config --global user.name "${GIT_USER_NAME}"
        has_name="${GIT_USER_NAME}"
    fi
    if [ -n "${GIT_USER_EMAIL:-}" ]; then
        git config --global user.email "${GIT_USER_EMAIL}"
        has_email="${GIT_USER_EMAIL}"
    fi

    if [ -z "${has_name}" ] || [ -z "${has_email}" ]; then
        _GIT_IDENTITY_MISSING=1
    else
        _GIT_IDENTITY_MISSING=0
    fi
}
