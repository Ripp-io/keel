#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Theia IDE — adapter install script
# ============================================================
# Theia is pre-installed in the base image (FROM the official
# GHCR image). This script verifies the installation and
# creates a launch wrapper at /usr/local/bin/theia.
# ============================================================

THEIA_APP_DIR="/home/theia/applications/browser"

# Verify Theia is present
if [ ! -f "${THEIA_APP_DIR}/lib/backend/main.js" ]; then
    echo "ERROR: Theia backend not found at ${THEIA_APP_DIR}" >&2
    echo "  The web Containerfile should FROM ghcr.io/eclipse-theia/theia-ide/theia-ide" >&2
    exit 1
fi

# Create launch wrapper
cat > /usr/local/bin/theia <<WRAPPER
#!/usr/bin/env bash
set -euo pipefail
cd "${THEIA_APP_DIR}"
exec node lib/backend/main.js "\$@"
WRAPPER
chmod +x /usr/local/bin/theia

echo "Theia IDE verified at ${THEIA_APP_DIR}."
