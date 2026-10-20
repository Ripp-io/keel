package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_OpenCode_Instructions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create AGENTS.md
	agentsContent := "# Test Instructions\n\nThis is a test."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsContent), 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)

	require.NoError(t, err)
	assert.Equal(t, "opencode", ir.SourceAgent)
	assert.NotNil(t, ir.Instructions)
	assert.Len(t, ir.Instructions, 1)
	assert.Equal(t, agentsContent, ir.Instructions[0].Content)
}

func TestReader_OpenCode_Settings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create opencode.json with model and OpenCode-specific settings
	settings := map[string]any{
		"model":         "claude-sonnet-4-20250514",
		"customSetting": "value",
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)

	require.NoError(t, err)
	assert.Equal(t, "opencode", ir.SourceAgent)

	// Model should be in ir.Settings
	assert.NotNil(t, ir.Settings)
	assert.Equal(t, "claude-sonnet-4-20250514", ir.Settings["model"])

	// Custom settings should be in AgentSpecific
	assert.NotNil(t, ir.AgentSpecific)
	assert.NotNil(t, ir.AgentSpecific["opencode"])
	assert.Equal(t, "value", ir.AgentSpecific["opencode"]["customSetting"])
}

func TestReader_OpenCode_MCP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create opencode.json with MCP in OpenCode V2 format
	settings := map[string]any{
		"mcp": map[string]any{
			"servers": map[string]any{
				"test-server": map[string]any{
					"type":    "local",
					"command": []any{"node", "server.js"},
					"environment": map[string]any{
						"API_KEY": "test",
					},
				},
			},
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)

	require.NoError(t, err)
	assert.Equal(t, "opencode", ir.SourceAgent)

	// MCP should be translated to IR format
	assert.NotNil(t, ir.MCP)
	mcpServers, ok := ir.MCP["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key should exist")

	testServer, ok := mcpServers["test-server"].(map[string]any)
	require.True(t, ok, "test-server should exist")

	// Verify translation: command array split into command + args
	assert.Equal(t, "node", testServer["command"])
	args, ok := testServer["args"].([]any)
	require.True(t, ok, "args should exist")
	assert.Equal(t, []any{"server.js"}, args)

	// Verify translation: environment → env
	env, ok := testServer["env"].(map[string]any)
	require.True(t, ok, "env should exist")
	assert.Equal(t, "test", env["API_KEY"])
}

func TestReader_OpenCode_MCP_Remote(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create opencode.json with remote MCP server
	settings := map[string]any{
		"mcp": map[string]any{
			"servers": map[string]any{
				"remote-server": map[string]any{
					"type": "remote",
					"url":  "https://mcp.example.com",
				},
			},
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)

	require.NoError(t, err)
	assert.Equal(t, "opencode", ir.SourceAgent)

	// MCP should be translated to IR format
	assert.NotNil(t, ir.MCP)
	mcpServers, ok := ir.MCP["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key should exist")

	remoteServer, ok := mcpServers["remote-server"].(map[string]any)
	require.True(t, ok, "remote-server should exist")

	// Verify remote server has URL and no command
	assert.Equal(t, "https://mcp.example.com", remoteServer["url"])
	_, hasCommand := remoteServer["command"]
	assert.False(t, hasCommand, "remote server should not have command field")
}

func TestReader_OpenCode_Permissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create opencode.json with permissions in OpenCode V2 format
	settings := map[string]any{
		"permissions": []any{
			map[string]any{"action": "shell", "resource": "npm test", "effect": "allow"},
			map[string]any{"action": "read", "resource": "*", "effect": "allow"},
			map[string]any{"action": "edit", "resource": "src/*", "effect": "allow"},
			map[string]any{"action": "shell", "resource": "rm *", "effect": "deny"},
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)

	require.NoError(t, err)
	assert.Equal(t, "opencode", ir.SourceAgent)

	// Permissions should be translated to IR format
	assert.NotNil(t, ir.Permissions)

	// Verify allow list
	allow, ok := ir.Permissions["allow"].([]any)
	require.True(t, ok, "allow key should exist")
	require.Len(t, allow, 3)
	assert.Contains(t, allow, "Bash(npm test)")
	assert.Contains(t, allow, "Read")
	assert.Contains(t, allow, "Edit(src/*)")

	// Verify deny list
	deny, ok := ir.Permissions["deny"].([]any)
	require.True(t, ok, "deny key should exist")
	require.Len(t, deny, 1)
	assert.Contains(t, deny, "Bash(rm *)")
}

func TestReader_OpenCode_Categories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create .opencode/commands/ and .opencode/skills/
	commandsDir := filepath.Join(dir, ".opencode", "commands")
	skillsDir := filepath.Join(dir, ".opencode", "skills")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	commandContent := "# Test Command\n\nCommand content."
	skillContent := "# Test Skill\n\nSkill content."

	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test-command.md"), []byte(commandContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte(skillContent), 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)

	require.NoError(t, err)
	assert.Equal(t, "opencode", ir.SourceAgent)

	// Verify commands category
	assert.NotNil(t, ir.Categories)
	commands, ok := ir.Categories["commands"]
	require.True(t, ok, "commands category should exist")
	require.Len(t, commands, 1)
	assert.Equal(t, "test-command.md", commands[0].RelPath)
	assert.Equal(t, commandContent, string(commands[0].Content))

	// Verify skills category
	skills, ok := ir.Categories["skills"]
	require.True(t, ok, "skills category should exist")
	require.Len(t, skills, 1)
	assert.Equal(t, "test-skill.md", skills[0].RelPath)
	assert.Equal(t, skillContent, string(skills[0].Content))
}

