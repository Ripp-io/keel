---
title: "Keel + Claude Code Managed Settings"
weight: 10
bookHidden: true
---

# Keel + Claude Code Managed Settings

Claude Code ships its own [managed settings system](https://code.claude.com/docs/en/settings) for enterprise governance. This is the closest overlap to Keel's configuration governance — both enforce organizational policy on Claude Code through layered config with merge semantics. The question is real: if you only use Claude Code, why not just use its native system?

## Where each system sits

```
┌───────────────────────────────────────────────┐
│  Claude Code session                          │  Both configure this
├───────────────────────────────────────────────┤
│  Managed settings (MDM / server-managed)      │  Claude native
├───────────────────────────────────────────────┤
│  Project / user / local settings              │  Claude native
├───────────────────────────────────────────────┤
│  Keel config compilation (layers, locks)      │  Keel compiles into the above
├───────────────────────────────────────────────┤
│  Config authoring (layers in git repos)       │  Keel governs
└───────────────────────────────────────────────┘
```

Keel compiles *into* Claude's native config format. The two systems are not alternatives at the same layer — Keel operates upstream and produces the files that Claude's settings system consumes.

## What each system does

| Concern | Claude Managed Settings | Keel |
|---|---|---|
| Scope | Claude Code only | Claude, Cursor, Theia (any agent with filesystem config) |
| Enforcement | Tier-based — managed scope always wins | Per-key — individual values can be force-locked while siblings remain overridable |
| Instruction composition | One CLAUDE.md per scope; scopes don't compose | Concatenates CLAUDE.md from N layers with section headers |
| Merge behavior | Arrays concatenate and dedup; objects deep-merge; scalars last-writer-wins | Six strategies: concatenate, deep-merge, overlay, additive, force-lock, env passthrough |
| Deployment | MDM (Jamf/Kandji/Intune), server-managed, or filesystem | Git repos compiled at container boot — no fleet push required |
| Delivery | Settings files deployed to endpoints | Container images or bare-metal compilation |
| Workspace setup | Not covered — settings only | Repos, init scripts, MCP servers, tmux, credentials |
| Config source format | Claude's native JSON schema | Tool-agnostic DSL or native formats |

## Where Claude's native system is sufficient

For orgs that meet all three conditions, Claude's managed settings may be enough:

1. **Claude Code only** — no Cursor, Theia, or future agent to support
2. **Settings only** — governance needed for permissions, model selection, and hooks, but not instruction composition or workspace delivery
3. **MDM exists** — the org already deploys managed preferences via Jamf, Kandji, Intune, or GPO

Claude's `managed-settings.d/` drop-in directory handles modular policy fragments well. Numeric prefixes control merge order. The system is production-tested and officially supported.

## Where Keel adds value beyond managed settings

### Cross-agent portability

Claude's managed settings are Claude-specific JSON. If the org also uses Cursor or evaluates a future agent, the governance investment doesn't transfer. Keel compiles one source to any supported agent's native format — the same force-locked MCP server appears in both `settings.json` (Claude) and `mcp.json` (Cursor).

### Instruction composition

Claude's CLAUDE.md is one file per scope. If the security team, platform team, and project each need to contribute instructions, they must coordinate on a single file or use a convention outside Claude's config system.

Keel concatenates instructions from ordered layers with section headers. Each team owns their layer. The compiler produces one CLAUDE.md that includes all contributions in trust order.

### Per-value force-locks

Claude's model is binary: managed scope wins over everything. You can't say "this one MCP server is immutable but the rest are overridable by the project." It's all-managed or all-project for a given settings key.

Keel's `_keel: { force: true }` operates per value. A security layer can force-lock a compliance scanner while leaving other MCP servers for the project to define.

### Config freshness without fleet deployment

Claude's managed settings require pushing files to endpoints via MDM or updating server-managed settings via the admin console. Config changes propagate when the MDM cycle runs or when the client polls.

Keel compiles at container boot from git repos. The security team merges a policy update to their layer repo. On next container start, the update is compiled in. No MDM push, no image rebuild.

### Workspace lifecycle

Claude's managed settings govern what the agent is *allowed to do*. They don't cover what repos should be cloned, what tools should be installed, what init scripts should run, or how the agent session should be launched. Keel's Tier 2/3 covers the full workspace lifecycle.

## How they work together

Keel doesn't replace Claude's managed settings — it compiles *into* them. The natural architecture:

1. **Keel compiles** layers from git repos into Claude-native `settings.json`, `CLAUDE.md`, hooks, MCP config
2. **Claude's settings system** loads the compiled output using its normal precedence chain
3. **MDM** (optional) deploys a minimal managed-settings layer for hard security constraints that must survive outside the container (direct API key policy, bypass-mode lockdown)

Keel handles the composition and governance. Claude's native system handles the runtime loading. They're different phases of the same pipeline.

## What each system brings to the other

| Claude Managed Settings brings to Keel | Keel brings to Claude Managed Settings |
|---|---|
| Official, supported enforcement at the agent level | Multi-layer composition with 6 merge strategies |
| MDM integration for non-containerized deployments | Cross-agent portability beyond Claude |
| Server-managed settings for cloud-administered teams | Per-value force-locks instead of tier-based override |
| Drop-in directory pattern (Keel's layer model validates this approach) | Instruction concatenation from multiple organizational layers |
| Runtime enforcement that survives outside containers | Container delivery with config freshness at boot |

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
