---
title: Getting Started
weight: 1
bookCollapseSection: false
---

# Getting Started

Install Keel, pull community config, and compile your AI agent's configuration.

## Install

```bash
go install gitlab.com/ripp.io/oss/agentic/keel/cmd/keel@latest
```

> Keel is distributed as a Go binary.
> Pre-built binaries and package manager support are planned.
> For now, Go 1.26+ is required for installation.

Verify:

```bash
keel --help
```

<details>
<summary>Build from source</summary>

```bash
git clone git@gitlab.com:ripp.io/oss/agentic/keel.git
cd keel
make install              # builds and installs to ~/.local/bin
# or build manually:
# make build && sudo mv dist/keel /usr/local/bin/
```
</details>

## Configure Your Agent

The fastest way to get started is `keel add` — it pulls a layer, updates your config, and compiles in one step:

```bash
keel add 'https://github.com/affaan-m/everything-claude-code.git#main'
```

That's it.
Keel adds the source to `.keel/config.yml`, auto-detects output targets from your project (`.claude/`, `.cursor/`, etc.), then compiles.
During compilation, the remote repo is cloned and its format is auto-detected.

> **Safe to run on existing projects.** Keel skips instruction files (CLAUDE.md, etc.) it didn't create and warns if compiled output would conflict with existing files. Use `--force` to overwrite.
> Run `keel clean` at any time to remove only what Keel produced, leaving your files untouched.

For user-level (global) config, add `--scope user`:

```bash
keel add --scope user 'https://github.com/affaan-m/everything-claude-code.git#main'
```

This compiles to whichever agent directories exist in your home directory (e.g., `~/.claude/`), affecting the agent's global configuration.

## Add Your Own Config

Create a personal layer to add your own preferences on top.
The fastest way is `keel init`:

```bash
keel init ./my-tools --name my-toolkit --agent claude
```

Or create one manually:

```bash
mkdir -p my-tools
```

`my-tools/keel.yml`:

```yaml
schema: v1
name: my-toolkit

instructions:
  - source: instructions.md

settings:
  model: opus
```

`my-tools/instructions.md`:

```markdown
# My Preferences

Use worktrees for parallel feature work.
Commit messages reference the ticket number.
```

Add your layer on top of the community layer:

```bash
keel add ./my-tools
```

Community defaults load first, your personal config gets the final say.

### How it works under the hood

`keel add` is a convenience wrapper around `keel compile`.
For explicit control or CI/CD workflows, use `keel compile` directly:

```bash
keel compile \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --layer ./my-tools \
  --output claude=.
```

This gives you full control over layer order, output targets, and variables — useful for advanced workflows and automation.

## Onboard in 30 Seconds

A new team member clones the repo, runs one command, and has a fully configured AI agent environment:

```bash
git clone https://gitlab.example.com/team/project.git
cd project
keel compile
```

That's the entire onboarding workflow for AI tools.
No "ask Sarah for her CLAUDE.md" or "copy the config from the wiki."
The `.keel/config.yml` in the repo defines exactly what layers to compile and what agents to target.
Governance comes from the layers themselves.

Everyone gets the same setup — same permissions, same MCP servers, same instructions, same commands.
Personal preferences go in `.keel/local/` (add it to `.gitignore`), so individuals can customize without affecting the shared config.

## Make It Repeatable

For project-level config, create a `.keel/config.yml` so anyone on the team can run `keel compile` with no arguments:

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

Commit `.keel/config.yml` to your repo.
CLI flags are for quick experiments and CI overrides — the config file is the standard project workflow.

## Compile for Multiple Agents

Add agents to your config file (the `output:` section above) and every `keel compile` targets them all.
The same layers compile for any supported agent — one source, every agent.

## What's Next

- **[Community Catalog]({{< relref "community-catalog" >}})** — browse popular skills, commands, and agent configs you can add as layers.
- **[Agent Migration]({{< relref "../guide/agent-migration" >}})** — already have Cursor or Claude config? Import it and compile for any agent.
- **[Personal Toolkit]({{< relref "../guide/personal-toolkit" >}})** — assemble your global agent config from community and personal sources.
- **[Create a Layer]({{< relref "create-a-layer" >}})** — write your own `keel.yml`, stack layers, and compile shared config for your team.
- **[Enterprise Example]({{< relref "../guide/enterprise-example" >}})** — a complete 5-layer setup with governance, snippets, conditionals, and cross-layer references.
