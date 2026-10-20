package amp

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

func TestCompile_Amp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-amp-compile
description: Minimal test layer
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test Instructions\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "amp", result.Output.Agent)

	// Verify AGENTS.md exists
	agentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, "AGENTS.md should exist")

	// Verify .amp directory exists
	ampDir := filepath.Join(dir, ".amp")
	_, err = os.Stat(ampDir)
	assert.NoError(t, err, ".amp directory should exist")
}

func TestCompile_Amp_MCPTranslation(t *testing.T) {
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
	assert.Equal(t, "amp", result.Output.Agent)

	// Verify settings file exists with embedded MCP
	settingsPath := filepath.Join(dir, ".amp", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "settings.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify MCP was embedded under amp.mcpServers
	mcpServers, ok := settings["amp.mcpServers"].(map[string]any)
	require.True(t, ok, "amp.mcpServers key should exist in settings")

	testServer, ok := mcpServers["test-server"].(map[string]any)
	require.True(t, ok, "test-server should exist")

	// Verify Amp format (same as IR/Claude format)
	assert.Equal(t, "node", testServer["command"])

	args, ok := testServer["args"].([]any)
	require.True(t, ok, "args should be an array")
	assert.Equal(t, []any{"server.js"}, args)

	env, ok := testServer["env"].(map[string]any)
	require.True(t, ok, "env key should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestCompile_Amp_PermissionTranslation(t *testing.T) {
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

	permsContent := `{"allow": ["Bash(npm test)", "Bash(npm build)", "Bash"], "deny": ["Edit", "WebSearch"]}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"), []byte(permsContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeProject})

	require.NoError(t, err)
	assert.Equal(t, "amp", result.Output.Agent)

	// Verify settings file exists with embedded permissions
	settingsPath := filepath.Join(dir, ".amp", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "settings.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify allowlist was translated to amp.commands.allowlist
	allowlist, ok := settings["amp.commands.allowlist"].([]any)
	require.True(t, ok, "amp.commands.allowlist key should exist in settings as array")
	require.Len(t, allowlist, 3, "should have 3 allowlist entries")

	// Verify translated commands
	assert.Contains(t, allowlist, "npm test")
	assert.Contains(t, allowlist, "npm build")
	assert.Contains(t, allowlist, "*")

	// Note: deny list is not tested here because Amp doesn't have a direct
	// equivalent for denying specific tools via amp.commands.allowlist.
	// The deny list would need to be handled via amp.tools.disable or similar.
}

func TestCompile_Amp_UserScope(t *testing.T) {
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
	assert.Equal(t, "amp", result.Output.Agent)

	// Verify instructions written to .config/amp/AGENTS.md (not root)
	agentsPath := filepath.Join(dir, ".config", "amp", "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, ".config/amp/AGENTS.md should exist")

	// Verify settings written to .config/amp/settings.json
	settingsPath := filepath.Join(dir, ".config", "amp", "settings.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, ".config/amp/settings.json should exist")

	// Verify .config/amp/ directory exists
	configDir := filepath.Join(dir, ".config", "amp")
	_, err = os.Stat(configDir)
	assert.NoError(t, err, ".config/amp/ directory should exist")

	// Verify no .amp/ at root
	rootAmpDir := filepath.Join(dir, ".amp")
	_, err = os.Stat(rootAmpDir)
	assert.True(t, os.IsNotExist(err), ".amp/ should not exist at root for user scope")

	// Verify no AGENTS.md at root
	rootAgentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(rootAgentsPath)
	assert.True(t, os.IsNotExist(err), "AGENTS.md should not exist at root for user scope")
}

func TestCompile_Amp_Categories(t *testing.T) {
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
	assert.Equal(t, "amp", result.Output.Agent)

	// Verify .amp/commands/ contains the command file
	commandPath := filepath.Join(dir, ".amp", "commands", "test-command.md")
	data, err := os.ReadFile(commandPath)
	require.NoError(t, err, ".amp/commands/test-command.md should exist")
	assert.Equal(t, commandContent, string(data))

	// Verify .amp/skills/ contains the skill file
	skillPath := filepath.Join(dir, ".amp", "skills", "test-skill.md")
	data, err = os.ReadFile(skillPath)
	require.NoError(t, err, ".amp/skills/test-skill.md should exist")
	assert.Equal(t, skillContent, string(data))
}

func TestTranslateMCPToAmp(t *testing.T) {
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
				"test": map[string]any{
					"command": "node",
					"args":    []any{"server.js"},
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
				"test": map[string]any{
					"command": "node",
					"args":    []any{"server.js"},
					"env": map[string]any{
						"API_KEY": "test",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPToAmp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslatePermissionsToAmp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected []any
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
			name: "bare Bash",
			input: map[string]any{
				"allow": []any{"Bash"},
			},
			expected: []any{"*"},
		},
		{
			name: "Bash with command",
			input: map[string]any{
				"allow": []any{"Bash(npm test)"},
			},
			expected: []any{"npm test"},
		},
		{
			name: "multiple Bash commands",
			input: map[string]any{
				"allow": []any{"Bash(npm test)", "Bash(npm build)", "Bash"},
			},
			expected: []any{"npm test", "npm build", "*"},
		},
		{
			name: "mixed tools - only Bash extracted",
			input: map[string]any{
				"allow": []any{"Bash(npm test)", "Read", "Edit(src/*)"},
			},
			expected: []any{"npm test"},
		},
		{
			name: "non-Bash permissions ignored",
			input: map[string]any{
				"allow": []any{"Read", "Edit", "WebSearch"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsToAmp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslateDenyListToAmp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []any
		expected []any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    []any{},
			expected: nil,
		},
		{
			name:     "single tool",
			input:    []any{"Bash"},
			expected: []any{"bash"},
		},
		{
			name:     "multiple tools",
			input:    []any{"Bash", "Edit", "WebSearch"},
			expected: []any{"bash", "edit", "websearch"},
		},
		{
			name:     "all standard tools",
			input:    []any{"Bash", "Read", "Edit", "Write", "WebSearch", "WebFetch", "Agent"},
			expected: []any{"bash", "read", "edit", "edit", "websearch", "webfetch", "agent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateDenyListToAmp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