func TestTranslateMCPFromOpenCode(t *testing.T) {
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
			name: "basic local server",
			input: map[string]any{
				"servers": map[string]any{
					"test": map[string]any{
						"type":    "local",
						"command": []any{"node", "server.js"},
					},
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
			input: map[string]any{
				"servers": map[string]any{
					"test": map[string]any{
						"type":    "local",
						"command": []any{"node", "server.js", "--port=3000"},
						"environment": map[string]any{
							"API_KEY": "test",
						},
					},
				},
			},
			expected: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "node",
						"args":    []any{"server.js", "--port=3000"},
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
				"servers": map[string]any{
					"remote": map[string]any{
						"type": "remote",
						"url":  "https://mcp.example.com",
					},
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
			name: "mixed local and remote servers",
			input: map[string]any{
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
			expected: map[string]any{
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPFromOpenCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslatePermissionsFromOpenCode(t *testing.T) {
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
			name: "bare tool allow",
			input: []any{
				map[string]any{"action": "read", "resource": "*", "effect": "allow"},
			},
			expected: map[string]any{
				"allow": []any{"Read"},
			},
		},
		{
			name: "tool with resource",
			input: []any{
				map[string]any{"action": "shell", "resource": "npm test", "effect": "allow"},
			},
			expected: map[string]any{
				"allow": []any{"Bash(npm test)"},
			},
		},
		{
			name: "mixed allow and deny",
			input: []any{
				map[string]any{"action": "read", "resource": "*", "effect": "allow"},
				map[string]any{"action": "edit", "resource": "src/*", "effect": "allow"},
				map[string]any{"action": "shell", "resource": "rm *", "effect": "deny"},
			},
			expected: map[string]any{
				"allow": []any{"Read", "Edit(src/*)"},
				"deny":  []any{"Bash(rm *)"},
			},
		},
		{
			name: "all tool mappings",
			input: []any{
				map[string]any{"action": "shell", "resource": "*", "effect": "allow"},
				map[string]any{"action": "read", "resource": "*", "effect": "allow"},
				map[string]any{"action": "edit", "resource": "*", "effect": "allow"},
				map[string]any{"action": "websearch", "resource": "*", "effect": "allow"},
				map[string]any{"action": "webfetch", "resource": "*", "effect": "allow"},
				map[string]any{"action": "subagent", "resource": "*", "effect": "allow"},
			},
			expected: map[string]any{
				"allow": []any{"Bash", "Read", "Edit", "WebSearch", "WebFetch", "Agent"},
			},
		},
		{
			name: "unknown tool title case",
			input: []any{
				map[string]any{"action": "customtool", "resource": "*", "effect": "allow"},
			},
			expected: map[string]any{
				"allow": []any{"Customtool"},
			},
		},
		{
			name: "mcp tool passthrough",
			input: []any{
				map[string]any{"action": "mcp__server__tool", "resource": "*", "effect": "allow"},
			},
			expected: map[string]any{
				"allow": []any{"Mcp__server__tool"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsFromOpenCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReader_OpenCode_RoundTrip(t *testing.T) {
	t.Parallel()

	// Create a test directory with a full OpenCode native config
	dir := t.TempDir()

	settings := map[string]any{
		"model": "claude-sonnet-4-20250514",
		"mcp": map[string]any{
			"servers": map[string]any{
				"local-server": map[string]any{
					"type":    "local",
					"command": []any{"node", "server.js"},
					"environment": map[string]any{
						"API_KEY": "test",
					},
				},
				"remote-server": map[string]any{
					"type": "remote",
					"url":  "https://mcp.example.com",
				},
			},
		},
		"permissions": []any{
			map[string]any{"action": "shell", "resource": "npm test", "effect": "allow"},
			map[string]any{"action": "read", "resource": "*", "effect": "allow"},
		},
		"customSetting": "value",
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0644))

	// Read it
	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	// Translate back to OpenCode format
	translatedMCP := TranslateMCPToOpenCode(ir.MCP)
	translatedPerms := TranslatePermissionsToOpenCode(ir.Permissions)

	// Verify round-trip integrity
	assert.NotNil(t, translatedMCP)
	assert.NotNil(t, translatedPerms)

	// MCP should have both servers back
	servers, ok := translatedMCP["servers"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, servers, 2)

	localServer, ok := servers["local-server"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "local", localServer["type"])

	remoteServer, ok := servers["remote-server"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "remote", remoteServer["type"])
	assert.Equal(t, "https://mcp.example.com", remoteServer["url"])

	// Permissions should have 2 entries
	assert.Len(t, translatedPerms, 2)
}
