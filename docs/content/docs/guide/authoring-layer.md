---
title: Authoring Layer
weight: 6.5
---

# Authoring Layer

Keel ships a configuration layer that teaches your AI agent how to write Keel config.
Pull it into any compilation and your agent gets DSL knowledge, skill authoring guidance, governance patterns, and templating recipes — all at zero context cost until invoked.

## Quick start

```bash
keel compile \
  --layer 'https://gitlab.com/ripp.io/oss/agentic/keel.git#main:keel' \
  --layer ./my-project \
  --output claude=.
```

Or add it as a permanent layer in your config:

```yaml
# .keel/config.yml
layers:
  - 'https://gitlab.com/ripp.io/oss/agentic/keel.git#main:keel'
  - .
```

## What it provides

### Instructions

A condensed DSL quick reference is compiled into your agent's instructions.
It covers keel.yml structure, merge strategies, auto-injected variables, conditional syntax, snippet usage, and CLI commands — everything an agent needs to author Keel config without reading the full documentation.

### Skills

| Skill | Purpose |
|-------|---------|
| `/keel-layer` | Create a complete layer — keel.yml, companion files, validation, testing |
| `/keel-skill` | Write an Agent Skills-compliant SKILL.md with proper frontmatter |
| `/keel-governance` | Add force-locks, merge policies, and trust hierarchies |
| `/keel-templating` | Variables, conditionals, snippets, and A/B testing patterns |

Skills are invoked on demand — they don't consume context until called.

## When to use

- You're building Keel layers and want your agent to know the DSL
- You're authoring skills and want Agent Skills spec compliance guidance
- You're setting up governance and need the force-lock patterns
- You're onboarding a team and want agents to help write config

## What's Next

- **[Layers]({{< relref "layers" >}})** — how layers stack, merge, and resolve.
- **[Agent Skills]({{< relref "agent-skills" >}})** — the SKILL.md format reference.
- **[Governance]({{< relref "governance" >}})** — force-locks and merge policies.
- **[Conditionals & Variables]({{< relref "conditionals" >}})** — templating reference.
