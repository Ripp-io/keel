package importer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/yaml.v3"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/importer"
)

func TestSerializeIR_Full(t *testing.T) {
	t.Parallel()
	ir := &compiler.LayerIR{
		Name:        "test-layer",
		Description: "A test configuration layer",
		SourceAgent: "claude",
		Instructions: []compiler.InstructionBlock{
			{Filename: "CLAUDE.md", Content: "# Test Instructions\n\nDo the thing."},
		},
		Permissions: map[string]any{
			"allow": []string{"Read", "Edit"},
			"deny":  []string{"Bash"},
		},
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"gitlab": map[string]any{
					"command": "npx",
					"args":    []string{"-y", "@gitlab/mcp"},
				},
			},
		},
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{"matcher": "Write", "hooks": []string{"echo pre"}},
			},
		},
		Settings: map[string]any{
			"model": "claude-sonnet-4-20250514",
		},
		AgentSpecific: map[string]map[string]any{
			"claude": {
				"sandbox": map[string]any{"enabled": true},
				"env":     map[string]any{"DEBUG": "1"},
			},
		},
		Categories: map[string][]compiler.CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("# Deploy\nRun deployment.")},
			},
		},
	}

	outputDir := t.TempDir()
	require.NoError(t, importer.SerializeIR(ir, outputDir))

	keelData, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err, "reading keel.yml")
	var keelCfg map[string]any
	require.NoError(t, yaml.Unmarshal(keelData, &keelCfg), "parsing keel.yml")
	assert.Equal(t, "v1", keelCfg["schema"])
	assert.Equal(t, "test-layer", keelCfg["name"])

	instrData, err := os.ReadFile(filepath.Join(outputDir, "instructions.md"))
	require.NoError(t, err, "reading instructions.md")
	assert.Contains(t, string(instrData), "Test Instructions")

	assertJSONFileHasKey(t, filepath.Join(outputDir, "permissions.json"), "allow")
	assertJSONFileHasKey(t, filepath.Join(outputDir, "permissions.json"), "deny")
	assertJSONFileHasKey(t, filepath.Join(outputDir, "mcp.json"), "mcpServers")
	assertJSONFileHasKey(t, filepath.Join(outputDir, "hooks.json"), "PreToolUse")

	claudeSettingsPath := filepath.Join(outputDir, "settings.claude.json")
	_, err = os.Stat(claudeSettingsPath)
	require.NoError(t, err, "expected settings.claude.json to exist")
	assertJSONFileHasKey(t, claudeSettingsPath, "sandbox")
	assertJSONFileHasKey(t, claudeSettingsPath, "env")

	deployData, err := os.ReadFile(filepath.Join(outputDir, "commands", "deploy.md"))
	require.NoError(t, err, "reading commands/deploy.md")
	assert.Contains(t, string(deployData), "Deploy")

	keelText := string(keelData)
	for _, ref := range []string{"instructions.md", "permissions.json", "mcp.json", "hooks.json", "commands/"} {
		assert.Contains(t, keelText, ref, "expected keel.yml to reference %q", ref)
	}
}

func TestSerializeIR_Empty(t *testing.T) {
	t.Parallel()
	ir := &compiler.LayerIR{}
	outputDir := t.TempDir()

	require.NoError(t, importer.SerializeIR(ir, outputDir))

	keelData, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err, "reading keel.yml")
	var keelCfg map[string]any
	require.NoError(t, yaml.Unmarshal(keelData, &keelCfg), "parsing keel.yml")
	assert.Equal(t, "v1", keelCfg["schema"])
	assert.Equal(t, "imported-config", keelCfg["name"])

	for _, file := range []string{"instructions.md", "permissions.json", "mcp.json", "hooks.json"} {
		path := filepath.Join(outputDir, file)
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "expected %s to not exist for empty IR", file)
	}

	// keel.yml should have no section references
	keelText := string(keelData)
	for _, section := range []string{"instructions:", "permissions:", "mcp:", "hooks:", "commands:", "agents:", "skills:"} {
		assert.NotContains(t, keelText, section,
			"empty IR keel.yml should not contain section %q", section)
	}
}

