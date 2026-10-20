package antigravity_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/antigravity"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompile_WithInstructions(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for input and output
	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a layer with instructions
	instructionsPath := filepath.Join(layerDir, "instructions.md")
	if err := os.WriteFile(instructionsPath, []byte("# Test Instructions\n\nThis is a test."), 0644); err != nil {
		t.Fatalf("Failed to write instructions: %v", err)
	}

	// Create keel.yml
	keelYmlContent := `schema: v1
name: test-layer
description: Test layer with instructions

instructions:
  - source: instructions.md
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYmlContent), 0644); err != nil {
		t.Fatalf("Failed to write keel.yml: %v", err)
	}

	// Create layer
	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	// Compile
	result, err := antigravity.Compile(layers, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})

	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if result == nil {
		t.Fatal("Compile returned nil result")
	}

	// Verify instructions were written
	instructionsOutput := filepath.Join(outputDir, "GEMINI.md")
	if _, err := os.Stat(instructionsOutput); os.IsNotExist(err) {
		t.Error("GEMINI.md was not created")
	}
}

func TestProfile_ProjectScope(t *testing.T) {
	t.Parallel()

	profile := antigravity.ProjectProfile()

	if profile.Name != "antigravity" {
		t.Errorf("Expected agent name 'antigravity', got %q", profile.Name)
	}

	if profile.OutputRoot != ".gemini" {
		t.Errorf("Expected OutputRoot '.gemini', got %q", profile.OutputRoot)
	}

	if profile.InstructionOutput != "GEMINI.md" {
		t.Errorf("Expected InstructionOutput 'GEMINI.md', got %q", profile.InstructionOutput)
	}

	if profile.MCPOutput != ".agents/mcp_config.json" {
		t.Errorf("Expected MCPOutput '.agents/mcp_config.json', got %q", profile.MCPOutput)
	}

	if profile.SettingsFile != "settings.json" {
		t.Errorf("Expected SettingsFile 'settings.json', got %q", profile.SettingsFile)
	}

	// Verify capabilities
	if !profile.Capabilities.MCP {
		t.Error("MCP capability should be enabled")
	}

	if !profile.Capabilities.Skills {
		t.Error("Skills capability should be enabled")
	}

	if profile.Capabilities.Hooks {
		t.Error("Hooks capability should be disabled (support unclear)")
	}

	if profile.Capabilities.Permissions {
		t.Error("Permissions capability should be disabled (support unclear)")
	}
}

func TestProfile_UserScope(t *testing.T) {
	t.Parallel()

	profile := antigravity.UserProfile()

	if profile.InstructionOutput != ".gemini/GEMINI.md" {
		t.Errorf("Expected InstructionOutput '.gemini/GEMINI.md', got %q", profile.InstructionOutput)
	}

	if profile.MCPOutput != ".gemini/config/mcp_config.json" {
		t.Errorf("Expected MCPOutput '.gemini/config/mcp_config.json', got %q", profile.MCPOutput)
	}
}

func TestCompile_Antigravity_MCP(t *testing.T) {
	t.Parallel()

	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Create keel.yml with MCP
	keelYml := `schema: v1
name: test-mcp-layer

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644); err != nil {
		t.Fatalf("Failed to write keel.yml: %v", err)
	}

	// Write instructions
	if err := os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write instructions: %v", err)
	}

	// Write MCP config
	mcpJSON := `{
  "mcpServers": {
    "test-server": {
      "command": "node",
      "args": ["server.js"]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpJSON), 0644); err != nil {
		t.Fatalf("Failed to write mcp.json: %v", err)
	}

	// Create layer
	layer := config.Layer{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}

	// Compile
	_, err := antigravity.Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify MCP file exists at the correct location
	mcpPath := filepath.Join(outputDir, ".agents", "mcp_config.json")
	if _, err := os.Stat(mcpPath); err != nil {
		t.Errorf(".agents/mcp_config.json not created: %v", err)
	}

	// Verify MCP content
	content, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("Failed to read MCP file: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "test-server") {
		t.Error("MCP file should contain test-server")
	}
	if !contains(contentStr, "mcpServers") {
		t.Error("MCP file should contain mcpServers")
	}
}

func TestCompile_Antigravity_Categories(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644); err != nil {
		t.Fatalf("Failed to write keel.yml: %v", err)
	}

	// Write instructions
	if err := os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write instructions: %v", err)
	}

	// Write a command
	commandsDir := filepath.Join(layerDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "test-cmd.md"), []byte("# Test Command"), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}

	// Write a skill
	skillsDir := filepath.Join(layerDir, "skills", "test-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Create layer
	layer := config.Layer{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}

	// Compile
	_, err := antigravity.Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify command appears in .agents/commands/
	cmdPath := filepath.Join(outputDir, ".agents", "commands", "test-cmd.md")
	if _, err := os.Stat(cmdPath); err != nil {
		t.Errorf(".agents/commands/test-cmd.md not created: %v", err)
	}

	// Verify skill appears in .agents/skills/
	skillPath := filepath.Join(outputDir, ".agents", "skills", "test-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf(".agents/skills/test-skill/SKILL.md not created: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
