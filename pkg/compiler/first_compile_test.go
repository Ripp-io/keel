package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestFirstCompileConflict_NoExistingFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a keel layer with settings
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
settings:
  model: claude-sonnet-4
`))

	// First compile with no existing files — should succeed
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify settings file was created
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	assert.True(t, testutil.FileExists(settingsPath))
}

func TestFirstCompileConflict_ExistingSettingsFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing settings file
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755))
	testutil.WriteFile(t, tmpDir, ".claude/settings.json", []byte(`{"model": "user-model"}`))

	// Create a keel layer with settings
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
settings:
  model: claude-sonnet-4
`))

	// First compile WITHOUT --force — should error
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "first compile would overwrite existing files")
	assert.Contains(t, err.Error(), ".claude/settings.json")
	assert.Contains(t, err.Error(), ".keel/.claude/")
}

func TestFirstCompileConflict_ExistingCategoryFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing command files
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude", "commands"), 0755))
	testutil.WriteFile(t, tmpDir, ".claude/commands/deploy.md", []byte("# User command"))
	testutil.WriteFile(t, tmpDir, ".claude/commands/review.md", []byte("# Another command"))

	// Create a keel layer with commands
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(filepath.Join(keelLayerDir, "commands"), 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
commands:
  - source: commands/
`))
	testutil.WriteFile(t, keelLayerDir, "commands/deploy.md", []byte("# Keel command"))

	// First compile WITHOUT --force — should error listing all conflicts
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "first compile would overwrite existing files")
	assert.Contains(t, err.Error(), ".claude/commands/deploy.md")
	// review.md exists on disk but is NOT in the layer, so it should not appear in the conflict list
	assert.NotContains(t, err.Error(), "review.md")
}

func TestFirstCompileConflict_MultipleConflictTypes(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing files of multiple types
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude", "commands"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude", "agents"), 0755))
	testutil.WriteFile(t, tmpDir, ".claude/settings.json", []byte(`{}`))
	testutil.WriteFile(t, tmpDir, ".claude/commands/foo.md", []byte("# Foo"))
	testutil.WriteFile(t, tmpDir, ".claude/agents/bar.md", []byte("# Bar"))
	testutil.WriteFile(t, tmpDir, ".mcp.json", []byte(`{}`))

	// Create a keel layer
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(filepath.Join(keelLayerDir, "commands"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(keelLayerDir, "agents"), 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
settings:
  model: claude-sonnet-4
commands:
  - source: commands/
agents:
  - source: agents/
mcp:
  servers:
    - source: mcp.json
`))
	testutil.WriteFile(t, keelLayerDir, "commands/foo.md", []byte("# Keel foo"))
	testutil.WriteFile(t, keelLayerDir, "agents/bar.md", []byte("# Keel bar"))
	testutil.WriteFile(t, keelLayerDir, "mcp.json", []byte(`{"mcpServers": {"test": {"command": "node"}}}`))

	// First compile — should error with all conflicts sorted
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.Error(t, err)
	assert.Nil(t, result)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "first compile would overwrite existing files")
	assert.Contains(t, errMsg, ".claude/agents/bar.md")
	assert.Contains(t, errMsg, ".claude/commands/foo.md")
	assert.Contains(t, errMsg, ".claude/settings.json")
	assert.Contains(t, errMsg, ".mcp.json")

	// Verify sorted order (paths should be listed in alphabetical order)
	lines := strings.Split(errMsg, "\n")
	var pathLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".") {
			pathLines = append(pathLines, trimmed)
		}
	}
	require.GreaterOrEqual(t, len(pathLines), 4)
	// Check first < second < third < fourth (alphabetically)
	for i := 0; i < len(pathLines)-1; i++ {
		assert.True(t, pathLines[i] < pathLines[i+1], "paths should be sorted: %v", pathLines)
	}
}

func TestFirstCompileConflict_WithForce(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing settings file
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755))
	testutil.WriteFile(t, tmpDir, ".claude/settings.json", []byte(`{"model": "user-model"}`))

	// Create a keel layer with settings
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
settings:
  model: claude-sonnet-4
`))

	// First compile WITH ForceInstructions=true — should succeed and overwrite
	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{ForceInstructions: true})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify settings file was overwritten
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "claude-sonnet-4")
}

func TestFirstCompileConflict_SecondCompileNoError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a keel layer
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
settings:
  model: claude-sonnet-4
`))

	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}

	// First compile (with force to create files)
	result1, err := Compile("claude", layers, tmpDir, CompileOptions{ForceInstructions: true})
	require.NoError(t, err)
	require.NotNil(t, result1)

	// Second compile — manifest exists now, so first-compile guard should NOT run
	// Even though settings.json exists, this should succeed
	result2, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result2)
}

func TestFirstCompileConflict_MCPMergeInto(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing .claude.json (merge-into file for user scope)
	testutil.WriteFile(t, tmpDir, ".claude.json", []byte(`{"oauth": {"token": "secret"}}`))

	// Create a keel layer with MCP
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
mcp:
  servers:
    - source: mcp.json
`))
	testutil.WriteFile(t, keelLayerDir, "mcp.json", []byte(`{"mcpServers": {"test": {}}}`))

	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}

	// First compile for user scope (MCPMergeInto=true) — should NOT error
	// Merge-into files are shared, not overwritten wholesale
	result, err := Compile("claude", layers, tmpDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify .claude.json still has oauth token (merge-into preserves other keys)
	content, err := os.ReadFile(filepath.Join(tmpDir, ".claude.json"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "oauth")
	assert.Contains(t, string(content), "mcpServers")
}

func TestFirstCompileConflict_EmptyAccumulatedContent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing settings file
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755))
	testutil.WriteFile(t, tmpDir, ".claude/settings.json", []byte(`{"model": "user-model"}`))

	// Create a keel layer with NO settings (empty)
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
`))

	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}

	// First compile — should succeed because no content would be written
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify settings file unchanged
	content, err := os.ReadFile(filepath.Join(tmpDir, ".claude", "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "user-model")
}

func TestFirstCompileConflict_ContentBlocks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing loop file
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755))
	testutil.WriteFile(t, tmpDir, ".claude/loop.md", []byte("User loop content"))

	// Create a keel layer with loop content
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
loop:
  - source: loop.md
`))
	testutil.WriteFile(t, keelLayerDir, "loop.md", []byte("Keel loop content"))

	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}

	// First compile WITHOUT --force — should error
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "first compile would overwrite existing files")
	assert.Contains(t, err.Error(), ".claude/loop.md")
}

func TestFirstCompileConflict_MCPFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create pre-existing .mcp.json
	testutil.WriteFile(t, tmpDir, ".mcp.json", []byte(`{"mcpServers": {"existing": {}}}`))

	// Create a keel layer with MCP
	keelLayerDir := filepath.Join(tmpDir, "my-layer")
	require.NoError(t, os.MkdirAll(keelLayerDir, 0755))
	testutil.WriteFile(t, keelLayerDir, "keel.yml", []byte(`
schema: v1
name: test-layer
mcp:
  servers:
    - source: mcp.json
`))
	testutil.WriteFile(t, keelLayerDir, "mcp.json", []byte(`{"mcpServers": {"myserver": {"command": "node"}}}`))

	layers := []config.Layer{
		{Path: keelLayerDir, Format: config.FormatKeel, Type: "keel"},
	}

	// First compile WITHOUT --force — should error
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "first compile would overwrite existing files")
	assert.Contains(t, err.Error(), ".mcp.json")
}
