---
name: update-docs
description: Update the documentation site when code changes warrant it — new features, CLI changes, DSL additions, bug fixes that affect documented behavior. Reads the style guide, maps changes to affected pages, and applies updates.
---

# Update Documentation

Detect what changed in the codebase and update the documentation site to match.
This skill handles new features, CLI flag changes, DSL additions, merge strategy changes, adapter updates, and bug fixes that affect documented behavior.

## When to use

- After implementing a new feature
- After changing CLI flags or adding commands
- After modifying the DSL schema (`pkg/schema/schema.go`)
- After adding or modifying a compiler adapter
- After fixing a bug that changes documented behavior
- When asked to "update the docs" or "document this change"

## Step 0: Read the style guide

Follow the project's documentation style conventions.
Every page follows these rules:
- Summary-first (1–3 sentences at the top)
- Examples before explanation
- Progressive disclosure (summary → examples → explanation → reference)
- One sentence per line in markdown
- Paragraphs ≤ 3 sentences
- Every page ends with a `## What's Next` section

## Step 1: Identify what changed

Determine the scope of changes that need documentation.
Use one or more of these approaches depending on context:

**From a session summary or description:**
The user provides a summary of what was built — extract the doc-relevant items.

**From git diff:**
```bash
git diff --name-only HEAD~N  # or against main
```

**From modified source files:**
Read the changed files and identify user-facing behavior changes.

### What warrants a doc update

| Change type | Doc impact | Example |
|---|---|---|
| New DSL field in `pkg/schema/schema.go` | Add to DSL reference, features page, merge table | New `bin:` field |
| New CLI flag in `cmd/keel/*.go` | Add to CLI reference, possibly features page | `--allow-install` flag |
| New CLI command | Add to CLI reference command table + section, features page | `keel global` |
| New merge strategy | Add to layers merge table, architecture/merge-strategies | New overlay behavior |
| New compiler adapter | Add to ecosystem/agents, update config coverage | Gemini CLI adapter |
| Changed compilation behavior | Update affected guide pages | MCP refactored to in-memory |
| New auto-injected variable | Update carve-outs.md auto-injected table | `keel.bin.project` |
| Variable substitution scope change | Update carve-outs.md substitution section | Variables in MCP configs |
| Bug fix affecting documented behavior | Correct the affected doc page | `-v` flag was misdocumented |
| New governance feature | Update governance.md, features page | New force-lock type |
| New validation rule | Update validation.md | SHA-256 warnings on deps |

### What does NOT need a doc update

- Internal refactoring with no user-facing change
- Test-only changes
- Performance improvements with no behavioral change
- Code style changes

## Step 2: Map changes to doc pages

Use this mapping to identify which pages need updating.

### Code → Doc page mapping

| Source file/package | Primary doc page(s) |
|---|---|
| `pkg/schema/schema.go` | `guide/dsl.md` (quick reference table + section), `features.md` |
| `cmd/keel/compile.go` | `reference/cli.md` (compile flags) |
| `cmd/keel/*.go` (new command) | `reference/cli.md` (command table + section), `features.md` |
| `pkg/compiler/claude/claude.go` | `ecosystem/config-coverage.md` (Claude section) |
| `pkg/compiler/cursor/cursor.go` | `ecosystem/config-coverage.md` (Cursor section) |
| `pkg/compiler/profile.go` | `guide/layers.md` (merge table), `architecture/merge-strategies.md` |
| `pkg/compiler/resolve.go` | `guide/layers.md` (resolution), `guide/conditionals.md` (variables) |
| `pkg/merge/` | `architecture/merge-strategies.md`, `guide/layers.md` (merge table) |
| `pkg/skills/` | `guide/agent-skills.md`, `guide/validation.md` |
| `pkg/remote/` | `guide/layers.md` (remote layers) |
| `adapters/*/` | `ecosystem/agents.md` |
| `containers/` | `reference/runtime-contract.md` |
| `pkg/config/compile_config.go` | `reference/cli.md` (compile flags) |

### Secondary pages often affected

| Change type | Also update |
|---|---|
| Any new DSL field | `guide/dsl.md` quick reference table |
| Any new feature | `features.md` table in the relevant category |
| Any merge behavior change | `guide/layers.md` merge behavior table |
| Any new auto-injected variable | `guide/conditionals.md` auto-injected variables table |
| Any new CLI command | CLI reference command summary table at top |
| Any governance change | `guide/governance.md` |
| Landing page feature cards | `_index.md` — only if the feature is a significant selling point |
| Enterprise example | `guide/enterprise-example.md` — only if the demo layers should showcase this |

