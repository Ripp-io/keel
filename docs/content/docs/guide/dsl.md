---
title: DSL Reference
weight: 2
---

# DSL Reference

Every Keel DSL layer has a `keel.yml` that declares what configuration the layer provides and references companion files containing the content.

## Quick Reference

| Field | Purpose | Merge Strategy |
|-------|---------|----------------|
| [`schema`](#schema) | Layer format identifier (`v1`, `v1-claude`, `v1-cursor`) | — |
| [`name`](#name) | Human-readable layer name | — |
| [`version`](#version) | Arbitrary version string | — |
| [`description`](#description) | Layer description | — |
| [`variables`](#variables) | Key-value pairs for substitution and conditionals | Last-writer-wins per key |
| [`instructions`](#instructions) | Markdown instruction files | Concatenate |
| [`permissions`](#permissions) | Tool permission rules | Configurable (additive or replace) |
| [`mcp`](#mcp) | MCP server definitions and governance | Deep-merge |
| [`hooks`](#hooks) | Event hook definitions | Deep-merge |
| [`commands`](#commands) | Slash command files | Overlay |
| [`agents`](#agents) | Sub-agent definitions | Overlay |
| [`skills`](#skills) | Skill directories | Overlay |
| [`plugins`](#plugins) | Plugin directories (Claude only) | Overlay |
| [`agent-memory`](#agent-memory) | Memory files (Claude only) | Overlay |
| [`environment`](#environment) | Environment config (Cursor only) | Deep-merge |
| [`rules`](#rules) | Rule files | Overlay |
| [`snippets`](#snippets) | Shared text blocks for template includes | Overlay |
| [`loop`](#loop) | Loop prompt files (Claude only) | Concatenate |
| [`worktree-include`](#worktree-include) | Worktree include patterns (Claude only) | Concatenate |
| [`init`](#init) | Initialization scripts | First-found |
| [`repos`](#repos) | Repository definitions for workspace setup | Last-writer-wins per name |
| [`settings`](#settings) | Model, sandbox, env vars, agent-specific settings | Deep-merge |
| [`bin`](#bin) | Portable scripts shipped with the layer | Overlay |
| [`deps`](#deps) | External binary dependencies (install or download) | First-occurrence-wins per name |
| [`files`](#files) | Per-file governance policies | — |
| [`validate`](#validate) | Validation options | — |

## Minimal Example

```yaml
schema: v1
name: my-project

instructions:
  - source: instructions.md
```

## Full Example

```yaml
schema: v1
name: platform-standards
version: "2.0"
description: Shared standards for all platform projects

variables:
  api_url: "https://api.example.com"
  environment: "production"

instructions:
  - source: instructions.md
  - source: conditionals.md

permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive

mcp:
  servers:
    - source: mcp.json
  policy:
    allowed: [sonarqube, gitlab]
    denied: [unsafe-server]
    enableAll: true

hooks:
  - source: hooks.json

commands:
  - source: commands/

agents:
  - source: agents/

skills:
  - source: skills/
  - source: https://github.com/mattpocock/skills.git#main:skills/productivity
    include: [grilling, grill-me]

plugins:
  - source: plugins/

agent-memory:
  - source: agent-memory/

environment:
  - source: environment.json

rules:
  - source: rules/

snippets:
  - source: snippets/

init:
  - source: init.d/

repos:
  party-service:
    url: https://gitlab.example.com/org/party-service.git
    path: party/party-service
    branch: main
    force: true
  lens-common:
    url: https://gitlab.example.com/org/lens-common.git
    path: libs/lens-common

settings:
  model: sonnet
  claude: settings.claude.json
  cursor: settings.cursor.json
  sandbox:
    enabled: true
    auto-allow-bash-if-sandboxed: true
    excluded-commands: ["rm -rf /"]
    filesystem:
      allow-write: ["/tmp"]
      deny-write: ["/etc"]
    network:
      allowed-domains: ["api.example.com"]
      denied-domains: ["evil.com"]
  env:
    PROJECT_ENV: production
    API_KEY: "${API_KEY}"
  worktree:
    base-ref: main
    symlink-directories: [node_modules]
    sparse-paths: ["docs/"]
  skill-overrides:
    legacy-skill: "off"
  auto-mode:
    environment: ["Trusted CI environment"]
    allow: [Read, Grep]
    soft-deny: [Write]
  attribution:
    commit: "Co-Authored-By: Keel"
    pr: ""

files:
  "commands/force-locked-cmd.md":
    force: true
  "skills/force-locked-skill/":
    force: true

validate:
  agentskills: true
  token-budget:
    skill-body: 5000
  agent-tools: 10
```

## Top-Level Fields

### `schema`

The layer format identifier.

| Value | Meaning |
|-------|---------|
| `v1` | Generic Keel DSL — compiles to any agent |
| `v1-claude` | Claude-targeted — skipped when compiling for other agents |
| `v1-cursor` | Cursor-targeted — skipped when compiling for other agents |

Omitting `schema` is equivalent to `v1`.

### `name`

A human-readable name for the layer.
Used in compilation summaries, instruction section headers, and the `keel explain` output.

### `version`

An arbitrary version string for the layer.
Recorded in the compilation manifest but not used for resolution logic.

### `description`

A human-readable description.
Accumulated across layers into the compiled manifest.

### `variables`

Key-value pairs substituted into instruction files and available in conditional expressions.

```yaml
variables:
  api_url: "https://api.example.com"
  environment: "production"
```

Variables merge across layers — later layers override earlier ones for the same key.
Keel auto-injects several variables that are always available without defining them: `keel.agent`, `keel.scope`, `keel.name`, `keel.output`, `keel.version`, `keel.bin.project`, `keel.bin.global`, `keel.layers.count`, `keel.state`, `keel.hooks.dir`, and per-dep `keel.deps.<name>.path` / `keel.deps.<name>.dir`.

Reference variables in instructions with `${keel:name}` syntax (e.g., `${keel:api_url}`).
Reference them in conditional expressions as dotted paths (e.g., `keel.agent == 'claude'`).

See [Conditionals & Variables]({{< relref "conditionals" >}}) for full syntax and examples.

## Instructions

Markdown files concatenated across layers with section headers.

```yaml
instructions:
  - source: instructions.md
  - source: coding-standards.md
```

When multiple layers contribute instructions, each layer's content is separated by a `# --- layer name ---` header in the compiled output.
Single-layer compilations produce no headers.

Instructions support [conditionals]({{< relref "conditionals" >}}) for agent-specific content and [snippets]({{< relref "snippets" >}}) for template includes.

Instructions support [frontmatter]({{< relref "../reference/frontmatter" >}}) for metadata like `force` and `scope`.

**Portability**: Both Claude and Cursor.
Claude outputs to `CLAUDE.md`.
Cursor outputs to `.cursor/rules/*.mdc`.

## FileRef Fields

Most collection fields in `keel.yml` use `FileRef` entries to reference companion files.

| Field | Type | Description |
|-------|------|-------------|
| `source` | string | Path to a file or directory, remote HTTPS URL, or Git repo URL |
| `force` | bool | Force-lock this file (downstream layers cannot override it) |
| `include` | string[] | Cherry-pick specific subdirectories from a directory source |

Files and directories are distinguished automatically at resolution time.

### Remote Sources

The `source` field supports remote URLs — plain HTTPS files, Git repositories, and archives:

```yaml
instructions:
  - source: https://example.com/shared/coding-standards.md

skills:
  - source: https://github.com/org/skills.git#main:skills/productivity
    include: [grilling, research]
```

Git URLs use the fragment syntax `#branch:path` to specify a branch and subdirectory within the repo.

### Include Filter

When a source points at a directory containing multiple entries, `include` filters to specific subdirectories:

```yaml
skills:
  - source: skills/
  - source: https://github.com/mattpocock/skills.git#main:skills/productivity
    include: [grilling, grill-me]
  - source: https://github.com/mattpocock/skills.git#main:skills/engineering
    include: [domain-modeling, to-spec, research]
```

Behavior:
- Applies to the **first directory level** only — `include: [grilling]` means "only the `grilling/` subdirectory."
Files and nested dirs within it are included normally.
- Empty or absent `include` means all entries (backward compatible).
- Ignored for file sources (non-directory FileRefs).
- Works for all category types (skills, agents, commands, rules, etc.).
- Include entries must be plain names — no path separators or `..`.

## Permissions

Permission rule files controlling which tools the agent can use.

```yaml
permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive
```

The companion JSON file:

```json
{
  "allow": ["Read", "Grep", "Glob", "Bash"],
  "deny": ["Write(/etc/*)"]
}
```

### Merge Policy

The `policy.merge` field controls how permissions merge across layers:

| Value | Behavior |
|-------|----------|
| `additive` | Union of all allow/deny arrays — later layers can only add, not remove |
| `replace` | Last-writer-wins — later layers replace the entire permissions set |

Default is `replace`.

**Portability**: Both Claude and Cursor.
Keel translates permission names between formats (e.g., bare names for Claude, parameterized `(*)` suffix for Cursor).

## MCP

MCP (Model Context Protocol) server definitions and governance policy.

```yaml
mcp:
  servers:
    - source: mcp.json
  policy:
    allowed: [sonarqube, gitlab]
    denied: [unsafe-server]
    enableAll: true
```

The companion JSON file uses the standard MCP server format:

```json
{
  "mcpServers": {
    "sonarqube": {
      "command": "sonar-mcp",
      "args": ["--token", "${SONAR_TOKEN}"]
    },
    "gitlab": {
      "command": "gitlab-mcp",
      "args": ["--token", "${GITLAB_TOKEN}"]
    }
  }
}
```

### MCP Governance Policy

| Field | Type | Description |
|-------|------|-------------|
| `allowed` | string[] | Allowlist of server names |
| `denied` | string[] | Denylist of server names |
| `enableAll` | bool | Enable all project MCP servers without prompt |

### Per-Server Governance

Individual MCP servers can be force-locked or targeted to specific agents using `_keel` metadata:

```json
{
  "mcpServers": {
    "sonarqube": {
      "_keel": { "force": true },
      "command": "sonar-mcp",
      "args": ["--token", "${SONAR_TOKEN}"]
    },
    "claude-only-server": {
      "_keel": { "condition": "keel.agent == 'claude'" },
      "command": "claude-specific-mcp"
    },
    "not-cursor-server": {
      "_keel": { "condition": "keel.agent != 'cursor'" },
      "command": "any-but-cursor-mcp"
    }
  }
}
```

See [Governance]({{< relref "governance" >}}) for details on force-locks and [Conditionals & Variables]({{< relref "conditionals" >}}) for agent targeting.

**Portability**: Both Claude and Cursor.

## Hooks

Event hook definitions for lifecycle automation.

```yaml
hooks:
  - source: hooks.json
```

The companion JSON file:

```json
{
  "PreToolUse": [
    {"type": "command", "command": "echo pre-tool", "force": true}
  ],
  "PostToolUse": [
    {"type": "command", "command": "echo post-tool"}
  ],
  "SessionStart": [
    {"type": "command", "command": "echo session-start"}
  ],
  "Stop": [
    {"type": "prompt", "prompt": "Summarize what was accomplished"}
  ]
}
```

Hook keys use PascalCase in Keel DSL.
Keel automatically converts between PascalCase (Claude) and camelCase (Cursor) when compiling.

Individual hook entries can be force-locked with `"force": true`.

See [Hooks Reference]({{< relref "../reference/hooks" >}}) for every hook event and the portability matrix.

**Portability**: Both Claude and Cursor, with automatic key transformation.

## Commands

Slash command files overlaid into the agent's command directory.

```yaml
commands:
  - source: commands/
```

Each file in the `commands/` directory becomes a slash command.
The filename (without extension) becomes the command name.

Commands use the overlay merge strategy — if two layers provide a command with the same filename, the later layer wins.

**Portability**: Both Claude and Cursor.

## Agents

Sub-agent definition files overlaid into the agent's agents directory.

```yaml
agents:
  - source: agents/
```

Uses the overlay merge strategy.

**Portability**: Both Claude and Cursor.

## Skills

Skill directories overlaid into the agent's skills directory.

```yaml
skills:
  - source: skills/
```

Each subdirectory under `skills/` is a complete skill with its `SKILL.md` and companion files.
The entire skill directory is preserved (scripts, references, assets).

Multiple sources can be combined — local skills alongside cherry-picked skills from community collections:

```yaml
skills:
  - source: skills/
  - source: https://github.com/mattpocock/skills.git#main:skills/productivity
    include: [grilling, grill-me]
  - source: https://github.com/mattpocock/skills.git#main:skills/engineering
    include: [domain-modeling, to-spec, wayfinder, research]
```

This pulls only the named skills from the remote repo without downloading everything else.
See [Include Filter](#include-filter) for details.

Uses the overlay merge strategy at the skill directory level.

**Portability**: Both Claude and Cursor.

## Plugins

Plugin directories overlaid into the agent's plugin directory.

```yaml
plugins:
  - source: plugins/
```

**Portability**: Claude only.
Cursor does not support plugins.
Keel emits a warning if a layer uses plugins when compiling for Cursor.

## Agent Memory

Memory files overlaid into the agent's memory directory.

```yaml
agent-memory:
  - source: agent-memory/
```

**Portability**: Claude only.
Keel emits a warning if a layer uses agent-memory when compiling for Cursor.

## Environment

Environment configuration files.

```yaml
environment:
  - source: environment.json
```

**Portability**: Cursor only.
Keel emits a warning if a layer uses environment when compiling for Claude.

## Rules

Rule files overlaid into the agent's rules directory.

```yaml
rules:
  - source: rules/
```

Uses the overlay merge strategy.

**Portability**: Both Claude and Cursor.

## Snippets

Shared text blocks that can be included in instruction files via template markers.

```yaml
snippets:
  - source: snippets/
```

Each file in the `snippets/` directory is a named snippet.
Reference a snippet in any instruction file with:

```markdown
<!-- keel:include reasoning-protocol -->
```

See [Snippets]({{< relref "snippets" >}}) for details.

## Loop

Loop prompt files concatenated across layers.

```yaml
loop:
  - source: loop.md
```

The loop prompt is shown to the agent when entering loop mode for iterative work.
Uses the same FileRef syntax as instructions, with concatenation merge strategy.

**Portability**: Claude only.

## Worktree Include

Worktree include pattern files concatenated across layers.

```yaml
worktree-include:
  - source: worktree-include.txt
```

Each line in a worktree include file is a glob pattern for files to include when creating worktrees.
Uses the same FileRef syntax as instructions, with concatenation merge strategy.

**Portability**: Claude only.

## Init

Initialization scripts run when setting up a workspace.

```yaml
init:
  - source: init.d/
```

The first `init.d/` found across all layers is recorded in the compiled manifest.

## Repos

Repository definitions for workspace initialization.

```yaml
repos:
  party-service:
    url: https://gitlab.example.com/org/party-service.git
    path: party/party-service
    branch: main
    required: true
    force: true
  lens-common:
    url: https://gitlab.example.com/org/lens-common.git
    path: libs/lens-common
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `url` | string | required | Git repository URL |
| `path` | string | required | Local clone path relative to workspace root |
| `branch` | string | — | Branch to clone |
| `required` | bool | — | Whether the repo is required for workspace init |
| `force` | bool | false | Force-lock this repo definition |

Repos merge by name across layers — later layers replace the entire repo definition for the same name.

## Bin

Portable scripts shipped with the layer, overlaid to `.keel/bin/` (shared across agents).

```yaml
bin:
  - source: bin/
```

Each file in the `bin/` directory is copied to `.keel/bin/` with executable permissions preserved.
The output directory is shared across agents — a multi-agent compile (`--output claude=. --output cursor=.`) writes bin files once, not per-agent.

Reference bin scripts from hooks, MCP configs, or instructions using the `${keel:keel.bin.project}` variable.

**Portability**: Agent-independent — `.keel/bin/` is not inside any agent's config directory.

## Deps

External binary dependencies that the layer requires.
Each dependency is installed via a shell command or downloaded from a URL.

```yaml
deps:
  sonar-scanner:
    download: "https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/sonar-scanner-cli-${os}-${arch}.zip"
    executable: sonar-scanner-cli/bin/sonar-scanner
    sha256: "abc123..."

  custom-linter:
    install: "npm install -g @acme/custom-linter"
    verify: "custom-linter --version"

  shared-tool:
    download: "https://releases.example.com/tool-v2.tar.gz"
    executable: tool
    global: true
```

| Field | Type | Description |
|-------|------|-------------|
| `install` | string | Shell command to install the dependency |
| `download` | string | URL to download — supports `${os}` and `${arch}` templates |
| `sha256` | string | SHA-256 hash to verify downloads (recommended, warned by `keel validate` if absent) |
| `executable` | string | Path to the binary within a downloaded archive |
| `verify` | string | Command to verify successful installation |
| `global` | bool | Install to `~/.keel/bin/` instead of `.keel/bin/` (shared across projects) |

Use `install` for package manager installs or `download` for direct binary downloads — not both.

**Security**: Deps are parsed but not executed unless `--allow-install` is passed to `keel compile`.
Without the flag, deps are recorded in the manifest as `skipped`.

**Auto-injected variables**: Each installed dep creates `keel.deps.<name>.path` and `keel.deps.<name>.dir` variables for use in hooks, MCP configs, and instructions.

Deps merge by name across layers — the first layer to define a dependency name wins; later layers cannot override it.

## Settings

Inline settings and references to agent-specific settings files.

### Common Settings

```yaml
settings:
  model: sonnet
```

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | AI model to use (e.g., `sonnet`, `opus`, `haiku`) |
| `claude` | string | Path to a `settings.claude.json` file |
| `cursor` | string | Path to a `settings.cursor.json` file |
| `opencode` | string | Path to a `settings.opencode.json` file |

The `claude` and `cursor` fields reference agent-specific JSON settings files.
These are deep-merged into the agent's native settings output.

See [Settings Reference]({{< relref "../reference/settings" >}}) for every settings key.

### Sandbox

```yaml
settings:
  sandbox:
    enabled: true
    auto-allow-bash-if-sandboxed: true
    excluded-commands: ["rm -rf /"]
    filesystem:
      allow-write: ["/tmp"]
      deny-write: ["/etc"]
      allow-read: ["/usr/local"]
      deny-read: ["/root"]
    network:
      allowed-domains: ["api.example.com"]
      denied-domains: ["evil.com"]
```

**Portability**: Claude only (Cursor does not have sandbox support).

### Environment Variables

```yaml
settings:
  env:
    PROJECT_ENV: production
    API_KEY: "${API_KEY}"
```

Values can reference environment variables with `${VAR}` syntax.
These are passed through as-is — they are resolved at runtime by the agent, not by Keel.

**Portability**: Both Claude and Cursor.

### Worktree

```yaml
settings:
  worktree:
    base-ref: main
    symlink-directories: [node_modules]
    sparse-paths: ["docs/"]
```

**Portability**: Claude only.

### Skill Overrides

```yaml
settings:
  skill-overrides:
    legacy-skill: "off"
    useful-skill: "on"
    info-only: "name-only"
```

**Portability**: Claude only.

### Auto Mode

```yaml
settings:
  auto-mode:
    environment: ["Trusted CI environment"]
    allow: [Read, Grep]
    soft-deny: [Write]
```

**Portability**: Claude only.

### Attribution

```yaml
settings:
  attribution:
    commit: "Co-Authored-By: Keel"
    pr: ""
```

Set to an empty string to disable.

**Portability**: Claude only.

## Files

Per-file governance policies.

```yaml
files:
  "commands/force-locked-cmd.md":
    force: true
  "skills/important-skill/":
    force: true
  "permissions.json":
    merge: additive
```

| Field | Type | Description |
|-------|------|-------------|
| `force` | bool | Force-lock this file — downstream layers cannot override it |
| `merge` | string | Merge strategy for this file (`deep-merge`, `replace`, `additive`) |

## Validate

Validation options that control how `keel validate` checks this layer.

```yaml
validate:
  agentskills: true
  token-budget:
    skill-body: 5000
    instructions: 50000
    layer: 100000
  agent-tools: 10
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `agentskills` | bool | `true` | Run Agent Skills format validation on skill directories |
| `token-budget.skill-body` | int | `5000` | Per-skill body token threshold (0 = disabled) |
| `token-budget.instructions` | int | `0` | Total instruction token threshold (0 = disabled) |
| `token-budget.layer` | int | `0` | Total layer content token threshold (0 = disabled) |
| `agent-tools` | int | `10` | Maximum tools per agent definition (0 = disabled) |

All thresholds: absent = use default, `0` = disabled, positive value = custom threshold.
See [Validation]({{< relref "validation" >}}) for details on what each check does and why.

## What's Next

- **[Enterprise Example]({{< relref "enterprise-example" >}})** — see every DSL field used in a real 5-layer setup.
- **[Governance]({{< relref "governance" >}})** — force-locks and merge policies.
- **[Conditionals & Variables]({{< relref "conditionals" >}})** — agent-specific content and variable substitution.
- **[Snippets]({{< relref "snippets" >}})** — template includes in instructions.
- **[Settings Reference]({{< relref "../reference/settings" >}})** — every settings key.
- **[Hooks Reference]({{< relref "../reference/hooks" >}})** — every hook event.
