// compiler_features_test.go — feature-specific integration tests covering
// carve-out (conditional) integration, MCP merge regression, native layer
// deep-merge behavior across formats, variable substitution in overlay
// category files, and .keel/state/ directory convention.
package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// --- Carve-out integration tests ---

func TestCompileClaude_CarveoutMarkdown(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	keelYAML := `schema: v1
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))

	instrContent := `# Shared Instructions

This content is for all agents.

<!-- keel-if: keel.agent == 'claude' -->
Claude-specific: use CLAUDE.md for configuration.
<!-- keel-elif: keel.agent == 'cursor' -->
Cursor-specific: use .cursorrules for configuration.
<!-- keel-else -->
Other agent: check your agent docs.
<!-- keel-endif -->

More shared content here.
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instrContent), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel, Version: "v1"},
	}

	// Compile for Claude
	claudeOutput := t.TempDir()
	_, err := Compile("claude", layers, claudeOutput, CompileOptions{})
	require.NoError(t, err)

	claudeMD, err := os.ReadFile(filepath.Join(claudeOutput, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	claudeText := string(claudeMD)

	assert.Contains(t, claudeText, "This content is for all agents.")
	assert.Contains(t, claudeText, "Claude-specific: use CLAUDE.md for configuration.")
	assert.NotContains(t, claudeText, "Cursor-specific: use .cursorrules for configuration.")
	assert.NotContains(t, claudeText, "Other agent: check your agent docs.")
	assert.Contains(t, claudeText, "More shared content here.")
	assert.NotContains(t, claudeText, "keel-if")
	assert.NotContains(t, claudeText, "keel-elif")
	assert.NotContains(t, claudeText, "keel-endif")

	// Compile for Cursor
	cursorOutput := t.TempDir()
	_, err = Compile("cursor", layers, cursorOutput, CompileOptions{})
	require.NoError(t, err)

	cursorRules, err := os.ReadFile(filepath.Join(cursorOutput, ".cursor", "rules", "00-keel.mdc"))
	require.NoError(t, err, "reading 00-keel.mdc")
	cursorText := string(cursorRules)

	assert.Contains(t, cursorText, "This content is for all agents.")
	assert.Contains(t, cursorText, "Cursor-specific: use .cursorrules for configuration.")
	assert.NotContains(t, cursorText, "Claude-specific: use CLAUDE.md for configuration.")
	assert.NotContains(t, cursorText, "Other agent: check your agent docs.")
	assert.Contains(t, cursorText, "More shared content here.")
	assert.NotContains(t, cursorText, "keel-if")
	assert.NotContains(t, cursorText, "keel-endif")
}

func TestCompileClaude_CarveoutJSON_MCP(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	keelYAML := `schema: v1
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYAML), 0644))

	mcpData := map[string]any{
		"mcpServers": map[string]any{
			"claude-only-server": map[string]any{
				"command": "npx",
				"args":    []any{"claude-mcp-server"},
				"_keel":   map[string]any{"condition": "keel.agent == 'claude'"},
			},
			"cursor-only-server": map[string]any{
				"command": "npx",
				"args":    []any{"cursor-mcp-server"},
				"_keel":   map[string]any{"condition": "keel.agent == 'cursor'"},
			},
			"shared-server": map[string]any{
				"command": "npx",
				"args":    []any{"shared-mcp-server"},
			},
		},
	}
	mcpJSON, err := json.MarshalIndent(mcpData, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), mcpJSON, 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel, Version: "v1"},
	}

	// Compile for Claude
	claudeOutput := t.TempDir()
	_, err = Compile("claude", layers, claudeOutput, CompileOptions{})
	require.NoError(t, err)

	claudeMCPData, err := os.ReadFile(filepath.Join(claudeOutput, ".mcp.json"))
	require.NoError(t, err, "reading Claude .mcp.json")

	var claudeMCP map[string]any
	require.NoError(t, json.Unmarshal(claudeMCPData, &claudeMCP))
	servers, ok := claudeMCP["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in Claude mcp.json")

	assert.Contains(t, servers, "claude-only-server")
	assert.NotContains(t, servers, "cursor-only-server")
	assert.Contains(t, servers, "shared-server")
	testutil.AssertNoKeelKeys(t, claudeMCP)

	// Compile for Cursor
	cursorOutput := t.TempDir()
	_, err = Compile("cursor", layers, cursorOutput, CompileOptions{})
	require.NoError(t, err)

	cursorMCPData, err := os.ReadFile(filepath.Join(cursorOutput, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading Cursor mcp.json")

	var cursorMCP map[string]any
	require.NoError(t, json.Unmarshal(cursorMCPData, &cursorMCP))
	cursorServers, ok := cursorMCP["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in Cursor mcp.json")

	assert.Contains(t, cursorServers, "cursor-only-server")
	assert.NotContains(t, cursorServers, "claude-only-server")
	assert.Contains(t, cursorServers, "shared-server")
	testutil.AssertNoKeelKeys(t, cursorMCP)
}

// --- Regression tests for MCP merge bugs ---

func TestNativeLayerOverlayDoesNotWipeAccumulatedMCP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dslLayer := filepath.Join(dir, "01-dsl")
	require.NoError(t, os.MkdirAll(dslLayer, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dslLayer, "keel.yml"), []byte(`schema: v1
mcp:
  servers:
    - source: mcp.json
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dslLayer, "mcp.json"),
		[]byte(`{"mcpServers":{"dsl-server":{"command":"dsl-cmd"}}}`), 0644))

	nativeLayer := filepath.Join(dir, "02-native")
	cursorDir := filepath.Join(nativeLayer, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "mcp.json"),
		[]byte(`{"mcpServers":{"native-server":{"command":"native-cmd"}}}`), 0644))

	layers := []config.Layer{
		{Path: dslLayer, Type: "base", Format: config.FormatKeel},
		{Path: nativeLayer, Type: "project", Format: "cursor"},
	}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")

	var mcpMap map[string]any
	require.NoError(t, json.Unmarshal(data, &mcpMap))
	servers, ok := mcpMap["mcpServers"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, servers, "dsl-server", "DSL server should be preserved after native overlay")
	assert.Contains(t, servers, "native-server", "native server should be present")
}

