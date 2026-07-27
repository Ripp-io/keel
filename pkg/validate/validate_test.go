package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Basic path checks
// ---------------------------------------------------------------------------

func TestRun_NonexistentPath(t *testing.T) {
	t.Parallel()
	result := Run("/nonexistent/path/to/layer", Options{})

	assert.Equal(t, 1, result.ErrorCount)
	assert.Contains(t, result.Items[0].Message, "path does not exist")
}

func TestRun_PathIsFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "validate-test-*")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	result := Run(f.Name(), Options{})

	assert.Equal(t, 1, result.ErrorCount)
	assert.Contains(t, result.Items[0].Message, "not a directory")
}

// ---------------------------------------------------------------------------
// Valid DSL layer
// ---------------------------------------------------------------------------

func TestRun_ValidDSLLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: Test\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Instructions"), 0o644))

	result := Run(dir, Options{})

	assert.Equal(t, 0, result.ErrorCount)
	assert.Equal(t, "Test", result.Name)
	assert.True(t, hasItem(result, true, false, "keel.yml schema valid"), "expected 'keel.yml schema valid' pass item")
}

// ---------------------------------------------------------------------------
// Missing referenced file
// ---------------------------------------------------------------------------

func TestRun_MissingReferencedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: Broken\ninstructions:\n  - source: missing.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	result := Run(dir, Options{})

	assert.Equal(t, 1, result.ErrorCount)
	assert.True(t, hasItem(result, false, false, "missing.md referenced but not found"), "expected error about missing.md")
}

// ---------------------------------------------------------------------------
// Invalid YAML
// ---------------------------------------------------------------------------

func TestRun_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(":::bad yaml["), 0o644))

	result := Run(dir, Options{})

	assert.Greater(t, result.ErrorCount, 0)
	assert.True(t, hasItem(result, false, false, "keel.yml:"), "expected keel.yml parse error")
}

// ---------------------------------------------------------------------------
// Empty directory (no keel.yml, no native markers)
// ---------------------------------------------------------------------------

func TestRun_EmptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := Run(dir, Options{})

	assert.Equal(t, 0, result.ErrorCount, "warnings only")
	assert.Equal(t, 1, result.WarnCount)
	assert.True(t, hasItem(result, false, true, "no keel.yml found"), "expected warning about no keel.yml")
}

// ---------------------------------------------------------------------------
// Native Claude layer
// ---------------------------------------------------------------------------

func TestRun_NativeClaudeLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))

	result := Run(dir, Options{})

	assert.Equal(t, 0, result.ErrorCount)
	assert.True(t, hasItem(result, true, false, "native claude layer"), "expected 'native claude layer' pass item")
}

// ---------------------------------------------------------------------------
// Agent compatibility checks
// ---------------------------------------------------------------------------

func TestRun_AgentCompatible(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: Test\n"), 0o644))

	result := Run(dir, Options{Agent: "claude"})

	assert.True(t, hasItem(result, true, false, "compatible with agent"), "expected agent compatibility pass")
}

func TestRun_AgentMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))

	result := Run(dir, Options{Agent: "cursor"})

	assert.True(t, hasItem(result, false, true, "targets a different agent"), "expected agent mismatch warning")
}

// ---------------------------------------------------------------------------
// Multiple errors
// ---------------------------------------------------------------------------

func TestRun_MultipleErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: Multi\ninstructions:\n  - source: a.md\n  - source: b.md\ncommands:\n  - source: c.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	result := Run(dir, Options{})

	assert.Equal(t, 3, result.ErrorCount)
}

// ---------------------------------------------------------------------------
// Settings file references
// ---------------------------------------------------------------------------

func TestRun_SettingsFileRefs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: Settings\nsettings:\n  claude: settings.claude.json\n  cursor: settings.cursor.json\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	// Create only claude settings.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.claude.json"), []byte("{}"), 0o644))

	result := Run(dir, Options{})

	assert.Equal(t, 1, result.ErrorCount, "missing cursor settings")
	assert.True(t, hasItem(result, true, false, "settings.claude.json exists"), "expected pass for settings.claude.json")
	assert.True(t, hasItem(result, false, false, "settings.cursor.json referenced but not found"), "expected error for settings.cursor.json")
}

