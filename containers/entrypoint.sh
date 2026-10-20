#!/usr/bin/env bash
set -euo pipefail
trap 'echo "ERROR: entrypoint failed at line $LINENO (exit $?)" >&2' ERR

source /usr/local/lib/adapter/adapter.sh
source /usr/local/lib/init.sh

keel_init

adapter_write_onboarding
configure_git_identity
print_status

adapter_launch "$@"
