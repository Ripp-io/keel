---
title: "Keel + ServiceNow Build Agent"
weight: 10
bookHidden: true
---

# Keel + ServiceNow Build Agent

[ServiceNow Build Agent](https://newsroom.servicenow.com/press-releases/details/2026/ServiceNow-Build-Agent-now-works-inside-every-major-AI-coding-tool-governed-by-default/default.aspx) (GA May 2026) is an AI coding agent that works inside Cursor, Windsurf, Claude Code, and GitHub Copilot with enterprise governance. It's the closest *commercial* product to Keel's "governed agent across multiple tools" vision — but it solves the problem from the opposite direction.

## Where each system sits

```
┌───────────────────────────────────────────────┐
│  IDE (Cursor, VS Code, Claude Code)           │  Both target this
├───────────────────────────────────────────────┤
│  Agent governance (audit, compliance, gates)  │  ServiceNow platform
├───────────────────────────────────────────────┤
│  Agent configuration (instructions, tools)    │  Keel compiles
├───────────────────────────────────────────────┤
│  Application platform (ServiceNow)            │  ServiceNow only
├───────────────────────────────────────────────┤
│  Config authoring (layers in git repos)       │  Keel only
└───────────────────────────────────────────────┘
```

ServiceNow governs from above (platform governance over applications). Keel governs from below (configuration compilation into agent-native formats).

## What each system does

| Concern | ServiceNow Build Agent | Keel |
|---|---|---|
| Model | SaaS platform — agent runs within ServiceNow's governance layer | Open-source infrastructure — compiler + containers you run yourself |
| Cross-tool | Works inside Cursor, Windsurf, Claude Code, Copilot | Compiles config for Claude, Cursor, Theia |
| Governance scope | Application lifecycle: deployment approvals, release management, compliance | Configuration composition: instructions, permissions, hooks, MCP servers, model selection |
| What it governs | Applications built *on ServiceNow* | AI agent workspaces for *any* codebase |
| Custom policy | Custom Instructions for development standards | `_keel` metadata with force-locks, merge strategies, trust-ordered layers |
| Enforcement | Platform-level — apps inherit governance when deployed to ServiceNow | Compiler-level — force-locked values are eliminated before the agent sees config |
| Quality gates | Self-healing test loop validates generated code | Hooks, pre-commit integration, CI runner with structured output |
| Cost | Enterprise SaaS license (included with ServiceNow AI Platform) | Open-source, self-hosted |

## The fundamental difference

ServiceNow Build Agent governs **applications built for the ServiceNow platform**. If you're building ServiceNow apps, Build Agent gives you governed AI assistance inside your preferred IDE. The governance comes from the platform — audit trails, deployment approvals, compliance — because the application ultimately runs on ServiceNow.

Keel governs **AI agent configuration for any codebase**. It doesn't care what you're building — it cares how the AI coding agent is configured while you build it. The governance is on the agent's workspace, not the application's lifecycle.

These are different problems:

- ServiceNow: "Ensure AI-generated ServiceNow apps meet our compliance standards"
- Keel: "Ensure every developer's AI agent has the right instructions, permissions, and tools — regardless of what they're building"

## Where ServiceNow Build Agent is the right choice

For teams building on the ServiceNow platform:

- Applications inherit enterprise governance (audit, compliance, deployment) automatically
- Build Agent has deep platform context — it understands ServiceNow's APIs, data model, and patterns
- The SDK brings this context into any IDE, not just ServiceNow Studio
- AEMC (App Engine Management Center) provides lifecycle governance at no additional cost

If your org builds on ServiceNow, Build Agent's platform-native governance is stronger than anything Keel could provide for that specific domain.

## Where Keel adds value

### Platform independence

ServiceNow Build Agent governs applications built *for ServiceNow*. If you're building microservices, data pipelines, frontend apps, or infrastructure — anything outside the ServiceNow ecosystem — Build Agent's governance doesn't apply.

Keel governs the agent workspace regardless of what's being built. The same config layers apply whether the developer is writing a Spring Boot service, a React frontend, or a Terraform module.

### Configuration composition

ServiceNow offers Custom Instructions for encoding development standards. This is a single layer — the organization's standards baked into Build Agent.

Keel compiles from N layers with different trust levels, merge strategies, and force-locks. Company security policy, org-level tooling standards, team conventions, and project specifics all compose into one governed environment.

### Self-hosted, open-source

Build Agent is SaaS. The agent, governance, and platform are ServiceNow-operated. For organizations with air-gapped environments, data sovereignty requirements, or preference for self-hosted infrastructure, Keel runs entirely on your own compute.

### Container-native delivery

Keel's container model (base → dev → runner) provides isolated, reproducible agent environments. Build Agent runs as an SDK inside existing IDEs — useful for its reach, but without the sandbox isolation that containers provide.

## Could they complement each other?

Possibly, in a narrow scenario: an org that builds ServiceNow apps *and* other software. Build Agent governs the ServiceNow work. Keel governs the rest. But the governance models are different enough that unifying them would be forced.

The more honest assessment: these are different tools for different contexts. ServiceNow Build Agent is a platform-specific governed agent. Keel is platform-agnostic configuration infrastructure. An org would use one or the other based on what they're building, not combine them.

## What each system brings

| ServiceNow Build Agent | Keel |
|---|---|
| Platform-native governance for ServiceNow apps | Platform-agnostic config governance for any codebase |
| Cross-IDE reach via SDK (Cursor, Windsurf, Claude Code, Copilot) | Cross-agent compilation (Claude, Cursor, Theia) |
| Application lifecycle management (deploy, release, compliance) | Configuration lifecycle management (layers, merge, compile) |
| Deep ServiceNow platform context and intelligence | Deep agent config format knowledge (adapters) |
| Enterprise SaaS with managed infrastructure | Open-source, self-hosted, container-native |

## What's Next

- **[All Comparisons]({{< relref "comparisons" >}})** — how Keel relates to every tool in this space.
- **[Governance]({{< relref "../guide/governance" >}})** — how Keel's force-lock system works.
- **[Getting Started]({{< relref "../getting-started" >}})** — try Keel in two minutes.
