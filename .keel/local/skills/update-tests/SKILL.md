---
name: update-tests
description: Update fixture and unit tests when code changes warrant it — new features, CLI changes, DSL additions, bug fixes, or compilation behavior changes. Maps changes to affected test layers and assertions, and applies updates.
---

# Update Tests

Detect what changed in the codebase and update tests to match.
The primary test surface is the **fixture** at `examples/compiler/` — static layers compiled by `fixture_test.go` with assertions against both Claude and Cursor output.
Unit tests in `pkg/` and `cmd/` packages complement the fixture for isolated logic.

## When to use

- After implementing a new feature
- After changing CLI flags or adding commands
- After modifying the DSL schema (`pkg/schema/schema.go`)
- After adding or modifying a compiler adapter
- After fixing a bug that changes compilation behavior
- After changing merge strategies or overlay mechanics
- When asked to "update the tests" or "add test coverage"

## Step 0: Understand the test architecture

### Fixture (integration)

The fixture at `examples/compiler/` is the primary regression surface.
It compiles 8 static layers through both Claude and Cursor adapters and asserts the output.

```
examples/compiler/
  fixture_test.go     ← 31 test functions with helpers
  layers/
    01-keel-base/         ← DSL: instructions, commands, skills, hooks, MCP, snippets, force-locks
    02-claude-base/       ← Native Claude layer with keel.yml policy
    03-cursor-base/       ← Native Cursor layer with keel.yml policy
    04-keel-overrides/    ← DSL: variable overrides, cross-layer snippets, conditionals
    05-claude-overrides/  ← Native Claude overrides
    06-cursor-overrides/  ← Native Cursor overrides
    07-deps-and-bin/      ← DSL: bin/ overlay, deps declarations, dep vars in MCP/hooks
    08-convention-defaults/ ← DSL: convention directory discovery (no explicit hooks/agents/snippets)
```

**Key fixture helpers** (in `fixture_test.go`):
- `compileTo(t, agent, opts)` — compile fixture layers, return outputDir and result
- `readFile(t, dir, relPath)` — read file content
- `assertContains(t, ctx, content, substr)` / `assertNotContains`
- `assertExists(t, dir, relPath)` / `assertNotExists`
- `assertForceLocked(t, result, category, key)`
- `captureLogsForCompile(t, agent, opts)` — compile with log capture for warning assertions
- `buildFixtureLayers(t)` — reads layers/ dirs in sorted order, expects correct count

**When adding a new layer**: update `buildFixtureLayers()` expected count.

### Unit tests (isolated logic)

| Package | Test file(s) | What they cover |
|---|---|---|
| `pkg/compiler/` | `compiler_claude_test.go` | Claude-specific compilation tests |
| `pkg/compiler/` | `compiler_cursor_test.go` | Cursor-specific compilation tests |
| `pkg/compiler/` | `compiler_cross_test.go` | Cross-agent consistency tests |
| `pkg/compiler/` | `compiler_features_test.go` | Feature-specific compilation tests |
| `pkg/compiler/` | `compiler_registry_test.go` | Compiler registry tests |
| `pkg/compiler/` | `compiler_scope_test.go` | Scope-specific compilation tests |
| `pkg/compiler/` | `compiler_snippet_test.go` | Snippet compilation tests |
| `pkg/compiler/` | `deps_test.go` | Deps processing: install, download, sha256, URL templates, verify |
| `pkg/compiler/` | `deps_integration_test.go` | Full compile with AllowInstall, variable resolution end-to-end |
| `pkg/compiler/` | `overlay_test.go` | Overlay mechanics, frontmatter parsing, symlink protection |
| `pkg/globaldeps/` | `manifest_test.go` | Manifest CRUD, consumer tracking, orphan detection |
| `pkg/merge/` | `variables_test.go` | Variable substitution in strings and maps |
| `pkg/schema/` | `schema_test.go` | Schema validation, parsing, bounds checks |
| `pkg/validate/` | `validate_test.go` | Validation output, warnings, file ref checking |
| `cmd/keel/` | `compile_test.go` | Compile command flag wiring |
| `cmd/keel/` | `clean_test.go` | Clean command, multi-agent clean, global deregistration |
| `cmd/keel/` | `global_test.go` | Global list/clean commands |
| `cmd/keel/` | `explain_test.go` | Explain command output |