func TestClaudeNativeMCPForceLockAcrossFormats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	claudeLayer := filepath.Join(dir, "01-claude")
	claudeDir := filepath.Join(claudeLayer, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeLayer, ".mcp.json"),
		[]byte(`{"mcpServers":{"locked-server":{"_keel":{"force":true},"command":"original-cmd","args":["--original"]}}}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeLayer, "keel.yml"),
		[]byte("schema: v1-claude\n"), 0644))

	cursorLayer := filepath.Join(dir, "02-cursor")
	cursorDir := filepath.Join(cursorLayer, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "mcp.json"),
		[]byte(`{"mcpServers":{"locked-server":{"command":"override-attempt"},"new-server":{"command":"new-cmd"}}}`), 0644))

	layers := []config.Layer{
		{Path: claudeLayer, Type: "base", Format: "claude"},
		{Path: cursorLayer, Type: "project", Format: "cursor"},
	}
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")

	var mcpMap map[string]any
	require.NoError(t, json.Unmarshal(data, &mcpMap))
	servers, ok := mcpMap["mcpServers"].(map[string]any)
	require.True(t, ok)

	lockedServer, ok := servers["locked-server"].(map[string]any)
	require.True(t, ok, "expected locked-server in .mcp.json")
	assert.Equal(t, "original-cmd", lockedServer["command"],
		"force-locked server should have original command")
	assert.Contains(t, servers, "new-server", "new server from cursor layer should be present")

	found := false
	for _, lock := range result.Governance.ForceLocked {
		if lock.Category == "mcp" && lock.Key == "locked-server" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected force-lock record for locked-server")
}

func TestCursorNativeMCPDeepMergedNotOverlaid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1 := filepath.Join(dir, "01")
	cursor1 := filepath.Join(layer1, ".cursor")
	require.NoError(t, os.MkdirAll(cursor1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursor1, "mcp.json"),
		[]byte(`{"mcpServers":{"first-server":{"command":"first"}}}`), 0644))

	layer2 := filepath.Join(dir, "02")
	cursor2 := filepath.Join(layer2, ".cursor")
	require.NoError(t, os.MkdirAll(cursor2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursor2, "mcp.json"),
		[]byte(`{"mcpServers":{"second-server":{"command":"second"}}}`), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "base", Format: "cursor"},
		{Path: layer2, Type: "project", Format: "cursor"},
	}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")

	var mcpMap map[string]any
	require.NoError(t, json.Unmarshal(data, &mcpMap))
	servers, ok := mcpMap["mcpServers"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, servers, "first-server")
	assert.Contains(t, servers, "second-server")
}

func TestCursorNativeSettingsDeepMergedNotOverlaid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1 := filepath.Join(dir, "01")
	cursor1 := filepath.Join(layer1, ".cursor")
	require.NoError(t, os.MkdirAll(cursor1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursor1, "cli-config.json"),
		[]byte(`{"version":1,"model":"sonnet"}`), 0644))

	layer2 := filepath.Join(dir, "02")
	cursor2 := filepath.Join(layer2, ".cursor")
	require.NoError(t, os.MkdirAll(cursor2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursor2, "cli-config.json"),
		[]byte(`{"version":1,"theme":"dark"}`), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "base", Format: "cursor"},
		{Path: layer2, Type: "project", Format: "cursor"},
	}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "sonnet", settings["model"])
	assert.Equal(t, "dark", settings["theme"])
}

// --- Variable substitution in overlay category files ---

func TestCompile_VariablesResolveInAgentFiles(t *testing.T) {
	t.Parallel()
	layerDir := filepath.Join(t.TempDir(), "layer")
	agentsDir := filepath.Join(layerDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	keel := `schema: v1
name: test
variables:
  db_path: "/data/coordination.db"
agents:
  - source: agents/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	agent := "---\nname: test-agent\n---\nDatabase is at ${keel:db_path} for coordination.\n"
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "test-agent.md"), []byte(agent), 0644))

	outputDir := t.TempDir()
	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	_, err := Compile("claude", layers, outputDir, CompileOptions{NoPrune: true})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "agents", "test-agent.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "/data/coordination.db")
	assert.NotContains(t, string(content), "${keel:db_path}")
}

