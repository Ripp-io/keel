---
title: Features
weight: 1.5
bookToC: false
---

# Features

Everything Keel does — at a glance.

---

<div class="features-section">

## Configuration

| Feature | What it does | Learn more |
|---------|-------------|------------|
| **Layer stacking** | Stack config from community → company → team → project → user with deterministic merge | [Layers]({{< relref "guide/layers" >}}) |
| **6 merge strategies** | Concatenate, deep-merge, overlay, force-lock, additive, replace — per feature type | [Merge Strategies]({{< relref "architecture/merge-strategies" >}}) |
| **4 layer formats** | Keel DSL, Claude native, Cursor native, AGENTS.md — auto-detected from marker files | [Layers]({{< relref "guide/layers" >}}) |
| **Remote layers** | Git repos (HTTPS/SSH), HTTP archives (tar.gz, zip), with local caching and offline mode | [Layers]({{< relref "guide/layers#remote-layers" >}}) |
| **Auto-discovery** | Zero-config when conventions are followed — `.keel/config.yml`, output target inference | [Layers]({{< relref "guide/layers#auto-discovery" >}}) |
| **User scope** | Compile to user-level paths for global agent config | [Personal Toolkit]({{< relref "guide/personal-toolkit" >}}) |

## Templating

| Feature | What it does | Learn more |
|---------|-------------|------------|
| **Variables** | `${keel:name}` substitution from layer-defined values, merged across the stack | [Conditionals & Variables]({{< relref "guide/conditionals" >}}) |
| **Conditionals** | `<!-- keel-if: expr -->` blocks for agent-specific or scope-specific content | [Conditionals & Variables]({{< relref "guide/conditionals" >}}) |
| **Snippets** | Reusable text blocks included across files and layers with `<!-- keel:include name -->` | [Snippets]({{< relref "guide/snippets" >}}) |
| **Parameterized snippets** | Key-value parameters on includes: `<!-- keel:include name key="value" -->` | [Snippets]({{< relref "guide/snippets" >}}) |
| **JSON conditions** | `_keel.condition` expressions target JSON objects to specific agents | [Conditionals & Variables]({{< relref "guide/conditionals#json-conditionals" >}}) |
| **A/B testing** | Compile prompt variants with `--var`, run both against the same task, compare results | [Prompt A/B Testing]({{< relref "guide/prompt-testing" >}}) |
| **Auto-injected variables** | `keel.agent` and `keel.scope` always available without defining them | [Conditionals & Variables]({{< relref "guide/conditionals#auto-injected-variables" >}}) |

## Governance

| Feature | What it does | Learn more |
|---------|-------------|------------|
| **Per-key force-locks** | Lock any individual value — a setting, hook, MCP server, file — so downstream can't override | [Governance]({{< relref "guide/governance" >}}) |
| **Additive permissions** | Union-only merge — downstream layers can add permissions but never remove | [Governance]({{< relref "guide/governance#merge-policies" >}}) |
| **MCP governance** | Allow/deny lists for MCP servers, force-lock individual servers | [Governance]({{< relref "guide/governance#mcp-governance" >}}) |
| **File-level policies** | Lock specific files or directories via the `files` map in `keel.yml` | [DSL Reference]({{< relref "guide/dsl#files" >}}) |
| **Distributable layers** | Compiled output that preserves governance metadata for supply-chain distribution | [Packaging]({{< relref "guide/packaging" >}}) |
| **Provenance tracking** | Compiled manifest records which layer set every value and every lock | [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}) |
| **Lockfile** | SHA-256 content hashes for reproducible, verifiable builds in CI | [Packaging]({{< relref "guide/packaging#lockfile" >}}) |
| **Supply chain security** | Lockfile verification, provenance tracking, and force-locks prevent config tampering | [Governance]({{< relref "guide/governance#config-supply-chain-security" >}}) |
| **Policy distribution** | Central teams publish governed layers via remote refs; downstream teams pull with `keel compile` | [Governance]({{< relref "guide/governance#enterprise-policy-distribution" >}}) |
| **Ed25519 signing** | Sign layers and compiled output — cryptographic proof of authorship and provenance | [Signing]({{< relref "guide/signing" >}}) |

## Cross-Agent

| Feature | What it does | Learn more |
|---------|-------------|------------|
| **Multi-agent output** | Compile once for Claude, Cursor, and more with `--output agent=path` | [Layers]({{< relref "guide/layers" >}}) |
| **Cross-format compilation** | Any layer format compiles to any agent via intermediate representation | [Transpilation]({{< relref "guide/transpilation" >}}) |
| **Agent migration** | Import existing config, compile for a new agent — switch tools without rewriting | [Agent Migration]({{< relref "guide/agent-migration" >}}) |
| **Import** | `keel import` converts native Claude/Cursor config to a Keel DSL layer | [Transpilation]({{< relref "guide/transpilation#quick-examples" >}}) |
| **Automatic translation** | Hook keys (PascalCase ↔ camelCase), permission formats, instruction wrapping | [Transpilation]({{< relref "guide/transpilation" >}}) |
| **Portability linting** | `keel validate --lint` detects non-portable agent-specific patterns | [Validation]({{< relref "guide/validation" >}}) |
| **Context budget** | `keel validate` estimates token counts and warns on skill bloat, tool sprawl, and instruction growth | [Validation]({{< relref "guide/validation" >}}) |
| **14 agents tracked** | Compatibility matrix covering CLI, IDE, and platform agents | [Agent Support]({{< relref "ecosystem/agents" >}}) |