## Step 1: Identify what changed

Determine the scope of changes that need test updates.

**From git diff:**
```bash
git diff --name-only HEAD~N  # or against main
```

**From a session summary:**
Extract the test-relevant items from the description.

### What warrants a test update

| Change type | Test impact | Where |
|---|---|---|
| New DSL field in `pkg/schema/schema.go` | Schema validation tests + fixture layer + fixture assertions | `schema_test.go` + fixture |
| New CLI flag | Compile test for flag wiring | `compile_test.go` |
| New CLI command | Command test functions | `cmd/keel/*_test.go` |
| New merge behavior | Fixture layer exercising the merge + assertions | fixture |
| New overlay category | Fixture layer with files in the category + assertions | fixture |
| Changed compilation output | Update affected fixture assertions | `fixture_test.go` |
| New auto-injected variable | Fixture assertion that it resolves | `fixture_test.go` |
| Variable substitution scope change | Fixture assertions for new scope (MCP, hooks, settings) | `fixture_test.go` |
| New force-lock type | Fixture layer with force-lock + assertions | fixture |
| Bug fix changing output | Update assertions that matched old (buggy) behavior | varies |
| New validation rule | Validate test for the warning/error | `validate_test.go` |
| New default in `keel.default.yml` | Convention defaults fixture layer + assertion | fixture layer 08 |

### What does NOT need test updates

- Documentation-only changes
- Comment changes
- Code style / formatting
- Changes to container adapters (tested separately)
- Changes to the demo example (not a test)

## Step 2: Map changes to test locations

### Code to test mapping

| Source file | Primary test location | Fixture impact |
|---|---|---|
| `pkg/schema/schema.go` | `pkg/schema/schema_test.go` | Add to fixture if it affects compilation output |
| `pkg/schema/defaults.go` | `pkg/schema/schema_test.go` | Update fixture layer 08 (convention defaults) |
| `pkg/schema/keel.default.yml` | Implicit via fixture layer 08 | Update layer 08 if new convention paths |
| `pkg/compiler/compile_pipeline.go` | `pkg/compiler/compiler_*_test.go` | Usually needs fixture update |
| `pkg/compiler/agent_build.go` | `pkg/compiler/compiler_*_test.go` | Fixture if IR assembly behavior changed |
| `pkg/compiler/profile.go` | `pkg/compiler/compiler_*_test.go` | Fixture if finalization behavior changed |
| `pkg/compiler/force_merge.go` | `pkg/compiler/force_test.go` | Fixture if force-lock behavior changed |
| `pkg/compiler/resolve.go` | Unit test + fixture | Fixture if resolution behavior changed |
| `pkg/compiler/deps.go` | `pkg/compiler/deps_test.go` | Fixture layer 07 |
| `pkg/globaldeps/manifest.go` | `pkg/globaldeps/manifest_test.go` | `cmd/keel/global_test.go` |
| `pkg/compiler/compiler.go` | `pkg/compiler/compiler_*_test.go` | Fixture if CompileOptions changed |
| `pkg/compiler/claude/claude.go` | `pkg/compiler/compiler_claude_test.go` | Fixture (Claude-specific assertions) |
| `pkg/compiler/cursor/cursor.go` | `pkg/compiler/compiler_cursor_test.go` | Fixture (Cursor-specific assertions) |
| `pkg/merge/variables.go` | `pkg/merge/variables_test.go` | Fixture if substitution scope changed |
| `pkg/validate/validate.go` | `pkg/validate/validate_test.go` | Rarely needs fixture update |
| `pkg/config/compile_config.go` | `cmd/keel/compile_test.go` | Add flag wiring test |
| `cmd/keel/compile.go` | `cmd/keel/compile_test.go` | Flag tests |
| `cmd/keel/clean.go` | `cmd/keel/clean_test.go` | If cleanup behavior changed |
| `cmd/keel/explain.go` | `cmd/keel/explain_test.go` | If new sections added |

## Step 3: Decide fixture vs unit test

