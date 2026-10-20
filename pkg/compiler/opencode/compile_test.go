package opencode

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

func TestCompile_OpenCode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-opencode-compile
description: Minimal test layer
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test Instructions\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "opencode", result.Output.Agent)

	// Verify AGENTS.md exists
	agentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, "AGENTS.md should exist")

	// Verify .opencode directory exists
	opencodeDir := filepath.Join(dir, ".opencode")
	_, err = os.Stat(opencodeDir)
	assert.NoError(t, err, ".opencode directory should exist")
}

func TestCompile_OpenCode_MCPTranslation(t *testing.T) {
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
	assert.Equal(t, "opencode", result.Output.Agent)

	// Verify settings file exists with embedded MCP
	settingsPath := filepath.Join(dir, ".opencode", "opencode.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "opencode.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify MCP was translated to V2 format
	mcp, ok := settings["mcp"].(map[string]any)
	require.True(t, ok, "mcp key should exist in settings")

	servers, ok := mcp["servers"].(map[string]any)
	require.True(t, ok, "servers key should exist in mcp")

	testServer, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "test-server should exist")

	// Verify V2 format: type field
	assert.Equal(t, "local", testServer["type"])

	// Verify V2 format: merged command array
	command, ok := testServer["command"].([]any)
	require.True(t, ok, "command should be an array")
	assert.Equal(t, []any{"node", "server.js"}, command)

	// Verify V2 format: renamed env → environment
	env, ok := testServer["environment"].(map[string]any)
	require.True(t, ok, "environment key should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestCompile_OpenCode_PermissionTranslation(t *testing.T) {
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
	assert.Equal(t, "opencode", result.Output.Agent)

	// Verify settings file exists with embedded permissions
	settingsPath := filepath.Join(dir, ".opencode", "opencode.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "opencode.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify permissions was translated to V2 format
	perms, ok := settings["permissions"].([]any)
	require.True(t, ok, "permissions key should exist in settings as array")
	require.Len(t, perms, 4, "should have 4 permission entries")

	// Helper to find permission entry
	findPerm := func(action, resource, effect string) bool {
		for _, p := range perms {
			entry, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if entry["action"] == action && entry["resource"] == resource && entry["effect"] == effect {
				return true
			}
		}
		return false
	}

	// Verify V2 format entries
	assert.True(t, findPerm("shell", "npm test", "allow"), "Bash(npm test) should translate to shell/npm test/allow")
	assert.True(t, findPerm("read", "*", "allow"), "Read should translate to read/*/allow")
	assert.True(t, findPerm("edit", "src/*", "allow"), "Edit(src/*) should translate to edit/src/*/allow")
	assert.True(t, findPerm("shell", "rm *", "deny"), "Bash(rm *) should translate to shell/rm */deny")
}

func TestCompile_OpenCode_UserScope(t *testing.T) {
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
	assert.Equal(t, "opencode", result.Output.Agent)

	// Verify instructions written to .config/opencode/AGENTS.md (not root)
	agentsPath := filepath.Join(dir, ".config", "opencode", "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, ".config/opencode/AGENTS.md should exist")

	// Verify settings written to .config/opencode/opencode.json
	settingsPath := filepath.Join(dir, ".config", "opencode", "opencode.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, ".config/opencode/opencode.json should exist")

	// Verify .config/opencode/ directory exists
	configDir := filepath.Join(dir, ".config", "opencode")
	_, err = os.Stat(configDir)
	assert.NoError(t, err, ".config/opencode/ directory should exist")

	// Verify no .opencode/ at root
	rootOpencodeDir := filepath.Join(dir, ".opencode")
	_, err = os.Stat(rootOpencodeDir)
	assert.True(t, os.IsNotExist(err), ".opencode/ should not exist at root for user scope")

	// Verify no AGENTS.md at root
	rootAgentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(rootAgentsPath)
	assert.True(t, os.IsNotExist(err), "AGENTS.md should not exist at root for user scope")
}

func TestCompile_OpenCode_Categories(t *testing.T) {
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
	assert.Equal(t, "opencode", result.Output.Agent)

	// Verify .opencode/commands/ contains the command file
	commandPath := filepath.Join(dir, ".opencode", "commands", "test-command.md")
	data, err := os.ReadFile(commandPath)
	require.NoError(t, err, ".opencode/commands/test-command.md should exist")
	assert.Equal(t, commandContent, string(data))

	// Verify .opencode/skills/ contains the skill file
	skillPath := filepath.Join(dir, ".opencode", "skills", "test-skill.md")
	data, err = os.ReadFile(skillPath)
	require.NoError(t, err, ".opencode/skills/test-skill.md should exist")
	assert.Equal(t, skillContent, string(data))
}

func TestCompile_OpenCode_NoHooksPassthrough(t *testing.T) {
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
	assert.Equal(t, "opencode", result.Output.Agent)

	// Verify settings file exists
	settingsPath := filepath.Join(dir, ".opencode", "opencode.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "opencode.json should exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Hooks should NOT be present in settings
	_, hasHooks := settings["hooks"]
	assert.False(t, hasHooks, "hooks should not be present in opencode.json")

	_, hasEvents := settings["events"]
	assert.False(t, hasEvents, "events should not be present in opencode.json")

	// MCP should be present (to verify compilation succeeded)
	_, hasMCP := settings["mcp"]
	assert.True(t, hasMCP, "mcp should be present in opencode.json")
}

func TestTranslateMCPToOpenCode(t *testing.T) {
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
						"type":    "local",
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
						"type":    "local",
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
		{
			name: "mixed local and remote servers",
			input: map[string]any{
				"mcpServers": map[string]any{
					"local": map[string]any{
						"command": "node",
						"args":    []any{"local.js"},
					},
					"remote": map[string]any{
						"url": "https://remote.example.com",
					},
				},
			},
			expected: map[string]any{
				"servers": map[string]any{
					"local": map[string]any{
						"type":    "local",
						"command": []any{"node", "local.js"},
					},
					"remote": map[string]any{
						"type": "remote",
						"url":  "https://remote.example.com",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPToOpenCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslatePermissionsToOpenCode(t *testing.T) {
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
			name: "bare tool name",
			input: map[string]any{
				"allow": []any{"Read"},
			},
			expected: []any{
				map[string]any{
					"action":   "read",
					"resource": "*",
					"effect":   "allow",
				},
			},
		},
		{
			name: "tool with args",
			input: map[string]any{
				"allow": []any{"Bash(npm test)"},
			},
			expected: []any{
				map[string]any{
					"action":   "shell",
					"resource": "npm test",
					"effect":   "allow",
				},
			},
		},
		{
			name: "tool with nested parens",
			input: map[string]any{
				"allow": []any{`Bash(echo "(hello)")`},
			},
			expected: []any{
				map[string]any{
					"action":   "shell",
					"resource": `echo "(hello)"`,
					"effect":   "allow",
				},
			},
		},
		{
			name: "mixed allow and deny",
			input: map[string]any{
				"allow": []any{"Read", "Edit(src/*)"},
				"deny":  []any{"Bash(rm *)"},
			},
			expected: []any{
				map[string]any{
					"action":   "read",
					"resource": "*",
					"effect":   "allow",
				},
				map[string]any{
					"action":   "edit",
					"resource": "src/*",
					"effect":   "allow",
				},
				map[string]any{
					"action":   "shell",
					"resource": "rm *",
					"effect":   "deny",
				},
			},
		},
		{
			name: "Agent→subagent",
			input: map[string]any{
				"allow": []any{"Agent"},
			},
			expected: []any{
				map[string]any{
					"action":   "subagent",
					"resource": "*",
					"effect":   "allow",
				},
			},
		},
		{
			name: "unknown tool passthrough",
			input: map[string]any{
				"allow": []any{"CustomTool(test)"},
			},
			expected: []any{
				map[string]any{
					"action":   "customtool",
					"resource": "test",
					"effect":   "allow",
				},
			},
		},
		{
			name: "mcp tool format",
			input: map[string]any{
				"allow": []any{"mcp__server__tool"},
			},
			expected: []any{
				map[string]any{
					"action":   "mcp__server__tool",
					"resource": "*",
					"effect":   "allow",
				},
			},
		},
		{
			name: "Write maps to edit",
			input: map[string]any{
				"allow": []any{"Write"},
			},
			expected: []any{
				map[string]any{
					"action":   "edit",
					"resource": "*",
					"effect":   "allow",
				},
			},
		},
		{
			name: "WebSearch maps",
			input: map[string]any{
				"allow": []any{"WebSearch"},
			},
			expected: []any{
				map[string]any{
					"action":   "websearch",
					"resource": "*",
					"effect":   "allow",
				},
			},
		},
		{
			name: "WebFetch maps",
			input: map[string]any{
				"allow": []any{"WebFetch"},
			},
			expected: []any{
				map[string]any{
					"action":   "webfetch",
					"resource": "*",
					"effect":   "allow",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsToOpenCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
