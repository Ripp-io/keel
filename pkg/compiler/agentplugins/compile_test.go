package agentplugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompile_AgentPlugins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: my-test-plugin
description: A test plugin for verification
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "agentplugins", result.Output.Agent)

	// Verify plugin.json exists at output root.
	pluginPath := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(pluginPath)
	require.NoError(t, err, "plugin.json should exist")

	var plugin map[string]any
	require.NoError(t, json.Unmarshal(data, &plugin))
	assert.Equal(t, schemaBaseURL+"/plugin.schema.json", plugin["$schema"])
	assert.Equal(t, "my-test-plugin", plugin["name"])
	assert.Equal(t, "A test plugin for verification", plugin["description"])

	// Verify .agentplugins/ directory exists (for .keel.json).
	agentDir := filepath.Join(dir, ".agentplugins")
	_, err = os.Stat(agentDir)
	assert.NoError(t, err, ".agentplugins/ should exist")

	// Verify provenance extension is present.
	ext, ok := plugin["extensions"].(map[string]any)
	require.True(t, ok, "extensions should exist in plugin.json")
	keel, ok := ext["io.ripp.keel"].(map[string]any)
	require.True(t, ok, "io.ripp.keel extension should exist")
	assert.NotEmpty(t, keel["compiled_at"], "compiled_at should be set")
	assert.NotEmpty(t, keel["keel_version"], "keel_version should be set")
}

func TestCompile_AgentPlugins_Skills(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	skillDir := filepath.Join(layerDir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))

	keelYML := `schema: v1
name: skill-plugin
skills:
  - source: skills/
`
	skillContent := "---\nname: my-skill\ndescription: A test skill\n---\n\n# My Skill\n\nDo the thing.\n"

	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "agentplugins", result.Output.Agent)

	// Verify skills/ at output root (not under .agentplugins/).
	skillPath := filepath.Join(dir, "skills", "my-skill", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	require.NoError(t, err, "skills/my-skill/SKILL.md should exist at output root")
	assert.Equal(t, skillContent, string(data))

	// Verify skills are NOT under .agentplugins/.
	wrongPath := filepath.Join(dir, ".agentplugins", "skills")
	_, err = os.Stat(wrongPath)
	assert.True(t, os.IsNotExist(err), "skills should NOT be under .agentplugins/")
}

func TestCompile_AgentPlugins_MCP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: mcp-plugin
mcp:
  servers:
    - source: mcp.json
