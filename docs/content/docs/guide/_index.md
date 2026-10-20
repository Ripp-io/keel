---
title: Guide
weight: 2
bookCollapseSection: false
---

# Guide

Feature-by-feature documentation with examples.

## Core Concepts

- **[Layers]({{< relref "layers" >}})** — stacking, formats, resolution, merge behavior
- **[DSL Reference]({{< relref "dsl" >}})** — every field in `keel.yml`
- **[Governance]({{< relref "governance" >}})** — force-locks, merge policies, MCP governance

## Configuration

- **[Conditionals & Variables]({{< relref "conditionals" >}})** — agent-specific content and variable substitution
- **[Snippets]({{< relref "snippets" >}})** — reusable text blocks across layers
- **[Agent Skills]({{< relref "agent-skills" >}})** — SKILL.md format, governance extensions, validation
- **[Validation]({{< relref "validation" >}})** — checking layers before compiling

## Workflows

- **[Personal Toolkit]({{< relref "personal-toolkit" >}})** — assemble global config from community sources
- **[Enterprise Example]({{< relref "enterprise-example" >}})** — complete 5-layer setup with all features
- **[Multi-Repo Workspaces]({{< relref "multi-repo-workspaces" >}})** — consistent config across a workspace of repos
- **[Prompt A/B Testing]({{< relref "prompt-testing" >}})** — systematically test prompt variants with variables and conditionals

## Cross-Agent

- **[Transpilation]({{< relref "transpilation" >}})** — cross-format compilation and portability
- **[Agent Migration]({{< relref "agent-migration" >}})** — switch agents without rewriting config

## Advanced

- **[Packaging]({{< relref "packaging" >}})** — distributable layers, archives, lockfile
- **[Output Lifecycle]({{< relref "output-lifecycle" >}})** — pruning, cleaning, manifest tracking
- **[Signing]({{< relref "signing" >}})** — Ed25519 layer and output signing
- **[Authoring Layer]({{< relref "authoring-layer" >}})** — give your agent Keel DSL knowledge and authoring skills
- **[Workspace Ideas]({{< relref "workspace-ideas" >}})** — community workspace image concepts
