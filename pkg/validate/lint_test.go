package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Claude-specific patterns
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_ClaudeArguments(t *testing.T) {
	t.Parallel()
	content := []byte("Use $ARGUMENTS to pass data.")
	warnings := lintBodyPatterns(content, "test.md")

	require.NotEmpty(t, warnings)

	found := false
	for _, w := range warnings {
		if w.Pattern == "$ARGUMENTS" && w.Agent == "claude" {
			found = true
			assert.Equal(t, 1, w.Line)
			assert.Equal(t, "test.md", w.File)
		}
	}
	assert.True(t, found, "expected warning for $ARGUMENTS pattern with agent=claude")
}

func TestLintBodyPatterns_ClaudeIndexedArgs(t *testing.T) {
	t.Parallel()
	content := []byte("First arg is $ARGUMENTS[0] and second is $ARGUMENTS[1].")
	warnings := lintBodyPatterns(content, "skill.md")

	indexedCount := 0
	for _, w := range warnings {
		if w.Pattern == "$ARGUMENTS[N]" {
			indexedCount++
		}
	}
	assert.Equal(t, 2, indexedCount, "indexed arg matches")
}

func TestLintBodyPatterns_ClaudePositionalArgs(t *testing.T) {
	t.Parallel()
	content := []byte("The first argument $1 and second $2 are used here.\nAlso $0 is the command.")
	warnings := lintBodyPatterns(content, "cmd.md")

	positionalCount := 0
	for _, w := range warnings {
		if w.Pattern == "$N" {
			positionalCount++
		}
	}
	assert.Equal(t, 3, positionalCount, "positional arg matches")
}

func TestLintBodyPatterns_ClaudePositionalArgs_NoFalsePositive(t *testing.T) {
	t.Parallel()
	// Dollar amounts like $100, $50k should NOT trigger the positional arg pattern.
	content := []byte("The price is $100 and the budget is $50k.\nAlso $99 is cheap.")
	warnings := lintBodyPatterns(content, "pricing.md")

	for _, w := range warnings {
		assert.NotEqual(t, "$N", w.Pattern, "false positive on dollar amount at line %d", w.Line)
	}
}

func TestLintBodyPatterns_ClaudeShellExec(t *testing.T) {
	t.Parallel()
	content := []byte("Run !`echo hello` to test.")
	warnings := lintBodyPatterns(content, "test.md")

	found := false
	for _, w := range warnings {
		if w.Pattern == "!`cmd`" && w.Agent == "claude" {
			found = true
		}
	}
	assert.True(t, found, "expected warning for !`cmd` inline shell execution")
}

func TestLintBodyPatterns_ClaudeFencedShell(t *testing.T) {
	t.Parallel()
	content := []byte("```!\necho hello\n```")
	warnings := lintBodyPatterns(content, "test.md")

	found := false
	for _, w := range warnings {
		if w.Pattern == "```!" && w.Agent == "claude" {
			found = true
			assert.Equal(t, 1, w.Line)
		}
	}
	assert.True(t, found, "expected warning for ```! fenced shell execution")
}

func TestLintBodyPatterns_ClaudeVariables(t *testing.T) {
	t.Parallel()
	content := []byte("Dir is ${CLAUDE_SKILL_DIR}\nSession ${CLAUDE_SESSION_ID}\nEffort ${CLAUDE_EFFORT}")
	warnings := lintBodyPatterns(content, "test.md")

	patterns := map[string]bool{
		"${CLAUDE_SKILL_DIR}":  false,
		"${CLAUDE_SESSION_ID}": false,
		"${CLAUDE_EFFORT}":     false,
	}
	for _, w := range warnings {
		if _, ok := patterns[w.Pattern]; ok {
			patterns[w.Pattern] = true
			assert.Equal(t, "claude", w.Agent, "pattern %q agent", w.Pattern)
		}
	}
	for p, found := range patterns {
		assert.Truef(t, found, "expected warning for pattern %q", p)
	}
}

// ---------------------------------------------------------------------------
// Cursor-specific patterns
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_CursorMaestro(t *testing.T) {
	t.Parallel()
	content := []byte("Use {{user_name}} and {{project}} variables.")
	warnings := lintBodyPatterns(content, "test.md")

	maestroCount := 0
	for _, w := range warnings {
		if w.Pattern == "{{var}}" && w.Agent == "cursor" {
			maestroCount++
		}
	}
	assert.Equal(t, 2, maestroCount, "Maestro template matches")
}

// ---------------------------------------------------------------------------
// No patterns (clean content)
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_NoPatterns(t *testing.T) {
	t.Parallel()
	content := []byte("# Clean Markdown\n\nThis is perfectly portable content.\n\nNo agent-specific patterns here.")
	warnings := lintBodyPatterns(content, "clean.md")

	assert.Empty(t, warnings, "expected 0 warnings for clean content")
}