## Step 3: Read affected pages

Read each page that needs updating to understand its current structure and content.
Check the section headings, existing tables, code examples, and What's Next links.

Do NOT rewrite pages from scratch — make targeted updates that fit the existing structure.

## Step 4: Apply updates

For each affected page, apply the minimum changes needed:

### Adding a new DSL field

1. Add a row to the quick reference table in `guide/dsl.md`
2. Add a full section with YAML example, field table, and portability note
3. Add a row to the merge behavior table in `guide/layers.md`
4. Add a row to the relevant category in `features.md`

### Adding a new CLI command

1. Add a row to the command summary table at the top of `reference/cli.md`
2. Add a full section with description, flags table, and examples
3. Add a row to the CLI category in `features.md`

### Adding a new CLI flag

1. Add a row to the relevant command's flags table in `reference/cli.md`

### Changing variable behavior

1. Update the auto-injected variables table in `guide/conditionals.md`
2. Update the substitution syntax section if scope changed
3. Update any code examples that reference variables

### Updating config coverage

1. Update the relevant agent's tables in `ecosystem/config-coverage.md`
2. Move items between Fully Compiled / Passthrough / Out of Scope as needed

## Step 5: Verify

After making changes:

```bash
hugo --source docs --quiet
```

If the build fails, fix broken links or syntax errors.

Check that:
- [ ] No broken `relref` links (Hugo build catches these)
- [ ] New sections follow the style guide (summary-first, examples before prose)
- [ ] Tables are consistent with existing format on the page
- [ ] What's Next sections link to relevant new pages if any were added

## Step 6: Check for cascading updates

Some changes ripple across pages.
After your primary updates, check:

- Does the landing page need a feature card update?
- Does the enterprise example demo need to showcase this feature?
- Does the community catalog or personal toolkit page reference this?
- Does the Why Keel page's "When Does Keel Become Worth It?" table need a new row?
- Does the comparisons overlap matrix need a new column?

Most changes don't cascade. Only significant new capabilities warrant landing page or comparisons updates.

## Doc site structure reference

```
docs/content/
├── _index.md                          Landing page
└── docs/
    ├── _index.md                      Docs index
    ├── why-keel.md                    Market positioning, personas, objections
    ├── features.md                    Feature catalog (7 tables)
    ├── getting-started/
    │   ├── _index.md                  Quick Start (consumer path)
    │   ├── community-catalog.md       Data-driven from data/community_layers.yaml
    │   └── create-a-layer.md          Producer path tutorial
    ├── guide/
    │   ├── layers.md                  Core concept + merge table
    │   ├── dsl.md                     Full DSL reference (quick ref table + sections)
    │   ├── governance.md              Force-locks, merge policies
    │   ├── carve-outs.md              Conditionals, variables, expressions
    │   ├── snippets.md                Template includes
    │   ├── transpilation.md           Cross-format compilation
    │   ├── validation.md              keel validate
    │   ├── packaging.md               Distributable, archives, lockfile
    │   ├── output-lifecycle.md        Manifest, pruning, clean
    │   ├── enterprise-example.md      5-layer demo with cascade table
    │   ├── personal-toolkit.md        User scope, remote layers
    │   └── user-scope.md              Redirect to personal-toolkit
    ├── reference/
    │   ├── cli.md                     All commands with flags
    │   ├── compatibility-matrix.md    Cross-agent compatibility (source of truth)
    │   ├── settings.md                Settings keys and portability
    │   ├── hooks.md                   Hook events and handlers
    │   ├── frontmatter.md             Frontmatter fields and portability
    │   └── runtime-contract.md        Container build/run API
    ├── architecture/
    │   ├── adapters.md                Adding a new adapter guide
    │   ├── ir.md                      Intermediate representation
    │   └── merge-strategies.md        Merge strategy implementations
    └── ecosystem/
        ├── agents.md                  30+ agent compatibility matrix
        ├── config-coverage.md         Per-agent config coverage tables
        ├── comparisons.md             Overlap matrix + comparison index
        └── vs-*.md                    Individual comparison pages
```