// ---------------------------------------------------------------------------
// Skills validation
// ---------------------------------------------------------------------------

func TestRun_SkillsCompliant(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SkillTest\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "code-review")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := "---\nname: code-review\ndescription: Reviews code for quality.\n---\nReview instructions.\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{AgentSkillsCheck: true})

	assert.True(t, hasItem(result, true, false, "skills: Agent Skills format compliant"), "expected skills compliant pass item")
}

func TestRun_SkillsCheckDisabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoSkill\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Create a broken skill directory (no SKILL.md).
	skillDir := filepath.Join(dir, "skills", "broken")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	result := Run(dir, Options{AgentSkillsCheck: false})
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "skill broken", "expected no skill warnings with AgentSkillsCheck=false")
	}
}

func TestRun_SkillsDisabledViaKeelYml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoSkill\nskills:\n  - source: skills/\nvalidate:\n  agentskills: false\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "broken")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	result := Run(dir, Options{AgentSkillsCheck: true})
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "skill broken", "expected no skill warnings when keel.yml disables agentskills check")
	}
}

// ---------------------------------------------------------------------------
// Repos count
// ---------------------------------------------------------------------------

func TestRun_ReposReported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: Repos\nrepos:\n  myrepo:\n    url: https://example.com/repo.git\n    path: myrepo\n    branch: main\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	result := Run(dir, Options{})

	assert.True(t, hasItem(result, true, false, "1 repo(s) defined"), "expected '1 repo(s) defined' pass item")
}

// ---------------------------------------------------------------------------
// Result type helpers
// ---------------------------------------------------------------------------

func TestResult_AddPass(t *testing.T) {
	t.Parallel()
	r := &Result{}
	r.addPass("test pass")

	require.Len(t, r.Items, 1)
	assert.True(t, r.Items[0].OK)
	assert.Equal(t, 0, r.ErrorCount)
}

func TestResult_AddError(t *testing.T) {
	t.Parallel()
	r := &Result{}
	r.addError("test error")

	require.Len(t, r.Items, 1)
	assert.False(t, r.Items[0].OK)
	assert.False(t, r.Items[0].Warn)
	assert.Equal(t, 1, r.ErrorCount)
}

func TestResult_AddWarn(t *testing.T) {
	t.Parallel()
	r := &Result{}
	r.addWarn("test warning")

	require.Len(t, r.Items, 1)
	assert.False(t, r.Items[0].OK)
	assert.True(t, r.Items[0].Warn)
	assert.Equal(t, 1, r.WarnCount)
	assert.Equal(t, 0, r.ErrorCount)
}

// ---------------------------------------------------------------------------
// Body content linting integration
// ---------------------------------------------------------------------------

func TestRun_LintDetectsNonPortablePatterns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: LintTest\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Use $ARGUMENTS here.\n"), 0o644))

	result := Run(dir, Options{Lint: true})

	assert.True(t, hasItem(result, false, true, "lint:", "$ARGUMENTS"), "expected lint warning for $ARGUMENTS")
}

func TestRun_LintSuppressesOwnAgentPatterns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: LintAgent\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	// Claude-only pattern should be suppressed when --agent claude is set.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Use $ARGUMENTS here.\n"), 0o644))

	result := Run(dir, Options{Agent: "claude", Lint: true})

	for _, item := range result.Items {
		if item.Warn && strings.Contains(item.Message, "lint:") {
			assert.NotContains(t, item.Message, "$ARGUMENTS", "Claude-only patterns should be suppressed with --agent claude")
		}
	}
}

func TestRun_LintWarnsAboutOtherAgentPatterns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: LintOther\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	// Claude-only pattern should still warn when --agent cursor is set.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Use $ARGUMENTS here.\n"), 0o644))

	result := Run(dir, Options{Agent: "cursor", Lint: true})

	assert.True(t, hasItem(result, false, true, "lint:", "$ARGUMENTS"), "expected lint warning for Claude-only $ARGUMENTS when targeting cursor")
}

func TestRun_LintDisabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoLint\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Use $ARGUMENTS here.\n"), 0o644))

	result := Run(dir, Options{Lint: false})

	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "lint:", "expected no lint messages with Lint=false")
	}
}

