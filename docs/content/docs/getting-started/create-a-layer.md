---
title: Create a Layer
weight: 3
---

# Create a Layer

Write your own configuration layer and compile it into your agent's native format.
This is the producer path — for team leads, platform engineers, and anyone who wants to author shared config.

## Your First Layer

A layer is a directory containing configuration.
The simplest layer has a `keel.yml` and a markdown instructions file.

Create a layer directory:

```bash
mkdir -p my-layer
```

Create `my-layer/keel.yml`:

```yaml
schema: v1
name: my-project
description: Getting started example

instructions:
  - source: instructions.md

settings:
  model: sonnet
```

Create `my-layer/instructions.md`:

```markdown
# My Project

Follow conventional commits (feat:, fix:, chore:).
Run tests before committing.
```

## Set Up the Config File

Create `.keel/config.yml` in your project root to make compilation repeatable:

```yaml
layers:
  - ./my-layer

output:
  claude: .
```

## Compile It

```bash
keel compile
```

Keel reads `.keel/config.yml`, compiles the layer, and produces:

```
.claude/
├── settings.json
├── .keel.json
CLAUDE.md
```

`CLAUDE.md` contains your instructions.
`settings.json` contains your model preference.
`.keel.json` is the compilation manifest — it records what was compiled and from where.

Commit `.keel/config.yml` to your repo — anyone on the team runs `keel compile` and gets the same result.

## Compile for Multiple Agents

Add more agents to the config:

```yaml
layers:
  - ./my-layer

output:
  claude: .
  cursor: .
```

```bash
keel compile
```

Same source, different output — Keel handles the translation.
Cursor gets `.cursor/rules/00-keel.mdc` and `.cursor/cli-config.json` instead of Claude's format.

## Stack Multiple Layers

The power of Keel is stacking layers.
Each layer adds or overrides config from the layers before it.

Create a second layer at `team-layer/keel.yml`:

```yaml
schema: v1
name: platform-team

instructions:
  - source: instructions.md

permissions:
  rules:
    - source: permissions.json

settings:
  model: opus
```

Create `team-layer/instructions.md`:

```markdown
# Platform Team

Use feature branches for all work.
Never commit directly to main.
```

Create `team-layer/permissions.json`:

```json
{
  "allow": ["Read", "Grep", "Glob", "Bash"],
  "deny": ["Write(/etc/*)"]
}
```

Update `.keel/config.yml` to include both layers:

```yaml
layers:
  - ./my-layer
  - ./team-layer

output:
  claude: .
```

```bash
keel compile
```

The result:
- Instructions from both layers are concatenated with section headers.
- The model setting is `opus` (the later layer overrides the earlier one).
- Permissions from the team layer are included.

## Scaffold Interactively

Use `keel init` to create a new layer with prompts:

```bash
keel init ./my-new-layer
```

Keel asks for a project name, target agent, and optional MCP servers, then generates a starter `keel.yml` and `instructions.md`.

## Inspect the Result

Use `keel explain` to see where each piece of output came from:

```bash
keel explain .
```

This shows the layer stack, which layer produced each file, and what values are force-locked.

## Validate Before Compiling

```bash
keel validate ./my-layer
```

Keel checks structure, schema, and file references.
Add `--agent claude` or `--agent cursor` for agent-specific compatibility warnings.

## Preview Without Writing

```bash
keel compile --layer ./my-layer --output claude=. --dry-run
```

## What's Next

- **[Layers]({{< relref "../guide/layers" >}})** — how layers stack, merge, and resolve.
- **[DSL Reference]({{< relref "../guide/dsl" >}})** — every field in `keel.yml`.
- **[Governance]({{< relref "../guide/governance" >}})** — force-lock values so downstream layers can't override them.
- **[Enterprise Example]({{< relref "../guide/enterprise-example" >}})** — a complete 5-layer enterprise setup.
