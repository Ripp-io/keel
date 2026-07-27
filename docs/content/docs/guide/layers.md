---
title: Layers
weight: 1
---

# Layers

A layer is a directory containing configuration for an AI coding agent.
Stack layers from company → team → project → user, and Keel compiles them into a single native config with clear precedence.

## Quick Examples

### Project config file

Create `.keel/config.yml` in your project root:

```yaml
layers:
  - ./layers/*

output:
  claude: .
  cursor: .
```

```bash
keel compile
```

Keel auto-discovers the config file and compiles all matched layers for every listed agent.

### Stack multiple layers

```yaml
layers:
  - ./layers/company
  - ./layers/team
  - ./layers/project

output:
  claude: .
```

Later layers override earlier ones.
Earlier layers have higher trust — they can [force-lock]({{< relref "governance" >}}) values that later layers cannot change.

### CLI flags for one-off usage

CLI flags work without a config file — useful for quick experiments and CI:

```bash
keel compile --layer ./my-layer --output claude=.
```

## Layer Formats

Keel supports three formats, auto-detected from marker files.

### Keel DSL (`v1`)

The canonical format.
A `keel.yml` file references companion files:

```
my-layer/
├── keel.yml
├── instructions.md
├── permissions.json
├── hooks.json
├── mcp.json
├── commands/
│   └── build.md
└── skills/
    └── deploy/
        └── SKILL.md
```

```yaml
schema: v1
name: platform-standards

instructions:
  - source: instructions.md

permissions:
  rules:
    - source: permissions.json

hooks:
  - source: hooks.json
```

See the [DSL Reference]({{< relref "dsl" >}}) for every field.

### Claude Native

A directory containing Claude Code's native config files.
Detected by `.claude/` or `CLAUDE.md`.

```
my-claude-layer/
├── .claude/
│   ├── settings.json
│   ├── commands/
│   └── agents/
├── CLAUDE.md
└── .mcp.json
```

