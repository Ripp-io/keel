package openhands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompile_BasicLayer(t *testing.T) {
	// Create a temporary layer directory
	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Create keel.yml
	keelYml := `schema: v1
name: test-layer
description: Test layer for OpenHands

instructions:
  - source: instructions.md
`
	err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644)
	require.NoError(t, err)

	// Create instructions.md
	instructions := `# Test Instructions

This is a test instruction file for OpenHands.
`
	err = os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instructions), 0644)
	require.NoError(t, err)

	// Create layer
	layer := config.Layer{
		Path:    layerDir,
		Type:    "project",
		Format:  config.FormatKeel,
		Version: "v1",
	}

	// Compile
	result, err := Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify output files exist
	agentsFile := filepath.Join(outputDir, "AGENTS.md")
	require.FileExists(t, agentsFile)

	// Read and verify content
	content, err := os.ReadFile(agentsFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "Test Instructions")
}

func TestCompile_WithMCP(t *testing.T) {
	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Create keel.yml with MCP
	keelYml := `schema: v1
name: test-layer-mcp
description: Test layer with MCP

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644)
	require.NoError(t, err)

	// Create instructions.md
	instructions := `# MCP Test`
	err = os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instructions), 0644)
	require.NoError(t, err)

	// Create mcp.json
	mcpConfig := `{
  "mcpServers": {
    "test-server": {
      "command": "node",
      "args": ["server.js"]
    }
  }
}`
	err = os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpConfig), 0644)
	require.NoError(t, err)

	// Create layer
	layer := config.Layer{
		Path:    layerDir,
		Type:    "project",
		Format:  config.FormatKeel,
		Version: "v1",
	}

	// Compile
	result, err := Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify MCP file exists
	mcpFile := filepath.Join(outputDir, ".openhands", "mcp.json")
	require.FileExists(t, mcpFile)

	// Verify MCP content
	content, err := os.ReadFile(mcpFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "test-server")
	require.Contains(t, string(content), "mcpServers")
}

func TestProfile_ProjectScope(t *testing.T) {
	profile := Profile(compiler.ScopeProject)
	require.Equal(t, "openhands", profile.Name)
	require.Equal(t, ".openhands", profile.OutputRoot)
	require.Equal(t, "AGENTS.md", profile.InstructionOutput)
	require.Equal(t, ".openhands/mcp.json", profile.MCPOutput)
	require.Equal(t, "config.toml", profile.SettingsFile)
}

func TestProfile_UserScope(t *testing.T) {
	profile := Profile(compiler.ScopeUser)
	require.Equal(t, "openhands", profile.Name)
	require.Equal(t, ".openhands", profile.OutputRoot)
	require.Equal(t, ".openhands/AGENTS.md", profile.InstructionOutput)
	require.Equal(t, ".openhands/mcp.json", profile.MCPOutput)
}

func TestCapabilities(t *testing.T) {
	profile := ProjectProfile()

	// OpenHands supports these
	require.True(t, profile.Capabilities.MCP)
	require.True(t, profile.Capabilities.Commands)
	require.True(t, profile.Capabilities.Agents)
	require.True(t, profile.Capabilities.Skills)
	require.True(t, profile.Capabilities.Bin)
	require.True(t, profile.Capabilities.Deps)

	// OpenHands doesn't support these
	require.False(t, profile.Capabilities.Hooks)
	require.False(t, profile.Capabilities.Permissions)
	require.False(t, profile.Capabilities.Plugins)
	require.False(t, profile.Capabilities.AgentMemory)
	require.False(t, profile.Capabilities.Rules)
}

func TestCompile_OpenHands_Categories(t *testing.T) {
	t.Parallel()

	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Create keel.yml with commands and skills
	keelYml := `schema: v1
name: test-categories

instructions:
  - source: instructions.md

commands:
  - source: commands/

skills:
  - source: skills/
`
	err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644)
	require.NoError(t, err)

	// Write instructions
	err = os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test"), 0644)
	require.NoError(t, err)

	// Write a command
	commandsDir := filepath.Join(layerDir, "commands")
	err = os.MkdirAll(commandsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(commandsDir, "test-cmd.md"), []byte("# Test Command"), 0644)
	require.NoError(t, err)

	// Write a skill
	skillsDir := filepath.Join(layerDir, "skills", "test-skill")
	err = os.MkdirAll(skillsDir, 0755)
	require.NoError(t, err)
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill
`
	err = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644)
	require.NoError(t, err)

	// Create layer
	layer := config.Layer{
		Path:    layerDir,
		Type:    "project",
		Format:  config.FormatKeel,
		Version: "v1",
	}

	// Compile
	_, err = Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	require.NoError(t, err)

	// Verify command appears in .openhands/commands/
	cmdPath := filepath.Join(outputDir, ".openhands", "commands", "test-cmd.md")
	require.FileExists(t, cmdPath)

	// Verify skill appears in .openhands/skills/
	skillPath := filepath.Join(outputDir, ".openhands", "skills", "test-skill", "SKILL.md")
	require.FileExists(t, skillPath)
}
