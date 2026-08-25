package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// assertRecordExists checks that a FileRecord with the given output path and
// strategy exists in the records slice.
func assertRecordExists(t *testing.T, records []manifest.FileRecord, output, strategy string) {
	t.Helper()
	for _, r := range records {
		if r.Output == output && r.Strategy == strategy {
			return
		}
	}
	t.Errorf("expected file record with output=%q strategy=%q, got records: %+v", output, strategy, records)
}

func TestWriter_Instructions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Instructions: "# My Project\n\nDo the thing.\n",
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dir, ".cursor", "rules", "00-keel.mdc"))
	require.NoError(t, err)

	content := string(got)
	assert.True(t, strings.HasPrefix(content, "---\n"), "should start with frontmatter")
	assert.Contains(t, content, "alwaysApply: true")
	assert.Contains(t, content, `description: "Keel workspace instructions (assembled from config layers)"`)
	assert.Contains(t, content, "# My Project")
	assert.Contains(t, content, "Do the thing.")

	assertRecordExists(t, records, ".cursor/rules/00-keel.mdc", "user-editable")
}

func TestWriter_SeparateHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Hooks: map[string]any{
				"PreToolUse": []any{
					map[string]any{"matcher": "Write", "hooks": []string{"echo check"}},
				},
				"UserPromptSubmit": []any{
					map[string]any{"hooks": []string{"echo prompt"}},
				},
				"PostToolUse": []any{
					map[string]any{"matcher": "Bash", "hooks": []string{"echo done"}},
				},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	records, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "hooks.json"))
	require.NoError(t, err)

	var hooks map[string]any
	require.NoError(t, json.Unmarshal(data, &hooks))

	assert.Contains(t, hooks, "preToolUse", "PreToolUse should become preToolUse")
	assert.Contains(t, hooks, "postToolUse", "PostToolUse should become postToolUse")
	assert.Contains(t, hooks, "beforeSubmitPrompt", "UserPromptSubmit should become beforeSubmitPrompt")
	assert.NotContains(t, hooks, "UserPromptSubmit", "PascalCase key should not remain")

	assertRecordExists(t, records, ".cursor/hooks.json", "deep-merge")

	configPath := filepath.Join(dir, ".cursor", "cli-config.json")
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var config map[string]any
	require.NoError(t, json.Unmarshal(configData, &config))
	assert.NotContains(t, config, "hooks", "hooks should not be in cli-config.json")
}

func TestWriter_PermissionSuffix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Permissions: map[string]any{
				"allow": []any{"Read", "Write", "Bash(git:*)"},
				"deny":  []any{"Bash(rm:*)"},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	records, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "cli-config.json"))
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	allowRaw, ok := config["permissions.allow"]
	require.True(t, ok, "expected permissions.allow key")
	allowSlice, ok := allowRaw.([]any)
	require.True(t, ok)
	assert.Contains(t, allowSlice, "Read(*)", "bare Read should get (*) appended")
	assert.Contains(t, allowSlice, "Write(*)", "bare Write should get (*) appended")
	assert.Contains(t, allowSlice, "Bash(git:*)", "already-parameterized should pass through")

	denyRaw, ok := config["permissions.deny"]
	require.True(t, ok, "expected permissions.deny key")
	denySlice, ok := denyRaw.([]any)
	require.True(t, ok)
	assert.Contains(t, denySlice, "Bash(rm:*)", "already-parameterized should pass through")

	assert.Equal(t, float64(1), config["version"])

	assertRecordExists(t, records, ".cursor/cli-config.json", "deep-merge")
}

func TestWriter_PermissionSuffix_StringSlice(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Permissions: map[string]any{
				"allow": []any{"Read", "Bash(git:*)"},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	_, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "cli-config.json"))
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	allowRaw, ok := config["permissions.allow"]
	require.True(t, ok, "expected permissions.allow key")
	allowSlice, ok := allowRaw.([]any)
	require.True(t, ok)
	assert.Contains(t, allowSlice, "Read(*)")
	assert.Contains(t, allowSlice, "Bash(git:*)")
}

