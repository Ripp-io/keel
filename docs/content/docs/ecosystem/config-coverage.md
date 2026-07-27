---
title: Config Coverage
weight: 1.5
---

# Config Coverage

What Keel compiles for each supported agent — fully compiled, passthrough, and what's out of scope.

Keel covers the core configuration surface that teams actually configure: instructions, settings, MCP, permissions, hooks, commands, agents, and skills.
This represents ~90% of what matters in practice.

---

## Claude Code

### Fully Compiled

| Config Item | Claude Location | Keel DSL Field |
|---|---|---|
| Instructions | `CLAUDE.md` | `instructions` |
| Settings (project) | `.claude/settings.json` | `settings.claude` + `permissions` |
| MCP (project) | `.mcp.json` | `mcp` |
| Permissions | `permissions` in settings.json | `permissions` |
| Hooks | `hooks` in settings.json | `hooks` |
| Commands | `.claude/commands/` | `commands` |
| Agents | `.claude/agents/` | `agents` |
| Skills | `.claude/skills/` | `skills` |
| Rules | `.claude/rules/` | `rules` |
| Plugins | `.claude/plugins/` | `plugins` |
| Agent memory | `.claude/agent-memory/` | `agent-memory` |
| Init scripts | (runtime) | `init` |

### Supported via DSL and Passthrough

These features have dedicated DSL fields under `settings` in `keel.yml`.
They can also be configured via agent-specific passthrough files (`settings.claude.json`) for keys not covered by the DSL.

| Config Item | Claude Location | How to Configure |
|---|---|---|
| Model config | `model` in settings.json | `settings.model` (DSL) |
| Sandbox config | `sandbox` in settings.json | `settings.sandbox` (DSL) |
| Environment vars | `env` in settings.json | `settings.env` (DSL) |
| Attribution | `attribution` in settings.json | `settings.attribution` (DSL) |
| Auto mode | `autoMode` in settings.json | `settings.auto-mode` (DSL) |
| Skill overrides | `skillOverrides` in settings.json | `settings.skill-overrides` (DSL) |
| Worktree config | `worktree` in settings.json | `settings.worktree` (DSL) |

### Out of Scope

| Config Item | Claude Location | Why |
|---|---|---|
| Settings (local) | `.claude/settings.local.json` | Intentionally personal/gitignored — not shareable config |
| Settings (managed) | System `managed-settings.json` | Admin/MDM-deployed, outside project tree |
| MCP (managed) | System `managed-mcp.json` | Admin-deployed, outside project tree |
| Memory (auto) | `~/.claude/MEMORY.md` | Runtime state, not configuration |
| Projects/Sessions | `.claude/projects/` | Runtime session state |

### MCP Locations

Claude Code reads MCP from multiple locations with merge precedence:

| Location | Compiled by Keel | Notes |
|---|---|---|
| `.mcp.json` (project) | ✅ | Primary compilation target |
| `mcpServers` in settings.json | ✅ | Via passthrough settings file |
| `~/.claude.json` (user) | ✅ (user scope) | Written when compiling with `--scope user` |
| System `managed-mcp.json` | ❌ | Admin-deployed, outside Keel's scope |

---

## Cursor Agent CLI

### Fully Compiled

| Config Item | Cursor Location | Keel DSL Field |
|---|---|---|
| Rules (MDC) | `.cursor/rules/*.mdc` | `instructions` (compiled to MDC format) |
| Settings | `.cursor/cli-config.json` | `settings.cursor` + `settings.model` |
| MCP servers | `.cursor/mcp.json` | `mcp` |
| Hooks | `.cursor/hooks.json` | `hooks` |
| Commands | `.cursor/commands/` | `commands` |
| Agents | `.cursor/agents/` | `agents` |
| Skills | `.cursor/skills/` | `skills` |
| Model selection | `model` in cli-config.json | `settings.model` |
| Permissions (allow) | `permissions.allow` in cli-config.json | `permissions` |
| User config | `~/.cursor/cli-config.json` | User layer via `--scope user` |
| User MCP | `~/.cursor/mcp.json` | User layer via `--scope user` |

### Out of Scope

| Config Item | Cursor Location | Why |
|---|---|---|
| Environment | `.cursor/environment.json` | Background Agents cloud environments only |
| Legacy `.cursorrules` | `.cursorrules` | Deprecated — not read by Cursor Agent mode |

---

## Coverage Summary

| Feature | Claude | Cursor |
|---|---|---|
| Instructions | ✅ `CLAUDE.md` | ✅ `.mdc` rules |
| Settings | ✅ Full | ✅ Full |
| MCP servers | ✅ Full | ✅ Full |
| Permissions | ✅ Full | ✅ Allow |
| Hooks | ✅ Full | ✅ Full |
| Commands | ✅ Full | ✅ Full |
| Agents | ✅ Full | ✅ Full |
| Skills | ✅ Full | ✅ Full |
| Rules | ✅ Full | ✅ Full |
| Plugins | ✅ Claude only | — |
| Agent memory | ✅ Claude only | — |
| Environment | — | ❌ Not yet |
| User scope | ✅ Full | ✅ Full |

## What's Next

- **[Agent Support]({{< relref "agents" >}})** — full compatibility matrix with 30+ agents.
- **[Transpilation]({{< relref "../guide/transpilation" >}})** — what translates cleanly between agents.
- **[DSL Reference]({{< relref "../guide/dsl" >}})** — every field in `keel.yml`.