`
	mcpContent := `{"mcpServers": {"local-server": {"command": "node", "args": ["server.js"], "env": {"API_KEY": "test"}}, "remote-server": {"url": "https://mcp.example.com/sse"}}}`

	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "agentplugins", result.Output.Agent)

	// Verify mcp.json at output root.
	mcpPath := filepath.Join(dir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "mcp.json should exist at output root")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))

	// Verify $schema.
	assert.Equal(t, schemaBaseURL+"/mcp.schema.json", mcp["$schema"])

	// Verify servers have type field.
	servers := mcp["mcpServers"].(map[string]any)

	localServer := servers["local-server"].(map[string]any)
	assert.Equal(t, "stdio", localServer["type"], "command-based server should be type stdio")
	assert.Equal(t, "node", localServer["command"])

	remoteServer := servers["remote-server"].(map[string]any)
	assert.Equal(t, "streamable-http", remoteServer["type"], "URL-based server should be type streamable-http")
	assert.Equal(t, "https://mcp.example.com/sse", remoteServer["url"])
}

func TestCompile_AgentPlugins_Variables(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: full-plugin
description: Plugin with all metadata
variables:
  plugin.version: "1.2.3"
  plugin.license: "MIT"
  plugin.homepage: "https://example.com"
  plugin.repository: "https://github.com/org/plugin"
  plugin.author.name: "Test Author"
  plugin.author.email: "test@example.com"
  plugin.author.url: "https://example.com/author"
  plugin.keywords: "testing, example, keel"
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	_, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)

	pluginPath := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(pluginPath)
	require.NoError(t, err)

	var plugin map[string]any
	require.NoError(t, json.Unmarshal(data, &plugin))

	assert.Equal(t, "1.2.3", plugin["version"])
	assert.Equal(t, "MIT", plugin["license"])
	assert.Equal(t, "https://example.com", plugin["homepage"])
	assert.Equal(t, "https://github.com/org/plugin", plugin["repository"])

	author := plugin["author"].(map[string]any)
	assert.Equal(t, "Test Author", author["name"])
	assert.Equal(t, "test@example.com", author["email"])
	assert.Equal(t, "https://example.com/author", author["url"])

	keywords := plugin["keywords"].([]any)
	assert.Equal(t, []any{"testing", "example", "keel"}, keywords)
}

func TestCompile_AgentPlugins_NoMCP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: no-mcp-plugin
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	_, err := Compile(layers, dir, compiler.CompileOptions{})
	require.NoError(t, err)

	// Verify no mcp.json when there are no MCP servers.
	mcpPath := filepath.Join(dir, "mcp.json")
	_, err = os.Stat(mcpPath)
	assert.True(t, os.IsNotExist(err), "mcp.json should NOT exist when no MCP servers")
}

func TestReader_PluginJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	pluginJSON := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "test-plugin",
  "version": "2.0.0",
  "description": "A test plugin",
  "author": {"name": "Test Author", "email": "test@example.com"},
  "license": "MIT",
  "homepage": "https://example.com",
  "repository": "https://github.com/org/plugin",
  "keywords": ["testing", "example"]
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(pluginJSON), 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	assert.Equal(t, "test-plugin", ir.Name)
	assert.Equal(t, "A test plugin", ir.Description)

	// Variables should be populated from plugin.json fields.
	assert.Equal(t, "2.0.0", ir.Variables["plugin.version"])
	assert.Equal(t, "MIT", ir.Variables["plugin.license"])
	assert.Equal(t, "https://example.com", ir.Variables["plugin.homepage"])
	assert.Equal(t, "https://github.com/org/plugin", ir.Variables["plugin.repository"])
	assert.Equal(t, "Test Author", ir.Variables["plugin.author.name"])
	assert.Equal(t, "test@example.com", ir.Variables["plugin.author.email"])
	assert.Equal(t, "testing, example", ir.Variables["plugin.keywords"])
}

func TestReader_MCP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	mcpJSON := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "local": {"type": "stdio", "command": "node", "args": ["server.js"]},
    "remote": {"type": "streamable-http", "url": "https://mcp.example.com"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(mcpJSON), 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	require.NotNil(t, ir.MCP)
	servers := ir.MCP["mcpServers"].(map[string]any)

	// Type field should be stripped.
	local := servers["local"].(map[string]any)
	assert.Nil(t, local["type"], "type should be stripped from IR")
	assert.Equal(t, "node", local["command"])

	remote := servers["remote"].(map[string]any)
	assert.Nil(t, remote["type"], "type should be stripped from IR")
	assert.Equal(t, "https://mcp.example.com", remote["url"])
}

func TestReader_Skills(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))

	skillContent := "---\nname: my-skill\n---\n\n# My Skill\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	require.Contains(t, ir.Categories, "skills")
	// Should contain the directory entry and the file.
	found := false
	for _, f := range ir.Categories["skills"] {
		if f.RelPath == "my-skill/SKILL.md" && !f.IsDir {
			found = true
			assert.Equal(t, skillContent, string(f.Content))
		}
	}
	assert.True(t, found, "should find my-skill/SKILL.md in categories")
}

func TestReader_Extensions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	pluginJSON := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "ext-plugin",
  "extensions": {
    "com.anthropic.claude": {"hooks": {"onSave": "lint"}},
    "com.example.unknown": {"setting": true},
    "io.ripp.keel": {"compiled_at": "2026-01-01T00:00:00Z"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(pluginJSON), 0644))

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)

	// Known vendor extension should map to IR agent name.
	require.Contains(t, ir.AgentSpecific, "claude")
	assert.Equal(t, "lint", ir.AgentSpecific["claude"]["hooks"].(map[string]any)["onSave"])

	// Unknown vendor extensions should be preserved under "agentplugins".
	require.Contains(t, ir.AgentSpecific, "agentplugins")
	ext := ir.AgentSpecific["agentplugins"]["extensions"].(map[string]any)
	unknownExt := ext["com.example.unknown"].(map[string]any)
	assert.Equal(t, true, unknownExt["setting"])

	// Keel provenance should NOT be round-tripped.
	assert.NotContains(t, ext, "io.ripp.keel")
}

func TestReader_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	reader := &Reader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "", ir.Name)
	assert.Nil(t, ir.MCP)
	assert.Nil(t, ir.Categories)
}

func TestTranslateMCPFromAgentPlugins(t *testing.T) {
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
			name: "strips type and schema",
			input: map[string]any{
				"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
				"mcpServers": map[string]any{
					"test": map[string]any{
						"type":    "stdio",
						"command": "node",
						"args":    []any{"server.js"},
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
			name: "preserves cwd and env",
			input: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"type":    "stdio",
						"command": "python",
						"cwd":     "${PLUGIN_ROOT}",
						"env":     map[string]any{"KEY": "val"},
					},
				},
			},
			expected: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "python",
						"cwd":     "${PLUGIN_ROOT}",
						"env":     map[string]any{"KEY": "val"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPFromAgentPlugins(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTrip_ReadWriteRead(t *testing.T) {
	t.Parallel()

	// Create a plugin directory with all supported content.
	srcDir := t.TempDir()

	pluginJSON := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "round-trip-test",
  "version": "1.0.0",
  "description": "Round trip test plugin",
  "author": {"name": "Author", "email": "a@b.com"},
  "license": "MIT",
  "keywords": ["test", "round-trip"],
  "extensions": {
    "com.anthropic.claude": {"custom": "value"},
    "com.example.unknown": {"keep": true}
  }
}`
	mcpJSON := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "local": {"type": "stdio", "command": "node", "args": ["s.js"]},
    "remote": {"type": "streamable-http", "url": "https://example.com"}
  }
}`
	skillContent := "---\nname: test-skill\n---\n\n# Test\n"
	skillDir := filepath.Join(srcDir, "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "plugin.json"), []byte(pluginJSON), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "mcp.json"), []byte(mcpJSON), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644))

	// Step 1: Read the plugin directory.
	reader := &Reader{}
	ir, err := reader.Read(srcDir)
	require.NoError(t, err)

	assert.Equal(t, "round-trip-test", ir.Name)
	assert.Equal(t, "Round trip test plugin", ir.Description)
	assert.Equal(t, "1.0.0", ir.Variables["plugin.version"])
	assert.Equal(t, "MIT", ir.Variables["plugin.license"])
	assert.Equal(t, "test, round-trip", ir.Variables["plugin.keywords"])
	assert.Contains(t, ir.AgentSpecific, "claude")
	assert.Contains(t, ir.Categories, "skills")
	assert.NotNil(t, ir.MCP)

	// Step 2: Compile through Keel using the read IR as a native layer.
	outDir := t.TempDir()
	layers := []config.Layer{{Path: srcDir, Format: "agentplugins", Type: "native"}}
	_, err = Compile(layers, outDir, compiler.CompileOptions{})
	require.NoError(t, err)

	// Step 3: Read the output back.
	reader2 := &Reader{}
	ir2, err := reader2.Read(outDir)
	require.NoError(t, err)

	// Verify round-trip fidelity.
	assert.Equal(t, ir.Name, ir2.Name)
	assert.Equal(t, ir.Description, ir2.Description)
	assert.Equal(t, ir.Variables["plugin.version"], ir2.Variables["plugin.version"])
	assert.Equal(t, ir.Variables["plugin.license"], ir2.Variables["plugin.license"])
	assert.Equal(t, ir.Variables["plugin.keywords"], ir2.Variables["plugin.keywords"])

	// MCP servers should round-trip (type is stripped on read, re-inferred on write).
	assert.NotNil(t, ir2.MCP)
	servers := ir2.MCP["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "local")
	assert.Contains(t, servers, "remote")

	// Known vendor extensions should round-trip.
	assert.Contains(t, ir2.AgentSpecific, "claude")

	// Unknown vendor extensions should round-trip.
	assert.Contains(t, ir2.AgentSpecific, "agentplugins")
	ext := ir2.AgentSpecific["agentplugins"]["extensions"].(map[string]any)
	assert.Contains(t, ext, "com.example.unknown")
}

func TestSanitizePluginName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"my-plugin", "my-plugin"},
		{"My Plugin", "my-plugin"},
		{"MY_PLUGIN_V2", "my-plugin-v2"},
		{"test@plugin!123", "test-plugin-123"},
		{"", "unnamed-plugin"},
		{"---invalid---", "invalid"},
		{"a", "a"},
		{"UPPER.case.MiXeD", "upper.case.mixed"},
		{"spaces and things", "spaces-and-things"},
		{"double--hyphen", "double-hyphen"},
		{"double..dot", "double-dot"},
		// Truncation: name longer than 64 chars.
		{strings.Repeat("a", 65), strings.Repeat("a", 64)},
		// Truncation trims trailing separators after cutting.
		// "aaa...a--b" (66 chars) → collapse -- → "aaa...a-b" (65) → cut to 64 → "aaa...a-" → trim → "aaa...a" (63)
		{strings.Repeat("a", 63) + "--b", strings.Repeat("a", 63)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizePluginName(tt.input))
		})
	}
}

func TestTranslateMCPToAgentPlugins(t *testing.T) {
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
			name: "stdio server",
			input: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "node",
						"args":    []any{"server.js"},
					},
				},
			},
			expected: map[string]any{
				"$schema": schemaBaseURL + "/mcp.schema.json",
				"mcpServers": map[string]any{
					"test": map[string]any{
						"type":    "stdio",
						"command": "node",
						"args":    []any{"server.js"},
					},
				},
			},
		},
		{
			name: "streamable-http server",
			input: map[string]any{
				"mcpServers": map[string]any{
					"remote": map[string]any{
						"url": "https://mcp.example.com",
					},
				},
			},
			expected: map[string]any{
				"$schema": schemaBaseURL + "/mcp.schema.json",
				"mcpServers": map[string]any{
					"remote": map[string]any{
						"type": "streamable-http",
						"url":  "https://mcp.example.com",
					},
				},
			},
		},
		{
			name: "stdio with env and cwd",
			input: map[string]any{
				"mcpServers": map[string]any{
					"test": map[string]any{
						"command": "python",
						"args":    []any{"-m", "server"},
						"env":     map[string]any{"KEY": "val"},
						"cwd":     "${PLUGIN_ROOT}",
					},
				},
			},
			expected: map[string]any{
				"$schema": schemaBaseURL + "/mcp.schema.json",
				"mcpServers": map[string]any{
					"test": map[string]any{
						"type":    "stdio",
						"command": "python",
						"args":    []any{"-m", "server"},
						"env":     map[string]any{"KEY": "val"},
						"cwd":     "${PLUGIN_ROOT}",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateMCPToAgentPlugins(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
