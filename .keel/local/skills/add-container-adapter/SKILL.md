---
name: add-container-adapter
description: Build a container adapter for an AI coding agent that already has a compiler adapter. Creates install.sh and adapter.sh scripts for the agent's container lifecycle. Use when adding container support (Tier 2/3) for an agent.
---

# Add Container Adapter

Build the container adapter scripts for an AI coding agent.
The container adapter handles agent installation, authentication, onboarding bypass, and launch inside a Keel container.

This skill assumes the agent already has a compiler adapter (`pkg/compiler/<agent>.go`).
If not, use the `add-compiler-adapter` skill first.

## Prerequisites

- The target agent must have a CLI binary or headless mode that can run inside a container.
- You need to know the agent's install method, auth model, and CLI interface.
- A compiler adapter must already exist (the container compiles config at boot using `keel compile`).

## Step 1: Research the target agent

Read `docs/content/docs/ecosystem/agents.md` for the target agent's detail card.
Gather these details:

| Detail | Why you need it | Example (Claude) | Example (Cursor) |
|--------|-----------------|-------------------|-------------------|
| Install method | For `install.sh` | `npm install -g @anthropic-ai/claude-code` | `curl https://cursor.com/install \| bash` |
| CLI binary name | For `TOOL_COMMAND` | `claude` | `agent` / `cursor-agent` |
| Auth model | For `adapter_configure_backend` | Vertex AI or API key | API key only |
| Home directory | For `TOOL_HOME_DIR` | `.claude` | `.cursor` |
| First-run behavior | For `adapter_write_onboarding` | Writes onboarding files | Self-contained, no onboarding |
| Headless mode | For `adapter_launch_headless` | `claude -p "prompt" --output-format stream-json` | `cursor-agent -p "prompt" --output-format stream-json` |
| Tmux compatibility | For `adapter_launch_tmux` | Works in tmux | Works in tmux |

## Step 2: Study existing adapters

Read these files to understand the adapter pattern:

1. `adapters/claude/install.sh` -- Simple npm install with version pinning.
2. `adapters/claude/adapter.sh` -- Full 7-function adapter with Vertex AI and API key auth, onboarding bypass, tmux launch.
3. `adapters/cursor/install.sh` -- More complex: downloads installer, relocates binaries, optional SHA256 verification.
4. `adapters/cursor/adapter.sh` -- API key-only auth, minimal onboarding.
5. `adapters/theia/install.sh` -- Theia IDE adapter, useful as a third reference.
6. `adapters/theia/adapter.sh` -- Desktop IDE adapter pattern.

All adapters source shared libraries from `/usr/local/lib/` (source tree: `containers/lib/`):
- `config.sh` -- Configuration utilities
- `git.sh` -- Git operations
- `launch.sh` -- Launch helpers (tmux session management, headless mode)
- `status.sh` -- Status display utilities
- `workspace.sh` -- Workspace setup and lifecycle

Both adapters set these variables at the top:
- `TOOL_NAME` -- Human-readable name (e.g., `"Claude Code"`)
- `TOOL_COMMAND` -- CLI binary name (e.g., `"claude"`)
- `TOOL_HOME_DIR` -- Dot-directory name (e.g., `".claude"`)
- `TOOL_SESSION_NAME` -- Tmux session name (e.g., `"claude"`)
- `TOOL_HOME` -- Full path (`"${HOME}/${TOOL_HOME_DIR}"`)

## Step 3: Create install.sh

Create `adapters/<agent>/install.sh`.

This script runs during `podman build` (image build time).
It receives `TOOL_VERSION` as an environment variable.

Requirements:
- `#!/usr/bin/env bash` with `set -euo pipefail`
- Install the agent CLI at the specified version (or latest)
- Clean up caches and temporary files
- Print installed version for build log
- The binary must be available on `PATH` after install

Pattern for npm-based agents:
```bash
TOOL_VERSION="${TOOL_VERSION:-latest}"
if [ "${TOOL_VERSION}" = "latest" ]; then
    npm install -g <package-name>
else
    npm install -g "<package-name>@${TOOL_VERSION}"
fi
npm cache clean --force
echo "<Agent> $(<command> --version 2>/dev/null || echo "${TOOL_VERSION}") installed."
```

Pattern for binary download agents:
```bash
TOOL_VERSION="${TOOL_VERSION:-latest}"
# Download binary
# Verify checksum if AGENT_INSTALLER_SHA256 is set
# Place in /usr/local/bin/
# Verify it runs
echo "<Agent> installed."
```

## Step 4: Create adapter.sh

