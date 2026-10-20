---
title: "Runtime Contract"
weight: 5
---

# Container Runtime Contract

The API surface for building, running, and extending Keel containers.

## Build Args

Set at image build time via `podman build --build-arg`.

| Arg | Default | Description |
|-----|---------|-------------|
| `AGENT` | `claude` | Which AI tool to install (see below) |
| `TOOL_VERSION` | `latest` | Pin the AI tool version (e.g. `2.1.126`) |
| `KEEL_BUILD` | `dev` | Keel engine version tag |
| `TAG` | `latest` | Image tag used in the FROM chain |

### Supported AGENT values

| Value | Agent | Install |
|-------|-------|---------|
| `claude` | Claude Code | npm |
| `cursor` | Cursor Agent CLI | binary |
| `opencode` | OpenCode | npm |
| `antigravity` | Antigravity CLI | binary |
| `goose` | Goose | binary |
| `kilo` | Kilo Code | npm |
| `codex` | Codex CLI | npm |
| `qwen` | Qwen Code | npm |
| `kiro` | Kiro | binary |
| `cline` | Cline | npm |
| `aider` | Aider | pip |
| `vibe` | Mistral Vibe | pip/uv |
| `theia` | Theia IDE | binary |

## Runtime Environment Variables

Set at container run time via `-e VAR=value`.

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_LAYERS` | `/keel/layers/*` | Colon-separated layer paths. Supports `/*` glob for auto-discovery |
| `USER_CONFIG` | `/mnt/user-config` | Path to user's native config directory |
| `USER_PRIORITY` | `high` | `high` = user layer last (wins), `low` = user layer first (overridden) |
| `REINIT` | `0` | Set to `1` to force re-run of workspace init scripts |
| `AUTO_CLONE` | `0` | Set to `1` to auto-clone repos defined in keel.yml |
| `AGENT` | Set at build | Which agent adapter to use (set by Containerfile, rarely overridden) |

### Agent-Specific Variables

These depend on the selected agent and are typically passed at run time.

**Claude:**
| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Direct API key (mutually exclusive with Vertex) |
| `ANTHROPIC_VERTEX_PROJECT_ID` | GCP project ID for Vertex AI |
| `CLOUD_ML_REGION` | GCP region (e.g. `us-east5`) |

**Cursor:**
| Variable | Description |
|----------|-------------|
| `CURSOR_API_KEY` | Cursor API key for agent CLI |

## Mount Points

Volumes bound at run time via `-v host:container`.

| Container Path | Purpose | Mode |
|----------------|---------|------|
| `/mnt/user-config` | User's `~/.claude` or `~/.cursor` directory | `ro` |
| `/keel/workspace` | Existing repos (skips clone when mounted) | `rw` |
| `/home/keel/.ssh` | SSH keys for git operations | `ro` |
| `/home/keel/.gitconfig` | Git identity and preferences | `ro` |
| `/secrets/` | API keys, service account credentials | `ro` |

## Internal Filesystem

After the entrypoint runs, the container filesystem looks like:

```
/keel/                              # WORKDIR parent
  .claude/ or .cursor/              # Compiled output (by keel compile)
    .keel.json                      # Compilation manifest (provenance, locks)
  workspace/                        # Cloned repos (or mounted)

/keel/layers/                        # Baked config layers (e.g. 00-base/)
  00-base/                           # Base layer (shipped with image)
/mnt/user-config/                   # Mounted user config (highest priority)

/usr/local/lib/adapter/             # Agent adapter (install.sh, adapter.sh)
/usr/local/bin/keel                 # Go CLI binary
/usr/local/bin/entrypoint.sh        # Container entrypoint
```

## Image Hierarchy

Keel images build in a chain. Downstream images extend upstream ones:

```
keel-{agent}:{tag}                  # Base: Go CLI + AI tool + adapter
  └─ keel-dev-{agent}:{tag}        # Dev: + tmux, ripgrep, vim, workspace.sh
       └─ keel-{project}-{agent}:{tag}   # Project: + project tooling + baked config
```

Each level adds capabilities without duplicating the layers below.

## Entrypoint Sequence

When the container starts, the entrypoint runs:

1. `adapter_configure_backend` — Set up AI tool authentication
2. `keel compile --output "${AGENT}=/keel"` — Compile config layers (`--output agent=path`, repeatable)
3. `adapter_write_onboarding` — Write tool-specific onboarding messages
4. `configure_git_identity` — Set git user from env/mount
5. `print_status` — Display ASCII banner with config summary
6. `init_repos` — Clone/update repos from compiled manifest (dev image only)
7. `init_workspace` — Run init.d/ scripts (dev image only)
8. `adapter_launch` — Start the AI tool

## Layer Resolution Order

Layers are compiled in this order (later = higher precedence):

1. `CONFIG_LAYERS` paths — in order specified
2. User config (`USER_CONFIG`) — position depends on `USER_PRIORITY`

With `USER_PRIORITY=high` (default), user config is applied last and wins on conflicts. With `USER_PRIORITY=low`, user config is applied first and can be overridden by project layers.

## Layer Format Detection

Each layer directory is classified by checking for `keel.yml`:

| Condition | Format | Behavior |
|-----------|--------|----------|
| No `keel.yml` | Native | Raw tool files processed directly |
| `keel.yml` with `schema: v1` (or absent) | DSL | Tool-agnostic, compiled to target agent |
| `keel.yml` with `schema: v1-claude` | Native + Policy | Claude-native files with governance; skipped by Cursor compiler |
| `keel.yml` with `schema: v1-cursor` | Native + Policy | Cursor-native files with governance; skipped by Claude compiler |

## What's Next

- **[CLI Reference]({{< relref "cli" >}})** — all CLI commands and flags.
- **[Packaging]({{< relref "../guide/packaging" >}})** — distributable layers and archives.
- **[Agent Support]({{< relref "../ecosystem/agents" >}})** — container tier status per agent.
