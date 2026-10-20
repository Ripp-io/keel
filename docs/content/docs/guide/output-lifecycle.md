---
title: Output Lifecycle
weight: 11
---

# Output Lifecycle

Keel tracks every file it produces — automatic pruning of stale files on recompilation, clean removal, and preservation of files you created manually.

## Quick Examples

### Remove stale files automatically

```bash
# First compile produces commands/old-cmd.md
keel compile --layer ./my-layer --output claude=.

# Remove old-cmd from the layer, recompile — Keel deletes it from output
keel compile --layer ./my-layer --output claude=.
```

### Clean all compiled output

```bash
keel clean
keel clean ./output
```

Removes only files Keel created.
Manually created files are never touched.

### Clean and recompile in one step

```bash
keel compile --layer ./my-layer --output claude=. --clean
```

## Compilation Manifest

Every compilation produces a `.keel.json` manifest in the output directory (inside `.claude/` or `.cursor/`).
Don't confuse this with `keel.yml` — the source layer definition.
The manifest records what was compiled, from where, and how:

```yaml
agent: claude
scope: project
compiled_at: "2026-05-10T14:30:00Z"
keel_version: "1.0.0"

layers:
  - path: /layers/01-company
    name: Acme Corp
    type: local
    format: keel

file_ops:
  - output: CLAUDE.md
    sources:
      - layer: /layers/01-company
        file: instructions.md
      - layer: /layers/04-project
        file: instructions.md
    strategy: concatenate

force_locked:
  - category: mcp
    key: sonarqube
    locked_by: /layers/01-company
```

The manifest can include a cryptographic signature when compiled with `--sign`.
See [Signing]({{< relref "signing" >}}) for details.

The manifest enables:
- `keel explain` — human-readable breakdown of the output
- `keel clean` — safe removal of compiled files
- `keel query` — programmatic access to compiled metadata
- Automatic pruning of stale files on recompilation
- `keel verify` — signature verification when the manifest is signed

## Automatic Pruning

On each compilation, Keel reads the previous manifest and removes files that are no longer produced.
This prevents stale config from accumulating across compilations.

Disable with `--no-prune`:

```bash
keel compile --layer ./my-layer --output claude=. --no-prune
```

## File Preservation

Files not tracked in any manifest are never deleted:
- `keel clean` only removes files Keel created
- Automatic pruning only removes files from a previous Keel compilation
- Manually created files (e.g., `.claude/settings.local.json`) are always safe

## Overlay Provenance

The manifest tracks which layers wrote which overlay files.
When multiple layers overlay the same file, the manifest records the full chain:

```yaml
file_ops:
  - output: .claude/commands/build.md
    sources:
      - layer: /layers/02-team
        file: commands/build.md
      - layer: /layers/04-project
        file: commands/build.md
    strategy: overlay
```

Use `--summary` to see shadowed files — files overwritten by later layers:

```
Shadowed files:
  .claude/commands/build.md
    /layers/02-team/commands/build.md  (shadowed)
    /layers/04-project/commands/build.md  (active)
```

## Rollback

When a rule causes problems — an instruction that confuses the agent, a hook that blocks legitimate work, a permission change that breaks a workflow — you need to revert quickly.

### Current workflow

Keel's source layers live in Git.
Rollback is a Git revert plus a recompile:

```bash
# Revert the problematic layer change
git revert <commit>

# Clean compiled output and recompile from the reverted state
keel compile --clean
```

This removes all compiled output, recompiles from the current (reverted) layer state, and prunes any stale files.
The agent immediately operates under the previous configuration.

For a faster path when you know which output to remove:

```bash
# Remove all compiled output
keel clean

# Recompile from the last known-good layer state
git checkout <known-good-tag> -- layers/
keel compile
```

### Decommissioning agent config entirely

To remove all Keel-managed agent config from a project:

```bash
keel clean
```

This removes every file Keel created — instructions, settings, hooks, commands, agents, skills — while preserving any files you created manually.
The `.keel.json` manifest is also removed.

### Snapshots

Keel can capture compiled output state as timestamped snapshots for behavioral rollback without touching source layers.

#### `keel snapshot create`

Capture the current compiled state:

```bash
keel snapshot create
keel snapshot create ./output
```

Auto-discovers `.keel.json` manifests in agent subdirectories and creates a snapshot for each discovered agent.
Snapshots are stored in `.keel/snapshots/` with a timestamp-based ID.

#### `keel snapshot list`

List all snapshots with ID, agent, and timestamp:

```bash
keel snapshot list
```

Output example:

```
20261019T143022Z  claude  2026-10-19 14:30:22 UTC
20261019T120000Z  cursor  2026-10-19 12:00:00 UTC
```

#### `keel snapshot restore`

Restore a previous snapshot, overwriting current compiled state:

```bash
keel snapshot restore 20261019T143022Z
keel snapshot restore 20261019T143022Z ./output
```

#### `keel snapshot prune`

Remove old snapshots:

```bash
keel snapshot prune --keep 5
keel snapshot prune --keep 3 ./output
```

Default: keeps the 5 newest snapshots.

#### Auto-snapshot on compile

Automatically capture a snapshot before each compilation:

```bash
keel compile --snapshot
```

This provides a rollback point if the new compilation introduces problems.

### Choosing between Git rollback and snapshots

**Git rollback** (revert + recompile) changes source configuration — use it when the layer change was wrong.
**Snapshots** preserve the exact compiled output state — use them for quick behavioral rollback without touching layers, or when experimenting with compilation flags and variables.

## What's Next

- **[CLI Reference]({{< relref "../reference/cli" >}})** — `keel clean`, `keel explain`, `--clean`, `--no-prune`.
- **[Packaging]({{< relref "packaging" >}})** — archiving compiled output and lockfile verification.
- **[Governance]({{< relref "governance" >}})** — force-lock provenance in the manifest.
