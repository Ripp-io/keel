---
title: Config File Reference
weight: 6
---

# Config File Reference

The `.keel/config.yml` file eliminates repetitive CLI flags by declaring compile parameters once in version-controlled config.
Every field maps to a CLI flag — the config file provides defaults, CLI flags override them.

## Location

Keel loads a single config file per invocation:

- **`--config <path>`** overrides auto-discovery and loads the specified file.
- **`--scope user`** checks `~/.keel/config.yml` first, falls back to `.keel/config.yml`.
- **Default (project scope)** checks `.keel/config.yml` only.

Only one config file is loaded — project and user configs are not merged.
CLI flags always take precedence over config file values.

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `layers` | list | `[]` | Layer paths or URLs. CLI `--layer` flags override this entirely. |
| `output` | map | auto-detected | Agent output targets as `agent: path` pairs (e.g. `claude: .`) |
| `distributable` | bool | `false` | Produce a distributable layer (strips project-specific paths) |
| `name` | string | derived from layer | Project name (overrides layer-derived name in manifest) |
| `scope` | string | `project` | Compilation scope: `project` or `user` |
| `offline` | bool | `false` | Use cached remote layers without network access |
| `locked` | bool | `false` | Verify layer content matches lockfile before compiling |
| `allow-install` | bool | `false` | Execute dep install/download commands during compilation |
| `variables` | map | `{}` | Template variables as `key: value` pairs |
| `user-config` | string | `/mnt/user-config` | User config directory path (container convention; typically overridden for desktop use) |
| `user-priority` | string | `high` | User config priority: `high` or `low` |
| `max-remote-layers` | int | `50` | Maximum remote layer count (`-1` for unlimited) |
| `sign` | bool | `false` | Sign the compiled manifest with Ed25519 key |
| `sign-key` | string | `default` | Key name for signing (refers to `~/.keel/keys/<name>.key`) |
| `verify-layers` | bool | `false` | Require and verify layer signatures before compiling |
| `include-local` | bool | `false` | Include `.keel/local` layer even at user scope (normally skipped) |
| `package` | string | `""` | Archive format for compiled output: `tar.gz` or `zip` |

## Layers Format

Layers can be specified as simple strings or extended objects:

```yaml
layers:
  - ./local-layer                          # Simple path
  - 'https://github.com/org/repo.git#main' # Remote Git URL
  - source: ./another-layer                # Extended format
    filter:
      skills:
        include: [skill-name]              # Cherry-pick specific skills
```

The extended format supports filtering to include only specific content from a layer.
See [Layer Filters]({{< relref "../guide/layers#filters" >}}) for details.

## Output Format

The `output` field maps agent names to output directories:

```yaml
output:
  claude: .
  cursor: ../cursor-config
```

If omitted, Keel auto-detects agents by scanning for known agent directories (`.claude/`, `.cursor/`, `.cline/`, `.aider/`, `.github/`, `.agents/`, `.opencode/`).

## CLI to Config Mapping

Most CLI flags have corresponding config fields.
Execution-mode flags (`--dry-run`, `--force`, `--strict`, `--config`, etc.) are CLI-only:

| CLI Flag | Config Field | Notes |
|----------|--------------|-------|
| `--output agent=path` | `output` | Map in config, repeatable flag in CLI |
| `--layer path` | `layers` | CLI flags replace config entirely |
| `--distributable` | `distributable` | Boolean |
| `--name value` | `name` | String |
| `--scope value` | `scope` | String (`project` or `user`) |
| `--offline` | `offline` | Boolean |
| `--locked` | `locked` | Boolean |
| `--allow-install` | `allow-install` | Boolean |
| `-v key=value` | `variables` | Map in config, repeatable flag in CLI |
| `--var-file path` | N/A | CLI-only, loads into `variables` |
| `--user-config path` | `user-config` | String |
| `--user-priority value` | `user-priority` | String (`high` or `low`) |
| `--max-remote-layers n` | `max-remote-layers` | Integer |
| `--sign` | `sign` | Boolean |
| `--sign-key name` | `sign-key` | String |
| `--verify-layers` | `verify-layers` | Boolean |
| `--include-local` | `include-local` | Boolean |
| `--package format` | `package` | String (`tar.gz` or `zip`) |

## Examples

### Minimal Project Config

```yaml
# .keel/config.yml
layers:
  - 'https://github.com/acme-corp/policy.git'
  - ./team-standards
  - ./project-config

output:
  claude: .
```

Running `keel compile` with this config is equivalent to:

```bash
keel compile \
  --layer 'https://github.com/acme-corp/policy.git' \
  --layer ./team-standards \
  --layer ./project-config \
  --output claude=.
```

### Multi-Agent Config

```yaml
# .keel/config.yml
layers:
  - 'https://github.com/acme-corp/policy.git'
  - ./shared-config

output:
  claude: .
  cursor: .

variables:
  environment: staging
  api_url: https://api.staging.example.com
```

### Enterprise Config with Signing

```yaml
# .keel/config.yml
layers:
  - 'https://github.com/acme-corp/security-policy.git'
  - 'https://github.com/acme-corp/team-standards.git'
  - ./project-config

output:
  claude: .

scope: project
locked: true
verify-layers: true
sign: true
sign-key: ci-release
allow-install: false
max-remote-layers: 20

variables:
  environment: production
  compliance_mode: strict
```

This config enforces:
- Lockfile verification (reproducible builds)
- Layer signature verification (trusted sources only)
- Output manifest signing (tamper detection)
- No install commands (security)
- Limited remote layer depth (prevent layer bombs)

### User-Scope Config

```yaml
# ~/.keel/config.yml — used when compiling with --scope user
layers:
  - ~/shared-layers/company-policy
  - ~/shared-layers/my-tools

output:
  claude: ~

variables:
  author_name: "Alice Engineer"
  author_email: "alice@example.com"
```

The `~/.keel/config.yml` file is only consulted when compiling with `--scope user`.
It is not merged with project-level config — only one config file is loaded per invocation.

## Environment Variable Fallbacks

When a field is not set in config or CLI, Keel checks these environment variables:

| Field | Environment Variable | Default |
|-------|---------------------|---------|
| `layers` | `CONFIG_LAYERS` | `[]` |
| `user-config` | `USER_CONFIG` | `/mnt/user-config` |
| `user-priority` | `USER_PRIORITY` | `high` |

## Path Expansion

Paths in config files support tilde expansion:

```yaml
user-config: ~/my-keel-config
output:
  claude: ~/projects/my-app
layers:
  - ~/shared-layers/company-policy
```

The `~` is expanded to the user's home directory at config load time.

## Unknown Keys

Unknown keys in config files produce warnings but do not cause errors.
This supports forward compatibility — new Keel versions can add fields without breaking old configs.

## What's Next

- **[DSL Reference]({{< relref "../guide/dsl" >}})** — the `keel.yml` layer manifest format.
- **[CLI Reference]({{< relref "cli" >}})** — all `keel` commands and flags.
- **[Packaging]({{< relref "../guide/packaging" >}})** — distributable layers and lockfile verification.
- **[Enterprise Example]({{< relref "../guide/enterprise-example" >}})** — a complete setup using config files.
