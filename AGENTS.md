# --- authoring ---

# Architecture Conventions

## Pipeline Architecture

The compilation pipeline follows an assembly-line model: GATHER → GOVERN → TRANSFORM → RESOLVE → OUTPUT.
Each stage has exactly one job.
No reaching forward or backward to other stages.
Scattering processing logic across multiple stages is spaghetti — refactor it.

NativeReaders and Writers are thin format translators.
Agent-specific complexity belongs in the IR layer, not in readers or writers.

# Keel DSL Quick Reference

Keel compiles layered configuration into AI agent native formats (Claude Code, Cursor, and more).
Layers are directories with a `keel.yml` manifest and companion files.

## keel.yml structure

```yaml
schema: v1
name: my-layer
description: What this layer provides

instructions:                    # Markdown files, concatenated across layers
  - source: instructions.md
  - source: instructions/        # All .md files in directory, sorted

permissions:                     # JSON files with allow/deny, deep-merged
  rules:
    - source: permissions.json
  policy:
    merge: additive              # additive | replace

mcp:                             # MCP server config, deep-merged
  servers:
    - source: mcp.json

hooks:                           # Hook definitions, deep-merged
  - source: hooks.json

commands:                        # Overlay into agent's commands dir
  - source: commands/

agents:                          # Overlay into agent's agents dir
  - source: agents/

skills:                          # Overlay into agent's skills dir
  - source: skills/

settings:                        # Agent-specific settings
  model: claude-sonnet-4-20250514
  claude: settings.claude.json
  cursor: settings.cursor.json

variables:                       # Template variables (layer-defined)
  api_url: "https://api.example.com"
  environment: "production"

snippets:                        # Reusable content blocks
  - source: snippets/

bin:                             # Executables copied to .keel/bin/
  - source: bin/

deps:                            # External binary dependencies
  mytool:
    install: "go install github.com/org/tool@latest"
    verify: "tool --version"
    global: true

loop:                            # Loop prompt (Claude-only)
  - source: loop.md

worktree-include:                # Worktree include patterns (Claude-only)
  - source: worktree-include.txt

repos:                           # Repository definitions for workspace init
  my-service:
    url: "https://github.com/org/service.git"
    path: services/my-service

files:                           # Per-file governance
  "commands/deploy.md":
    force: true
```

## FileRef types

- `source: file.md` — single file
- `source: dir/` — all matching files in directory
- `source: https://github.com/org/repo.git#branch:path` — remote Git source
- Supports `force: true` for governance
- Supports `include: [name1, name2]` to cherry-pick subdirectories from a directory source

## Merge strategies by section

| Section | Strategy | Behavior |
|---------|----------|----------|
| instructions | concatenate | Appended with layer headers |
| permissions | deep-merge | Additive by default |
| mcp | deep-merge | Server entries merged by name |
| hooks | deep-merge | Events merged, entries appended |
| settings | deep-merge | Keys merged, scalars last-wins |
| commands, agents, skills, rules | overlay | Last writer wins per file |

## Auto-injected variables

These are available in all content without declaring them:

| Variable | Value |
|----------|-------|
| `keel.agent` | Target agent name (`claude`, `cursor`, `agentsmd`) |
| `keel.scope` | Compilation scope (`project`, `user`) |
| `keel.name` | Project name from keel.yml |
| `keel.output` | Output directory path |
| `keel.version` | Keel build version |
| `keel.bin.project` | Project bin dir (`.keel/bin/`) |
| `keel.bin.global` | Global bin dir (`~/.keel/bin/`) |
| `keel.layers.count` | Number of layers compiled |
| `keel.state` | State directory path (`.keel/state/`) |
| `keel.deps.<name>.path` | Path to installed dep binary |
| `keel.deps.<name>.dir` | Directory containing installed dep |

## Variable substitution

In markdown: `${keel:variable_name}`
In JSON: `${keel:variable_name}` (substituted at compile time)
CLI override: `keel compile -v key=value`
File override: `keel compile --var-file vars.env`

## Conditional blocks (markdown only)

```markdown
<!-- keel-if: keel.agent == "claude" -->
Claude-specific content.
<!-- keel-elif: keel.agent == "cursor" -->
Cursor-specific content.
<!-- keel-else -->
Fallback content.
<!-- keel-endif -->
```

Operators: `==`, `!=`, `in`, `not in`, `&&`, `||`

## Snippet includes (markdown only)

Define in `snippets/checklist.md`, use in any markdown:

```markdown
<!-- keel:include checklist -->
```

With parameters:

```markdown
<!-- keel:include checklist style="security" -->
```

Snippet content uses `{{key}}` for parameter placeholders.

## CLI commands

```bash
keel compile --layer ./path --output claude=.     # Compile
keel compile --dry-run --layer ./path             # Preview
keel compile --show-vars --layer ./path --output claude=.  # Show variables
keel compile --output agentsmd=.                  # Compile to AGENTS.md
keel compile --strict                             # Fail on instruction conflicts (CI)
keel compile --force                              # Overwrite user-edited instructions
keel validate ./path                              # Validate a layer
keel explain ./output                             # Explain compiled output
keel report ./output                              # Compliance report from manifest
keel report --format yaml                         # Compliance report in YAML
keel import --from ./native --to ./keel-layer     # Import native config
keel clean ./output                               # Remove compiled output
keel drift ./output                               # Detect manual changes
keel drift --semantic                             # Show what changed (semantic diff)
keel drift --policy                               # Check force-locked files only
keel init ./new-layer                             # Scaffold a new layer
keel package --from ./layer --format tar.gz       # Create archive
```
