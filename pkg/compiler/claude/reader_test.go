package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdataDir returns the path to the parent compiler package's testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "unable to determine test file location")
	// From pkg/compiler/claude/ go up to pkg/compiler/testdata
	return filepath.Join(filepath.Dir(filename), "..", "testdata")
}

// readerTestdataDir returns the path to the reader-claude testdata directory.
func readerTestdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testdataDir(t), "reader-claude")
}

func TestReader_Full(t *testing.T) {
	t.Parallel()
	reader := &Reader{}
	layerPath := filepath.Join(readerTestdataDir(t), "full")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	assert.Equal(t, "claude", ir.SourceAgent)

	require.Len(t, ir.Instructions, 1)
	assert.Equal(t, "CLAUDE.md", ir.Instructions[0].Filename)
	assert.Contains(t, ir.Instructions[0].Content, "Test Project")

	require.NotNil(t, ir.Permissions)
	assert.Contains(t, ir.Permissions, "allow")
	assert.Contains(t, ir.Permissions, "deny")

	require.NotNil(t, ir.Hooks)
	assert.Contains(t, ir.Hooks, "PreToolUse")

	require.NotNil(t, ir.Settings)
	model, ok := ir.Settings["model"]
	require.True(t, ok, "expected Settings to have 'model' key")
	assert.Equal(t, "claude-sonnet-4-20250514", model)

	require.NotNil(t, ir.AgentSpecific)
	claudeSpecific, ok := ir.AgentSpecific["claude"]
	require.True(t, ok, "expected AgentSpecific to have 'claude' key")
	assert.NotContains(t, claudeSpecific, "sandbox", "sandbox is now portable on ir.Sandbox")
	assert.Contains(t, claudeSpecific, "env")
	require.NotNil(t, ir.Sandbox, "sandbox should be on ir.Sandbox")

	require.NotNil(t, ir.MCP)
	assert.Contains(t, ir.MCP, "mcpServers")

	require.NotNil(t, ir.Categories)
	commands, ok := ir.Categories["commands"]
	require.True(t, ok, "expected Categories to have 'commands' key")
	assert.Len(t, commands, 1)
	foundDeploy := false
	for _, f := range commands {
		if f.RelPath == "deploy.md" {
			foundDeploy = true
		}
	}
	assert.True(t, foundDeploy, "expected commands to include deploy.md")

	agents, ok := ir.Categories["agents"]
	require.True(t, ok, "expected Categories to have 'agents' key")
	assert.Len(t, agents, 1)
	foundReviewer := false
	for _, f := range agents {
		if f.RelPath == "reviewer.md" {
			foundReviewer = true
		}
	}
	assert.True(t, foundReviewer, "expected agents to include reviewer.md")

	skills, ok := ir.Categories["skills"]
	require.True(t, ok, "expected Categories to have 'skills' key")
	require.NotEmpty(t, skills, "expected skills category to have files")
	foundSkillDir := false
	foundSkillFile := false
	for _, f := range skills {
		if f.RelPath == "security" && f.IsDir {
			foundSkillDir = true
		}
		if f.RelPath == filepath.Join("security", "SKILL.md") && !f.IsDir {
			foundSkillFile = true
		}
	}
	assert.True(t, foundSkillDir, "expected skills to include security/ directory entry")
	assert.True(t, foundSkillFile, "expected skills to include security/SKILL.md file entry")
}

func TestReader_Minimal(t *testing.T) {
	t.Parallel()
	reader := &Reader{}
	layerPath := filepath.Join(readerTestdataDir(t), "minimal")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	assert.Equal(t, "claude", ir.SourceAgent)
	require.Len(t, ir.Instructions, 1)
	assert.Contains(t, ir.Instructions[0].Content, "Minimal Project")

	assert.Nil(t, ir.Permissions)
	assert.Nil(t, ir.Hooks)
	assert.Nil(t, ir.Settings)
	assert.Nil(t, ir.MCP)
	assert.Empty(t, ir.Categories)
	assert.Nil(t, ir.AgentSpecific)
}

