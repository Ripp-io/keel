package qwen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompile_Qwen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-qwen-compile
description: Minimal test layer
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test Instructions\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "qwen", result.Output.Agent)

	// Verify QWEN.md exists and contains instructions (single-agent mode)
	qwenPath := filepath.Join(dir, "QWEN.md")
	data, err := os.ReadFile(qwenPath)
	require.NoError(t, err, "QWEN.md should exist")
	assert.Contains(t, string(data), "Test Instructions", "QWEN.md should contain instructions in single-agent mode")

	// Verify .qwen directory exists
	qwenDir := filepath.Join(dir, ".qwen")
	_, err = os.Stat(qwenDir)
	assert.NoError(t, err, ".qwen directory should exist")
}

func TestCompile_Qwen_MCP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-mcp
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	mcpContent := `{"mcpServers": {"test-server": {"command": "node", "args": ["server.js"], "env": {"API_KEY": "test"}}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeProject})

	require.NoError(t, err)
	assert.Equal(t, "qwen", result.Output.Agent)

	// Verify settings file exists with embedded MCP
	settingsPath := filepath.Join(dir, ".qwen", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "settings.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify MCP in standard mcpServers format
	mcpServers, ok := settings["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key should exist in settings")

	testServer, ok := mcpServers["test-server"].(map[string]any)
	require.True(t, ok, "test-server should exist")

	// Verify standard MCP format (command, args, env)
	assert.Equal(t, "node", testServer["command"])

	args, ok := testServer["args"].([]any)
	require.True(t, ok, "args should be an array")
	assert.Equal(t, []any{"server.js"}, args)

	env, ok := testServer["env"].(map[string]any)
	require.True(t, ok, "env key should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestCompile_Qwen_UserScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-user-scope
description: Test user scope output
instructions:
  - source: instructions.md
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# User Instructions\n"), 0644))

	mcpContent := `{"mcpServers": {"user-server": {"command": "node", "args": ["user.js"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeUser})

	require.NoError(t, err)
	assert.Equal(t, "qwen", result.Output.Agent)

	// Verify instructions written to .config/qwen/QWEN.md (single-agent mode)
	qwenPath := filepath.Join(dir, ".config", "qwen", "QWEN.md")
	data, err := os.ReadFile(qwenPath)
	require.NoError(t, err, ".config/qwen/QWEN.md should exist")
	assert.Contains(t, string(data), "User Instructions", "QWEN.md should contain instructions")

	// Verify settings written to .config/qwen/settings.json
	settingsPath := filepath.Join(dir, ".config", "qwen", "settings.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, ".config/qwen/settings.json should exist")

	// Verify .config/qwen/ directory exists
	configDir := filepath.Join(dir, ".config", "qwen")
	_, err = os.Stat(configDir)
	assert.NoError(t, err, ".config/qwen/ directory should exist")

	// Verify no .qwen/ at root
	rootQwenDir := filepath.Join(dir, ".qwen")
	_, err = os.Stat(rootQwenDir)
	assert.True(t, os.IsNotExist(err), ".qwen/ should not exist at root for user scope")

	// Verify no QWEN.md at root
	rootQwenPath := filepath.Join(dir, "QWEN.md")
	_, err = os.Stat(rootQwenPath)
	assert.True(t, os.IsNotExist(err), "QWEN.md should not exist at root for user scope")
}

func TestCompile_Qwen_Categories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	// Create commands and skills directories with test files
	commandsDir := filepath.Join(layerDir, "commands")
	skillsDir := filepath.Join(layerDir, "skills")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	commandContent := "# Test Command\n\nThis is a test command."
	skillContent := "# Test Skill\n\nThis is a test skill."

	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test-command.md"), []byte(commandContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte(skillContent), 0644))

	keelYML := `schema: v1
name: test-categories
description: Test categories overlay
commands:
  - source: commands/
skills:
  - source: skills/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "qwen", result.Output.Agent)

	// Verify .qwen/commands/ contains the command file
	commandPath := filepath.Join(dir, ".qwen", "commands", "test-command.md")
	data, err := os.ReadFile(commandPath)
	require.NoError(t, err, ".qwen/commands/test-command.md should exist")
	assert.Equal(t, commandContent, string(data))

	// Verify .qwen/skills/ contains the skill file
	skillPath := filepath.Join(dir, ".qwen", "skills", "test-skill.md")
	data, err = os.ReadFile(skillPath)
	require.NoError(t, err, ".qwen/skills/test-skill.md should exist")
	assert.Equal(t, skillContent, string(data))
}

func TestCompile_Qwen_Settings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-settings
settings:
  model: qwen-coder-3
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "qwen", result.Output.Agent)

	// Verify settings file exists with model
	settingsPath := filepath.Join(dir, ".qwen", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "settings.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify model setting
	assert.Equal(t, "qwen-coder-3", settings["model"])
}

func TestCompile_Qwen_NoHooksPassthrough(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-no-hooks
description: Test hooks are not passed through
hooks:
  - source: hooks.json
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	hooksContent := `{
  "events": {
    "before:commit": [
      {"command": "npm test"}
    ]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "hooks.json"), []byte(hooksContent), 0644))

	mcpContent := `{"mcpServers": {"test-server": {"command": "node", "args": ["server.js"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeProject})

	// Compilation should succeed
	require.NoError(t, err)
	assert.Equal(t, "qwen", result.Output.Agent)

	// Verify settings file exists
	settingsPath := filepath.Join(dir, ".qwen", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "settings.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Hooks should NOT be present in settings
	_, hasHooks := settings["hooks"]
	assert.False(t, hasHooks, "hooks should not be present in settings.json")

	_, hasEvents := settings["events"]
	assert.False(t, hasEvents, "events should not be present in settings.json")

	// MCP should be present (to verify compilation succeeded)
	_, hasMCP := settings["mcpServers"]
	assert.True(t, hasMCP, "mcpServers should be present in settings.json")
}
