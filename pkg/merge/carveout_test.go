package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessConditionals_NoMarkers(t *testing.T) {
	t.Parallel()
	input := []byte("Just plain markdown.\n\nNo markers here.\n")
	got := ProcessConditionals(input, map[string]string{"keel.agent": "claude"})
	assert.Equal(t, string(input), string(got), "content should be unchanged when no markers")
}

func TestProcessConditionals_EmptyVars(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.agent == 'claude' -->\nClaude stuff.\n<!-- keel-endif -->\n")
	got := ProcessConditionals(input, nil)
	assert.Equal(t, string(input), string(got), "content should be unchanged when vars is nil")
}

func TestProcessConditionals_IfMatch(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nClaude only.\n<!-- keel-endif -->\n\nOutro.\n")
	got := ProcessConditionals(input, map[string]string{"keel.agent": "claude"})
	assert.Equal(t, "Intro.\n\nClaude only.\n\nOutro.\n", string(got))
}

func TestProcessConditionals_IfNoMatch(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nClaude only.\n<!-- keel-endif -->\n\nOutro.\n")
	got := ProcessConditionals(input, map[string]string{"keel.agent": "cursor"})
	assert.Equal(t, "Intro.\n\n\nOutro.\n", string(got))
}

func TestProcessConditionals_IfElifElse(t *testing.T) {
	t.Parallel()
	input := []byte("Shared intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nUse Agent Teams.\n<!-- keel-elif: keel.agent == 'cursor' -->\nUse coordination MCP.\n<!-- keel-else -->\nUse sequential.\n<!-- keel-endif -->\n\nShared outro.\n")
	tests := []struct {
		name, agent, want string
	}{
		{"claude gets its block", "claude", "Shared intro.\n\nUse Agent Teams.\n\nShared outro.\n"},
		{"cursor gets its block", "cursor", "Shared intro.\n\nUse coordination MCP.\n\nShared outro.\n"},
		{"gemini falls to else", "gemini", "Shared intro.\n\nUse sequential.\n\nShared outro.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ProcessConditionals(input, map[string]string{"keel.agent": tt.agent})
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestProcessConditionals_IfWithoutElse(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.agent == 'claude' -->\nClaude only.\n<!-- keel-endif -->\n")
	t.Run("matching", func(t *testing.T) {
		t.Parallel()
		got := ProcessConditionals(input, map[string]string{"keel.agent": "claude"})
		assert.Equal(t, "Claude only.\n", string(got))
	})
	t.Run("non-matching", func(t *testing.T) {
		t.Parallel()
		got := ProcessConditionals(input, map[string]string{"keel.agent": "gemini"})
		assert.Equal(t, "", string(got))
	})
}

func TestProcessConditionals_MultipleBlocks(t *testing.T) {
	t.Parallel()
	input := []byte("Header.\n\n<!-- keel-if: keel.agent == 'claude' -->\nGroup1 Claude.\n<!-- keel-elif: keel.agent == 'cursor' -->\nGroup1 Cursor.\n<!-- keel-endif -->\n\nMiddle.\n\n<!-- keel-if: keel.agent == 'claude' -->\nGroup2 Claude.\n<!-- keel-else -->\nGroup2 Other.\n<!-- keel-endif -->\n\nFooter.\n")
	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		got := ProcessConditionals(input, map[string]string{"keel.agent": "claude"})
		assert.Equal(t, "Header.\n\nGroup1 Claude.\n\nMiddle.\n\nGroup2 Claude.\n\nFooter.\n", string(got))
	})
	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		got := ProcessConditionals(input, map[string]string{"keel.agent": "cursor"})
		assert.Equal(t, "Header.\n\nGroup1 Cursor.\n\nMiddle.\n\nGroup2 Other.\n\nFooter.\n", string(got))
	})
}

func TestProcessConditionals_EmptyContent(t *testing.T) {
	t.Parallel()
	vars := map[string]string{"keel.agent": "claude"}
	assert.Empty(t, ProcessConditionals([]byte{}, vars))
	assert.Empty(t, ProcessConditionals(nil, vars))
}

func TestProcessConditionals_CodeBlockNotParsed(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n```markdown\n<!-- keel-if: keel.agent == 'claude' -->\nThis is an example.\n<!-- keel-endif -->\n```\n\nOutro.\n")
	got := ProcessConditionals(input, map[string]string{"keel.agent": "cursor"})
	assert.Equal(t, string(input), string(got), "markers inside code blocks should not be parsed")
}