Create `adapters/<agent>/adapter.sh`.

This script runs at container start time.
It must implement 7 functions that the entrypoint calls at specific lifecycle stages.

### Header and shared libs

```bash
#!/usr/bin/env bash
set -euo pipefail

# Source lib utilities
source /usr/local/lib/config.sh
source /usr/local/lib/git.sh
source /usr/local/lib/launch.sh
source /usr/local/lib/status.sh

TOOL_NAME="<Agent Name>"
TOOL_COMMAND="<binary>"
TOOL_HOME_DIR=".<agent>"
TOOL_SESSION_NAME="<agent>"
TOOL_HOME="${HOME}/${TOOL_HOME_DIR}"
```

Each adapter can also include a `config/` directory with default agent configuration.
This is referenced at runtime via `KEEL_AGENT_CONFIG_DIR` (e.g., `adapters/claude/config/` contains default commands and settings).

### Function 1: adapter_configure_backend

Configure authentication.
Check for required environment variables and set up credentials.

Common patterns:
- API key: Check `<AGENT>_API_KEY` env var, set a missing flag
- OAuth: Configure token files
- Vertex AI / cloud provider: Resolve project ID from env or credential JSON

Set `_<AGENT>_AUTH_MISSING=0` on success, `_<AGENT>_AUTH_MISSING=1` on failure.
The status display functions use this flag.

### Function 2: adapter_write_onboarding

Bypass first-run setup flows so the agent starts cleanly in the container.

Common patterns:
- Write config files that skip onboarding prompts
- Accept terms of service programmatically
- Set telemetry opt-out preferences
- Create expected directory structures

If the agent has no first-run flow, this function can be empty.

### Function 3: adapter_print_backend_status

Print authentication status to the container boot log.
Called during the status display phase.

Show which auth method is configured and whether credentials are present.

### Function 4: adapter_print_warnings

Print any warnings about the agent configuration.
Called during the status display phase.

Common warnings: missing optional config, deprecated settings, version mismatches.

### Function 5: adapter_launch

Launch the agent in the default mode (interactive terminal).
This is the simple launch -- just exec the agent command.

```bash
adapter_launch() {
    exec ${TOOL_COMMAND}
}
```

### Function 6: adapter_launch_tmux

Launch the agent inside a tmux session for session management.
The shared `launch.sh` library provides helpers for tmux session management.

```bash
adapter_launch_tmux() {
    launch_tmux_session "${TOOL_SESSION_NAME}" "${TOOL_COMMAND}"
}
```

### Function 7: adapter_launch_headless

Launch the agent in headless mode for CI/CD runners.
Takes a prompt as an argument, runs the agent non-interactively, exits.

```bash
adapter_launch_headless() {
    local prompt="$1"
    ${TOOL_COMMAND} -p "${prompt}" --output-format stream-json
}
```

Adapt the CLI flags to the target agent's headless interface.

## Step 5: Build and test

Build the container image:

```bash
make build AGENT=<agent>
```

If the build fails:
- Check that `install.sh` completes without error
- Verify the binary is on PATH after install
- Check that `adapter.sh` sources the shared libs correctly

## Step 6: Run and verify

Start the container:

```bash
make run AGENT=<agent>
```

Verify:
- The agent starts without errors
- Auth is configured correctly (check status output)
- `keel explain` works inside the container
- The agent can accept prompts and respond
- Tmux mode works (`adapter_launch_tmux`)

## Step 7: Update docs/content/docs/ecosystem/agents.md

Update the target agent's entry in `docs/content/docs/ecosystem/agents.md`:
- Change Container column from `🔜` to `✅ CLI` (or `✅ CLI/Dev` etc.)
- Change Status to reflect the shipped adapter
- Add an Install row to the detail card

## Checklist

- [ ] Target agent CLI researched (install method, auth, headless mode)
- [ ] `adapters/<agent>/install.sh` created and tested
- [ ] `adapters/<agent>/adapter.sh` created with all 7 functions
- [ ] Shared lib variables set (`TOOL_NAME`, `TOOL_COMMAND`, `TOOL_HOME_DIR`, `TOOL_SESSION_NAME`, `TOOL_HOME`)
- [ ] `make build AGENT=<agent>` succeeds
- [ ] `make run AGENT=<agent>` launches the agent correctly
- [ ] Auth configuration works (API key or cloud provider)
- [ ] Onboarding bypass works (no first-run prompts)
- [ ] Headless mode works (for runner tier)
- [ ] `docs/content/docs/ecosystem/agents.md` updated with shipped status
