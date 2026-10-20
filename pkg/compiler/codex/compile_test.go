package codex

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

func TestCompile_Codex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-codex-compile
description: Minimal test layer
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test Instructions\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "codex", result.Output.Agent)

	// Verify AGENTS.md exists
	agentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, "AGENTS.md should exist")

	// Verify .codex directory exists
	codexDir := filepath.Join(dir, ".codex")
	_, err = os.Stat(codexDir)
	assert.NoError(t, err, ".codex directory should exist")
}

func TestCompile_Codex_MCPTranslation(t *testing.T) {
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
	assert.Equal(t, "codex", result.Output.Agent)

	// Verify settings file exists with embedded MCP
	settingsPath := filepath.Join(dir, ".codex", "config.toml")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "config.toml should exist")

	var settings map[string]any
	require.NoError(t, toml.Unmarshal(data, &settings))

	// Verify MCP was translated to Codex format
	mcp, ok := settings["mcp"].(map[string]any)
	require.True(t, ok, "mcp key should exist in settings")

	servers, ok := mcp["servers"].(map[string]any)
	require.True(t, ok, "servers key should exist in mcp")

	testServer, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "test-server should exist")

	// Verify Codex format: merged command array
	command, ok := testServer["command"].([]any)
	require.True(t, ok, "command should be an array")
	assert.Equal(t, []any{"node", "server.js"}, command)

	// Verify Codex format: env key (not environment)
	env, ok := testServer["env"].(map[string]any)
	require.True(t, ok, "env key should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestCompile_Codex_PermissionTranslation(t *testing.T) {
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
	assert.Equal(t, "codex", result.Output.Agent)

	// Verify settings file exists with embedded permissions
	settingsPath := filepath.Join(dir, ".codex", "config.toml")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "config.toml should exist")

	var settings map[string]any
	require.NoError(t, toml.Unmarshal(data, &settings))

	// Verify permissions was translated to Codex approval_policy format
	policy, ok := settings["approval_policy"].(map[string]any)
	require.True(t, ok, "approval_policy key should exist in settings")

	autoApprove, ok := policy["auto_approve"].([]any)
	require.True(t, ok, "auto_approve should exist")
	require.Len(t, autoApprove, 3, "should have 3 auto_approve entries")

	autoDeny, ok := policy["auto_deny"].([]any)
	require.True(t, ok, "auto_deny should exist")
	require.Len(t, autoDeny, 1, "should have 1 auto_deny entry")

	// Helper to find approval entry
	contains := func(list []any, value string) bool {
		for _, v := range list {
			if v.(string) == value {
				return true
			}
		}
		return false
	}

	// Verify Codex format entries
	assert.True(t, contains(autoApprove, "shell:npm test"), "Bash(npm test) should translate to shell:npm test")
	assert.True(t, contains(autoApprove, "read:*"), "Read should translate to read:*")
	assert.True(t, contains(autoApprove, "edit:src/*"), "Edit(src/*) should translate to edit:src/*")
	assert.True(t, contains(autoDeny, "shell:rm *"), "Bash(rm *) should translate to shell:rm *")
}

func TestCompile_Codex_UserScope(t *testing.T) {
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
	assert.Equal(t, "codex", result.Output.Agent)

	// Verify instructions written to .config/codex/AGENTS.md (not root)
	agentsPath := filepath.Join(dir, ".config", "codex", "AGENTS.md")
	_, err = os.Stat(agentsPath)
	assert.NoError(t, err, ".config/codex/AGENTS.md should exist")

	// Verify settings written to .config/codex/config.toml
	settingsPath := filepath.Join(dir, ".config", "codex", "config.toml")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, ".config/codex/config.toml should exist")

	// Verify .config/codex/ directory exists
	configDir := filepath.Join(dir, ".config", "codex")
	_, err = os.Stat(configDir)
	assert.NoError(t, err, ".config/codex/ directory should exist")

	// Verify no .codex/ at root
	rootCodexDir := filepath.Join(dir, ".codex")
	_, err = os.Stat(rootCodexDir)
	assert.True(t, os.IsNotExist(err), ".codex/ should not exist at root for user scope")

	// Verify no AGENTS.md at root
	rootAgentsPath := filepath.Join(dir, "AGENTS.md")
	_, err = os.Stat(rootAgentsPath)
	assert.True(t, os.IsNotExist(err), "AGENTS.md should not exist at root for user scope")
}

func TestCompile_Codex_Categories(t *testing.T) {
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
	assert.Equal(t, "codex", result.Output.Agent)

	// Verify .codex/commands/ contains the command file
	commandPath := filepath.Join(dir, ".codex", "commands", "test-command.md")
	data, err := os.ReadFile(commandPath)
	require.NoError(t, err, ".codex/commands/test-command.md should exist")
	assert.Equal(t, commandContent, string(data))

	// Verify .codex/skills/ contains the skill file
	skillPath := filepath.Join(dir, ".codex", "skills", "test-skill.md")
	data, err = os.ReadFile(skillPath)
	require.NoError(t, err, ".codex/skills/test-skill.md should exist")
	assert.Equal(t, skillContent, string(data))
}

func TestTranslateMCPToCodex(t *testing.T) {
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
						"env": map[string]any{
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
						"url": "https://mcp.example.com",
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
						"command": []any{"node", "local.js"},
					},
					"remote": map[string]any{
						"url": "https://remote.example.com",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPToCodex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslatePermissionsToCodex(t *testing.T) {
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
			name: "bare tool name",
			input: map[string]any{
				"allow": []any{"Read"},
			},
			expected: map[string]any{
				"auto_approve": []any{"read:*"},
			},
		},
		{
			name: "tool with args",
			input: map[string]any{
				"allow": []any{"Bash(npm test)"},
			},
			expected: map[string]any{
				"auto_approve": []any{"shell:npm test"},
			},
		},
		{
			name: "mixed allow and deny",
			input: map[string]any{
				"allow": []any{"Read", "Edit(src/*)"},
				"deny":  []any{"Bash(rm *)"},
			},
			expected: map[string]any{
				"auto_approve": []any{"read:*", "edit:src/*"},
				"auto_deny":    []any{"shell:rm *"},
			},
		},
		{
			name: "Agent→subagent",
			input: map[string]any{
				"allow": []any{"Agent"},
			},
			expected: map[string]any{
				"auto_approve": []any{"subagent:*"},
			},
		},
		{
			name: "unknown tool passthrough",
			input: map[string]any{
				"allow": []any{"CustomTool(test)"},
			},
			expected: map[string]any{
				"auto_approve": []any{"customtool:test"},
			},
		},
		{
			name: "Write maps to edit",
			input: map[string]any{
				"allow": []any{"Write"},
			},
			expected: map[string]any{
				"auto_approve": []any{"edit:*"},
			},
		},
		{
			name: "WebSearch maps",
			input: map[string]any{
				"allow": []any{"WebSearch"},
			},
			expected: map[string]any{
				"auto_approve": []any{"websearch:*"},
			},
		},
		{
			name: "WebFetch maps",
			input: map[string]any{
				"allow": []any{"WebFetch"},
			},
			expected: map[string]any{
				"auto_approve": []any{"webfetch:*"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsToCodex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