func TestRun_LintCleanFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: Clean\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Clean Markdown\n\nNo agent patterns.\n"), 0o644))

	result := Run(dir, Options{Lint: true})

	assert.True(t, hasItem(result, true, false, "lint: no non-portable body patterns found"), "expected pass for clean files")
}

func TestRun_LintOnlyWarns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: WarnOnly\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("$ARGUMENTS\n{{variable}}\n${CLAUDE_EFFORT}\n"), 0o644))

	result := Run(dir, Options{Lint: true})

	// Lint findings should never produce errors, only warnings.
	assert.Equal(t, 0, result.ErrorCount, "lint should never produce errors")
	assert.Greater(t, result.WarnCount, 0, "expected warnings for non-portable patterns")
}

// ---------------------------------------------------------------------------
// Snippet placeholder validation (K1a)
// ---------------------------------------------------------------------------

func TestRun_SnippetUnresolvedPlaceholder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SnipCheck\ninstructions:\n  - source: instructions.md\nsnippets:\n  - source: snippets/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	snippetDir := filepath.Join(dir, "snippets")
	require.NoError(t, os.MkdirAll(snippetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetDir, "greeting.md"), []byte("Hello {{typo}}, welcome!"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Intro\n<!-- keel:include greeting -->\n"), 0o644))

	result := Run(dir, Options{})

	assert.True(t, hasItem(result, false, true, `snippet "greeting" has unresolved placeholder {{typo}}`),
		"expected warning about unresolved placeholder {{typo}}")
}

func TestRun_SnippetPlaceholderResolved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SnipOK\ninstructions:\n  - source: instructions.md\nsnippets:\n  - source: snippets/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	snippetDir := filepath.Join(dir, "snippets")
	require.NoError(t, os.MkdirAll(snippetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetDir, "greeting.md"), []byte("Hello {{name}}, welcome!"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("<!-- keel:include greeting name=\"Alice\" -->\n"), 0o644))

	result := Run(dir, Options{})

	for _, item := range result.Items {
		if item.Warn {
			assert.NotContains(t, item.Message, "unresolved placeholder")
		}
	}
}

// ---------------------------------------------------------------------------
// Snippet inventory validation (K1c)
// ---------------------------------------------------------------------------

func TestRun_SnippetDefinedButNeverReferenced(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SnipOrphan\ninstructions:\n  - source: instructions.md\nsnippets:\n  - source: snippets/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	snippetDir := filepath.Join(dir, "snippets")
	require.NoError(t, os.MkdirAll(snippetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetDir, "orphan.md"), []byte("Unused snippet content."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# No includes here.\n"), 0o644))

	result := Run(dir, Options{})

	assert.True(t, hasItem(result, false, true, `snippet "orphan" is defined but never referenced`),
		"expected warning about orphan snippet")
}

func TestRun_SnippetReferencedButNotDefined(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SnipMissing\ninstructions:\n  - source: instructions.md\nsnippets:\n  - source: snippets/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	snippetDir := filepath.Join(dir, "snippets")
	require.NoError(t, os.MkdirAll(snippetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("<!-- keel:include nonexistent -->\n"), 0o644))

	result := Run(dir, Options{})

	assert.True(t, hasItem(result, false, true, `snippet "nonexistent" is referenced but not defined`),
		"expected warning about snippet referenced but not defined")
}

func TestRun_SnippetInventoryMatched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SnipMatch\ninstructions:\n  - source: instructions.md\nsnippets:\n  - source: snippets/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	snippetDir := filepath.Join(dir, "snippets")
	require.NoError(t, os.MkdirAll(snippetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(snippetDir, "banner.md"), []byte("Welcome banner content."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Intro\n<!-- keel:include banner -->\n"), 0o644))

	result := Run(dir, Options{})

	// No snippet-related warnings expected.
	for _, item := range result.Items {
		if item.Warn {
			assert.NotContains(t, item.Message, "not defined")
			assert.NotContains(t, item.Message, "never referenced")
		}
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// hasItem checks if result.Items contains an item matching the given criteria.
// substrings are all checked against the item's Message.
func hasItem(result Result, wantOK, wantWarn bool, substrings ...string) bool {
	for _, item := range result.Items {
		if item.OK != wantOK || item.Warn != wantWarn {
			continue
		}
		match := true
		for _, sub := range substrings {
			if !strings.Contains(item.Message, sub) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
