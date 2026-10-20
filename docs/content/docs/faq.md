---
title: FAQ
weight: 5
---

# Frequently Asked Questions

### Does Keel work with my existing .claude/ or .cursor/ config?

Yes.
Existing native config directories work as Keel layers with no changes.
Keel auto-detects the format.
You can compile from native config: `keel compile --layer . --output cursor=.` transpiles Claude config to Cursor format.
See [Transpilation]({{< relref "guide/transpilation" >}}).

### Will Keel overwrite my manually created files?

No.
Keel only writes files it manages, tracked in `.keel/manifest.json`.
Manual files are preserved.
`keel clean` removes only Keel-created files.
See [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}).

### Can I use Keel without writing a keel.yml?

Yes.
Any project with native `.claude/` or `.cursor/` config works as a layer without a manifest.
`keel compile --layer . --output cursor=.` compiles your project's Claude config to Cursor format.
`keel.yml` is only required when you need layer metadata, governance, or multi-section composition.
See [Layers]({{< relref "guide/layers" >}}).

### How do I add personal overrides without affecting the team config?

Put them in `.keel/local/` and add it to `.gitignore`.
Keel auto-appends it as the highest-priority layer at project scope.
Your overrides merge last, so they take precedence over team config.
See [Layers]({{< relref "guide/layers#keellocal--personal-overrides" >}}).

### What happens when two layers define the same setting?

Depends on the section type.
Instructions concatenate (all layers contribute).
Settings deep-merge (last writer wins for scalar values, maps merge).
Permissions use configurable policy (`additive` or `replace`).
Overlays (commands/skills/agents) use last-writer-wins per file.
See [Layers]({{< relref "guide/layers#merge-behavior" >}}).

### Can I compile for multiple agents at once?

Yes.
Use `keel compile --output claude=. --output cursor=.` or list multiple outputs in `.keel/config.yml`:

```yaml
output:
  claude: .
  cursor: .
```

See [Transpilation]({{< relref "guide/transpilation" >}}).

### How do I lock a security hook so downstream layers can't override it?

Use `force: true` on the hook in your layer's JSON file:

```json
{
  "PreToolUse": [
    {
      "type": "command",
      "command": "security-check.sh",
      "force": true
    }
  ]
}
```

Earlier layers (higher trust) can force-lock values that later layers cannot override.
See [Governance]({{< relref "guide/governance" >}}).

### Does Keel require Go?

Currently yes — install via `go install gitlab.com/ripp.io/oss/agentic/keel@latest`.
Pre-built binaries are planned.
See [Getting Started]({{< relref "getting-started" >}}).

### How do I distribute a layer to my team?

Use `keel compile --distributable --name my-layer` to produce a portable layer, then package it:

```bash
keel package --from ./my-layer --format tar.gz
```

Recipients extract and reference it in their layer stack.
See [Packaging]({{< relref "guide/packaging" >}}).

### Can I test my layer before deploying it?

Yes.
Use `keel validate ./my-layer` to check for schema errors, broken references, and portability issues.
Use `keel compile --dry-run` to preview the output without writing files.
See [Validation]({{< relref "guide/validation" >}}).

### What's the difference between project scope and user scope?

Project scope (default) compiles for a specific project and includes `.keel/local/` overrides.
User scope (`--scope user`) compiles user-level config (e.g. `~/.keel/layers/`) and skips `.keel/local/`.
User scope is for global agent configuration independent of any project.
See [Personal Toolkit]({{< relref "guide/personal-toolkit" >}}).

### How do I see what Keel changed in my agent config?

Use `keel drift ./output` to detect manual changes to Keel-managed files.
Use `keel drift --semantic` to see what changed (semantic diff).
Use `keel explain ./output` to see provenance — which layer set which value.
See [Output Lifecycle]({{< relref "guide/output-lifecycle" >}}).

### Can I use variables in my layer content?

Yes.
Define variables in `keel.yml` or pass them via CLI:

```bash
keel compile -v environment=staging -v api_url=https://api.staging.example.com
```

Reference them in markdown as `${keel:environment}` and in JSON as `"${keel:environment}"`.
See [Conditionals & Variables]({{< relref "guide/conditionals" >}}).

### How do I migrate from Claude to Cursor?

Import your existing config, then compile for Cursor:

```bash
keel import --to ./keel-layer
keel compile --layer ./keel-layer --output cursor=.
```

Keel handles format translation automatically.
See [Agent Migration]({{< relref "guide/agent-migration" >}}).

### What's the relationship between CLI flags and .keel/config.yml?

Most CLI flags have corresponding config fields.
The config file provides defaults; CLI flags override them.
Execution-mode flags (`--dry-run`, `--force`, `--strict`, etc.) are CLI-only.
Three environment variables (`CONFIG_LAYERS`, `USER_CONFIG`, `USER_PRIORITY`) provide fallback defaults for select settings.
See [Config File Reference]({{< relref "reference/config" >}}).