func TestProcessConditionals_CodeBlockWithRealMarkers(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n```\n<!-- keel-if: keel.agent == 'claude' -->\nCode example.\n<!-- keel-endif -->\n```\n\n<!-- keel-if: keel.agent == 'claude' -->\nReal claude content.\n<!-- keel-elif: keel.agent == 'cursor' -->\nReal cursor content.\n<!-- keel-endif -->\n\nOutro.\n")
	got := string(ProcessConditionals(input, map[string]string{"keel.agent": "claude"}))
	assert.Contains(t, got, "<!-- keel-if: keel.agent == 'claude' -->", "marker inside code block should be preserved")
	assert.Contains(t, got, "Code example.")
	assert.Contains(t, got, "Real claude content.")
	assert.NotContains(t, got, "Real cursor content.")
}

func TestProcessConditionals_NotEquals(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.agent != 'cursor' -->\nNot cursor.\n<!-- keel-endif -->\n")
	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Not cursor.\n", string(ProcessConditionals(input, map[string]string{"keel.agent": "claude"})))
	})
	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", string(ProcessConditionals(input, map[string]string{"keel.agent": "cursor"})))
	})
}

func TestProcessConditionals_Nesting(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.agent == 'claude' -->\nouter claude\n<!-- keel-if: keel.scope == 'project' -->\ninner project\n<!-- keel-else -->\ninner user\n<!-- keel-endif -->\n<!-- keel-endif -->\n")
	tests := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"project scope", map[string]string{"keel.agent": "claude", "keel.scope": "project"}, "outer claude\ninner project\n"},
		{"user scope", map[string]string{"keel.agent": "claude", "keel.scope": "user"}, "outer claude\ninner user\n"},
		{"outer no match", map[string]string{"keel.agent": "cursor", "keel.scope": "project"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, string(ProcessConditionals(input, tt.vars)))
		})
	}
}

func TestProcessConditionals_MarkersStripped(t *testing.T) {
	t.Parallel()
	input := []byte("Before.\n<!-- keel-if: keel.agent == 'claude' -->\nContent.\n<!-- keel-endif -->\nAfter.\n")
	got := string(ProcessConditionals(input, map[string]string{"keel.agent": "claude"}))
	assert.NotContains(t, got, "keel-if")
	assert.NotContains(t, got, "keel-endif")
}

func TestProcessConditionals_WindowsLineEndings(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\r\n\r\n<!-- keel-if: keel.agent == 'claude' -->\r\nClaude stuff.\r\n<!-- keel-elif: keel.agent == 'cursor' -->\r\nCursor stuff.\r\n<!-- keel-endif -->\r\n\r\nOutro.\r\n")
	got := ProcessConditionals(input, map[string]string{"keel.agent": "claude"})
	assert.Equal(t, "Intro.\n\nClaude stuff.\n\nOutro.\n", string(got))
}

func TestFilterConditionJSON_NoKeelKey(t *testing.T) {
	t.Parallel()
	input := map[string]any{"gitlab": map[string]any{"command": "gitlab-mcp"}, "context7": map[string]any{"command": "npx"}}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestFilterConditionJSON_EmptyVars(t *testing.T) {
	t.Parallel()
	input := map[string]any{"tool": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'"}, "command": "node"}}
	got, err := FilterConditionJSON(input, nil)
	require.NoError(t, err)
	tool := got["tool"].(map[string]any)
	assert.Contains(t, tool, "_keel", "_keel should be preserved when vars is empty")
}

func TestFilterConditionJSON_ConditionMatch(t *testing.T) {
	t.Parallel()
	input := map[string]any{"claude-server": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'"}, "command": "claude-tool"}}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	server, ok := got["claude-server"].(map[string]any)
	require.True(t, ok, "claude-server should be kept when condition matches")
	assert.NotContains(t, server, "_keel")
	assert.Equal(t, "claude-tool", server["command"])
}

func TestFilterConditionJSON_ConditionNoMatch(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"claude-server": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'"}, "command": "claude-tool"},
		"universal":     map[string]any{"command": "universal-tool"},
	}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "cursor"})
	require.NoError(t, err)
	assert.NotContains(t, got, "claude-server")
	assert.Contains(t, got, "universal")
}

