---
title: Validation
weight: 9
---

# Validation

Check a layer for structural errors, schema issues, and portability warnings before you compile.

## Quick Example

```bash
keel validate ./my-layer
```

```
Validating ./my-layer ...

  Format:  keel (v1)
  Project: my-project

  ✓ keel.yml schema valid
  ✓ instructions.md exists
  ✓ permissions.json exists
  ⚠ skill "deploy": description field is empty
  ⚠ $ARGUMENTS pattern found in commands/build.md (Claude-only)

  1 warning(s), 0 error(s)
```

Warnings are non-blocking — they indicate potential issues but don't prevent compilation.
Errors are blocking — they cause a non-zero exit code.

## What Gets Checked

Keel checks:
- **Layer structure** — directory exists and contains expected marker files
- **Schema parsing** — `keel.yml` fields are valid with correct types and bounds
- **File references** — all files and directories referenced in `keel.yml` exist
- **Collection bounds** — arrays don't exceed maximum sizes (e.g., max 1000 instructions, max 100 hooks)
- **Repo path safety** — repo paths are relative without `..` traversal
- **Merge strategy values** — `files` map uses valid strategy names
- **FileRef consistency** — no entry has both `source` and `path` set

## Agent Compatibility

Add `--agent` to check portability against a specific agent:

```bash
keel validate ./my-layer --agent claude
keel validate ./my-layer --agent cursor
```

This adds warnings for features the target agent does not support.
For example, using `plugins` in a layer validated against Cursor produces a warning because Cursor does not support plugins.

## Agent Skills Validation

Keel validates skill directories against the [Agent Skills specification](https://agentskills.io/specification):

1. `SKILL.md` file exists
2. `name` field is present and follows kebab-case naming
3. `description` field is present
4. Frontmatter is valid YAML
5. Companion files referenced by the skill exist
6. Metadata fields are correctly typed
7. `allowed-tools` entries are valid tool names
8. `paths` entries are valid glob patterns
9. Body content is non-empty
10. Directory name matches the skill name

Disable with `--agentskills-check=false`:

```bash
keel validate ./my-layer --agentskills-check=false
```

## Body Content Linting

The `--lint` flag (enabled by default) scans markdown content for non-portable agent-specific patterns:

| Pattern | Agent | Warning |
|---------|-------|---------|
| `$ARGUMENTS` | Claude | Positional argument substitution is Claude-only |
| `$ARGUMENTS[N]` | Claude | Indexed argument access is Claude-only |
| `$0`, `$1` | Claude | Positional arguments are Claude-only |
| `${CLAUDE_SKILL_DIR}` | Claude | Claude-specific variable |
| `` !`command` `` | Claude | Dynamic shell injection is Claude-only |
| `{{variable}}` | Cursor | Template variables are Cursor-only |

Linting is code-block aware — patterns inside fenced code blocks are ignored.

When `--agent` is set, warnings for the target agent's own patterns are suppressed.
For example, `$ARGUMENTS` does not warn when validating with `--agent claude`.

Disable linting with `--lint=false`:

```bash
keel validate ./my-layer --lint=false
```

## Context Budget

Keel estimates token counts and warns when content exceeds research-backed thresholds.
LLM performance degrades as context grows — skills become harder to discover, instructions compete for attention, and tool selection accuracy drops.
These checks catch budget drift before it causes agent degradation.

All context budget findings are warnings, never errors.
The layer author decides whether to act.

### Skill body tokens

Warns when a skill's markdown body (after frontmatter) exceeds ~5,000 tokens.
Research shows skill quality degrades above this threshold — the LLM struggles to follow lengthy instructions reliably.

Default threshold: 5,000 tokens (~20,000 bytes).
Configure in `keel.yml`:

```yaml
validate:
  token-budget:
    skill-body: 8000
```

Set to 0 to disable.

### Skill description length

Warns when a skill's `description` field exceeds 1,024 characters.
Descriptions are the discovery tier — the LLM reads every description to decide which skill to invoke.
Bloated descriptions increase context cost at the discovery level, degrading selection accuracy across all skills.

This check runs independently of the Agent Skills spec check (`--agentskills-check`).
Both may warn about the same skill, but for different reasons: spec compliance vs context budget.

### Instructions total

Warns when the total instruction content across all instruction files exceeds a threshold.
Disabled by default (threshold 0) — set a project-appropriate limit:

```yaml
validate:
  token-budget:
    instructions: 50000
```

### Layer content total

Warns when the total content of the layer (instructions + commands + agents + skills + rules) exceeds a threshold.
Disabled by default (threshold 0):

```yaml
validate:
  token-budget:
    layer: 100000
```

### Token estimation

Keel uses a simple heuristic: ~4 bytes per token (ceiling division).
This is conservative for UTF-8 text and close enough for English/code content.
No external tokenizer dependency is required.

Disable all context budget checks with `--token-budget=false`:

```bash
keel validate ./my-layer --token-budget=false
```

## Agent Tool Count

Warns when an agent definition's `allowed-tools` list exceeds a configurable threshold.

Tool selection accuracy degrades sharply past ~10 tools and collapses past ~12 in flat configurations.
In benchmarks, GPT-4o dropped from 43% accuracy with 4 tools to 2% accuracy with 51 tools on the same task ([LangChain ReAct benchmark](https://achan2013.medium.com/how-tool-complexity-impacts-ai-agents-selection-accuracy-a3b6280ddce5)).
Separately, research on semantic tool discovery found that per-tool schemas cost 200-800 tokens each, meaning 50 tools consume 10,000-40,000 tokens before any work begins ([arXiv 2603.20313](https://arxiv.org/html/2603.20313)).

Default threshold: 10 tools.
Configure in `keel.yml`:

```yaml
validate:
  agent-tools: 15
```

Set to 0 to disable.
Disable via CLI with `--agent-tools-check=false`:

```bash
keel validate ./my-layer --agent-tools-check=false
```

## Layer-Level Options

Control validation behavior from within `keel.yml`:

```yaml
validate:
  agentskills: true
  token-budget:
    skill-body: 5000
    instructions: 0
    layer: 0
  agent-tools: 10
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `agentskills` | bool | `true` | Run Agent Skills format validation |
| `token-budget.skill-body` | int | `5000` | Per-skill body token threshold (0 = disabled) |
| `token-budget.instructions` | int | `0` | Total instruction tokens threshold (0 = disabled) |
| `token-budget.layer` | int | `0` | Total layer content tokens threshold (0 = disabled) |
| `agent-tools` | int | `10` | Agent tool list threshold (0 = disabled) |

Thresholds use `*int` semantics: absent = use default, `0` = disabled, positive = custom.

## What's Next

- **[CLI Reference]({{< relref "../reference/cli" >}})** — the `keel validate` command with all flags.
- **[Transpilation]({{< relref "transpilation" >}})** — cross-format portability and linting.
- **[DSL Reference]({{< relref "dsl" >}})** — the `validate` field in `keel.yml`.
