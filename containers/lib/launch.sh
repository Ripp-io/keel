#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Shared launch helpers
# ============================================================
#
# Provides:
#   keel_launch_tmux — launch a command in a tmux session with
#                      a guard against terminal escape sequence
#                      leakage on fast exit
# ============================================================

# Launch a command inside a tmux session. If the command exits
# with a non-zero status, the pane stays alive so the user can
# read the error. This also prevents tmux's terminal capability
# handshake responses (DA2, XTVERSION) from leaking into the
# host shell when the tool exits before tmux processes them.
#
# Usage: keel_launch_tmux <session-name> <command> [args...]
keel_launch_tmux() {
    local session="$1"; shift
    exec tmux new-session -s "${session}" -- \
        bash -c '"$@"; ec=$?; if [ "$ec" -ne 0 ]; then printf "\nProcess exited (code %d). Press Enter to close.\n" "$ec"; read -r; fi; exit "$ec"' \
        _ "$@"
}
