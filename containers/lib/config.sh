#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Config variables for keel shell scripts
# ============================================================
#
# Config assembly is handled by `keel compile`.
# Shell scripts access compiled data via `keel query`.
#
# Variables:
#   CONFIG_LAYERS  — glob pattern for layer directories (set by image ENV)
#   USER_CONFIG    — user config mount point (default: /mnt/user-config)
# ============================================================

CONFIG_LAYERS="${CONFIG_LAYERS:-}"
USER_CONFIG="${USER_CONFIG:-/mnt/user-config}"
