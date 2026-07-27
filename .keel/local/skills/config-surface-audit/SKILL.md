---
name: config-surface-audit
description: Audit what an AI coding agent reads from the filesystem and compare against what Keel's DSL and compiler currently support. Writes public coverage tables to the doc site and internal bugs/recommendations to local/. For new agents, also adds them to the compatibility matrix. Use when checking for new config features in agent updates, verifying adapter completeness, evaluating a new agent, or preparing to add a new adapter.
---

# Config Surface Audit

Discover every configuration file, directory, and settings key that a target AI coding agent reads from the filesystem, then compare against what Keel currently compiles.
Produces a structured gap table, an updated or new schema document, and actionable recommendations.

## When to use

- An agent shipped a new version and you want to check if Keel covers the new config features.
- You're evaluating a new agent for Keel support and need to add it to the compatibility matrix.
- You're about to build a new compiler adapter and need a complete map of the target agent's config surface.
- Periodic audit to ensure Keel's DSL stays current with supported agents.

## Inputs

- **Agent name** — which agent to audit (e.g., `claude`, `cursor`, `gemini`).
- **Scope** — `full` (everything) or `delta` (only what changed since last audit). Default: `full`.

## Step 0: Check the cross-agent matrix

Check `docs/content/docs/reference/compatibility-matrix.md` — the single source of truth for all cross-agent portability claims.
It contains the complete file inventory, hook events, handler types, frontmatter specs, settings keys, and portability annotations.
`portability.go` is derived from the matrix and verified by `TestPortabilityMatchesMatrix`.

Use the matrix as the baseline for the audit.
Compare the matrix against the agent's current documentation to find deltas.

If the agent is not yet in the matrix, this audit will produce the data needed to add it.

## Step 1: Identify authoritative sources

Start with the agent's official documentation.
Record every source URL in a **Sources** table at the top of the schema doc.
Typical sources:

- Official docs site (settings, skills, hooks, agents, MCP, plugins, commands)
- JSON schema if one exists (e.g., `json.schemastore.org`)
- Agent Skills spec at agentskills.io for the skills directory standard
- The agent's GitHub/GitLab repo for source-of-truth on undocumented config
- Community guides for practical patterns (flag as lower confidence)

The sources table enables future audits to re-check the same URLs for changes.

Search queries to try:
- `"{agent name} configuration files"`
- `"{agent name} settings.json reference"`
- `"{agent name} MCP configuration"`
- `"{agent name} hooks reference"`
- `"{agent name} changelog {current year}"`
- `"{agent name} CLI configuration"`

## Step 2: Inventory every config file

Walk the agent's filesystem contract — every file and directory it reads.
For each file, document:

- **Path(s)** with precedence order (managed → user → project → local)
- **Format** (JSON, YAML, markdown, plain text)
- **Required or optional**
- **Structure** — top-level keys, nested objects, field types
- **Merge behavior** — how multiple instances combine (concatenation, deep-merge, last-wins, no merge)
- **Keel handling status** — handled, partially handled, not handled (by design), or gap

Group files into an inventory list at the top of the schema doc with anchor links to each section.

## Step 3: Document frontmatter for markdown files

For every markdown file type the agent reads (skills, agents, commands, rules), list every supported frontmatter field in a table with:

- Field name
- Type (string, boolean, array, object)
- Required or optional
- Default value
- Description
- **Portable** column — whether the field works across agents

Use these portability tags:

| Tag | Meaning |
|-----|---------|
| **Portable** | Works across all supported agents |
| **Claude-only** | Claude Code specific — silently ignored by others |
| **Cursor-only** | Cursor specific — no Claude equivalent |
| **Format differs** | Same concept, different syntax per agent (Keel translates) |
| **Runtime** | Not authored config — generated at runtime |

## Step 4: Document body-level patterns

Some agents support variable substitution or dynamic injection in markdown bodies.
These are not portable across agents and cannot be automatically translated.
Document each pattern with which agent(s) support it.

## Step 5: Document settings keys

For the agent's main settings file (e.g., `settings.json`, `cli-config.json`), list every key with its type and purpose.
Group keys by category (permissions, sandbox, hooks, MCP, UI, etc.).
Add a portability summary at the top of the settings section noting which key groups are portable, format-differs, or agent-specific.

### Settings key audit checklist

When auditing settings, check these categories:

- [ ] Permissions (allow/deny/ask)
- [ ] Hooks (event names, handler types)
- [ ] MCP control (allowedMcpServers, deniedMcpServers, enableAllProjectMcpServers)
- [ ] Sandbox (enabled, filesystem, network)
- [ ] Environment variables (env map)
- [ ] Model selection
- [ ] Worktree (baseRef, symlinks, sparse)
- [ ] Skill overrides (on/off/name-only)
- [ ] Auto mode (environment, allow, soft_deny)
- [ ] Attribution (commit, PR)
- [ ] UI/display settings (outputStyle, language, viewMode)
- [ ] Plugin/marketplace controls
- [ ] Managed policy flags

