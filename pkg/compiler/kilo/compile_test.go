package kilo

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

func TestCompile_Kilo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-kilo-compile
description: Minimal test layer
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test Instructions\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "kilo", result.Output.Agent)

	// Verify AGENTS.md exists
	agentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, "AGENTS.md should exist")

	// Verify .kilo directory exists
	kiloDir := filepath.Join(dir, ".kilo")
	_, err = os.Stat(kiloDir)
	assert.NoError(t, err, ".kilo directory should exist")
}

func TestCompile_Kilo_MCPTranslation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-mcp-translation
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
	assert.Equal(t, "kilo", result.Output.Agent)

	// Verify settings file exists with embedded MCP
	settingsPath := filepath.Join(dir, ".kilo", "kilo.jsonc")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "kilo.jsonc should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify MCP was translated to Kilo format
	mcp, ok := settings["mcp"].(map[string]any)
	require.True(t, ok, "mcp key should exist in settings")

	servers, ok := mcp["servers"].(map[string]any)
	require.True(t, ok, "servers key should exist in mcp")

	testServer, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "test-server should exist")

	// Verify Kilo format: merged command array
	command, ok := testServer["command"].([]any)
	require.True(t, ok, "command should be an array")
	assert.Equal(t, []any{"node", "server.js"}, command)

	// Verify Kilo format: renamed env → environment
	env, ok := testServer["environment"].(map[string]any)
	require.True(t, ok, "environment key should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestCompile_Kilo_PermissionTranslation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-permission-translation
permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	permsContent := `{"allow": ["Bash(npm test)", "Read", "Edit(src/*)"], "deny": ["Bash(rm *)"]}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"), []byte(permsContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeProject})

	require.NoError(t, err)
	assert.Equal(t, "kilo", result.Output.Agent)

	// Verify settings file exists with embedded permissions
	settingsPath := filepath.Join(dir, ".kilo", "kilo.jsonc")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "kilo.jsonc should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify permissions was translated to Kilo format (singular "permission", not "permissions")
	perms, ok := settings["permission"].(map[string]any)
	require.True(t, ok, "permission key should exist in settings")

	// Verify allow list
	allow, ok := perms["allow"].([]any)
	require.True(t, ok, "allow key should exist in permission")
	require.Len(t, allow, 3, "should have 3 allowed permissions")

	// Verify deny list
	deny, ok := perms["deny"].([]any)
	require.True(t, ok, "deny key should exist in permission")
	require.Len(t, deny, 1, "should have 1 denied permission")
}

func TestCompile_Kilo_UserScope(t *testing.T) {
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
	assert.Equal(t, "kilo", result.Output.Agent)

	// Verify instructions written to .config/kilo/AGENTS.md (not root)
	agentsPath := filepath.Join(dir, ".config", "kilo", "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, ".config/kilo/AGENTS.md should exist")

	// Verify settings written to .config/kilo/kilo.jsonc
	settingsPath := filepath.Join(dir, ".config", "kilo", "kilo.jsonc")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, ".config/kilo/kilo.jsonc should exist")

	// Verify .config/kilo/ directory exists
	configDir := filepath.Join(dir, ".config", "kilo")
	_, err = os.Stat(configDir)
	assert.NoError(t, err, ".config/kilo/ directory should exist")

	// Verify no .kilo/ at root
	rootKiloDir := filepath.Join(dir, ".kilo")
	_, err = os.Stat(rootKiloDir)
	assert.True(t, os.IsNotExist(err), ".kilo/ should not exist at root for user scope")

	// Verify no AGENTS.md at root
	rootAgentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(rootAgentsPath)
	assert.True(t, os.IsNotExist(err), "AGENTS.md should not exist at root for user scope")
}

func TestCompile_Kilo_Categories(t *testing.T) {
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
	assert.Equal(t, "kilo", result.Output.Agent)

	// Verify .kilo/commands/ contains the command file
	commandPath := filepath.Join(dir, ".kilo", "commands", "test-command.md")
	data, err := os.ReadFile(commandPath)
	require.NoError(t, err, ".kilo/commands/test-command.md should exist")
	assert.Equal(t, commandContent, string(data))

	// Verify .kilo/skills/ contains the skill file
	skillPath := filepath.Join(dir, ".kilo", "skills", "test-skill.md")
	data, err = os.ReadFile(skillPath)
	require.NoError(t, err, ".kilo/skills/test-skill.md should exist")
	assert.Equal(t, skillContent, string(data))
}

func TestTranslateMCPToKilo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    map[string]any{},
			expected: nil,
		},
		{
			name: "basic server",
			input: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "node",
						"args":    []any{"server.js"},
					},
				},
			},
			expected: map[string]any{
				"servers": map[string]any{
					"test": map[string]any{
						"command": []any{"node", "server.js"},
					},
				},
			},
		},
		{
			name: "server with env",
			input: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "node",
						"args":    []any{"server.js"},
						"env": map[string]any{
							"API_KEY": "test",
						},
					},
				},
			},
			expected: map[string]any{
				"servers": map[string]any{
					"test": map[string]any{
						"command": []any{"node", "server.js"},
						"environment": map[string]any{
							"API_KEY": "test",
						},
					},
				},
			},
		},
		{
			name: "remote server",
			input: map[string]any{
				"mcpServers": map[string]any{
					"remote": map[string]any{
						"url": "https://mcp.example.com",
					},
				},
			},
			expected: map[string]any{
				"servers": map[string]any{
					"remote": map[string]any{
						"type": "remote",
						"url":  "https://mcp.example.com",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPToKilo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslatePermissionsToKilo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    map[string]any{},
			expected: nil,
		},
		{
			name: "allow list",
			input: map[string]any{
				"allow": []any{"Read", "Bash(npm test)"},
			},
			expected: map[string]any{
				"allow": []any{"Read", "Bash(npm test)"},
			},
		},
		{
			name: "deny list",
			input: map[string]any{
				"deny": []any{"Bash(rm *)"},
			},
			expected: map[string]any{
				"deny": []any{"Bash(rm *)"},
			},
		},
		{
			name: "mixed allow and deny",
			input: map[string]any{
				"allow": []any{"Read", "Edit(src/*)"},
				"deny":  []any{"Bash(rm *)"},
			},
			expected: map[string]any{
				"allow": []any{"Read", "Edit(src/*)"},
				"deny":  []any{"Bash(rm *)"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsToKilo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