func TestReader_Empty(t *testing.T) {
	t.Parallel()
	reader := &Reader{}
	layerPath := filepath.Join(readerTestdataDir(t), "empty")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	assert.Equal(t, "claude", ir.SourceAgent)
	assert.Empty(t, ir.Instructions)
	assert.Nil(t, ir.Permissions)
	assert.Nil(t, ir.Hooks)
	assert.Nil(t, ir.Settings)
	assert.Nil(t, ir.MCP)
	assert.Empty(t, ir.Categories)
	assert.Nil(t, ir.AgentSpecific)
}

func TestReader_SettingsDecomposition(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	settings := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Read"},
			"deny":  []string{"Bash"},
		},
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"matcher": "Write", "hooks": []string{"echo check"}},
			},
		},
		"model":   "claude-sonnet-4-20250514",
		"sandbox": map[string]any{"enabled": true},
		"env":     map[string]any{"FOO": "bar"},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	require.NotNil(t, ir.Permissions)
	assert.Contains(t, ir.Permissions, "allow")

	require.NotNil(t, ir.Hooks)
	assert.Contains(t, ir.Hooks, "PreToolUse")

	require.NotNil(t, ir.Settings)
	assert.Equal(t, "claude-sonnet-4-20250514", ir.Settings["model"])

	require.NotNil(t, ir.AgentSpecific)
	require.NotNil(t, ir.AgentSpecific["claude"])
	assert.NotContains(t, ir.AgentSpecific["claude"], "sandbox", "sandbox is now portable on ir.Sandbox")
	assert.Contains(t, ir.AgentSpecific["claude"], "env")
	require.NotNil(t, ir.Sandbox, "sandbox should be on ir.Sandbox")

	for _, key := range []string{"permissions", "hooks", "env"} {
		assert.NotContains(t, ir.Settings, key,
			"key %q should NOT be in Settings (it belongs elsewhere)", key)
	}
	assert.NotContains(t, ir.Settings, "sandbox", "sandbox belongs on ir.Sandbox, not Settings")
}

func TestReader_SettingsClassification_PortableVsSpecific(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	settings := map[string]any{
		"model":       "sonnet",
		"editorMode":  "vim",
		"defaultShell": "/bin/zsh",
		"effortLevel": "high",
		"sandbox":     map[string]any{"enabled": true},
		"permissions": map[string]any{"allow": []string{"Read"}},
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "command", "command": "echo test"},
			},
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	// model is portable
	require.NotNil(t, ir.Settings)
	assert.Equal(t, "sonnet", ir.Settings["model"])

	// Claude-specific keys go into AgentSpecific["claude"]
	require.NotNil(t, ir.AgentSpecific)
	require.NotNil(t, ir.AgentSpecific["claude"])
	assert.Equal(t, "vim", ir.AgentSpecific["claude"]["editorMode"])
	assert.Equal(t, "/bin/zsh", ir.AgentSpecific["claude"]["defaultShell"])
	assert.Equal(t, "high", ir.AgentSpecific["claude"]["effortLevel"])
	assert.NotContains(t, ir.AgentSpecific["claude"], "sandbox", "sandbox is now portable on ir.Sandbox")
	require.NotNil(t, ir.Sandbox, "sandbox should be on ir.Sandbox")

	// Permissions extracted separately
	require.NotNil(t, ir.Permissions)
	assert.Contains(t, ir.Permissions, "allow")

	// Hooks extracted separately
	require.NotNil(t, ir.Hooks)
	assert.Contains(t, ir.Hooks, "PreToolUse")

	// Settings does NOT contain Claude-specific keys
	for _, key := range []string{"editorMode", "defaultShell", "effortLevel"} {
		assert.NotContains(t, ir.Settings, key,
			"key %q should NOT be in portable Settings", key)
	}
	assert.NotContains(t, ir.Settings, "sandbox", "sandbox belongs on ir.Sandbox")
}
