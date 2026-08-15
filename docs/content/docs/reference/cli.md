---
title: CLI Reference
weight: 1
---

# CLI Reference

Every Keel CLI command with flags, examples, and expected behavior.

| Command | Purpose |
|---------|---------|
| [`keel add`](#keel-add) | Add a layer and compile |
| [`keel remove`](#keel-remove) | Remove a layer and recompile |
| [`keel compile`](#keel-compile) | Compile layers into tool-native output |
| [`keel validate`](#keel-validate) | Check a layer for errors and warnings |
| [`keel explain`](#keel-explain) | Show which layer produced each output value |
| [`keel report`](#keel-report) | Generate compliance report from compiled manifest |
| [`keel init`](#keel-init) | Scaffold a new layer interactively |
| [`keel import`](#keel-import) | Convert native agent config to Keel DSL |
| [`keel query`](#keel-query) | Query compiled config values |
| [`keel cache`](#keel-cache) | Manage the remote layer cache |
| [`keel clean`](#keel-clean) | Remove compiled output files |
| [`keel drift`](#keel-drift) | Detect manual changes to compiled output |
| [`keel package`](#keel-package) | Create an archive from a directory |
| [`keel global`](#keel-global) | Manage globally installed dependencies |
| [`keel key`](#keel-key) | Manage Ed25519 signing keys |
| [`keel sign`](#keel-sign) | Sign a layer with an Ed25519 key |
| [`keel verify`](#keel-verify) | Verify signatures on a layer or compiled output |

## `keel add`

Add a configuration layer to the project and compile.

```bash
keel add <layer>
```

If no `.keel/config.yml` exists, one is created with auto-detected output targets.
The layer is appended as the last entry (highest priority before `.keel/local`).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--no-compile` | `bool` | `false` | Add layer without auto-compiling |
| `--output` | `agent=path` | auto-detected | Agent output targets (repeatable) |
| `--scope` | `string` | `project` | Scope: `project` or `user` (user writes to `~/.keel/config.yml`) |

### Examples

Add a local layer:

```bash
keel add ./layers/team-standards
```

Add a remote layer from a Git repo:

```bash
keel add 'https://github.com/org/shared-config.git#main'
```

Add a layer with explicit output targets:

```bash
keel add ./my-layer --output claude=. --output cursor=.
```

Add without compiling:

```bash
keel add ./my-layer --no-compile
```

Add a layer to your user-level config:

```bash
keel add --scope user ./my-tools
```

### Behavior

**New project** (no `.keel/config.yml`):
Creates `.keel/config.yml`, auto-detects output targets from existing agent directories (e.g. `.claude/`, `.cursor/`), adds the layer, and compiles.

**Existing project** (`.keel/config.yml` exists):
Appends the layer to the config, merges any `--output` targets, and recompiles.
If the layer is already present, prints a message and skips.

**User scope** (`--scope user`):
Operates on `~/.keel/config.yml` instead of the project-level config.
Output targets are auto-detected from agent directories in your home directory (e.g. `~/.claude/`).
Auto-compilation uses user scope, which skips `.keel/local` (project-local overrides don't belong in user-level output).

---

## `keel remove`

Remove a configuration layer from the project and recompile.

```bash
keel remove <layer>
```

The layer source must match exactly as it appears in the config file.
Use `keel explain` or inspect `.keel/config.yml` to see current layers.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--no-compile` | `bool` | `false` | Remove layer without recompiling |

### Examples

Remove a local layer:

```bash
keel remove ./layers/team-standards
```

Remove a remote layer:

```bash
keel remove 'https://github.com/org/shared-config.git#main'
```

Remove without recompiling:

```bash
keel remove ./my-layer --no-compile
```

---

## `keel compile`

Compile configuration layers into tool-native output for one or more agents.

The recommended approach is a `.keel/config.yml` file checked into your repo — run `keel compile` with no arguments and get repeatable results every time.
CLI flags work for one-off usage, CI overrides, and quick experimentation.

### Config File

Create `.keel/config.yml` in your project root:

```yaml
layers:
  - ./layers/*

output:
  claude: .
  cursor: .
```

Then compile:

```bash
keel compile
```

Keel auto-discovers `.keel/config.yml` and compiles all matched layers for every listed agent.
With `--scope user`, Keel checks `~/.keel/config.yml` first, falling back to the project-level config.

A full config file with all options:

```yaml
layers:
  - ./layers/*
  - https://github.com/org/shared-config.git#main

output:
  claude: .
  cursor: .

scope: project
offline: false
allow-install: false

variables:
  environment: staging
  api_url: https://api.example.com
```

CLI flags take precedence over config file values — use them for overrides:

```bash
keel compile -v environment=production
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | `string` | `.keel/config.yml` | Config file for compile parameters |
| `--output` | `agent=path` | from config | Agent output targets (repeatable) |
| `--layer` | `string` | from config or `CONFIG_LAYERS` env | Layer paths (repeatable) |
| `--distributable` | `string` | — | Produce a distributable layer with this name |
| `--user-config` | `string` | `/mnt/user-config` | User config directory |
| `--user-priority` | `string` | `high` | User config priority: `high` or `low` |
| `--scope` | `string` | `project` | Compilation scope: `project` or `user` |
| `--include-local` | `bool` | `false` | Include `.keel/local` layer even at user scope (normally skipped) |
| `--offline` | `bool` | `false` | Use cached remote layers without network |
| `--locked` | `bool` | `false` | Verify layer content matches lockfile |
| `--package` | `string` | — | Archive compiled output: `tar.gz` or `zip` |
| `--dry-run` | `bool` | `false` | Preview without writing to disk |
| `--summary` | `bool` | `false` | Print compilation report after writing |
| `--stats` | `bool` | `false` | Print compiled content metrics (token estimates, counts, sizes) |
| `--clean` | `bool` | `false` | Remove previous output before compiling |
| `--no-prune` | `bool` | `false` | Skip automatic stale file removal |
| `--force` | `bool` | `false` | Overwrite user-edited instruction files without warning |
| `--strict` | `bool` | `false` | Exit with error when instruction files conflict (for CI) |
| `-v`, `--var` | `key=value` | — | Set a template variable (repeatable, e.g. `-v env=staging`) |
| `--var-file` | `string` | — | Load variables from a KEY=VALUE file (repeatable) |
| `--allow-install` | `bool` | `false` | Execute `deps` install/download commands (safe by default — without this flag, deps are parsed but skipped) |
| `--show-vars` | `bool` | `false` | Print resolved variable map after compilation |
| `--sign` | `bool` | `false` | Sign the compiled manifest with Ed25519 |
| `--sign-key` | `string` | `default` | Key name for signing |
| `--verify-layers` | `bool` | `false` | Require and verify layer signatures before compiling |

### Examples

Most projects use a config file (above).
These CLI examples are for one-off usage and quick experimentation.

Compile a single layer for Claude:

```bash
keel compile --layer ./my-layer --output claude=.
```

Compile multiple layers for both Claude and Cursor:

```bash
keel compile \
  --layer ./company-layer \
  --layer ./team-layer \
  --layer ./project-layer \
  --output claude=. \
  --output cursor=.
```

Preview what would be compiled:

```bash
keel compile --dry-run
```

Compile with debug output:

```bash
keel compile --debug
```

Compile in user scope (outputs to user-level paths):

```bash
keel compile --layer ./my-layer --output claude=. --scope user
```

Compile with lockfile verification:

```bash
keel compile --locked
```

Produce a distributable layer:

```bash
keel compile --distributable "platform-standards"
```

Compile for AGENTS.md output (alongside Claude and Cursor):

```bash
keel compile --output claude=. --output cursor=. --output agentsmd=.
```

Fail in CI when instruction files conflict (e.g. user-edited CLAUDE.md):

```bash
keel compile --strict
```

Force-overwrite user-edited instruction files:

```bash
keel compile --force
```

Archive the output after compilation:

```bash
keel compile --package tar.gz
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_LAYERS` | `/keel/layers/*` | Colon-separated layer paths (used when no `--layer` flags) |
| `USER_CONFIG` | `/mnt/user-config` | User config directory |
| `USER_PRIORITY` | `high` | User config priority |

### Output

On success, writes tool-native files to the output directory plus a `.keel.json` compilation manifest.
With `--summary` or `--debug`, prints a report showing layer stack, files written, and force-locked items.
With `--stats`, prints compiled content metrics: token estimates per section, item counts, and total size.
Both `--summary` and `--stats` can be used together.
Dry-run mode (`--dry-run`) always includes content stats.

---

## `keel validate`

Check a layer for structure errors, schema issues, and missing file references.

```bash
keel validate [path]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--agent` | `string` | — | Target agent for compatibility checks (`claude`, `cursor`) |
| `--agentskills-check` | `bool` | `true` | Run Agent Skills format validation on skill directories |
| `--lint` | `bool` | `true` | Scan markdown for non-portable agent-specific patterns |
| `--token-budget` | `bool` | `true` | Estimate token counts and warn about budget thresholds |
| `--agent-tools-check` | `bool` | `true` | Warn when agent tool lists exceed threshold |

### Examples

Validate the current directory:

```bash
keel validate
```

Validate a specific layer:

```bash
keel validate ./my-layer
```

Validate with agent-specific compatibility checks:

```bash
keel validate ./my-layer --agent claude
```

Disable linting:

```bash
keel validate ./my-layer --lint=false
```

### Output

Prints format, project name, and a list of warnings and errors.
Warnings are non-blocking.
Errors cause a non-zero exit code.

See [Validation]({{< relref "../guide/validation" >}}) for details on what is checked.

---

## `keel explain`

Show a human-readable breakdown of compiled output.

```bash
keel explain [dir] [section]
```

### Arguments

| Argument | Default | Description |
|----------|---------|-------------|
| `dir` | `.` | Directory containing compiled `.keel.json` |
| `section` | all | Show only a section: `files`, `locks`, `layers`, `deps`, or `signatures` |

### Examples

Explain everything:

```bash
keel explain
```

Show only the layer stack:

```bash
keel explain layers
```

Show only force-locked items:

```bash
keel explain locks
```

Explain a specific directory:

```bash
keel explain ./output files
```

### Output

Prints a summary with:
- **Header** — agent, scope, compiled timestamp, Keel version.
- **Layers** — ordered list of input layers with format and source.
- **Files** — every output file with source layers and merge strategy.
- **Locks** — force-locked items with category, key, and locking layer.
- **Repos** — merged repository definitions (if any).
- **Deps** — installed dependencies with name, scope, method, and path (if any).
- **Signatures** — manifest and layer signature details (if signed).

---

## `keel report`

Generate a compliance report from a compiled manifest.

```bash
keel report [dir]
```

### Arguments

| Argument | Default | Description |
|----------|---------|-------------|
| `dir` | `.` | Directory containing compiled `.keel.json` |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | `string` | `json` | Report format: `json`, `yaml`, or `text` |
| `-o`, `--output` | `string` | stdout | Write to file instead of stdout |

### Examples

Generate JSON report:

```bash
keel report
```

Generate YAML report:

```bash
keel report --format yaml
```

Generate text report:

```bash
keel report --format text
```

Write report to file:

```bash
keel report --format json -o compliance-report.json
```

Report from specific directory:

```bash
keel report ./output
```

### Output

The report includes:
- **Schema version** — report format version
- **Compilation metadata** — agent, scope, timestamp, Keel version
- **Layer BOM** — ordered layer stack with format, source, and content hash
- **Governance record** — permissions policy, MCP policy, override conflicts, hook lock strategy
- **Force locks** — locked content by category and key
- **File operations** — every file written with strategy and content hash
- **Dependencies** — installed binaries with scope, method, and path
- **Signatures** — manifest signature and layer signature chain (if signed)
- **Effective state** — authoritative compile-time snapshot:
  - Permissions (allow/deny)
  - MCP servers (name, command, args)
  - Hooks (event, command, background flag)
  - Settings (portable settings map)
  - Instructions (concatenated content)
  - Categories (overlay file paths by category)

Auto-discovers `.keel.json` in agent subdirectories (`.claude`, `.cursor`, `.agents`).

---

## `keel init`

Scaffold a new Keel project with interactive prompts.

```bash
keel init [dir]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | `string` | — | Project name (skips prompt) |
| `--agent` | `string` | — | Target agent (skips prompt) |

### Examples

Interactive scaffold:

```bash
keel init ./my-new-layer
```

Non-interactive:

```bash
keel init ./my-new-layer --name my-project --agent claude
```

### Output

Creates a directory with a `keel.yml` and `instructions.md` ready for compilation.

---

## `keel import`

Read native agent config and produce a Keel DSL layer.

```bash
keel import --to ./my-layer
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from` | `string` | `.` | Source directory with native agent config |
| `--agent` | `string` | auto-detected | Source agent format (`claude`, `cursor`) |
| `--to` | `string` | required | Output directory for the generated layer |
| `--name` | `string` | source dir basename | Layer name |

### Examples

Import from the current directory:

```bash
keel import --to ./my-layer
```

Import from a specific project:

```bash
keel import --from ./project --to ./my-layer
```

Import with explicit agent format:

```bash
keel import --from ./project --agent claude --to ./my-layer
```

### Output

Creates a self-contained Keel DSL layer directory with a `keel.yml` and all referenced companion files.

See [Transpilation]({{< relref "../guide/transpilation" >}}) for details on the import process.

---

## `keel query`

Query values from the compiled `.keel.json` manifest.
Run after `keel compile`.

```bash
keel query <subcommand> [--dir <path>]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `name` | Print the compiled project name |
| `repos` | Print the merged repos map (JSON) |
| `repo <name>` | Print a single repo definition (JSON) |
| `init-dir` | Print the init directory path |
| `layers` | Print the layer stack |
| `deps` | Print installed dependencies (JSON) |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir` | `string` | `.` | Compiled output directory |

### Examples

```bash
keel query name
keel query repos
keel query repo party-service
keel query init-dir
keel query layers --dir ./output
keel query deps
```

---

## `keel cache`

Manage the local cache of remote layers.

### `keel cache list`

List all cached remote layers with type, URL, age, and size.

```bash
keel cache list
```

### `keel cache clean`

Remove cached remote layers.

```bash
keel cache clean
```

Remove entries older than a duration:

```bash
keel cache clean --older-than 30d
keel cache clean --older-than 12h
keel cache clean --older-than 45m
```

| Flag | Type | Description |
|------|------|-------------|
| `--older-than` | `string` | Remove only entries older than this duration (suffixes: `d`, `h`, `m`) |

---

## `keel clean`

Remove all files produced by a previous compilation, preserving user-created files.

```bash
keel clean [dir]
```

Reads the `.keel.json` manifest to determine which files were compiled, then removes them.
The `.keel.json` manifest is also removed (along with any legacy `keel.yml` or `keel-lock.json` files).
Files not tracked in any manifest are preserved.

### Examples

Clean the current directory:

```bash
keel clean
```

Clean a specific directory:

```bash
keel clean ./output
```

---

## `keel drift`

Detect when native config files have been manually edited outside Keel.

Compares the current disk state against the compiled `.keel.json` manifest.
All files are checked for content changes using content hashes.
Overlay files (commands, skills, agents) are checked for existence and content.

Returns non-zero exit code when drift is detected.

```bash
keel drift [dir]
```

### Arguments

| Argument | Default | Description |
|----------|---------|-------------|
| `dir` | `.` | Directory containing compiled `.keel.json` |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--semantic` | `bool` | `false` | Show semantic drift (what changed in effective state) |
| `--policy` | `bool` | `false` | Check only force-locked files |
| `--format` | `string` | `text` | Output format: `text` or `json` |
| `--verify` | `bool` | `false` | Include signature verification in drift check |

### Examples

Check for drift in the current directory:

```bash
keel drift
```

Show semantic drift (what changed):

```bash
keel drift --semantic
```

Check only force-locked files:

```bash
keel drift --policy
```

Machine-readable output for CI:

```bash
keel drift --format json
```

Check a specific output directory:

```bash
keel drift ./output
```

Use in CI to gate deployments:

```bash
keel compile --strict
keel drift || echo "Config has been manually modified"
```

### Output

When no drift:

```
No drift detected.
```

When file drift is detected:

```
  modified  CLAUDE.md
  missing   .claude/commands/deploy.md

Error: drift detected
```

When semantic drift is detected (with `--semantic`):

```
Semantic drift detected:

Permissions:
  + allow: Write (added)
  - allow: Delete (removed)

MCP servers:
  ~ server1: command changed from "npx" to "node"

Hooks:
  + SubagentStart: ./hooks/start.sh (added)

Settings:
  ~ model: "claude-sonnet-4" → "claude-opus-4"

Instructions:
  Content changed (1234 → 1456 bytes)
```

Auto-discovers `.keel.json` in agent subdirectories (`.claude`, `.cursor`, `.agents`).
If multiple agents are found, results are grouped by agent.

---

## `keel package`

Create a tar.gz or zip archive from a directory.

```bash
keel package --from <dir>
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from` | `string` | required | Directory to package |
| `--format` | `string` | `tar.gz` | Archive format: `tar.gz`, `tgz`, or `zip` |
| `--to` | `string` | `<from>.<ext>` | Output archive path |

### Examples

```bash
keel package --from /keel/output/claude
keel package --from /keel/output/claude --format zip
keel package --from /keel/output/claude --to /tmp/claude-config.tar.gz
```

---

## `keel global`

Manage dependencies installed to `~/.keel/bin/` by layers with `global: true`.

### `keel global list`

List all globally installed dependencies with consumer counts and install age.

```bash
keel global list
```

### `keel global clean`

Remove global dependencies that have no remaining consumer projects.

```bash
keel global clean         # interactive confirmation per orphan
keel global clean --yes   # remove all orphans without prompting
```

Global deps use reference counting — each project that compiles with a global dep is registered as a consumer.
`keel clean` deregisters the project.
`keel global clean` finds deps with zero remaining consumers and removes them.

---

## `keel key`

Manage Ed25519 signing keys for layer and manifest signing.

Keys are stored in `~/.keel/keys/` with private keys in PKCS8 format (mode 0600) and public keys in PKIX format (mode 0644).

### `keel key generate`

Generate a new Ed25519 signing key.

```bash
keel key generate
keel key generate --name work
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | `string` | `default` | Key name |
| `--force` | `bool` | `false` | Overwrite existing key with the same name |

### `keel key list`

List all signing keys in the keystore.

```bash
keel key list
```

Shows NAME, KEY ID, TYPE, and whether the private key is present.

### `keel key export`

Export a public key in PEM format.

```bash
keel key export default
keel key export default > my-key.pub
```

### `keel key trust`

Add a public key to the trusted keyring.

```bash
keel key trust coworker-key.pub
```

Trusted keys are stored in `~/.keel/keys/trusted/`.

### `keel key revoke`

Remove a key from the trusted keyring.

```bash
keel key revoke alice
keel key revoke keel:sha256:abc123...
```

Use the key name or key ID.

---

## `keel sign`

Sign a layer with an Ed25519 key.

```bash
keel sign <layer-path>
```

Creates a `keel.sig` file alongside `keel.yml` with the signature, content hash, and signing metadata.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--key` | `string` | `default` | Key name to use for signing |

### Examples

```bash
keel sign ./layers/company-policy
keel sign ./layers/team --key work
```

---

## `keel verify`

Verify signatures on a layer or compiled output.

```bash
keel verify [path]
```

Auto-detects whether the path is a layer (has `keel.yml` and `keel.sig`) or compiled output (has `.keel.json` with signature).

### Examples

Verify a layer:

```bash
keel verify ./layers/company-policy
```

Verify compiled output:

```bash
keel verify .
```

Returns non-zero exit code if verification fails.

---

## Global Flags

These flags are available on all commands:

| Flag | Description |
|------|-------------|
| `-d`, `--debug` | Increase verbosity (`--debug` info, `-dd` debug) |
| `--help` | Show help for the command |
