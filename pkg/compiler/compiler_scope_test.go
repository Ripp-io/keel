// compiler_scope_test.go — user-scope compilation integration tests for
// Claude and Cursor, verifying MCP routing, instructions, settings, and
// overlay behavior when compiling in user (non-project) scope.
package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// --- Claude user-scope integration tests ---

func TestCompileClaude_UserScope_MCPFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "mcp_layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	mcpContent := `{"mcpServers":{"context7":{"command":"npx","args":["context7-mcp"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude.json"))
	require.NoError(t, err, "expected .claude.json at output root")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "context7")

	_, err = os.Stat(filepath.Join(outputDir, ".mcp.json"))
	assert.True(t, os.IsNotExist(err), ".mcp.json should NOT exist with user scope")
}

func TestCompileClaude_UserScope_MCPDeepMerged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1 := filepath.Join(dir, "layer1")
	require.NoError(t, os.MkdirAll(layer1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "mcp.json"),
		[]byte(`{"mcpServers":{"first":{"command":"first-cmd"}}}`), 0644))

	layer2 := filepath.Join(dir, "layer2")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "mcp.json"),
		[]byte(`{"mcpServers":{"second":{"command":"second-cmd"}}}`), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "base", Format: config.FormatKeel},
		{Path: layer2, Type: "project", Format: config.FormatKeel},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude.json"))
	require.NoError(t, err, "reading .claude.json")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "first")
	assert.Contains(t, servers, "second")
}

func TestCompileClaude_UserScope_InstructionsStillWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "instr_layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("user-scope instructions"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "expected CLAUDE.md at user scope")
	assert.Contains(t, string(content), "user-scope instructions")
}

func TestCompileClaude_UserScope_SettingsStillWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "settings_layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"model":"claude-sonnet-4-20250514"}`), 0644))

	// Layer has .claude/ directory — auto-detected as claude format.
	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "expected settings.json at user scope")
	assert.Contains(t, string(data), "claude-sonnet-4-20250514")
}

func TestCompileClaude_UserScope_CommandsStillOverlaid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "cmd_layer")
	cmdDir := filepath.Join(layerDir, "commands")
	require.NoError(t, os.MkdirAll(cmdDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cmdDir, "my-command.md"), []byte("my command"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".claude", "commands", "my-command.md"))
	assert.NoError(t, err, "expected command file at user scope")
}

func TestCompileClaude_UserScope_FullEnterprise(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	// .claude.json should exist (not .mcp.json)
	data, err := os.ReadFile(filepath.Join(outputDir, ".claude.json"))
	require.NoError(t, err, "expected .claude.json at output root")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	for _, s := range []string{"sonarqube", "jira", "gitlab", "obsidian"} {
		assert.Contains(t, servers, s)
	}

	_, err = os.Stat(filepath.Join(outputDir, ".mcp.json"))
	assert.True(t, os.IsNotExist(err), ".mcp.json should NOT exist with user scope")

	_, err = os.Stat(filepath.Join(outputDir, "CLAUDE.md"))
	assert.NoError(t, err, "expected CLAUDE.md at user scope")

	_, err = os.Stat(filepath.Join(outputDir, ".claude", "settings.json"))
	assert.NoError(t, err, "expected settings.json at user scope")

	entries, err := os.ReadDir(filepath.Join(outputDir, ".claude", "commands"))
	require.NoError(t, err, "expected commands dir at user scope")
	assert.NotEmpty(t, entries, "expected command files at user scope")
}

// --- Cursor user-scope integration tests ---

func TestCompileCursor_UserScope_NoInstructions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "instr_layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("project instructions"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
	assert.True(t, os.IsNotExist(err), "00-keel.mdc should NOT exist with user scope")
}

func TestCompileCursor_UserScope_NoRulesOverlay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "rules_layer")
	rulesDir := filepath.Join(layerDir, "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rulesDir, "my-rule.mdc"), []byte("---\nalwaysApply: true\n---\nsome rule"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	outputRulesDir := filepath.Join(outputDir, ".cursor", "rules")
	if _, err := os.Stat(outputRulesDir); err == nil {
		entries, err := os.ReadDir(outputRulesDir)
		require.NoError(t, err, "reading rules dir")
		assert.Empty(t, entries, "expected no rule files in user scope")
	}
}

func TestCompileCursor_UserScope_NoEnvironment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "env_layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := []byte("schema: v1\nenvironment:\n  - source: environment.json\n")
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), keelYML, 0644))

	envJSON := []byte(`{"NODE_ENV": "production", "LOG_LEVEL": "info"}`)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "environment.json"), envJSON, 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "environment.json"))
	assert.True(t, os.IsNotExist(err),
		".cursor/environment.json should NOT exist with user scope")
}

func TestCompileCursor_UserScope_CommandsPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "cmd_layer")
	cmdDir := filepath.Join(layerDir, "commands")
	require.NoError(t, os.MkdirAll(cmdDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cmdDir, "deploy.md"), []byte("deploy command"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "commands", "deploy.md"))
	assert.NoError(t, err, "expected command file at user scope")
}

func TestCompileCursor_UserScope_MCPPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "mcp_layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	mcpContent := `{"mcpServers":{"context7":{"command":"npx","args":["context7-mcp"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "expected .cursor/mcp.json at user scope")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "context7")
}

func TestCompileCursor_UserScope_AgentsPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "agent_layer")
	agentsDir := filepath.Join(layerDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "code-reviewer.md"), []byte("code reviewer agent"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "agents", "code-reviewer.md"))
	assert.NoError(t, err, "expected agent file at user scope")
}

func TestCompileCursor_UserScope_FullEnterprise(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{Scope: ScopeUser})
	require.NoError(t, err)

	// 00-keel.mdc should NOT exist (instructions skipped)
	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
	assert.True(t, os.IsNotExist(err), "00-keel.mdc should NOT exist in user scope")

	// Rule overlay files should NOT exist
	rulesDir := filepath.Join(outputDir, ".cursor", "rules")
	if _, err := os.Stat(rulesDir); err == nil {
		entries, err := os.ReadDir(rulesDir)
		require.NoError(t, err, "reading rules dir")
		assert.Empty(t, entries, "expected no rule files in user scope")
	}

	// MCP should still be present
	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "mcp.json"))
	assert.NoError(t, err, "expected mcp.json at user scope")

	// Commands should still be present
	commandsDir := filepath.Join(outputDir, ".cursor", "commands")
	if _, err := os.Stat(commandsDir); err == nil {
		entries, err := os.ReadDir(commandsDir)
		require.NoError(t, err, "reading commands dir")
		assert.NotEmpty(t, entries, "expected command files at user scope")
	}
}
