package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
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

	got, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, "# My Project\n\nDo the thing.\n", string(got))

	assertRecordExists(t, records, "CLAUDE.md", "user-editable")
}

func TestWriter_SettingsWithHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Settings: map[string]any{
				"model": "claude-sonnet-4-20250514",
			},
			Hooks: map[string]any{
				"PreToolUse": []any{
					map[string]any{"matcher": "Write", "hooks": []string{"echo check"}},
				},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	records, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	assert.Equal(t, "claude-sonnet-4-20250514", settings["model"])
	assert.NotNil(t, settings["hooks"], "hooks should be embedded in settings.json")

	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "hooks should be a map")
	assert.Contains(t, hooks, "PreToolUse")

	assertRecordExists(t, records, ".claude/settings.json", "deep-merge")
}

func TestWriter_Permissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Permissions: map[string]any{
				"allow": []any{"Read", "Write"},
				"deny":  []any{"Bash(rm:*)"},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	records, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "permissions should be nested in settings")
	assert.Contains(t, perms, "allow")
	assert.Contains(t, perms, "deny")

	assertRecordExists(t, records, ".claude/settings.json", "deep-merge")
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

	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	require.NoError(t, err)

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	assert.Contains(t, mcp, "mcpServers")

	assertRecordExists(t, records, ".mcp.json", "deep-merge")
}

func TestWriter_Categories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Categories: map[string][]compiler.CategoryFile{
				"commands": {
					{RelPath: "deploy.md", Content: []byte("# Deploy\nRun deploy.")},
				},
				"skills": {
					{RelPath: "security", IsDir: true},
					{RelPath: filepath.Join("security", "SKILL.md"), Content: []byte("# Security Skill")},
				},
				"agents": {
					{RelPath: "reviewer.md", Content: []byte("# Reviewer Agent")},
				},
			},
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	// Verify command file
	got, err := os.ReadFile(filepath.Join(dir, ".claude", "commands", "deploy.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Deploy\nRun deploy.", string(got))

	// Verify skill dir and file
	info, err := os.Stat(filepath.Join(dir, ".claude", "skills", "security"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	got, err = os.ReadFile(filepath.Join(dir, ".claude", "skills", "security", "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Security Skill", string(got))

	// Verify agent file
	got, err = os.ReadFile(filepath.Join(dir, ".claude", "agents", "reviewer.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Reviewer Agent", string(got))

	// Now expects per-file records instead of directory-level records
	assertRecordExists(t, records, ".claude/commands/deploy.md", "overlay")
	assertRecordExists(t, records, ".claude/skills/security/SKILL.md", "overlay")
	assertRecordExists(t, records, ".claude/agents/reviewer.md", "overlay")
}

func TestWriter_MCPPolicy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Governance: compiler.GovernanceOutput{
			MCPPolicy: &schema.MCPPolicy{
				Allowed:   []string{"context7", "pencil"},
				Denied:    []string{"evil-server"},
				EnableAll: new(bool),
			},
		},
	}
	*result.Governance.MCPPolicy.EnableAll = true

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	records, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	allowed, ok := settings["allowedMcpServers"]
	require.True(t, ok, "expected allowedMcpServers key")
	allowedSlice, ok := allowed.([]any)
	require.True(t, ok)
	assert.Contains(t, allowedSlice, "context7")
	assert.Contains(t, allowedSlice, "pencil")

	denied, ok := settings["deniedMcpServers"]
	require.True(t, ok, "expected deniedMcpServers key")
	deniedSlice, ok := denied.([]any)
	require.True(t, ok)
	assert.Contains(t, deniedSlice, "evil-server")

	enableAllVal, ok := settings["enableAllProjectMcpServers"]
	require.True(t, ok, "expected enableAllProjectMcpServers key")
	assert.Equal(t, true, enableAllVal)

	assertRecordExists(t, records, ".claude/settings.json", "deep-merge")
}

func TestWriter_EmptyResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(err), "CLAUDE.md should not be written when instructions are empty")

	_, err = os.Stat(filepath.Join(dir, ".claude", "settings.json"))
	assert.True(t, os.IsNotExist(err), "settings.json should not be written when empty")

	_, err = os.Stat(filepath.Join(dir, ".mcp.json"))
	assert.True(t, os.IsNotExist(err), ".mcp.json should not be written when MCP is empty")

	_, err = os.Stat(filepath.Join(dir, ".keel", "state"))
	assert.NoError(t, err, ".keel/state/ should always be created")

	assert.Empty(t, records, "no file records expected for empty result (besides state)")
}

func TestWriter_BinFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			Bin: []compiler.BinFile{
				{RelPath: "mytool", Content: []byte("#!/bin/sh\necho hi"), Mode: 0755},
				{RelPath: filepath.Join("sub", "helper"), Content: []byte("#!/bin/sh\necho help"), Mode: 0755},
			},
		},
	}

	records, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dir, ".keel", "bin", "mytool"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho hi", string(got))

	got, err = os.ReadFile(filepath.Join(dir, ".keel", "bin", "sub", "helper"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho help", string(got))

	// Now expects per-file records instead of directory-level record
	assertRecordExists(t, records, ".keel/bin/mytool", "overlay")
	assertRecordExists(t, records, ".keel/bin/sub/helper", "overlay")
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
				"claude": {
					"sandbox": map[string]any{"enabled": true},
					"env":     map[string]any{"FOO": "bar"},
				},
			},
		},
	}

	profile := Profile(compiler.ScopeProject)
	compiler.AssembleAgentOutput(result, profile)

	_, err := w.Write(result, profile, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	assert.Equal(t, "claude-sonnet-4-20250514", settings["model"])
	assert.NotNil(t, settings["sandbox"])
	assert.NotNil(t, settings["env"])
}

func TestWriter_NoEnvironmentFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w := &Writer{}

	result := &compiler.AccumulatedResult{
		Content: compiler.CompilationContent{
			ConfigBlocks: map[string]map[string]any{
				"environment": {
					"SOME_VAR": "value",
				},
			},
		},
	}

	_, err := w.Write(result, Profile(compiler.ScopeProject), dir, compiler.CompileOptions{})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ".claude", "environment.json"))
	assert.True(t, os.IsNotExist(err), "Claude should not write environment.json")
}

// --- Profile tests ---

func TestProfile_UserScope(t *testing.T) {
	t.Parallel()
	project := ProjectProfile()
	user := UserProfile()

	assert.Equal(t, ".claude.json", user.MCPOutput)
	assert.Equal(t, project.Name, user.Name)
	assert.Equal(t, project.OutputRoot, user.OutputRoot)
	assert.Equal(t, ".claude/CLAUDE.md", user.InstructionOutput)
	assert.Equal(t, ".claude/AGENTS.md", user.SharedInstructionOutput)
	assert.Equal(t, project.SettingsFile, user.SettingsFile)
	assert.Equal(t, project.HooksInSettings, user.HooksInSettings)
	assert.Len(t, user.Categories, len(project.Categories))
}

func TestProfile_ScopeDispatch(t *testing.T) {
	t.Parallel()
	p := Profile(compiler.ScopeProject)
	assert.Equal(t, ".mcp.json", p.MCPOutput)

	u := Profile(compiler.ScopeUser)
	assert.Equal(t, ".claude.json", u.MCPOutput)

	d := Profile("")
	assert.Equal(t, ".mcp.json", d.MCPOutput)
}