func TestCompile_VariablesResolveInSnippetWithinAgentFile(t *testing.T) {
	t.Parallel()
	layerDir := filepath.Join(t.TempDir(), "layer")
	agentsDir := filepath.Join(layerDir, "agents")
	snippetsDir := filepath.Join(layerDir, "snippets")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))
	require.NoError(t, os.MkdirAll(snippetsDir, 0755))

	keel := `schema: v1
name: test
variables:
  evolution_path: "/state/evolution.db"
agents:
  - source: agents/
snippets:
  - source: snippets/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	snippet := "Evolution database: ${keel:evolution_path}\n"
	require.NoError(t, os.WriteFile(filepath.Join(snippetsDir, "evolution-marking.md"), []byte(snippet), 0644))

	agent := "---\nname: ripp-agent\n---\n# Agent\n\n<!-- keel:include evolution-marking -->\n"
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "ripp-agent.md"), []byte(agent), 0644))

	outputDir := t.TempDir()
	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	_, err := Compile("claude", layers, outputDir, CompileOptions{NoPrune: true})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "agents", "ripp-agent.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "/state/evolution.db")
	assert.NotContains(t, string(content), "${keel:evolution_path}")
}

func TestCompile_VariablesResolveInCommandFiles(t *testing.T) {
	t.Parallel()
	layerDir := filepath.Join(t.TempDir(), "layer")
	cmdsDir := filepath.Join(layerDir, "commands")
	require.NoError(t, os.MkdirAll(cmdsDir, 0755))

	keel := `schema: v1
name: test
variables:
  tool_path: "/usr/local/bin/mytool"
