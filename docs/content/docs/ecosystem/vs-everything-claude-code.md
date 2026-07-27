---
title: "Keel + Everything Claude Code (ECC)"
weight: 10
bookHidden: true
---

# Keel + Everything Claude Code (ECC)

[Everything Claude Code](https://github.com/anthropics/claude-code) (163K GitHub stars) is the largest open-source collection of AI coding agent configuration — 48 agents, 183 skills, rules, hooks, and security scanning packaged for Claude Code, Cursor, Codex, and OpenCode.
It's the most visible attempt at a shared configuration layer for AI coding agents.
It overlaps with Keel's "one config source, multiple agents" vision — but it's a curated collection, not a compiler.

## Where each system sits

```
┌───────────────────────────────────────────────┐
│  Agent session (Claude, Cursor, Codex, ...)   │  Both target this
├───────────────────────────────────────────────┤
│  Config files (skills, agents, rules, hooks)  │  ECC provides these
├───────────────────────────────────────────────┤
│  Config compilation (layers, merge, locks)    │  Keel only
├───────────────────────────────────────────────┤
│  Config governance (force-locks, trust)        │  Keel only
├───────────────────────────────────────────────┤
│  Container delivery (sandbox, lifecycle)      │  Keel only
└───────────────────────────────────────────────┘
```

ECC is content.
Keel is infrastructure.
ECC gives you good defaults to put in a Keel layer.

## What each system does

| Concern | ECC | Keel |
|---|---|---|
| What it is | A curated collection of config files | A compiler + container runtime |
| Cross-agent | Manual per-agent files (separate Claude/Cursor/Codex configs) | One source compiles to any agent's native format |
| Governance | None — files are user-managed | Force-locks, additive-only, trust-ordered layers |
| Composition | Static — you get what's in the repo | Multi-layer compilation from N sources |
| Customization | Fork and edit | Override in a downstream layer without touching the source |
| Distribution | Git clone / copy files | Git repos compiled at container boot |
| Delivery | Files on disk | Containers or bare-metal compilation |
| Updates | Pull from upstream, resolve conflicts | Recompile — upstream layer changes propagate automatically |

## Where ECC is sufficient

For individual developers or small teams that want a comprehensive starting point:

- Clone ECC, get 183 skills and 48 agents immediately
- Cherry-pick what you need, delete what you don't
- No compilation step, no containers, no infrastructure

ECC is the best "batteries included" starting point for AI coding agent configuration.
If you need good defaults without organizational governance, it's the right choice.

## Where Keel adds value

### Governance over community content

ECC content is user-managed.
Anyone can modify or delete any file.
There's no concept of "this security hook came from the platform team and cannot be removed."

Keel can consume ECC content *as a layer* and enforce governance on top.
A company layer force-locks the security hooks.
A team layer adds project-specific skills.
A developer cannot remove the security hooks, but can add their own skills.

### Composition without conflicts

ECC is a monorepo — if you want company-specific config alongside ECC defaults, you fork and maintain a divergent copy.
Upstream updates require conflict resolution.

Keel treats ECC as one layer among many.
Company policy is a separate layer.
Team conventions are a third.
The compiler merges them deterministically.
Upstream ECC updates don't conflict with company policy because they're in different layers.

### Cross-agent from one source

ECC maintains separate config files for each agent (Claude skills in one place, Cursor rules in another).
When you customize, you customize per-agent.

Keel compiles from a single source.
A skill written in the Keel DSL appears in Claude's skill directory *and* Cursor's rules directory after compilation — maintained once.

## How they work together

ECC is a natural source of content for Keel layers:

1. **ECC provides the content** — skills, agents, hooks, rules
2. **A Keel layer wraps ECC content** — adds `_keel` policy metadata where governance is needed
3. **Other layers add organizational config** — company policy, team conventions, project specifics
4. **The compiler merges everything** — ECC defaults + company governance + team config + project overrides
5. **The agent sees one coherent config** — it never knows the content came from multiple sources

## What each system brings to the other

| ECC brings to Keel | Keel brings to ECC |
|---|---|
| 183 skills and 48 agents as layer content | Governance over community-sourced config |
| Community curation and maintenance | Multi-layer composition without fork conflicts |
| Cross-agent config examples (how Claude/Cursor/Codex differ) | Automated cross-agent compilation from one source |
| Security scanning and hook patterns | Force-locked security hooks that users can't remove |
| A large community validating agent config patterns | Container delivery with config freshness at boot |

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
