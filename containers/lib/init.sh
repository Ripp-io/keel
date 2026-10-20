#!/usr/bin/env bash
set -euo pipefail
# ============================================================
# Shared entrypoint initialization
# ============================================================
#
# Provides:
#   keel_verify_adapter — verify adapter implements the contract
#   keel_init           — verify + configure backend + compile
#
# Source after adapter.sh and before calling adapter functions.
# ============================================================

# Source lib utilities so entrypoints have explicit access to
# configure_git_identity, print_status, etc. — without relying
# on the adapter having sourced them.
source /usr/local/lib/config.sh
source /usr/local/lib/git.sh
source /usr/local/lib/launch.sh
source /usr/local/lib/status.sh

# Full adapter contract. Adapters must implement all 7 functions
# regardless of which entrypoint (base, dev, runner) uses them.
KEEL_ADAPTER_CONTRACT=(
    adapter_configure_backend
    adapter_write_onboarding
    adapter_print_backend_status
    adapter_print_warnings
    adapter_launch
    adapter_launch_tmux
    adapter_launch_headless
)

# Verify the sourced adapter implements the full contract.
keel_verify_adapter() {
    for fn in "${KEEL_ADAPTER_CONTRACT[@]}"; do
        if ! declare -f "${fn}" >/dev/null 2>&1; then
            echo "ERROR: Adapter missing required function: ${fn}" >&2
            exit 1
        fi
    done
}

# Standard initialization sequence: verify adapter, configure
# backend, compile config layers into tool-native output.
keel_init() {
    keel_verify_adapter
    adapter_configure_backend
    keel compile --output "${AGENT}=/keel"
}