// ---------------------------------------------------------------------------
// Code block exclusion
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_InsideCodeBlock(t *testing.T) {
	t.Parallel()
	content := []byte("Some text.\n```\n$ARGUMENTS and ${CLAUDE_SKILL_DIR}\n{{variable}}\n```\nMore text.")
	warnings := lintBodyPatterns(content, "docs.md")

	assert.Empty(t, warnings, "expected 0 warnings for patterns inside code blocks")
}

func TestLintBodyPatterns_InsideCodeBlockWithLanguage(t *testing.T) {
	t.Parallel()
	content := []byte("Example:\n```bash\n$ARGUMENTS\n```\nOutside $ARGUMENTS here.")
	warnings := lintBodyPatterns(content, "test.md")

	// Only the one outside the code block should be flagged.
	require.Len(t, warnings, 1)
	assert.Equal(t, 5, warnings[0].Line, "should be on line 5 (outside code block)")
}

func TestLintBodyPatterns_FencedShellBlockIsNotCodeBlock(t *testing.T) {
	t.Parallel()
	// ```! is a Claude-only execution pattern, not a regular code fence.
	content := []byte("```!\necho hello\n```")
	warnings := lintBodyPatterns(content, "test.md")

	found := false
	for _, w := range warnings {
		if w.Pattern == "```!" {
			found = true
		}
	}
	assert.True(t, found, "expected ```! to be flagged even though it looks like a fence opener")
}

// ---------------------------------------------------------------------------
// Multiple patterns in one file
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_MultiplePatterns(t *testing.T) {
	t.Parallel()
	content := []byte("$ARGUMENTS\n${CLAUDE_SKILL_DIR}\n{{template}}\n$1")
	warnings := lintBodyPatterns(content, "mixed.md")

	assert.GreaterOrEqual(t, len(warnings), 4, "expected at least 4 warnings")

	// Check both agents are represented.
	agents := map[string]bool{}
	for _, w := range warnings {
		agents[w.Agent] = true
	}
	assert.True(t, agents["claude"], "expected at least one claude warning")
	assert.True(t, agents["cursor"], "expected at least one cursor warning")
}

// ---------------------------------------------------------------------------
// Line number accuracy
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_LineNumbers(t *testing.T) {
	t.Parallel()
	content := []byte("line one\nline two\n$ARGUMENTS on line three\nline four\n${CLAUDE_EFFORT} on line five")
	warnings := lintBodyPatterns(content, "lines.md")

	expectedLines := map[string]int{
		"$ARGUMENTS":      3,
		"${CLAUDE_EFFORT}": 5,
	}

	for _, w := range warnings {
		if expected, ok := expectedLines[w.Pattern]; ok {
			assert.Equalf(t, expected, w.Line, "pattern %q line", w.Pattern)
		}
	}
}

// ---------------------------------------------------------------------------
// lintWarning struct fields
// ---------------------------------------------------------------------------

func TestLintWarning_Fields(t *testing.T) {
	t.Parallel()
	content := []byte("$ARGUMENTS")
	warnings := lintBodyPatterns(content, "test.md")

	require.NotEmpty(t, warnings)

	w := warnings[0]
	assert.Equal(t, "test.md", w.File)
	assert.Equal(t, 1, w.Line)
	assert.Equal(t, "claude", w.Agent)
	assert.NotEmpty(t, w.Message, "Message should be non-empty")
	assert.Contains(t, w.Message, "Claude")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestLintBodyPatterns_EmptyContent(t *testing.T) {
	t.Parallel()
	warnings := lintBodyPatterns([]byte{}, "empty.md")
	assert.Empty(t, warnings)
}

func TestLintBodyPatterns_NestedCodeBlocks(t *testing.T) {
	t.Parallel()
	// Nested fences with different tick counts.
	content := []byte("````\n```\n$ARGUMENTS\n```\n````\nOutside $ARGUMENTS.")
	warnings := lintBodyPatterns(content, "nested.md")

	// Only the one on line 6 (outside all fences) should match.
	require.Len(t, warnings, 1)
	assert.Equal(t, 6, warnings[0].Line)
}

func TestLintBodyPatterns_ArgumentsIndexedNotDoubleCounted(t *testing.T) {
	t.Parallel()
	// $ARGUMENTS[0] should match $ARGUMENTS[N] but NOT also $ARGUMENTS.
	content := []byte("$ARGUMENTS[0]")
	warnings := lintBodyPatterns(content, "test.md")

	argCount := 0
	indexedCount := 0
	for _, w := range warnings {
		if w.Pattern == "$ARGUMENTS" {
			argCount++
		}
		if w.Pattern == "$ARGUMENTS[N]" {
			indexedCount++
		}
	}
	assert.Equal(t, 1, indexedCount, "$ARGUMENTS[N] matches")
	assert.Equal(t, 0, argCount, "$ARGUMENTS should not double-count indexed")
}
