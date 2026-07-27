---
title: "Keel + SKILL.md / Agent Skills Standard"
weight: 10
bookHidden: true
---

# Keel + SKILL.md / Agent Skills Standard

The [Agent Skills standard](https://www.agensi.io/learn/claude-code-skills-vs-cursor-rules-vs-codex-skills) (agentskills.io) defines a portable format for AI coding agent skills. A SKILL.md file with YAML frontmatter works across Claude Code, Codex, Cursor, Gemini CLI, Copilot, and others. It's the closest thing to a universal cross-agent config format — but it only covers skills.

## Where each system sits

```
┌───────────────────────────────────────────────┐
│  Agent session (Claude, Cursor, Codex, ...)   │  Both target this
├───────────────────────────────────────────────┤
│  Skills (SKILL.md files)                      │  SKILL.md standard
├───────────────────────────────────────────────┤
│  Settings, permissions, hooks, MCP servers    │  Keel only
├───────────────────────────────────────────────┤
│  Config compilation (layers, merge, locks)    │  Keel only
├───────────────────────────────────────────────┤
│  Config authoring (layers in git repos)       │  Keel only
└───────────────────────────────────────────────┘
```

SKILL.md is a file format. Keel is infrastructure. They occupy the same row (skills) but Keel covers the entire column.

## What each system does

| Concern | SKILL.md | Keel |
|---|---|---|
| Scope | Skills only (reusable agent behaviors) | Full agent config: instructions, settings, permissions, hooks, MCP servers, skills, repos, workspace |
| Format | Markdown with YAML frontmatter | Tool-agnostic DSL + native formats (including SKILL.md) |
| Cross-agent | ✅ Works across all major agents | ✅ Compiles to Claude, Cursor, Theia native formats |
| Governance | None — skills are user-managed files | Force-locks, additive-only, trust-ordered layers |
| Composition | Single files, no layering | Skills overlay from multiple layers (all contribute, last writer wins per filename) |
| Distribution | Manual install or CLI (`agensi install`) | Git repos, mounted volumes, baked into container images |
| Delivery | Files on disk | Containers or bare-metal compilation |

## Where SKILL.md is sufficient

For individual developers or small teams that need portable agent skills without organizational governance:

- Write a SKILL.md once, use it in Claude Code, Cursor, and Codex
- Share skills via repos or the Agensi registry
- No compilation step, no containers, no infrastructure

SKILL.md is the right format for the skill *content*. The question is whether you need governance, composition, and delivery around it.

## Where Keel adds value

### Governed skill delivery

SKILL.md files are user-managed. Anyone can add, modify, or delete them. There's no concept of "this skill was published by the security team and cannot be overridden."

Keel's overlay system compiles skills from ordered layers. A higher-trust layer's skill cannot be replaced by a downstream layer when force-locked. The skill file exists in the compiled output because the compiler put it there, not because the developer chose to install it.

### Skills as one part of a governed whole

A skill alone isn't useful without the MCP servers it references, the permissions it requires, and the hooks that trigger it. SKILL.md defines the skill content but not the supporting infrastructure.

Keel compiles the skill *and* its dependencies — the MCP server config, the permission grants, the hook definitions — from the same layer, as a governed unit.

### Multi-layer skill composition

A company-wide skill library, a team-specific set of skills, and project-specific skills all need to coexist. SKILL.md has no layering model — you get whatever files are on disk.

Keel's overlay strategy merges skill directories from all layers. Every layer contributes files. Filename collisions resolve by layer precedence (last writer wins, unless force-locked).

## How they work together

SKILL.md is a natural input format for Keel skill layers. The integration:

1. **Authors write SKILL.md files** following the Agent Skills standard
2. **Keel layers contain SKILL.md files** in their skill directories
3. **The compiler overlays** skill directories from all layers into the agent's skill path
4. **Force-locks** prevent downstream layers from replacing governed skills
5. **The agent discovers** SKILL.md files in its native skill directory — it never knows they were compiled

Keel doesn't replace SKILL.md — it distributes, governs, and composes SKILL.md files across organizational layers.

## What each system brings to the other

| SKILL.md brings to Keel | Keel brings to SKILL.md |
|---|---|
| A standard, portable skill format all agents understand | Governance — force-locked skills that can't be removed |
| Cross-agent compatibility without Keel's adapter layer | Multi-layer composition from company → team → project |
| Community skill ecosystem (Agensi registry, awesome-agent-skills) | Delivery infrastructure (containers, git-based freshness) |
| Adoption momentum (Claude, Codex, Cursor, Gemini, Copilot) | Co-delivery of supporting config (MCP servers, permissions, hooks) |

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