func TestWriter_MCP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			MCP: map[string]any{
				"mcpServers": map[string]any{
					"context7": map[string]any{
						"command": "npx",
						"args":    []string{"-y", "@upstash/context7-mcp"},
					},
				},
			},
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "mcp.json"))
	require.NoError(t, err)

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	assert.Contains(t, mcp, "mcpServers")

	assertRecordExists(t, records, ".cursor/mcp.json", "deep-merge")
}

func TestWriter_Environment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			ConfigBlocks: map[string]map[string]any{
				"environment": {
					"variables": map[string]any{
						"NODE_ENV":  "development",
						"LOG_LEVEL": "debug",
					},
				},
			},
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "environment.json"))
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal(data, &env))
	assert.Contains(t, env, "variables")

	assertRecordExists(t, records, ".cursor/environment.json", "deep-merge")
}

func TestWriter_EnvironmentNotWrittenWhenEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{}

	_, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ".cursor", "environment.json"))
	assert.True(t, os.IsNotExist(err), "environment.json should not be written when empty")
}

func TestWriter_Categories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Categories: map[string][]compiler.CategoryFile{
				"commands": {
					{RelPath: "deploy.md", Content: []byte("# Deploy")},
				},
				"rules": {
					{RelPath: "style.mdc", Content: []byte("---\nalwaysApply: true\n---\nUse clean code.")},
				},
				"agents": {
					{RelPath: "reviewer.md", Content: []byte("# Reviewer")},
				},
				"skills": {
					{RelPath: "debug.md", Content: []byte("# Debug")},
				},
				"plugins": {
					{RelPath: "my-plugin.md", Content: []byte("# Plugin")},
				},
				"agent-memory": {
					{RelPath: "memory.md", Content: []byte("# Memory")},
				},
			},
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dir, ".cursor", "commands", "deploy.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Deploy", string(got))

	got, err = os.ReadFile(filepath.Join(dir, ".cursor", "rules", "style.mdc"))
	require.NoError(t, err)
	assert.Contains(t, string(got), "Use clean code.")

	got, err = os.ReadFile(filepath.Join(dir, ".cursor", "agents", "reviewer.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Reviewer", string(got))

	got, err = os.ReadFile(filepath.Join(dir, ".cursor", "skills", "debug.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Debug", string(got))

	_, err = os.Stat(filepath.Join(dir, ".cursor", "plugins"))
	assert.True(t, os.IsNotExist(err), "plugins dir should not exist for Cursor")

	_, err = os.Stat(filepath.Join(dir, ".cursor", "agent-memory"))
	assert.True(t, os.IsNotExist(err), "agent-memory dir should not exist for Cursor")

	// Now expects per-file records instead of directory-level records
	assertRecordExists(t, records, ".cursor/commands/deploy.md", "overlay")
	assertRecordExists(t, records, ".cursor/rules/style.mdc", "overlay")
	assertRecordExists(t, records, ".cursor/agents/reviewer.md", "overlay")
	assertRecordExists(t, records, ".cursor/skills/debug.md", "overlay")
}

func TestWriter_VersionAlwaysPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	_, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "cli-config.json"))
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))
	assert.Equal(t, float64(1), config["version"])
}

func TestWriter_AgentSpecificMerged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Settings: map[string]any{
				"model": "claude-sonnet-4-20250514",
			},
			AgentSpecific: map[string]map[string]any{
				"cursor": {
					"editor.vimMode": true,
				},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	_, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "cli-config.json"))
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	assert.Equal(t, "claude-sonnet-4-20250514", config["model"])
	assert.Equal(t, true, config["editor.vimMode"])
	assert.Equal(t, float64(1), config["version"])
}