func TestFilterConditionJSON_NotEqualsOperator(t *testing.T) {
	t.Parallel()
	input := map[string]any{"not-cursor": map[string]any{"_keel": map[string]any{"condition": "keel.agent != 'cursor'"}, "command": "any-but-cursor"}}
	t.Run("claude keeps entry", func(t *testing.T) {
		t.Parallel()
		got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
		require.NoError(t, err)
		assert.Contains(t, got, "not-cursor")
	})
	t.Run("cursor removes entry", func(t *testing.T) {
		t.Parallel()
		got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "cursor"})
		require.NoError(t, err)
		assert.NotContains(t, got, "not-cursor")
	})
}

func TestFilterConditionJSON_OtherKeelFieldsPreserved(t *testing.T) {
	t.Parallel()
	input := map[string]any{"tool": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'", "force": true}, "command": "some-tool"}}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	tool := got["tool"].(map[string]any)
	keel, ok := tool["_keel"].(map[string]any)
	require.True(t, ok, "_keel should remain because 'force' field still present")
	assert.Equal(t, true, keel["force"])
	assert.NotContains(t, keel, "condition")
}

func TestFilterConditionJSON_NestedObjects(t *testing.T) {
	t.Parallel()
	input := map[string]any{"servers": map[string]any{
		"inner": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'cursor'"}, "command": "inner-cmd"},
		"kept":  map[string]any{"command": "kept-cmd"},
	}}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	servers := got["servers"].(map[string]any)
	assert.NotContains(t, servers, "inner")
	assert.Contains(t, servers, "kept")
}

func TestFilterConditionJSON_MultipleEntries(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"claude-only": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'"}, "command": "claude-tool"},
		"cursor-only": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'cursor'"}, "command": "cursor-tool"},
		"universal":   map[string]any{"command": "universal-tool"},
	}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	assert.Contains(t, got, "claude-only")
	assert.NotContains(t, got, "cursor-only")
	assert.Contains(t, got, "universal")
}

