#!/usr/bin/env bash
set -euo pipefail
trap 'echo "ERROR: entrypoint failed at line $LINENO (exit $?)" >&2' ERR

source /usr/local/lib/adapter/adapter.sh
source /usr/local/lib/init.sh

keel_init

# Launch the adapter in the background so we can verify it started
adapter_launch_headless "$@" &
ADAPTER_PID=$!
echo "${ADAPTER_PID}" > /tmp/keel-adapter.pid

# Wait briefly, then mark healthy only if the process is still alive
sleep 2
if kill -0 "${ADAPTER_PID}" 2>/dev/null; then
    touch /tmp/keel-healthy
else
    echo "ERROR: adapter process exited early (pid ${ADAPTER_PID})" >&2
    exit 1
fi

# Wait for the adapter process so the container stays alive
wait "${ADAPTER_PID}"
