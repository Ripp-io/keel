package vibe

import (
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompile_Vibe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-vibe-compile
description: Minimal test layer
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test Instructions\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "vibe", result.Output.Agent)

	// Verify AGENTS.md exists
	agentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, "AGENTS.md should exist")

	// Verify .vibe directory exists
	vibeDir := filepath.Join(dir, ".vibe")
	_, err = os.Stat(vibeDir)
	assert.NoError(t, err, ".vibe directory should exist")
}

func TestCompile_Vibe_MCPTranslation(t *testing.T) {
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
	assert.Equal(t, "vibe", result.Output.Agent)

	// Verify settings file exists with embedded MCP
	settingsPath := filepath.Join(dir, ".vibe", "config.toml")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "config.toml should exist")

	var settings map[string]any
	require.NoError(t, toml.Unmarshal(data, &settings))

	// Verify MCP was translated to Vibe format (array, not object)
	mcpServers, ok := settings["mcp_servers"].([]any)
	require.True(t, ok, "mcp_servers key should exist in settings as array")
	require.Len(t, mcpServers, 1, "should have 1 MCP server")

	server, ok := mcpServers[0].(map[string]any)
	require.True(t, ok, "server should be a map")

	// Verify Vibe format fields
	assert.Equal(t, "test-server", server["name"])
	assert.Equal(t, "stdio", server["transport"])
	assert.Equal(t, "node", server["command"])

	args, ok := server["args"].([]any)
	require.True(t, ok, "args should be an array")
	assert.Equal(t, []any{"server.js"}, args)

	env, ok := server["env"].(map[string]any)
	require.True(t, ok, "env key should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestCompile_Vibe_UserScope(t *testing.T) {
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
	assert.Equal(t, "vibe", result.Output.Agent)

	// Verify instructions written to .vibe/AGENTS.md (not root)
	agentsPath := filepath.Join(dir, ".vibe", "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, ".vibe/AGENTS.md should exist")

	// Verify settings written to .vibe/config.toml
	settingsPath := filepath.Join(dir, ".vibe", "config.toml")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, ".vibe/config.toml should exist")

	// Verify .vibe/ directory exists
	vibeDir := filepath.Join(dir, ".vibe")
	_, err = os.Stat(vibeDir)
	assert.NoError(t, err, ".vibe/ directory should exist")

	// Verify no AGENTS.md at root
	rootAgentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(rootAgentsPath)
	assert.True(t, os.IsNotExist(err), "AGENTS.md should not exist at root for user scope")
}

func TestCompile_Vibe_Categories(t *testing.T) {
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
	assert.Equal(t, "vibe", result.Output.Agent)

	// Verify .vibe/commands/ contains the command file
	commandPath := filepath.Join(dir, ".vibe", "commands", "test-command.md")
	data, err := os.ReadFile(commandPath)
	require.NoError(t, err, ".vibe/commands/test-command.md should exist")
	assert.Equal(t, commandContent, string(data))

	// Verify .vibe/skills/ contains the skill file
	skillPath := filepath.Join(dir, ".vibe", "skills", "test-skill.md")
	data, err = os.ReadFile(skillPath)
	require.NoError(t, err, ".vibe/skills/test-skill.md should exist")
	assert.Equal(t, skillContent, string(data))
}

func TestCompile_Vibe_RemoteMCPServer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-remote-mcp
mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	mcpContent := `{"mcpServers": {"remote-server": {"url": "https://api.example.com/mcp"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeProject})

	require.NoError(t, err)
	assert.Equal(t, "vibe", result.Output.Agent)

	// Verify settings file
	settingsPath := filepath.Join(dir, ".vibe", "config.toml")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, toml.Unmarshal(data, &settings))

	mcpServers, ok := settings["mcp_servers"].([]any)
	require.True(t, ok)
	require.Len(t, mcpServers, 1)

	server, ok := mcpServers[0].(map[string]any)
	require.True(t, ok)

	// Verify remote server format
	assert.Equal(t, "remote-server", server["name"])
	assert.Equal(t, "http", server["transport"])
	assert.Equal(t, "https://api.example.com/mcp", server["url"])
	assert.Nil(t, server["command"], "remote server should not have command field")
}

func TestTranslateMCPToVibe(t *testing.T) {
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
			name: "basic local server",
			input: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "node",
						"args":    []any{"server.js"},
					},
				},
			},
			expected: []any{
				map[string]any{
					"name":      "test",
					"transport": "stdio",
					"command":   "node",
					"args":      []any{"server.js"},
				},
			},
		},
		{
			name: "local server with env",
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
			expected: []any{
				map[string]any{
					"name":      "test",
					"transport": "stdio",
					"command":   "node",
					"args":      []any{"server.js"},
					"env": map[string]any{
						"API_KEY": "test",
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
			expected: []any{
				map[string]any{
					"name":      "remote",
					"transport": "http",
					"url":       "https://mcp.example.com",
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
			// Note: Result order is non-deterministic for map iteration
			expected: nil, // We'll check length and contents individually
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPToVibe(tt.input)
			if tt.expected == nil && tt.name != "mixed local and remote servers" {
				assert.Nil(t, result)
				return
			}

			if tt.name == "mixed local and remote servers" {
				// Special case: check length and presence
				require.Len(t, result, 2)
				// Just verify structure, not order
				for _, serverRaw := range result {
					server, ok := serverRaw.(map[string]any)
					require.True(t, ok)
					name, ok := server["name"].(string)
					require.True(t, ok)
					assert.Contains(t, []string{"local", "remote"}, name)
				}
				return
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslateMCPFromVibe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []any
		expected map[string]any
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
			name: "basic local server",
			input: []any{
				map[string]any{
					"name":      "test",
					"transport": "stdio",
					"command":   "node",
					"args":      []any{"server.js"},
				},
			},
			expected: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "node",
						"args":    []any{"server.js"},
					},
				},
			},
		},
		{
			name: "local server with env",
			input: []any{
				map[string]any{
					"name":      "test",
					"transport": "stdio",
					"command":   "node",
					"args":      []any{"server.js"},
					"env": map[string]any{
						"API_KEY": "test",
					},
				},
			},
			expected: map[string]any{
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
		},
		{
			name: "remote server",
			input: []any{
				map[string]any{
					"name":      "remote",
					"transport": "http",
					"url":       "https://mcp.example.com",
				},
			},
			expected: map[string]any{
				"mcpServers": map[string]any{
					"remote": map[string]any{
						"url": "https://mcp.example.com",
					},
				},
			},
		},
		{
			name: "server without name (skip)",
			input: []any{
				map[string]any{
					"transport": "stdio",
					"command":   "node",
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPFromVibe(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