func TestSerializeIR_NilIR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := importer.SerializeIR(nil, dir)
	require.Error(t, err, "expected error when serializing nil IR")
	assert.Contains(t, err.Error(), "nil")
}

func TestSerializeIR_PartialFields(t *testing.T) {
	t.Parallel()
	ir := &compiler.LayerIR{
		SourceAgent: "claude",
		Instructions: []compiler.InstructionBlock{
			{Filename: "CLAUDE.md", Content: "# Partial\n\nOnly instructions and MCP."},
		},
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"context7": map[string]any{"command": "npx"},
			},
		},
	}

	outputDir := t.TempDir()
	require.NoError(t, importer.SerializeIR(ir, outputDir))

	_, err := os.Stat(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err, "expected keel.yml to exist")
	_, err = os.Stat(filepath.Join(outputDir, "instructions.md"))
	require.NoError(t, err, "expected instructions.md to exist")
	_, err = os.Stat(filepath.Join(outputDir, "mcp.json"))
	require.NoError(t, err, "expected mcp.json to exist")

	for _, file := range []string{"permissions.json", "hooks.json", "environment.json"} {
		path := filepath.Join(outputDir, file)
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "expected %s to not exist for partial IR", file)
	}

	for _, catDir := range []string{"commands", "agents", "skills"} {
		path := filepath.Join(outputDir, catDir)
		_, statErr := os.Stat(path)
		assert.True(t, os.IsNotExist(statErr), "expected %s/ to not exist for partial IR", catDir)
	}
}

// compilerTestdataDir returns the absolute path to the compiler package's
// testdata directory, used by the round-trip test.
func compilerTestdataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "unable to determine test file location")
	return filepath.Join(filepath.Dir(filename), "..", "compiler", "testdata")
}

func TestRoundTrip_ReaderSerializer(t *testing.T) {
	t.Parallel()
	reader, err := compiler.GetReader("claude")
	require.NoError(t, err)
	layerPath := filepath.Join(compilerTestdataDir(t), "reader-claude", "full")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	outputDir := t.TempDir()
	require.NoError(t, importer.SerializeIR(ir, outputDir))

	keelData, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err, "reading keel.yml")
	var keelCfg map[string]any
	require.NoError(t, yaml.Unmarshal(keelData, &keelCfg), "keel.yml is not valid YAML")

	instrData, err := os.ReadFile(filepath.Join(outputDir, "instructions.md"))
	require.NoError(t, err, "reading instructions.md")
	assert.Contains(t, string(instrData), "Test Project")

	permsData, err := os.ReadFile(filepath.Join(outputDir, "permissions.json"))
	require.NoError(t, err, "reading permissions.json")
	var perms map[string]any
	require.NoError(t, json.Unmarshal(permsData, &perms), "parsing permissions.json")
	assert.Contains(t, perms, "allow")
	assert.Contains(t, perms, "deny")

	mcpData, err := os.ReadFile(filepath.Join(outputDir, "mcp.json"))
	require.NoError(t, err, "reading mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp), "parsing mcp.json")
	assert.Contains(t, mcp, "mcpServers")

	deployPath := filepath.Join(outputDir, "commands", "deploy.md")
	deployData, err := os.ReadFile(deployPath)
	require.NoError(t, err, "reading commands/deploy.md")
	assert.Contains(t, string(deployData), "Deploy")

	skillPath := filepath.Join(outputDir, "skills", "security", "SKILL.md")
	skillData, err := os.ReadFile(skillPath)
	require.NoError(t, err, "reading skills/security/SKILL.md")
	assert.Contains(t, string(skillData), "Security Skill")
}

// assertJSONFileHasKey reads a JSON file and checks that the given key exists
// at the top level.
func assertJSONFileHasKey(t *testing.T, path, key string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", filepath.Base(path))
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m), "parsing %s: content: %s", filepath.Base(path), string(data))
	assert.Contains(t, m, key, "expected %s to have key %q", filepath.Base(path), key)
}
