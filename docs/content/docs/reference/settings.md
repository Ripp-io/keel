---
title: Settings Reference
weight: 3
---

# Settings Reference

Settings control agent behavior — model selection, permissions, sandbox configuration, environment variables, and more.
Keel defines common settings in the DSL and supports agent-specific passthrough files for everything else.

## DSL Settings

These settings are defined inline in `keel.yml` under the `settings` key:

```yaml
settings:
  model: sonnet
  env:
    PROJECT_ENV: production
```

## Passthrough Settings Files

For agent-specific settings not covered by the DSL, reference a JSON file:

```yaml
settings:
  claude: settings.claude.json
  cursor: settings.cursor.json
```

These files are deep-merged into the agent's native settings output.
The passthrough file is only used when compiling for its target agent — `settings.claude.json` is ignored when compiling for Cursor.

## Portable Settings

These settings work across both Claude and Cursor:

| DSL Key | Claude Output | Cursor Output | Notes |
|---------|---------------|---------------|-------|
| `model` | `settings.json` → `model` | `cli-config.json` → `model` | Model name translated |
| `permissions.allow` | `settings.json` → `permissions.allow` | `cli-config.json` → `permissions.allow` | Bare names → `(*)` suffix |
| `permissions.deny` | `settings.json` → `permissions.deny` | `cli-config.json` → `permissions.deny` | Bare names → `(*)` suffix |
| `env` | `settings.json` → `env` | — | Claude settings key; Cursor uses `environment.json` |
| `hooks` | `settings.json` → `hooks` | `hooks.json` (separate file) | PascalCase → camelCase |

## Claude-Only Settings

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

| Key | Description |
|-----|-------------|
| `sandbox.enabled` | Enable sandbox isolation |
| `sandbox.auto-allow-bash-if-sandboxed` | Auto-approve Bash when sandboxed |
| `sandbox.excluded-commands` | Commands excluded from sandbox |
| `sandbox.filesystem.allow-write` | Writable paths |
| `sandbox.filesystem.deny-write` | Non-writable paths |
| `sandbox.filesystem.allow-read` | Readable paths |
| `sandbox.filesystem.deny-read` | Non-readable paths |
| `sandbox.network.allowed-domains` | Allowed network domains |
| `sandbox.network.denied-domains` | Denied network domains |

### Worktree

```yaml
settings:
  worktree:
    base-ref: main
    symlink-directories: [node_modules]
    sparse-paths: ["docs/"]
```

| Key | Description |
|-----|-------------|
| `worktree.base-ref` | Default base ref for worktrees |
| `worktree.symlink-directories` | Directories to symlink into worktrees |
| `worktree.sparse-paths` | Paths for sparse checkout |

### Skill Overrides

```yaml
settings:
  skill-overrides:
    legacy-skill: "off"
    useful-skill: "on"
    info-only: "name-only"
```

Values: `"on"`, `"off"`, `"name-only"`.

### Auto Mode

```yaml
settings:
  auto-mode:
    environment: ["Trusted CI environment"]
    allow: [Read, Grep]
    soft-deny: [Write]
```

| Key | Description |
|-----|-------------|
| `auto-mode.environment` | Trusted domain descriptions |
| `auto-mode.allow` | Operations allowed in auto mode |
| `auto-mode.soft-deny` | Operations soft-denied in auto mode |

### Attribution

```yaml
settings:
  attribution:
    commit: "Co-Authored-By: Keel"
    pr: ""
```

Set to an empty string to disable.

## All Claude Settings Keys

For settings not covered by the Keel DSL, use the `settings.claude` passthrough file.
These keys go directly into `.claude/settings.json`:

| Category | Key | Scope | Notes |
|----------|-----|-------|-------|
| Model | `model` | project, user | Model selection |
| Model | `agent` | project, user | Sub-agent type |
| Model | `availableModels` | user | Available model list |
| Model | `modelOverrides` | user | Model overrides |
| Model | `effortLevel` | project, user | Effort level |
| Model | `alwaysThinkingEnabled` | project, user | Always-on thinking |
| Permissions | `permissions.allow` | project, user | Allowed tools |
| Permissions | `permissions.ask` | project, user | Ask-before-use tools |
| Permissions | `permissions.deny` | project, user | Denied tools |
| Permissions | `permissions.additionalDirectories` | project, user | Extra directories |
| Permissions | `permissions.defaultMode` | user | Default permission mode |
| Environment | `env` | project, user | Environment variables |
| Environment | `apiKeyHelper` | user | API key helper command |
| Git | `attribution.commit` | project, user | Commit trailer |
| Git | `attribution.pr` | project, user | PR attribution |
| Git | `includeGitInstructions` | project, user | Git instructions |
| Worktree | `worktree.baseRef` | project, user | Base ref |
| Worktree | `worktree.symlinkDirectories` | project, user | Symlink dirs |
| Worktree | `worktree.sparsePaths` | project, user | Sparse paths |
| Auto Mode | `autoMode.environment` | user | Trusted environments |
| Auto Mode | `autoMode.allow` | user | Allowed operations |
| Auto Mode | `autoMode.soft_deny` | user | Soft-denied operations |
| Skills | `skillOverrides` | project, user | Skill on/off/name-only |
| UI | `editorMode` | user | Editor mode |
| UI | `viewMode` | user | View mode |
| UI | `outputStyle` | user | Output style |
| UI | `language` | user | Interface language |
| Features | `autoMemoryEnabled` | project, user | Auto-memory |
| Features | `fastMode` | user | Fast mode |
| MCP | `enableAllProjectMcpServers` | project, user | Enable all project MCP |

## All Cursor Settings Keys

For settings not covered by the Keel DSL, use the `settings.cursor` passthrough file.
These keys go into `.cursor/cli-config.json`:

| Key | Scope | Notes |
|-----|-------|-------|
| `version` | global | Schema version |
| `model` | global | Model selection |
| `permissions.allow` | project, global | Allowed tools |
| `permissions.deny` | project, global | Denied tools |
| `attribution.attributeCommitsToAgent` | global | Boolean |
| `attribution.attributePRsToAgent` | global | Boolean |
| `editor.vimMode` | global | Vim keybindings |
| `network.useHttp1ForAgent` | global | HTTP/1.1 preference |

> [!NOTE]
> Cursor's `cli-config.json` at project scope is restricted to the `permissions` key only.
> All other settings are global-scope only.

## Merge Behavior

Settings merge across layers using deep-merge:
- Scalar values: later layers override earlier layers.
- Arrays: concatenated and deduplicated.
- Objects: recursively merged.
- Force-locked values: cannot be overridden by later layers.

## What's Next

- **[DSL Reference]({{< relref "../guide/dsl" >}})** — the `settings` block in `keel.yml`.
- **[Hooks Reference]({{< relref "hooks" >}})** — hook configuration in settings.
- **[Governance]({{< relref "../guide/governance" >}})** — force-locking settings values.