For each key, determine whether it warrants a dedicated DSL field or is adequately served by passthrough.

## Step 6: Verify completeness against machine-readable sources (MANDATORY)

Documentation pages are curated and skip niche keys.
This step is mandatory — it catches keys that documentation pages omit.

**Settings keys:**
- Fetch the agent's JSON schema if one exists (e.g., `json.schemastore.org`).
- Extract every top-level property name.
- Diff against the settings keys listed in the schema doc.
- Every key in the JSON schema must appear in the doc.
  If a key is absent, add it — it was missed during documentation research.

**Verification command pattern** (adapt per agent):
```bash
# Fetch JSON schema, extract top-level keys, diff against schema doc
curl -sL <schema-url> | jq '.properties | keys[]' -r | sort > /tmp/schema-keys.txt
grep '^\- `' docs/content/docs/reference/compatibility-matrix.md | sed 's/^- `//;s/`.*//' | sort -u > /tmp/doc-keys.txt
comm -23 /tmp/schema-keys.txt /tmp/doc-keys.txt  # Keys in schema but not in doc
```

**Frontmatter fields:**
- Check the agent's source code or plugin system for field parsing logic.
- Search the agent's GitHub repo for frontmatter field names (grep for YAML unmarshal targets).
- Cross-reference the Agent Skills spec at agentskills.io for base fields.

**Config files:**
- Search the agent's source code for file path constants (e.g., `settings.json`, `CLAUDE.md`).
- Check for recently added files mentioned in changelogs but not yet in docs.

Any keys found in the machine-readable source but missing from the doc are gaps.
Add them before proceeding.

## Step 7: Cross-reference with Keel's coverage

Read these files to understand Keel's current coverage:

- `pkg/schema/schema.go` — `KeelConfig` struct, `Settings` struct, all sub-types
- `pkg/compiler/{agent}/{agent}.go` — the agent's adapter profile, `SettingsAssembler`, capabilities
- `pkg/compiler/profile.go` — `AgentProfile` struct, `Capabilities` struct
- `pkg/compiler/{agent}/reader.go` — what the reader imports
- `pkg/compiler/compile_pipeline.go` — DSL compilation logic, category iteration
- `pkg/compiler/agent_build.go` — IR assembly and processing
- `pkg/compiler/portability.go` — portable/agent-only field sets
- `pkg/skills/validate.go` — frontmatter field allowlist
- `docs/content/docs/ecosystem/agents.md` — the agent's detail card for reference

Assess coverage across four dimensions:

| Dimension | Question | Where to check |
|-----------|----------|----------------|
| **DSL** | Can a keel.yml layer express this feature? | `pkg/schema/schema.go` — `KeelConfig` fields |
| **Compiler** | Does the adapter emit this file/key in native output? | `pkg/compiler/{agent}/{agent}.go` — profile, settings assembler |
| **Reader** | Can `keel import` read this from a native project? | `pkg/compiler/{agent}/reader.go` |
| **Transpiler** | Does cross-format compilation preserve or warn? | `pkg/compiler/portability.go`, `agent_build.go` |

A feature may be "compiled but not importable" (no reader support) or "importable but not in the DSL" (reader extracts it, but no keel.yml field to author it).

### Classify every config item

For each config item the agent supports, classify it:

| Classification | Meaning |
|---|---|
| ✅ Compiled | Keel has a DSL field and produces this output |
| ⚠️ Passthrough | Works via `settings.{agent}.json` but no dedicated DSL field |
| ❌ Not compiled | Keel does not produce this output at all |
| 🔇 Intentionally excluded | Runtime state, user-local, or managed/system scope |

## Step 8: Build the Keel coverage matrix

Add a summary table to the schema doc showing at-a-glance coverage:

```markdown
| Config Item | DSL | Compile | Import | Transpile | Notes |
|-------------|-----|---------|--------|-----------|-------|
| instructions | ✅ | ✅ | ✅ | ✅ | Concatenated |
| permissions | ✅ | ✅ | ✅ | ✅ | Format differs |
| plugins | ✅ | ✅ | ✅ | ⚠️ lossy | Claude-only category |
| rules | ✅ | ✅ | ✅ | ⚠️ lossy | Frontmatter not portable |
| loop.md | — | — | — | — | Not handled |
```

This matrix is the primary artifact for gap analysis.

## Step 9: Check for bugs

For each ✅ Compiled item, verify the compilation is correct:

- Read the adapter's `SettingsAssembler` function and check that every DSL field is actually written to the output.
- Check that JSON key names match the agent's native format (camelCase vs snake_case vs kebab-case).
- Check that the `Capabilities` struct reflects reality (true for supported features, false for unsupported).
- Check that the `Categories` slice includes all directory-overlay categories.
- Check that `DSLFileRefs` maps all categories to their `KeelConfig` fields.
- Check that `validateFileRefFields` includes all FileRef collections.
- Check that `Validate()` has bounds checks for all collection fields.
- Check that `portability.go` `portableFields` matches the schema doc's Portable columns.
- Check that `portability.go` `agentOnlyFields` includes all agent-specific frontmatter fields.
- Check that `portableHookEvents` matches the hook event mapping in the schema doc.

## Step 10: Produce recommendations

For each ❌ gap, recommend:

- **Effort estimate** — trivial (add to existing pattern), low (new DSL field + compilation), medium (new compilation path), high (new package or significant design)
- **Priority** — based on how commonly the feature is used and whether governance adds value
- **Implementation approach** — which pattern to follow (directory overlay, JSON deep-merge, settings sub-key, new profile field)

## Step 11: Update the schema document

Update `docs/content/docs/reference/compatibility-matrix.md` with any new files, fields, or portability changes discovered.
Update the Keel coverage matrix sections.
Update the `Last verified` date.
Update `portability.go` if new portable or agent-only fields were discovered.
Run `go test ./pkg/compiler/ -run Matrix` to verify the matrix and code stay in sync.

## Step 12: Write the reports

Write **two outputs** — one public, one internal.

### Public: Config Coverage page

Write the user-facing coverage tables to `docs/content/docs/ecosystem/config-coverage.md`.
This is a Hugo page that renders on the doc site.

Format:
- Hugo frontmatter: `title: Config Coverage`, `weight: 1.5`
- Summary paragraph explaining what Keel covers (~90% of what teams configure)
- Per-agent sections with three tables each:
  - **Fully Compiled** — ✅ items with Config Item, Agent Location, Keel DSL Field columns
  - **Supported via Passthrough** — items that work through `settings.{agent}.json` with how-to
  - **Out of Scope** — items Keel intentionally doesn't compile, with clear reasoning
- Coverage summary table comparing all agents at a glance
- "What's Next" links to Agent Support, Transpilation, DSL Reference

Do NOT include in the public page:
- Bug reports
- Internal backlog references
- Design questions
- Effort estimates or implementation recommendations

### Internal: Audit report

Write bugs, recommendations, and implementation details to `local/CONFIG-AUDIT.md`.
Include:
- Date of audit
- Agent name and version checked
- Bugs found (with file paths and fix descriptions)
- Recommendations with effort and priority
- Design questions
- Sources (URLs to documentation consulted)
- Whether the schema doc was created or updated

## Step 13: Update the compatibility matrix (new agents only)

If this is a new agent not yet in `docs/content/docs/ecosystem/agents.md`, add it.
Skip this step for audits of existing agents.

### Classify the agent

| Condition | Classification |
|-----------|---------------|
| CLI + filesystem config | Compiler + container candidate |
| Desktop IDE + filesystem config | Compiler-only target |
| SaaS + standard SKILL.md | Platform agent |
| Clones repos + reads workspace | Indirect compiler target |

### Add to the compatibility matrix table

```markdown
| **<Agent Name>** | 🔜 | 🔜 | <Status>. <Brief note>. | `<config format>` | ✅ |
```

Use `✅` shipped, `🔜` planned, `⚠️` partial/uncertain, `❌` not viable.

### Add a detail card

```markdown
### <Agent Name>

<One-line description.>

| | |
|---|---|
| Config | `<config paths>` |
| Auth | <auth model> |
| Skills | `<skill paths>` |
```

### Update the Config Format Landscape table

If the agent uses a format not already listed, add a new row.
If it uses an existing format, add the agent name to that row.

## MCP location matrix

When auditing MCP configuration, always check all three scopes:

| Scope | Claude Location | Cursor Location | Keel Support |
|---|---|---|---|
| Project | `.mcp.json` | `.cursor/mcp.json` | Check `MCPOutput` on profile |
| User | `~/.claude.json` | `~/.cursor/mcp.json` | User-level compilation (backlog) |
| Managed | System path | — | Out of scope |

Note any scope where the project-level and user-level formats differ (Claude's user MCP is `~/.claude.json` which is a different format than `.mcp.json`).

## Related skills

- `add-compiler-adapter` — uses the audit output as adapter specification
- `add-container-adapter` — uses the audit output to understand container filesystem needs
- `add-product-comparison` — may reference audit data when comparing Keel to other tools
