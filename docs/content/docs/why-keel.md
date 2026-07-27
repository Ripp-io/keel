---
title: Why Keel
weight: 1.2
bookToC: false
---

# Why Keel

AI coding agents are transforming software development — but deploying them at scale is where most organizations stall.

## The Problem Is Real

- **88% of agent pilots never reach production** — the blockers are infrastructure and governance, not model quality ([Deloitte State of AI 2026](https://www.deloitte.com/us/en/what-we-do/capabilities/applied-artificial-intelligence/content/state-of-ai-in-the-enterprise.html))
- **Only 17% of enterprises** have formal AI governance frameworks ([McKinsey](https://www.mckinsey.com/capabilities/quantumblack/our-insights/the-state-of-ai))
- **82% have discovered shadow AI agents** that security and IT didn't know about ([Cloud Security Alliance](https://cloudsecurityalliance.org/blog/2026/04/28/the-shadow-ai-agent-problem-in-enterprise-environments))
- **65% experienced an AI agent security incident** in the past year with real business impact ([Security Boulevard](https://securityboulevard.com/2026/05/the-shadow-ai-governance-crisis-why-80-of-fortune-500-companies-have-already-lost-control-of-their-ai-infrastructure/))
- **29% of employees** use unsanctioned agents for work tasks ([Cloud Security Alliance](https://cloudsecurityalliance.org/blog/2026/04/28/the-shadow-ai-agent-problem-in-enterprise-environments))

The agents work.
The infrastructure to deploy, configure, and govern them doesn't exist yet.

## Compliance Frameworks Expect Auditability

Organizations running AI agents at scale face audit requirements from existing compliance frameworks — SOC 2 change management (CC8), HIPAA tamper-evident audit trails, SEC 17a-4 record retention.
These frameworks require demonstrating what configuration an agent was running under, who approved it, and what changed.

Keel's compiled manifest records full provenance — which layer set which value, which locks were applied, who published each layer.
`keel explain` surfaces this at the command line.
`keel report` (planned) generates audit-ready artifacts from this data.

[Gartner](https://www.gartner.com/) forecasts AI governance spending at $492M in 2026, surpassing $1B by 2030 — driven primarily by operational risk at scale, not any single regulation.

## What Makes Keel Different

No other tool combines these three things:

### 1. Compile-Time Config Composition

Six merge strategies turn a stack of config layers into one deterministic output.
Company → org → team → project → user, compiled into the agent's native format.
No symlinks, no copy-paste, no drift.

### 2. Per-Value Governance with Provenance

Force-lock any individual value — a settings key, a hook, an MCP server, a permission rule.
Not binary tier-level override.
Per-key granularity with full provenance tracking: which layer locked what, auditable in the compiled manifest.

### 3. Cross-Agent Portability

One `keel.yml` compiles to Claude Code, Cursor, and more.
An intermediate representation handles format translation — hook key casing, permission formats, instruction wrapping.
As multi-agent stacks normalize, this becomes essential infrastructure.

Switching agents is a flag change, not a rewrite.
`keel import` captures your existing config; `keel compile --output <new-agent>=.` produces the new format.
See [Agent Migration]({{< relref "guide/agent-migration" >}}).

## The Landscape

Several tools solve pieces of the puzzle.
None combine compile-time composition, governance, and cross-agent portability.

| Tool | What it does | Gap vs Keel |
|---|---|---|
| **Claude Managed Settings** | Server-managed governance for Claude Code | Claude-only, binary tier override, no instruction composition |
| **KiloClaw for Organizations** | Enterprise managed platform for AI agents | Platform-first, SaaS-only, no config compilation |
| **Microsoft AGT** | Open-source runtime governance (OPA/Rego) | Runtime-only, no compile-time config, no cross-agent |
| **ServiceNow Build Agent** | AI coding agent with enterprise governance | ServiceNow platform-specific, SaaS-only |
| **DevContainers** | Containerized dev environments | No agent awareness, no config governance |
| **E2B / Daytona** | Sandboxed execution environments | Config-unaware — solve isolation, not configuration |

See [Comparisons]({{< relref "ecosystem/comparisons" >}}) for detailed analysis of each.

## When Does Keel Become Worth It?

If you're a solo developer on one project with one AI tool, your `CLAUDE.md` works fine.
Keel's value emerges at specific inflection points:

| Inflection | Trigger | What breaks without Keel |
|---|---|---|
| **Scale** | 3+ projects | Copy-pasting config between repos, drift between setups |
| **Team** | 2+ people sharing config | "Just copy my .claude/" stops working, no governance |
| **Governance** | Security or compliance needs | No way to enforce rules that individuals can't override |
| **Multi-agent** | Using Claude AND Cursor | Maintaining two configs that should say the same thing |
| **Onboarding** | New team members | "How do I set up my AI tool?" becomes a 2-hour walkthrough |
| **CI/CD** | Agents in pipelines | Need reproducible, headless, governed agent environments |
| **Multi-repo** | Project spans 3+ repos | Agent needs context across repos, manual setup per repo |

## "Why Not Just Use Native Config?"

| Objection | Response |
|---|---|
| "My CLAUDE.md works fine" | It does. Keel's value starts when you need to share it, govern it, or compile it to another tool. |
| "I'll just commit .claude/ to my repo" | That works for one repo, one tool, one team. It breaks at scale, multi-repo, multi-tool, or governance. |
| "I don't use containers" | Use the compiler standalone. `keel compile` works on bare metal. Containers are optional. |
| "This adds complexity" | It does. The question is whether the complexity you're replacing — manual config management, copy-paste, no governance — is worse. |
| "What if Claude/Cursor change their config format?" | Keel absorbs the breaking change in the compiler. Your config stays the same. |
| "I don't need governance" | Today. When your org adopts AI at scale and someone's agent accesses production data, you'll wish you had force-locks. |

## Who Uses Keel

### Write once, target any agent
**Roles**: Solo developer, senior engineer, platform engineer, enterprise architect

Define your agent config once, compile to Claude, Cursor, or any future tool.
Stop maintaining parallel configs.
Value starts when you use more than one agent — or want the freedom to switch later.

### Governed AI at scale
**Roles**: Security engineer, staff/principal engineer, VP/CTO, compliance officer, enterprise architect

Enforce policies that downstream teams can't override.
Sandboxed execution.
Full audit trail — which layer set which value, provenance in the compiled manifest.
This is where regulated industries (financial services, healthcare, defense) find the most urgency.

### Standardized team environments
**Roles**: Tech lead, engineering manager, DevOps/platform engineer, new hires

Same setup for everyone.
New developer runs one command, gets repos cloned, tools configured, AI agent ready.
Ten minutes to productive, not two hours.

### Context isolation
**Roles**: Full-stack developer, consultant, freelancer

Different workspaces per project, client, or stack.
Switch contexts without reconfiguring your agent.
Project A's API keys never exist in Project B's environment.

### Agents in CI/CD pipelines
**Roles**: DevOps engineer, QA engineer, SRE

Headless, governed agents for automated code review, test generation, and security scanning.
Reproducible builds with lockfile verification.
Credentials injected at runtime, never baked in.

### Portable skill development
**Roles**: Skill developers, open source authors, marketplace publishers

Write skills once, validate against the [Agent Skills spec](https://agentskills.io), test across agents.
Use templating (variables, conditionals) to make skills adapt per-project.
`keel validate` checks portability before you publish.

## Open Source, Self-Hosted

Keel is a CLI compiler — not a platform, not a SaaS, not an agent.
Config stays in Git.
Changes follow your PR process.
No vendor lock-in, no admin console, no MDM dependency.

For organizations that can't use SaaS governance or vendor-specific solutions, Keel is the self-hosted, vendor-neutral option.

<a class="landing-cta" href="{{< relref "getting-started" >}}">Get Started →</a>

## What's Next

- **[Features]({{< relref "features" >}})** — everything Keel does, at a glance.
- **[Getting Started]({{< relref "getting-started" >}})** — install, pull community config, compile.
- **[Agent Migration]({{< relref "guide/agent-migration" >}})** — switch agents without rewriting config.
- **[Multi-Repo Workspaces]({{< relref "guide/multi-repo-workspaces" >}})** — consistent config across a workspace of repos.
- **[Enterprise Example]({{< relref "guide/enterprise-example" >}})** — a complete 5-layer setup showing all features in context.
- **[Comparisons]({{< relref "ecosystem/comparisons" >}})** — how Keel relates to other tools in this space.