func TestWriter_BinFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Bin: []compiler.BinFile{
				{RelPath: "mytool", Content: []byte("#!/bin/sh\necho hi"), Mode: 0755},
			},
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dir, ".keel", "bin", "mytool"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho hi", string(got))

	// Now expects per-file record instead of directory-level record
	assertRecordExists(t, records, ".keel/bin/mytool", "overlay")
}

func TestWriter_StateDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{}
	_, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	gitignore, err := os.ReadFile(filepath.Join(dir, ".keel", "state", ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "*\n", string(gitignore))
}

func TestWriter_HookKeyTransform(t *testing.T) {
	t.Parallel()
	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Hooks: map[string]any{
				"PreToolUse":       "val1",
				"PostToolUse":      "val2",
				"UserPromptSubmit": "val3",
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	assert.Equal(t, "val1", result.Content.Hooks["preToolUse"])
	assert.Equal(t, "val2", result.Content.Hooks["postToolUse"])
	assert.Equal(t, "val3", result.Content.Hooks["beforeSubmitPrompt"])
	assert.NotContains(t, result.Content.Hooks, "PreToolUse")
	assert.NotContains(t, result.Content.Hooks, "PostToolUse")
	assert.NotContains(t, result.Content.Hooks, "UserPromptSubmit")
}

// --- Profile tests ---

func TestProfile_UserScope(t *testing.T) {
	t.Parallel()
	project := ProjectProfile()
	user := UserProfile()

	assert.Empty(t, user.InstructionOutput)
	assert.Empty(t, user.ConfigOutputs)

	for _, cat := range user.Categories {
		assert.NotEqual(t, "rules", cat.DirName, "rules category should be removed in user scope")
	}
	assert.Len(t, user.Categories, len(project.Categories)-1)

	assert.Equal(t, project.Name, user.Name)
	assert.Equal(t, project.OutputRoot, user.OutputRoot)
	assert.Equal(t, project.MCPOutput, user.MCPOutput)
	assert.Equal(t, project.SettingsFile, user.SettingsFile)
}

func TestFilterOut_Category(t *testing.T) {
	t.Parallel()
	cats := []compiler.CategoryDef{
		{DirName: "commands", LockCategory: "command"},
		{DirName: "agents", LockCategory: "agent"},
		{DirName: "rules", LockCategory: "rule"},
	}

	nameFunc := func(c compiler.CategoryDef) string { return c.DirName }

	filtered := compiler.FilterOut(cats, nameFunc, "rules")
	require.Len(t, filtered, 2)
	for _, c := range filtered {
		assert.NotEqual(t, "rules", c.DirName, "rules should be filtered out")
	}

	assert.Len(t, cats, 3, "original slice was mutated")

	same := compiler.FilterOut(cats, nameFunc, "nonexistent")
	assert.Len(t, same, 3, "filtering non-existent should return all")
}

func TestProfile_ScopeDispatch(t *testing.T) {
	t.Parallel()
	p := Profile(compiler.ScopeProject)
	assert.NotEmpty(t, p.InstructionOutput)

	u := Profile(compiler.ScopeUser)
	assert.Empty(t, u.InstructionOutput)

	d := Profile("")
	assert.NotEmpty(t, d.InstructionOutput)
}

// --- Helper unit tests ---

func TestTranslatePermissionToCursor(t *testing.T) {
	t.Parallel()
	tests := []struct{ input, expected string }{
		{"Read", "Read(*)"},
		{"Write", "Write(*)"},
		{"Edit", "Edit(*)"},
		{"Bash(git:*)", "Bash(git:*)"},
		{"Bash(npm:*)", "Bash(npm:*)"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, TranslatePermissionToCursor(tt.input))
	}
}

func TestPascalToCamel(t *testing.T) {
	t.Parallel()
	tests := []struct{ input, expected string }{
		{"PreToolUse", "preToolUse"},
		{"PostToolUse", "postToolUse"},
		{"SessionStart", "sessionStart"},
		{"", ""},
		{"a", "a"},
		{"A", "a"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, PascalToCamel(tt.input))
	}
}
