#!/usr/bin/env bash
set -euo pipefail
trap 'echo "ERROR: entrypoint failed at line $LINENO (exit $?)" >&2' ERR

source /usr/local/lib/adapter/adapter.sh
source /usr/local/lib/workspace.sh
source /usr/local/lib/init.sh

AUTO_CLONE="${AUTO_CLONE:-0}"
USE_TMUX="${KEEL_TMUX:-1}"
REINIT="${REINIT:-0}"

keel_init

adapter_write_onboarding
configure_git_identity

title=$(keel query name 2>/dev/null || true)
title="${title:-Dev Environment}"

print_status "${title}"
init_repos
init_workspace

if [ "${USE_TMUX}" = "1" ] && command -v tmux &>/dev/null && [ -t 0 ]; then
    adapter_launch_tmux "$@"
else
    adapter_launch "$@"
fi
