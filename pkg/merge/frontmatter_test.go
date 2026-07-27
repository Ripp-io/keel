package merge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
)

func TestParseFrontmatter_KeelOnly(t *testing.T) {
	t.Parallel()
	input := []byte("---\nforce: true\n---\nContent here.\n")
	fm, body, err := ParseAndCleanFrontmatter(input)
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.NotContains(t, string(body), "---")
	assert.Equal(t, "Content here.\n", string(body))
}

func TestParseFrontmatter_KeelOnlyScope(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nscope: commands/\n---\nBody text.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	assert.Equal(t, "commands/", fm.Scope)
	assert.Equal(t, "Body text.\n", string(body))
}

func TestParseFrontmatter_KeelForceAndScope(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nforce: true\nscope: agents/\n---\nAgent content.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.Equal(t, "agents/", fm.Scope)
	assert.Equal(t, "Agent content.\n", string(body))
}

func TestParseFrontmatter_Mixed(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nname: code-reviewer\ndescription: Reviews code\nforce: true\n---\nYou are a code reviewer.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	bodyStr := string(body)
	assert.True(t, strings.HasPrefix(bodyStr, "---\n"))
	assert.Contains(t, bodyStr, "name: code-reviewer")
	assert.Contains(t, bodyStr, "description: Reviews code")
	assert.NotContains(t, bodyStr, "force:")
	assert.Contains(t, bodyStr, "You are a code reviewer.")
}

func TestParseFrontmatter_MixedWithScope(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\ndescription: PCI compliance rules\nglobs: \"**/*.go\"\nalwaysApply: true\nforce: true\nscope: rules/\n---\nRule content.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.Equal(t, "rules/", fm.Scope)
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "description: PCI compliance rules")
	assert.Contains(t, bodyStr, "alwaysApply: true")
	assert.NotContains(t, bodyStr, "force:")
	assert.NotContains(t, bodyStr, "scope:")
	assert.Contains(t, bodyStr, "Rule content.")
}

func TestParseFrontmatter_NoKeelFields(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: code-reviewer\ndescription: Reviews code\nmodel: sonnet\n---\nContent.\n")
	fm, body, err := ParseAndCleanFrontmatter(input)
	require.NoError(t, err)
	assert.Nil(t, fm)
	assert.Equal(t, string(input), string(body))
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("Just content, no frontmatter.\n\nMore content.\n")
	fm, body, err := ParseAndCleanFrontmatter(input)
	require.NoError(t, err)
	assert.Nil(t, fm)
	assert.Equal(t, string(input), string(body))
}

func TestParseFrontmatter_ForceExplicitFalse(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nforce: false\n---\nContent.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.False(t, *fm.Force)
	assert.Equal(t, "Content.\n", string(body))
}

func TestParseFrontmatter_EmptyInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty byte slice", []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fm, body, err := ParseAndCleanFrontmatter(tt.input)
			require.NoError(t, err)
			assert.Nil(t, fm)
			assert.Empty(t, body)
		})
	}
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input string
	}{
		{"empty block with content", "---\n---\nContent.\n"},
		{"only delimiters", "---\n---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := []byte(tt.input)
			fm, body, err := ParseAndCleanFrontmatter(input)
			require.NoError(t, err)
			assert.Nil(t, fm)
			assert.Equal(t, string(input), string(body))
		})
	}
}

func TestParseFrontmatter_HorizontalRuleInBody(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nforce: true\n---\nContent here.\n\n---\n\nMore content after horizontal rule.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "Content here.")
	assert.Contains(t, bodyStr, "---")
	assert.Contains(t, bodyStr, "More content after horizontal rule.")
}

func TestParseFrontmatter_InvalidDelimiters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input string
	}{
		{"no closing delimiter", "---\nforce: true\nSome content that never closes.\n"},
		{"leading whitespace", "  ---\nforce: true\n---\nContent.\n"},
		{"leading blank line", "\n---\nforce: true\n---\nContent.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := []byte(tt.input)
			fm, body, err := ParseAndCleanFrontmatter(input)
			require.NoError(t, err)
			assert.Nil(t, fm)
			assert.Equal(t, string(input), string(body))
		})
	}
}

func TestParseFrontmatter_NoBodyAfterFrontmatter(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nforce: true\n---\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.Empty(t, strings.TrimSpace(string(body)))
}

func TestParseFrontmatter_MergeField(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nmerge: replace\n---\nContent.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	assert.Equal(t, policy.MergeStrategyReplace, fm.Merge)
	assert.NotContains(t, string(body), "---")
	assert.Equal(t, "Content.\n", string(body))
}

func TestParseFrontmatter_AllKeelFields(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nforce: true\nmerge: deep-merge\nscope: rules/\n---\nAll fields.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.Equal(t, policy.MergeStrategyDeepMerge, fm.Merge)
	assert.Equal(t, "rules/", fm.Scope)
	assert.Equal(t, "All fields.\n", string(body))
}

func TestParseFrontmatter_MixedWithMerge(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\nname: reviewer\nmerge: additive\nforce: true\n---\nBody.\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.Equal(t, policy.MergeStrategyAdditive, fm.Merge)
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "name: reviewer")
	assert.NotContains(t, bodyStr, "merge:")
	assert.NotContains(t, bodyStr, "force:")
}

