---
title: Merge Strategies
weight: 3
---

# Merge Strategies

When multiple layers define the same feature, Keel merges them according to a defined strategy.
Each feature has a default strategy, and some can be overridden per-file via the `files` map in `keel.yml`.

## Concatenate

**Used by**: Instructions.

Instruction files from all layers are appended in order, separated by section headers:

```markdown
# --- Company Standards ---

Follow conventional commits.
All code must pass security scanning.

# --- Project Config ---

Use feature branches for all work.
Run tests before committing.
```

The header format is `# --- layer name ---`.
If a layer has no name, the directory basename is used.

Force-locked instruction files cannot be overridden — if a downstream layer provides a file with the same output path, it is rejected.

## Deep-Merge

**Used by**: Settings, MCP servers, hooks, repos.

Recursive map merge where later values override earlier ones at the leaf level:

```
Layer 1:                    Layer 2:                    Result:
{                           {                           {
  "model": "sonnet",          "model": "opus",            "model": "opus",
  "env": {                    "env": {                    "env": {
    "A": "1"                    "B": "2"                    "A": "1",
  }                           }                             "B": "2"
}                           }                             }
                                                        }
```

Rules:
- Scalar values: later layer wins.
- Objects: recursively merged.
- Arrays: concatenated and deduplicated.
- Force-locked values: reject override attempts from later layers.

For hooks, arrays at the event level are concatenated — handlers from multiple layers stack:

```
Layer 1:                          Layer 2:                    Result:
{                                 {                           {
  "PreToolUse": [                   "PreToolUse": [             "PreToolUse": [
    {"command": "check-1"}            {"command": "check-2"}      {"command": "check-1"},
  ]                                 ]                             {"command": "check-2"}
}                                 }                             ]
                                                              }
```

## Overlay

**Used by**: Commands, agents, skills, plugins, agent-memory, environment, rules.

File-level copy where the last layer providing a file with the same name wins:

```
Layer 1:                    Layer 2:                    Result:
commands/                   commands/                   commands/
├── build.md                ├── build.md  ← wins        ├── build.md  (from layer 2)
└── test.md                 └── deploy.md               ├── test.md   (from layer 1)
                                                        └── deploy.md (from layer 2)
```

For skills, the overlay unit is the skill directory — a downstream layer providing the same skill name replaces the entire skill directory.

Force-locked overlay files or directories cannot be overridden.
The `files` map in `keel.yml` controls force-locking:

```yaml
files:
  "commands/security-scan.md":
    force: true
```

## Additive

**Used by**: Permissions (when `policy.merge: additive` is set).

Arrays are unioned — later layers can only add entries, never remove:

```
Layer 1:                    Layer 2:                    Result:
{                           {                           {
  "allow": ["Read", "Grep"],  "allow": ["Bash"],          "allow": ["Read", "Grep", "Bash"],
  "deny": ["Write(/etc/*)"]  "deny": []                  "deny": ["Write(/etc/*)"]
}                           }                           }
```

This prevents downstream layers from weakening security policies set by higher-trust layers.

## Replace

**Used by**: Permissions (when `policy.merge: replace` is set, which is the default).

The entire value from the later layer replaces the earlier one:

```
Layer 1:                    Layer 2:                    Result:
{                           {                           {
  "allow": ["Read", "Grep"],  "allow": ["Bash"],          "allow": ["Bash"],
  "deny": ["Write(/etc/*)"]  "deny": []                  "deny": []
}                           }                           }
```

## Force-Lock

Force-lock is not a merge strategy itself — it's a modifier that applies on top of any strategy.

When a value is force-locked by a higher-trust layer:
- Deep-merge: the locked key's value is preserved; downstream attempts to set it are silently rejected.
- Overlay: the locked file is preserved; downstream files with the same name are rejected.
- Concatenate: the locked instruction file is preserved; downstream files targeting the same output are rejected.
- Additive: the locked entries are preserved (additive already prevents removal).

## Per-File Strategy Override

The `files` map in `keel.yml` can override the merge strategy for specific files:

```yaml
files:
  "permissions.json":
    merge: additive
  "settings.json":
    merge: replace
```

Valid strategy values: `deep-merge`, `replace`, `additive`.

## Implementation

Merge strategies are implemented in `pkg/merge/`:
- `merge.go` -- deep-merge for maps and arrays.
- Force-lock enforcement is in `pkg/compiler/accumulator.go` within the `forceRegistry`.
- MCP, hooks, settings, categories, and file policies all use a contribution model — collected during Fold, then processed in the governance and transform phases (`ApplyGovernance`, `ApplyAgentGovernance`, `TransformForAgent`, `Finalize`).
- Overlay tracking with provenance is in the accumulator's category index.

## What's Next

- **[Governance]({{< relref "../guide/governance" >}})** — force-locks and merge policies.
- **[IR Design]({{< relref "ir" >}})** — how data flows through the merge pipeline.
- **[Layers]({{< relref "../guide/layers" >}})** — merge behavior summary per feature.
