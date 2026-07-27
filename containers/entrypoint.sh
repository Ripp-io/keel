#!/usr/bin/env bash
set -euo pipefail
trap 'echo "ERROR: entrypoint failed at line $LINENO (exit $?)" >&2' ERR

source /usr/local/lib/adapter/adapter.sh

# Verify adapter implements required contract
for fn in adapter_configure_backend adapter_write_onboarding adapter_print_backend_status adapter_print_warnings adapter_launch adapter_launch_tmux adapter_launch_headless; do
    if ! declare -f "${fn}" >/dev/null 2>&1; then
        echo "ERROR: Adapter missing required function: ${fn}" >&2
        exit 1
    fi
done

adapter_configure_backend

# Compile config layers into tool-native output
keel compile --output "${AGENT}=/keel"

adapter_write_onboarding

# Configure git identity
configure_git_identity

print_status

adapter_launch "$@"
