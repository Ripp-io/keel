---
title: Transpilation
weight: 7
---

# Transpilation

Any layer format compiles to any agent's output.
A Claude native layer can produce Cursor output, and vice versa.
An AGENTS.md file can produce Claude or Cursor output, and any layer can produce AGENTS.md.
Keel translates through an intermediate representation (IR) that captures configuration in a format-neutral way.

## Quick Examples

### Cross-compile a Claude layer for Cursor

A team already has Claude Code config in their repo.
Compile it as a layer targeting Cursor output:

```bash
keel compile --layer ./team-claude-config --output cursor=.
```

Keel reads `CLAUDE.md`, `.claude/settings.json`, and `.mcp.json`, translates them through the IR, and writes `.cursor/rules/`, `.cursor/mcp.json`, and `.cursor/cli-config.json`.

### Compile to AGENTS.md

Produce an AGENTS.md file from any layer source:

```bash
keel compile --layer ./my-config --output agentsmd=.
```

Since AGENTS.md is instructions-only, settings, hooks, MCP servers, and categories are dropped with warnings.
This is useful for publishing a universal instruction file that works with any AAIF-compatible agent.

### Compile for all three targets at once

```bash
keel compile --output claude=. --output cursor=. --output agentsmd=.
```

### Import native config into Keel DSL

Convert an existing Claude or Cursor config into a Keel DSL layer:

```bash
keel import --from ./my-claude-project --to ./my-keel-layer
```

Keel auto-detects the source format from marker files and produces a self-contained `keel.yml` with companion files.

| Source | Imported As |
|--------|-------------|
| `CLAUDE.md` / `.cursor/rules/*.mdc` | `instructions` entries |
| `.claude/settings.json` / `.cursor/cli-config.json` | `settings` fields + passthrough file |
| `.mcp.json` / `.cursor/mcp.json` | `mcp.servers` entry |
| Hooks | `hooks` entry (normalized to PascalCase) |
| Commands, agents, skills, rules | Corresponding FileRef entries |
| Plugins, agent-memory | Claude-only entries |
| Environment | Cursor-only entry |

## What Translates

### Lossless

These features translate cleanly between agents:

- **Instructions** — markdown content is identical; only the output format differs
- **MCP servers** — the server entry format (`command`, `args`, `env`) is the same
- **Hook events** (portable subset) — PascalCase ↔ camelCase conversion is automatic
- **Permissions** — bare names ↔ `(*)` suffix translation is automatic

### Lossy

These features lose information during cross-compilation:

| Feature | Claude → Cursor | Cursor → Claude | Any → AGENTS.md |
|---------|-----------------|-----------------|-----------------|
| Instructions | Lossless | Lossless | Lossless |
| Agent frontmatter | Heavily lossy (16 fields → 5) | Mostly lossless | Dropped |
| Skill frontmatter | 10 Claude extensions dropped | Lossless (Cursor uses base spec only) | Dropped |
| Command frontmatter | All frontmatter stripped | Lossless (Cursor has no frontmatter) | Dropped |
| Rule frontmatter | `paths` has no exact equivalent | `alwaysApply`/`globs` have no exact equivalent | Dropped |
| Non-portable hooks | `http`, `mcp_tool`, `agent` handlers dropped | Cursor-only events dropped | All dropped |
| Permission `ask` tier | Dropped (Cursor has only allow/deny) | N/A | Dropped |
| Plugins | Entire category dropped | N/A | Dropped |
| Agent memory | Entire category dropped | N/A | Dropped |
| Environment | N/A | Entire category dropped | Dropped |
| Settings | Lossy (key sets differ) | Lossy (key sets differ) | All dropped |
| MCP servers | Lossless | Lossless | Dropped |

## Behavioral Differences

Some features exist in both agents but work differently:

| Feature | Difference |
|---------|-----------|
| Hook file structure | Claude embeds in `settings.json`; Cursor uses separate `hooks.json` |
| Settings merge scope | Claude cascades project → user; Cursor restricts project to permissions only |
| Instruction output | Claude concatenates into `CLAUDE.md`; Cursor wraps in `.mdc` with frontmatter |
| Attribution | Claude uses template strings; Cursor uses booleans |

## Linting for Portability

Use `keel validate --lint` to detect non-portable patterns in instruction files:

```bash
keel validate ./my-layer --lint --agent cursor
```

This scans for patterns like `$ARGUMENTS` (Claude-only) or `{{variable}}` (Cursor-only) and warns about portability issues.
When `--agent` is set, warnings for the target agent's own patterns are suppressed.

## How the Pipeline Works

1. **Read** — Keel reads the source layer using the appropriate reader (Claude, Cursor, AGENTS.md, or Keel DSL parser).
2. **Normalize** — The reader produces an IR containing instructions, settings, hooks, MCP servers, permissions, and overlays.
3. **Transform** — The IR passes through agent-specific transforms (hook key casing, permission format, instruction wrapping). Features unsupported by the target adapter emit warnings.
4. **Write** — The target agent's writer serializes the IR to native format (`CLAUDE.md`, `.cursor/rules/`, `AGENTS.md`, etc.).

See [Architecture: IR]({{< relref "../architecture/ir" >}}) for the intermediate representation design.

## What's Next

- **[Agent Migration]({{< relref "agent-migration" >}})** — step-by-step guide for switching agents using import and compile.
- **[Compatibility Matrix]({{< relref "../reference/compatibility-matrix" >}})** — exhaustive cross-agent reference for every config file, setting, hook event, and frontmatter field.
- **[Layers]({{< relref "layers" >}})** — layer formats and cross-format compilation.
- **[Hooks Reference]({{< relref "../reference/hooks" >}})** — hook event portability matrix.
- **[Frontmatter Reference]({{< relref "../reference/frontmatter" >}})** — field-by-field portability.
