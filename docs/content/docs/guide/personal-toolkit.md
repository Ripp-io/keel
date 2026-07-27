---
title: "Personal Toolkit"
weight: 13
---

# Personal Toolkit

Assemble your global AI agent config from your own tools, internal resources, and community sources.
Compile them together with `--scope user` and Keel installs everything as your global config.

## Quick Example

You have your own commands and snippets.
A colleague maintains an optional internal tools layer.
You also want to pull in a community config repo.

```bash
keel compile \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --layer 'https://git.internal.com/platform/keel-extras.git#main' \
  --layer ./my-tools \
  --output claude=~ --scope user
```

Keel fetches the remote layers, merges everything together, and writes the result to your user-level config paths (`~/.claude/`).
The community repo doesn't need to know Keel exists — its `.claude/` and `CLAUDE.md` are auto-detected as a native layer.

## Building Your Layers

### Your own tools

Create a layer with your personal commands, agents, and preferences:

```
my-tools/
├── keel.yml
├── instructions.md
├── commands/
│   ├── daily-standup.md
│   └── pr-review.md
├── agents/
│   └── refactor-buddy.md
└── snippets/
    └── commit-checklist.md
```

```yaml
# my-tools/keel.yml
schema: v1
name: my-toolkit
description: Personal commands, agents, and preferences

instructions:
  - source: instructions.md

commands:
  - source: commands/

agents:
  - source: agents/

snippets:
  - source: snippets/

settings:
  model: opus
```

### Internal team tools

Someone on your team maintains a shared tools layer in a Git repo.
Reference it as a remote layer — Keel clones and caches it:

```bash
--layer 'https://git.internal.com/platform/keel-extras.git#main'
```

No need to clone it yourself.
Keel caches remote layers under `~/.cache/keel/remote/`.
Use `keel cache list` to see what's cached and `keel cache clean` to refresh.

### Community sources

Pull from public repositories the same way:

```bash
--layer 'https://github.com/affaan-m/everything-claude-code.git#main'
```

The URL format is `<repo-url>#<branch>`.
Keel clones the repo and auto-detects the layer format from marker files (`.claude/`, `CLAUDE.md`, `.cursor/`, or `keel.yml`).
Community repos don't need any Keel-specific structure — their existing native config just works.

If the layer lives in a subdirectory, append `:<path>`:

```bash
--layer 'https://github.com/org/monorepo.git#main:tools/claude-config'
```

HTTP archives work too:

```bash
--layer 'https://releases.example.com/tools/v2.tar.gz'
```

## Layer Order

Later layers override earlier ones.
Put your personal tools last so your preferences win:

```bash
keel compile \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --layer 'https://git.internal.com/platform/keel-extras.git#main' \
  --layer ./my-tools \
  --output claude=~ --scope user
```

Community defaults load first, internal tools add to them, and your personal config gets the final say.

No governance or force-locks are involved — this is personal curation, not enterprise policy.

## Offline Compilation

Remote layers are cached after the first fetch.
Compile from cache without network access:

```bash
keel compile \
  --layer 'https://github.com/community/shared-config.git#main' \
  --layer ./my-tools \
  --output claude=~ --scope user --offline
```

## User Scope Output Paths

`--scope user` writes to user-level paths instead of the project directory.

### Claude

| Content | Project Scope | User Scope |
|---------|---------------|------------|
| Instructions | `CLAUDE.md` | `~/.claude/CLAUDE.md` |
| Settings | `.claude/settings.json` | `~/.claude/settings.json` |
| Commands | `.claude/commands/` | `~/.claude/commands/` |
| Agents | `.claude/agents/` | `~/.claude/agents/` |
| Skills | `.claude/skills/` | `~/.claude/skills/` |
| Rules | `.claude/rules/` | `~/.claude/rules/` |
| Plugins | `.claude/plugins/` | `~/.claude/plugins/` |
| Agent memory | `.claude/agent-memory/` | `~/.claude/agent-memory/` |

### Cursor

| Content | Project Scope | User Scope |
|---------|---------------|------------|
| Settings | `.cursor/cli-config.json` | `~/.cursor/cli-config.json` |
| Commands | `.cursor/commands/` | `~/.cursor/commands/` |
| Skills | `.cursor/skills/` | `~/.cursor/skills/` |
| MCP | `.cursor/mcp.json` | `~/.cursor/mcp.json` |
| Hooks | `.cursor/hooks.json` | `~/.cursor/hooks.json` |

Cursor does not support user-level instruction files (no filesystem path for `.mdc` rules).

> **Note**: Claude's user-level MCP config lives inside `~/.claude.json` alongside OAuth tokens and session state.
> Keel writes MCP server definitions to the output directory but cannot safely merge into `~/.claude.json`.
> Integrating them requires a manual step or the Claude CLI.

## Combining with Project Scope

Use user scope for personal preferences and project scope for team/project config:

```bash
# Global: personal toolkit
keel compile \
  --layer ./my-tools \
  --output claude=~ --scope user

# Per-project: company + team + project layers
keel compile \
  --layer /layers/company \
  --layer /layers/team \
  --layer /layers/project \
  --output claude=./my-project
```

User-scope config applies across all projects.
Project-scope config applies only to that project.
The agent merges both at runtime — project config overrides user config where they overlap.

## What's Next

- **[Enterprise Example]({{< relref "enterprise-example" >}})** — the governance story: top-down policy enforcement.
- **[Layers]({{< relref "layers" >}})** — remote layers, caching, and resolution order.
- **[CLI Reference]({{< relref "../reference/cli" >}})** — `--scope`, `--offline`, and `keel cache` commands.