commands:
  - source: commands/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	cmd := "Run the tool at ${keel:tool_path} with default args.\n"
	require.NoError(t, os.WriteFile(filepath.Join(cmdsDir, "run-tool.md"), []byte(cmd), 0644))

	outputDir := t.TempDir()
	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	_, err := Compile("claude", layers, outputDir, CompileOptions{NoPrune: true})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "run-tool.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "/usr/local/bin/mytool")
	assert.NotContains(t, string(content), "${keel:tool_path}")
}

// --- .keel/state/ directory convention tests ---

func TestCompile_StateDirectoryCreated(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	layerDir := filepath.Join(t.TempDir(), "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	_, err := Compile("claude", layers, outputDir, CompileOptions{NoPrune: true})
	require.NoError(t, err)

	// .keel/state/ directory should exist.
	stateDir := filepath.Join(outputDir, ".keel", "state")
	info, err := os.Stat(stateDir)
	require.NoError(t, err, ".keel/state/ should exist")
	assert.True(t, info.IsDir(), ".keel/state/ should be a directory")

	// .keel/state/.gitignore should contain "*".
	gitignore, err := os.ReadFile(filepath.Join(stateDir, ".gitignore"))
	require.NoError(t, err, ".keel/state/.gitignore should exist")
	assert.Equal(t, "*\n", string(gitignore))
}

func TestCompile_StateDirectoryVariable(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	layerDir := filepath.Join(t.TempDir(), "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	// Create a layer that references the keel.state variable in instructions.
	keel := "schema: v1\nname: test\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))
	instr := "State dir: ${keel:keel.state}\n"
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instr), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	result, err := Compile("claude", layers, outputDir, CompileOptions{NoPrune: true})
	require.NoError(t, err)

	// Variable should resolve to the absolute state dir path.
	absOutputDir, _ := filepath.Abs(outputDir)
	expectedState := filepath.Join(absOutputDir, ".keel", "state")
	assert.Equal(t, expectedState, result.Tracking.Variables["keel.state"])

	// Verify it was resolved in the instructions output.
	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), expectedState)
	assert.NotContains(t, string(content), "${keel:keel.state}")
}

func TestCompile_StateDirectoryNotPruned(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	layerDir := filepath.Join(t.TempDir(), "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	// First compile — creates .keel/state/ and .keel.json manifest.
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Place a file in .keel/state/ simulating runtime data.
	stateDir := filepath.Join(outputDir, ".keel", "state")
	runtimeDB := filepath.Join(stateDir, "coordination.db")
	require.NoError(t, os.WriteFile(runtimeDB, []byte("runtime data"), 0644))

	// Second compile (with prune) — should NOT touch .keel/state/.
	_, err = Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Runtime data should survive.
	data, err := os.ReadFile(runtimeDB)
	require.NoError(t, err, "runtime data in .keel/state/ should survive prune")
	assert.Equal(t, "runtime data", string(data))

	// .gitignore should still be there.
	gitignore, err := os.ReadFile(filepath.Join(stateDir, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "*\n", string(gitignore))
}

func TestCompile_StateDirectoryNotCleaned(t *testing.T) {
	t.Parallel()
	outputDir := t.TempDir()
	layerDir := filepath.Join(t.TempDir(), "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	layers := []config.Layer{
		{Path: layerDir, Type: "test", Format: config.FormatKeel},
	}

	// Compile to create manifest and state dir.
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Place runtime data.
	stateDir := filepath.Join(outputDir, ".keel", "state")
	runtimeDB := filepath.Join(stateDir, "evolution.db")
	require.NoError(t, os.WriteFile(runtimeDB, []byte("evolution data"), 0644))

	// --clean removes all tracked output.
	_, err = manifest.CleanAllOutput(outputDir, ".claude", false)
	require.NoError(t, err)

	// .keel/state/ and its contents should survive --clean.
	data, err := os.ReadFile(runtimeDB)
	require.NoError(t, err, ".keel/state/ should survive --clean")
	assert.Equal(t, "evolution data", string(data))
}