func TestParseFrontmatter_WindowsLineEndings(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\r\nforce: true\r\n---\r\nContent.\r\n"))
	require.NoError(t, err)
	require.NotNil(t, fm)
	require.NotNil(t, fm.Force)
	assert.True(t, *fm.Force)
	assert.Contains(t, string(body), "Content.")
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	t.Parallel()
	fm, body, err := ParseAndCleanFrontmatter([]byte("---\n: [invalid yaml\n---\nContent.\n"))
	require.Error(t, err)
	assert.Nil(t, fm)
	assert.Nil(t, body)
}

func TestStripFrontmatter_KeelOnly(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Clean content.\n", string(StripFrontmatter([]byte("---\nforce: true\n---\nClean content.\n"))))
}

func TestStripFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("No frontmatter here.\n")
	assert.Equal(t, string(input), string(StripFrontmatter(input)))
}

func TestStripFrontmatter_Mixed(t *testing.T) {
	t.Parallel()
	got := string(StripFrontmatter([]byte("---\nname: reviewer\nforce: true\n---\nBody.\n")))
	assert.Contains(t, got, "name: reviewer")
	assert.NotContains(t, got, "force:")
	assert.Contains(t, got, "Body.")
}

func TestStripFrontmatter_NoKeelFields(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: reviewer\ndescription: A reviewer\n---\nBody.\n")
	assert.Equal(t, string(input), string(StripFrontmatter(input)))
}

func TestExtractRawFrontmatter_NoFrontmatter(t *testing.T) {
	t.Parallel()
	m, err := ExtractRawFrontmatter([]byte("Just plain markdown."))
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestExtractRawFrontmatter_EmptyInput(t *testing.T) {
	t.Parallel()
	m, err := ExtractRawFrontmatter([]byte{})
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestExtractRawFrontmatter_AllFieldsPreserved(t *testing.T) {
	t.Parallel()
	m, err := ExtractRawFrontmatter([]byte("---\nname: code-review\ndescription: Reviews code.\nforce: true\n---\nBody.\n"))
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "code-review", m["name"])
	assert.Equal(t, "Reviews code.", m["description"])
	assert.Equal(t, true, m["force"])
}

func TestExtractRawFrontmatter_InvalidYAML(t *testing.T) {
	t.Parallel()
	_, err := ExtractRawFrontmatter([]byte("---\n: invalid: yaml: [[\n---\nBody.\n"))
	require.Error(t, err)
}

func TestExtractRawFrontmatter_EmptyFrontmatter(t *testing.T) {
	t.Parallel()
	m, err := ExtractRawFrontmatter([]byte("---\n---\nBody.\n"))
	require.NoError(t, err)
	assert.Nil(t, m)
}

// --- ReplaceFrontmatter tests ---

func TestReplaceFrontmatter_ReplaceExisting(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: worker\nbackground: true\n---\nAgent body.\n")
	newFM := map[string]any{
		"name":          "worker",
		"is_background": true,
	}

	result, ok := ReplaceFrontmatter(input, newFM)
	require.True(t, ok)
	resultStr := string(result)

	assert.Contains(t, resultStr, "is_background: true")
	assert.Contains(t, resultStr, "name: worker")
	// The old "background:" key (without "is_" prefix) should be gone.
	// We can't use NotContains for "background:" because "is_background:"
	// contains that substring. Check the original key is absent by verifying
	// only "is_background" form appears.
	lines := strings.Split(resultStr, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "background:") {
			t.Errorf("found bare 'background:' key that should have been renamed: %q", trimmed)
		}
	}
	assert.Contains(t, resultStr, "Agent body.")
	// Should start with frontmatter
	assert.True(t, strings.HasPrefix(resultStr, "---\n"))
}

func TestReplaceFrontmatter_NoExistingFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("Plain content without frontmatter.\n")
	newFM := map[string]any{
		"name": "test",
	}

	result, ok := ReplaceFrontmatter(input, newFM)
	require.True(t, ok)
	resultStr := string(result)

	assert.True(t, strings.HasPrefix(resultStr, "---\n"))
	assert.Contains(t, resultStr, "name: test")
	assert.Contains(t, resultStr, "Plain content without frontmatter.")
}

func TestReplaceFrontmatter_PreservesBody(t *testing.T) {
	t.Parallel()
	input := []byte("---\nold: value\n---\nFirst line.\n\nSecond line.\n---\nNot frontmatter.\n")
	newFM := map[string]any{
		"new": "value",
	}

	result, ok := ReplaceFrontmatter(input, newFM)
	require.True(t, ok)
	resultStr := string(result)

	assert.Contains(t, resultStr, "new: value")
	assert.NotContains(t, resultStr, "old: value")
	assert.Contains(t, resultStr, "First line.")
	assert.Contains(t, resultStr, "Second line.")
	assert.Contains(t, resultStr, "Not frontmatter.")
}

func TestIsMarkdownFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"file.md", true},
		{"path/to/file.md", true},
		{"CLAUDE.md", true},
		{"skill.mdc", true},
		{"file.MDC", true},
		{"file.Md", true},
		{"file.txt", false},
		{"file.go", false},
		{"file.yaml", false},
		{"file", false},
		{"", false},
		{".md", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsMarkdownFile(tt.path))
		})
	}
}