Useful when you already have Claude Code config and want to use it as a Keel layer without rewriting it.
This is how community repos like [everything-claude-code](https://github.com/affaan-m/everything-claude-code) work as layers — they already have `.claude/` at the root.

### Cursor Native

A directory containing Cursor's native config files.
Detected by `.cursor/`.

```
my-cursor-layer/
├── .cursor/
│   ├── rules/
│   ├── hooks.json
│   └── mcp.json
└── cli-config.json
```

### Cross-Format Compilation

Any layer format compiles to any agent's output.
A Claude native layer can produce Cursor output, and vice versa.
Keel translates through an intermediate representation, emitting warnings for features that don't translate cleanly.

See [Transpilation]({{< relref "transpilation" >}}) for the portability matrix.

## Layer Resolution

### Order matters

Layers compile in the order specified.
Earlier layers have higher trust:

| Position | Layer | Trust | Purpose |
|----------|-------|-------|---------|
| 1st | Company | Highest | Security policies, required tools |
| 2nd | Organization | High | Org-wide standards |
| 3rd | Team | Medium | Team conventions |
| 4th | Project | Low | Project-specific config |
| 5th | User | Lowest | Personal preferences |

### Local paths

Paths can be absolute or relative.
Glob patterns are expanded:

```bash
keel compile --layer '/keel/layers/*' --output claude=.
```

Layers from a glob are sorted alphabetically.
Use numeric prefixes to control ordering:

```
layers/
├── 01-company/
├── 02-org/
├── 03-team/
├── 04-project/
└── 05-user/
```

### Remote layers

Layers can be fetched from Git repositories or HTTP archives:

```bash
# Git (HTTPS or SSH) — quote to prevent shell interpretation of #
--layer 'https://git.example.com/org/config.git#main'

# Subdirectory within a repo
--layer 'https://git.example.com/org/monorepo.git#main:layers/company'

# HTTP archive
--layer 'https://artifacts.example.com/keel-layers/company-v2.tar.gz'
```

Remote layers are cached under `~/.cache/keel/remote/`.
Use `keel cache list` to inspect and `keel cache clean` to clear.
Use `--offline` to compile from cache without network access.

See [Personal Toolkit]({{< relref "personal-toolkit" >}}) for a practical guide to using remote layers.

### Remote file references

Individual files in `keel.yml` can also reference remote URLs:

```yaml
instructions:
  - source: https://example.com/shared/coding-standards.md
```

### Auto-discovery

When no `--layer` flag is provided, Keel checks:
1. The `CONFIG_LAYERS` environment variable (colon-separated paths)
2. Falls back to `/keel/layers/*`

Keel also auto-discovers:
- `.keel/config.yml` — loaded as a config file for compile parameters
- `.keel/local/` — appended as the final layer if the directory exists (see below)
- Output targets — inferred from `.claude/` or `.cursor/` in the current directory

#### `keel/` — shared project config

The standard location for agent-agnostic, committed configuration is a `keel/` directory at the project root.
It is a regular Keel DSL layer containing skills, instructions, and settings that every team member shares.

```
my-project/
├── keel/                     ← committed, shared (agent-agnostic)
│   ├── keel.yml
│   ├── instructions.md
│   └── skills/
│       └── deploy/
│           └── SKILL.md
├── .keel/
│   ├── config.yml            ← committed, compile parameters
│   └── local/                ← gitignored, personal overrides
└── .claude/                  ← gitignored, compiled output
```

Reference it in your config or CLI:

```yaml
# .keel/config.yml
layers:
  - keel/
output:
  claude: .
```

Because it uses `schema: v1` (the Keel DSL format), the same layer compiles to Claude, Cursor, or any supported agent.

**Remote auto-detection:** When a remote source has no explicit subdirectory, Keel checks if the repo root is a recognized layer.
If not, but a `keel/` directory exists and is a layer, Keel uses it automatically:

```bash
# These are equivalent when the repo has keel/ at root:
--layer 'https://git.example.com/org/my-project.git#main'
--layer 'https://git.example.com/org/my-project.git#main:keel'
```

This means any project that follows the `keel/` convention can be used as a remote layer without specifying the subdirectory.
The fallback does not apply when the root is already a layer, when you specify an explicit subpath, or when `keel/` is not a recognized layer format.

#### `.keel/local/` — personal overrides

If `.keel/local/` exists, Keel appends it as the last layer — giving it the highest override priority.
Add `.keel/local/` to `.gitignore` so each developer can carry personal overrides (model preferences, extra instructions, debug hooks) without affecting the shared config.

### Agent-targeted layers

A layer with `schema: v1-claude` is skipped when compiling for Cursor, and vice versa.
A layer with `schema: v1` compiles for all agents.

This lets you have agent-specific layers in the same stack without contaminating other agents' output.

## Merge Behavior

When multiple layers define the same feature, Keel merges them according to the feature's merge strategy:

| Feature | Strategy | Behavior |
|---------|----------|----------|
| Instructions | Concatenate | Appended with `# --- layer name ---` headers |
| Settings | Deep-merge | Recursive map merge, later values override |
| Permissions | Configurable | Additive (union) or replace (last-writer-wins) |
| MCP servers | Deep-merge | Server definitions merged by name |
| Hooks | Deep-merge | Hook events merged, arrays concatenated |
| Commands | Overlay | File-level copy, last layer wins per file |
| Agents | Overlay | File-level copy, last layer wins per file |
| Skills | Overlay | Directory-level copy, last layer wins per skill |
| Repos | Deep-merge | Repository definitions merged by name |
| Bin | Overlay | Scripts copied to `.keel/bin/`, last layer wins per file |
| Deps | Last-writer-wins | Later layers override dependencies by name |

See [Merge Strategies]({{< relref "../architecture/merge-strategies" >}}) for implementation details.
See [Governance]({{< relref "governance" >}}) for how force-locks override merge behavior.

## What's Next

- **[DSL Reference]({{< relref "dsl" >}})** — every field in `keel.yml`.
- **[Governance]({{< relref "governance" >}})** — force-lock values across the layer stack.
- **[Enterprise Example]({{< relref "enterprise-example" >}})** — 5-layer enterprise setup showing all features in context.
- **[Personal Toolkit]({{< relref "personal-toolkit" >}})** — assemble your global config from community sources.