**Use the fixture when:**
- The feature affects compiled output (what ends up in `.claude/`, `.cursor/`, `.keel/`)
- The feature involves cross-layer interaction (later layers override earlier ones)
- The feature should work for both Claude AND Cursor
- You need to test variable resolution through the full pipeline
- You need to test force-lock governance through the full pipeline

**Use a unit test when:**
- Testing isolated logic (parsing, validation, manifest CRUD)
- Testing error paths (invalid input, missing files, sha256 mismatch)
- Testing edge cases (empty input, nil maps, boundary conditions)
- Testing concurrency (file locking)
- The feature doesn't affect compiled output

**Use both when:**
- A new DSL feature needs schema validation tests AND compilation output assertions

## Step 4: Apply test updates

### Adding to an existing fixture layer

If the feature fits an existing layer's theme, add files there:
- Layer 01 (keel-base): Core DSL features, force-locks, governance
- Layer 04 (keel-overrides): Override behavior, cross-layer interaction
- Layer 07 (deps-and-bin): Binary dependencies, bin/ overlay, dep variables
- Layer 08 (convention-defaults): Convention directory discovery, defaults

### Adding a new fixture layer

If the feature is distinct enough to warrant its own layer:
1. Create `examples/compiler/layers/NN-descriptive-name/`
2. Add `keel.yml` and supporting files
3. Update `buildFixtureLayers()` expected count in `fixture_test.go`
4. Add `TestFixture_FeatureName_Claude` and `TestFixture_FeatureName_Cursor` test functions

### Fixture test naming convention

```
TestFixture_FeatureArea_Agent         — e.g., TestFixture_DepsAndBin_Claude
TestFixture_FeatureArea_Aspect        — e.g., TestFixture_DepsAndBin_MultiAgent
TestFixture_FeatureArea_EdgeCase      — e.g., TestFixture_NonDistributable_FrontmatterStripped
```

Subtests within use descriptive names:
```go
t.Run("MCP_DepPathResolved", func(t *testing.T) { ... })
t.Run("Hooks_ShScriptIgnored", func(t *testing.T) { ... })
```

### Cross-agent coverage rule

Every fixture assertion for Claude should have a Cursor counterpart unless the feature is agent-specific.
Output paths differ:
- Claude: `CLAUDE.md`, `.claude/settings.json`, `.mcp.json`, `.claude/commands/`
- Cursor: `.cursor/rules/00-keel.mdc`, `.cursor/cli-config.json`, `.cursor/mcp.json`, `.cursor/commands/`

Hook locations differ:
- Claude: hooks inside `.claude/settings.json` under `"hooks"` key
- Cursor: separate `.cursor/hooks.json` file

### Unit test patterns

Follow existing patterns in the package:
- Table-driven tests with `t.Run`
- `t.TempDir()` for filesystem fixtures
- `httptest.NewServer` for HTTP tests
- No testify — standard `testing` package only
- `buildTestLayers()` helper for compiler tests
- `assertFileMatchesExpected()` for golden file comparisons

## Step 5: Verify

```bash
go test ./...       # all tests pass
go vet ./...        # no issues
```

Check that:
- [ ] All new test functions have descriptive names
- [ ] Cross-agent coverage: Claude test has Cursor counterpart
- [ ] Layer count in `buildFixtureLayers()` matches actual layer count
- [ ] No hardcoded paths (use `t.TempDir()`, `filepath.Join`)
- [ ] Tests are deterministic (sorted iteration for maps, no timing dependencies)

## Step 6: Check for cascading test needs

After primary updates, check:
- Did you change a DSL field? Schema validation test + fixture assertion both needed.
- Did you change `keel.default.yml`? Layer 08 (convention-defaults) may need updating.
- Did you add a new auto-injected variable? Fixture assertions for resolution.
- Did you change force-lock behavior? Fixture assertions for both preserve and strip cases.
- Did you change `MergeWithDefaults`? Ensure all fields are passed through.
- Did the `buildFixtureLayers()` count change? Update the assertion.

## Current test counts (keep updated)

```
compiler:          ~460 test functions across split test files
CLI commands:      ~210 test functions
Total:             ~670 test functions across all packages
```

Run `grep -c '^func Test' pkg/compiler/*_test.go | awk -F: '{sum+=$2} END {print sum}'` to get current count.
