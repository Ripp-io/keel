---
title: "Keel + chezmoi"
weight: 10
bookHidden: true
---

# Keel + chezmoi

[chezmoi](https://www.chezmoi.io/) is a dotfile manager that developers increasingly use to sync AI coding agent configuration (`.claude/`, `CLAUDE.md`, `.cursor/`) across machines.
It's the most common "why not just use chezmoi?" question when evaluating Keel.

## Where each system sits

```
                chezmoi                               Keel
                ───────                               ────
Purpose         Sync dotfiles across machines          Compile layered config for agents
Config model    Template + sync (1 source → N hosts)   Multi-layer merge (N sources → 1 output)
Governance      None                                   Per-key force-locks with provenance
Cross-agent     Manual (separate templates per agent)  Automatic (one source, any agent output)
Team sharing    Fork the dotfiles repo                 Compose team + project + personal layers
Output          Symlinks or copies of source files     Agent-native compiled config
```

## The key difference: sync vs compile

chezmoi **syncs** your dotfiles to multiple machines.
You have one source repo and chezmoi ensures every machine has the same files.

Keel **compiles** multiple config sources into one output.
You have company + team + project + personal layers and Keel merges them with deterministic strategies.

```
chezmoi:   1 source repo  ──sync──►  machine A, machine B, machine C
                                     (same files everywhere)

Keel:      company layer ─┐
           team layer ────┤─compile─► agent-native output
           project layer ─┤           (merged, governed, translated)
           personal layer ┘
```

## What each system does well

| Capability | chezmoi | Keel |
|---|---|---|
| Sync dotfiles across machines | ✅ Core value prop | ❌ Not a sync tool |
| Go templates in config files | ✅ Rich templating | ✅ Variables, conditionals, snippets |
| Cross-platform (macOS/Linux/Windows) | ✅ Full support | ⚠️ Runs anywhere Go runs |
| Multi-layer config composition | ❌ Single source | ✅ N layers with merge strategies |
| Governance / force-locks | ❌ | ✅ Per-key locks with provenance |
| Cross-agent compilation | ❌ Manual per-agent templates | ✅ One source → Claude, Cursor, etc. |
| Team config inheritance | ❌ Fork or copy | ✅ Layer stacking with override rules |
| Remote layers (Git, HTTP) | ❌ (chezmoi externals are partial) | ✅ Full remote layer support |
| Agent Skills validation | ❌ | ✅ Agent Skills spec compliance |

## When to use which

**chezmoi** is the right choice if:
- You're a solo developer syncing your personal config across machines
- You only use one AI agent and don't need cross-agent portability
- You don't need governance or team config inheritance
- Your config is simple enough that templates handle it

**Keel** is the right choice if:
- You need to compose config from multiple sources (team, project, personal)
- You use multiple agents or want agent portability
- You need governance (force-locks, additive permissions, MCP policies)
- You pull community config as remote layers

**Both together** is a valid pattern:
- chezmoi syncs your *personal* Keel layer across machines
- Keel compiles your personal layer + team/project layers into agent-native output
- chezmoi handles the "same config on every machine" problem
- Keel handles the "merge multiple sources with governance" problem

## The community pattern

Developers are already using chezmoi to manage `CLAUDE.md`, `.claude/settings.json`, and skills across machines ([Dr. Mowinckel](https://drmowinckels.io/blog/2026/dotfiles-coding-agents/), [Waki285](https://zenn.dev/waki285/articles/chezmoi-dotfiles?locale=en)).
Some have built Go scripts to auto-generate permission configs from YAML data files.

This works well for personal use.
It breaks down when team config needs to compose with personal config, or when governance is required, or when the same config needs to target multiple agents.
That's where Keel picks up.

Sources:
- [chezmoi.io](https://www.chezmoi.io/)
- [Dr. Mowinckel: Dotfiles + AI Coding Agents](https://drmowinckels.io/blog/2026/dotfiles-coding-agents/)
- [Waki285: chezmoi + AI Agent Config](https://zenn.dev/waki285/articles/chezmoi-dotfiles?locale=en)

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