## DSL Surface

| Feature | What it does | Learn more |
|---------|-------------|------------|
| **Instructions** | Markdown files concatenated across layers with section headers | [DSL Reference]({{< relref "guide/dsl#instructions" >}}) |
| **Permissions** | Allow/deny rules with configurable merge policy | [DSL Reference]({{< relref "guide/dsl#permissions" >}}) |
| **MCP servers** | Server definitions with governance policy | [DSL Reference]({{< relref "guide/dsl#mcp" >}}) |
| **Hooks** | Event-driven lifecycle handlers with force-lock support | [Hooks Reference]({{< relref "reference/hooks" >}}) |
| **Commands** | Slash command files, overlaid across layers | [DSL Reference]({{< relref "guide/dsl#commands" >}}) |
| **Agents** | Sub-agent definition files | [DSL Reference]({{< relref "guide/dsl#agents" >}}) |
| **Skills** | Skill directories with SKILL.md and companion files | [Agent Skills]({{< relref "guide/agent-skills" >}}) |
| **Settings** | Model, sandbox, env vars, worktree, auto-mode, attribution | [Settings Reference]({{< relref "reference/settings" >}}) |
| **Repos** | Repository definitions for workspace initialization | [DSL Reference]({{< relref "guide/dsl#repos" >}}) |
| **Multi-repo workspaces** | Consistent config across a workspace of repos with per-repo overrides | [Multi-Repo Workspaces]({{< relref "guide/multi-repo-workspaces" >}}) |
| **Bin** | Portable scripts shipped with layers, overlaid to `.keel/bin/` | [DSL Reference]({{< relref "guide/dsl#bin" >}}) |
| **Deps** | External binary dependencies — install or download with SHA-256 verification | [DSL Reference]({{< relref "guide/dsl#deps" >}}) |

## CLI

| Command | What it does | Learn more |
|---------|-------------|------------|
| `keel compile` | Compile layers into agent-native output with optional `--stats` metrics | [CLI Reference]({{< relref "reference/cli" >}}) |
| `keel validate` | Check layers for errors, warnings, and portability issues | [Validation]({{< relref "guide/validation" >}}) |
| `keel explain` | Show which layer produced each output value | [CLI Reference]({{< relref "reference/cli" >}}) |
| `keel import` | Convert native agent config to Keel DSL | [Transpilation]({{< relref "guide/transpilation" >}}) |
| `keel init` | Scaffold a new layer interactively | [Create a Layer]({{< relref "getting-started/create-a-layer" >}}) |
| `keel query` | Query compiled manifest metadata | [CLI Reference]({{< relref "reference/cli" >}}) |
| `keel cache` | Manage cached remote layers | [CLI Reference]({{< relref "reference/cli" >}}) |
| `keel clean` | Remove compiled output, preserving user files | [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}) |
| `keel drift` | Detect manual changes to compiled output | [CLI Reference]({{< relref "reference/cli#keel-drift" >}}) |
| `keel package` | Create tar.gz or zip archive | [Packaging]({{< relref "guide/packaging" >}}) |
| `keel global list` | List globally installed dependencies with consumer counts | [CLI Reference]({{< relref "reference/cli#keel-global" >}}) |
| `keel global clean` | Remove orphaned global dependencies | [CLI Reference]({{< relref "reference/cli#keel-global" >}}) |
| `keel key` | Generate, list, export, trust, and revoke Ed25519 signing keys | [CLI Reference]({{< relref "reference/cli#keel-key" >}}) |
| `keel sign` | Sign a layer with an Ed25519 key | [CLI Reference]({{< relref "reference/cli#keel-sign" >}}) |
| `keel verify` | Verify signatures on layers or compiled output | [CLI Reference]({{< relref "reference/cli#keel-verify" >}}) |

## Output Management

| Feature | What it does | Learn more |
|---------|-------------|------------|
| **Automatic pruning** | Stale files from previous compilations are removed automatically | [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}) |
| **User-file preservation** | Files not created by Keel are never touched | [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}) |
| **Overlay provenance** | Tracks which layers wrote which files, reports shadowed files | [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}) |
| **Compilation manifest** | Records every input, output, strategy, and lock for full auditability | [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}) |
| **Drift detection** | `keel drift` compares disk state against the manifest to find manual edits | [CLI Reference]({{< relref "reference/cli#keel-drift" >}}) |
| **Signature verification** | `keel drift --verify` checks manifest signatures alongside content drift | [CLI Reference]({{< relref "reference/cli#keel-drift" >}}) |
| **CI strict mode** | `--strict` flag makes instruction conflicts a hard error for CI pipelines | [CLI Reference]({{< relref "reference/cli#keel-compile" >}}) |
| **Rollback** | `keel clean` + recompile from a previous layer version for instant behavioral rollback | [Output Lifecycle]({{< relref "guide/output-lifecycle#rollback" >}}) |

</div>