func TestFilterConditionJSON_NilMap(t *testing.T) {
	t.Parallel()
	got, err := FilterConditionJSON(nil, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestFilterConditionJSON_InputNotMutated(t *testing.T) {
	t.Parallel()
	input := map[string]any{"tool": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'"}, "command": "some-tool"}}
	_, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	tool := input["tool"].(map[string]any)
	assert.Contains(t, tool, "_keel", "input map should not be mutated")
}

func TestFilterConditionJSON_EmptyKeelAfterStrip(t *testing.T) {
	t.Parallel()
	input := map[string]any{"tool": map[string]any{"_keel": map[string]any{"condition": "keel.agent == 'claude'"}, "command": "some-tool"}}
	got, err := FilterConditionJSON(input, map[string]string{"keel.agent": "claude"})
	require.NoError(t, err)
	tool := got["tool"].(map[string]any)
	assert.NotContains(t, tool, "_keel", "_keel should be removed when empty after stripping condition")
}

func TestFilterConditionJSON_MaxDepth(t *testing.T) {
	t.Parallel()
	// Create a deeply nested map (depth > 128) to trigger the depth guard
	data := make(map[string]any)
	current := data
	for i := 0; i < 130; i++ {
		nested := make(map[string]any)
		current["level"] = nested
		current = nested
	}
	current["value"] = "deep"

	_, err := FilterConditionJSON(data, map[string]string{"keel.agent": "claude"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded maximum nesting depth")
}

func TestSplitAgentConditionals_SimpleAgentConditional(t *testing.T) {
	t.Parallel()
	input := []byte("Shared intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nClaude-specific content.\n<!-- keel-endif -->\n\nShared outro.\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	assert.Equal(t, "Shared intro.\n\n\nShared outro.\n", string(universal))
	require.Contains(t, agentAddenda, "claude")
	assert.Equal(t, "<!-- keel-if: keel.agent == 'claude' -->\nClaude-specific content.\n<!-- keel-endif -->", string(agentAddenda["claude"]))
}

func TestSplitAgentConditionals_MultipleAgentsElif(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nClaude content.\n<!-- keel-elif: keel.agent == 'cursor' -->\nCursor content.\n<!-- keel-endif -->\n\nOutro.\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	assert.Equal(t, "Intro.\n\n\nOutro.\n", string(universal))
	require.Contains(t, agentAddenda, "claude")
	require.Contains(t, agentAddenda, "cursor")
	assert.Equal(t, "<!-- keel-if: keel.agent == 'claude' -->\nClaude content.", string(agentAddenda["claude"]))
	assert.Equal(t, "<!-- keel-elif: keel.agent == 'cursor' -->\nCursor content.\n<!-- keel-endif -->", string(agentAddenda["cursor"]))
}

func TestSplitAgentConditionals_MixedAgentAndNonAgent(t *testing.T) {
	t.Parallel()
	input := []byte("Start.\n\n<!-- keel-if: keel.agent == 'claude' && keel.scope == 'project' -->\nClaude project-only.\n<!-- keel-endif -->\n\n<!-- keel-if: keel.scope == 'user' -->\nUser-scope content.\n<!-- keel-endif -->\n\nEnd.\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	// Non-agent conditional stays in universal
	assert.Contains(t, string(universal), "<!-- keel-if: keel.scope == 'user' -->")
	assert.Contains(t, string(universal), "User-scope content.")
	assert.Contains(t, string(universal), "Start.")
	assert.Contains(t, string(universal), "End.")
	// Agent conditional goes to addenda
	require.Contains(t, agentAddenda, "claude")
	assert.Contains(t, string(agentAddenda["claude"]), "Claude project-only.")
}

func TestSplitAgentConditionals_ElseGoesToUniversal(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.agent == 'claude' -->\nClaude content.\n<!-- keel-else -->\nFallback for other agents.\n<!-- keel-endif -->\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	require.Contains(t, agentAddenda, "claude")
	assert.Equal(t, "<!-- keel-if: keel.agent == 'claude' -->\nClaude content.", string(agentAddenda["claude"]))
	assert.Contains(t, string(universal), "<!-- keel-else -->")
	assert.Contains(t, string(universal), "Fallback for other agents.")
}

func TestSplitAgentConditionals_NoConditionals(t *testing.T) {
	t.Parallel()
	input := []byte("Just plain content.\n\nNo markers here.\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	assert.Equal(t, string(input), string(universal))
	assert.Empty(t, agentAddenda)
}

func TestSplitAgentConditionals_NonAgentConditionalOnly(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.scope == 'project' -->\nProject content.\n<!-- keel-endif -->\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	assert.Equal(t, string(input), string(universal))
	assert.Empty(t, agentAddenda)
}

func TestSplitAgentConditionals_NestedConditionals(t *testing.T) {
	t.Parallel()
	input := []byte("<!-- keel-if: keel.agent == 'claude' -->\nOuter claude.\n<!-- keel-if: keel.scope == 'project' -->\nInner project.\n<!-- keel-endif -->\n<!-- keel-endif -->\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	assert.Empty(t, string(universal))
	require.Contains(t, agentAddenda, "claude")
	assert.Contains(t, string(agentAddenda["claude"]), "Outer claude.")
	assert.Contains(t, string(agentAddenda["claude"]), "Inner project.")
}

func TestSplitAgentConditionals_CodeBlockNotParsed(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n```\n<!-- keel-if: keel.agent == 'claude' -->\nExample code.\n<!-- keel-endif -->\n```\n\nOutro.\n")
	universal, agentAddenda := SplitAgentConditionals(input)
	assert.Equal(t, string(input), string(universal))
	assert.Empty(t, agentAddenda)
}

func TestSplitAgentConditionals_EmptyContent(t *testing.T) {
	t.Parallel()
	universal, agentAddenda := SplitAgentConditionals([]byte{})
	assert.Empty(t, universal)
	assert.Nil(t, agentAddenda)
}

func TestSplitAgentConditionJSON_NoConditions(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"gitlab":   map[string]any{"command": "gitlab-mcp"},
		"context7": map[string]any{"command": "npx"},
	}
	universal, agentSpecific := SplitAgentConditionJSON(input)
	assert.Len(t, universal, 2)
	assert.Contains(t, universal, "gitlab")
	assert.Contains(t, universal, "context7")
	assert.Empty(t, agentSpecific)
}

func TestSplitAgentConditionJSON_AgentCondition(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"claude-server": map[string]any{
			"_keel":   map[string]any{"condition": "keel.agent == 'claude'"},
			"command": "claude-tool",
		},
		"universal": map[string]any{"command": "universal-tool"},
	}
	universal, agentSpecific := SplitAgentConditionJSON(input)
	assert.Len(t, universal, 1)
	assert.Contains(t, universal, "universal")
	require.Contains(t, agentSpecific, "claude")
	assert.Contains(t, agentSpecific["claude"], "claude-server")
}

func TestSplitAgentConditionJSON_MultipleAgents(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"claude-server": map[string]any{
			"_keel":   map[string]any{"condition": "keel.agent == 'claude'"},
			"command": "claude-tool",
		},
		"cursor-server": map[string]any{
			"_keel":   map[string]any{"condition": "keel.agent == 'cursor'"},
			"command": "cursor-tool",
		},
		"shared": map[string]any{"command": "shared-tool"},
	}
	universal, agentSpecific := SplitAgentConditionJSON(input)
	assert.Len(t, universal, 1)
	assert.Contains(t, universal, "shared")
	require.Contains(t, agentSpecific, "claude")
	require.Contains(t, agentSpecific, "cursor")
	assert.Contains(t, agentSpecific["claude"], "claude-server")
	assert.Contains(t, agentSpecific["cursor"], "cursor-server")
}

func TestSplitAgentConditionJSON_NonAgentCondition(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"project-tool": map[string]any{
			"_keel":   map[string]any{"condition": "keel.scope == 'project'"},
			"command": "project-cmd",
		},
	}
	universal, agentSpecific := SplitAgentConditionJSON(input)
	assert.Len(t, universal, 1)
	assert.Contains(t, universal, "project-tool")
	assert.Empty(t, agentSpecific)
}

func TestSplitAgentConditionJSON_MixedCondition(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"claude-project": map[string]any{
			"_keel":   map[string]any{"condition": "keel.agent == 'claude' && keel.scope == 'project'"},
			"command": "claude-project-tool",
		},
	}
	universal, agentSpecific := SplitAgentConditionJSON(input)
	assert.Empty(t, universal)
	require.Contains(t, agentSpecific, "claude")
	assert.Contains(t, agentSpecific["claude"], "claude-project")
}

func TestSplitAgentConditionJSON_NoKeelKey(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"tool": map[string]any{"command": "some-cmd"},
	}
	universal, agentSpecific := SplitAgentConditionJSON(input)
	assert.Len(t, universal, 1)
	assert.Contains(t, universal, "tool")
	assert.Empty(t, agentSpecific)
}

func TestSplitAgentConditionJSON_NilMap(t *testing.T) {
	t.Parallel()
	universal, agentSpecific := SplitAgentConditionJSON(nil)
	assert.Nil(t, universal)
	assert.Nil(t, agentSpecific)
}

func TestSplitAgentConditionJSON_PreservesKeelMetadata(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"claude-server": map[string]any{
			"_keel": map[string]any{
				"condition": "keel.agent == 'claude'",
				"force":     true,
			},
			"command": "claude-tool",
		},
	}
	_, agentSpecific := SplitAgentConditionJSON(input)
	require.Contains(t, agentSpecific, "claude")
	server := agentSpecific["claude"]["claude-server"].(map[string]any)
	keel := server["_keel"].(map[string]any)
	assert.Equal(t, "keel.agent == 'claude'", keel["condition"])
	assert.Equal(t, true, keel["force"])
}

func TestSplitAgentConditionals_MultipleBlocksSameAgent(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nFirst Claude block.\n<!-- keel-endif -->\n\nMiddle universal.\n\n<!-- keel-if: keel.agent == 'claude' -->\nSecond Claude block.\n<!-- keel-endif -->\n\nOutro.\n")
	universal, agentAddenda := SplitAgentConditionals(input)

	// Both blocks should be in the claude addenda (appended, not overwritten)
	require.Contains(t, agentAddenda, "claude")
	claudeContent := string(agentAddenda["claude"])
	assert.Contains(t, claudeContent, "First Claude block.")
	assert.Contains(t, claudeContent, "Second Claude block.")

	// Universal should have intro, middle, and outro
	uniStr := string(universal)
	assert.Contains(t, uniStr, "Intro.")
	assert.Contains(t, uniStr, "Middle universal.")
	assert.Contains(t, uniStr, "Outro.")
}

func TestSplitAgentConditionals_UnclosedBlock(t *testing.T) {
	t.Parallel()
	input := []byte("Intro.\n\n<!-- keel-if: keel.agent == 'claude' -->\nUnclosed Claude content.\nMore content.\n")
	universal, agentAddenda := SplitAgentConditionals(input)

	// Unclosed block should be flushed to addenda, not lost
	require.Contains(t, agentAddenda, "claude")
	assert.Contains(t, string(agentAddenda["claude"]), "Unclosed Claude content.")
	assert.Contains(t, string(agentAddenda["claude"]), "More content.")

	// Universal should have the intro
	assert.Contains(t, string(universal), "Intro.")
}
