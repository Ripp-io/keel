package goose

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gopkg.in/yaml.v3"
)

// TestCompileMinimal verifies that the Goose compiler produces expected output
// files for a minimal layer with instructions only.
func TestCompileMinimal(t *testing.T) {
	// Create a temporary layer directory
	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Write minimal keel.yml
	keelYML := `schema: v1
name: test-layer
description: Test layer for Goose

instructions:
  - source: instructions.md
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write instructions
	instructions := "# Test Instructions\n\nThis is a test."
	if err := os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instructions), 0644); err != nil {
		t.Fatal(err)
	}

	// Create layer
	layer := config.Layer{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}

	// Compile
	result, err := Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Verify .goosehints exists
	goosehints := filepath.Join(outputDir, ".goosehints")
	if _, err := os.Stat(goosehints); err != nil {
		t.Errorf(".goosehints not created: %v", err)
	}

	// Note: AGENTS.md is only created during multi-agent compilation via CompileMulti.
	// Single-agent Compile() does not write shared instruction files.

	// Verify manifest
	if len(result.Output.Files) == 0 {
		t.Error("manifest has no file records")
	}

	// Verify .goosehints is in the manifest
	foundGoosehints := false
	for _, f := range result.Output.Files {
		if f.Output == ".goosehints" {
			foundGoosehints = true
			break
		}
	}
	if !foundGoosehints {
		t.Error(".goosehints not in manifest")
	}
}

// TestCompileWithMCP verifies that MCP servers are translated to Goose's
// extensions format.
func TestCompileWithMCP(t *testing.T) {
	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Write keel.yml with MCP
	keelYML := `schema: v1
name: test-mcp-layer

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write instructions
	if err := os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write MCP config
	mcpJSON := `{
  "mcpServers": {
    "test-server": {
      "command": "node",
      "args": ["server.js"],
      "env": {
        "API_KEY": "test-key"
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create layer
	layer := config.Layer{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}

	// Compile
	_, err := Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Verify config.yaml exists
	configPath := filepath.Join(outputDir, ".goose", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not created: %v", err)
	}

	// Parse config.yaml
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config.yaml: %v", err)
	}

	// Verify extensions array exists
	extensions, ok := config["extensions"].([]any)
	if !ok {
		t.Fatal("extensions not found in config.yaml")
	}

	if len(extensions) == 0 {
		t.Fatal("extensions array is empty")
	}

	// Verify the MCP server was translated to an extension
	ext := extensions[0].(map[string]any)
	if ext["name"] != "test-server" {
		t.Errorf("expected extension name 'test-server', got %v", ext["name"])
	}
	if ext["type"] != "local" {
		t.Errorf("expected type 'local', got %v", ext["type"])
	}

	// Verify command array
	command, ok := ext["command"].([]any)
	if !ok {
		t.Fatal("command not found or not an array")
	}
	if len(command) != 2 {
		t.Errorf("expected 2 command parts, got %d", len(command))
	}
	if command[0] != "node" {
		t.Errorf("expected command[0] = 'node', got %v", command[0])
	}
	if command[1] != "server.js" {
		t.Errorf("expected command[1] = 'server.js', got %v", command[1])
	}

	// Verify environment
	env, ok := ext["environment"].(map[string]any)
	if !ok {
		t.Fatal("environment not found or not a map")
	}
	if env["API_KEY"] != "test-key" {
		t.Errorf("expected API_KEY = 'test-key', got %v", env["API_KEY"])
	}
}

// TestTranslateMCPToGoose verifies the MCP translation function.
func TestTranslateMCPToGoose(t *testing.T) {
	input := map[string]any{
		"mcpServers": map[string]any{
			"test-local": map[string]any{
				"command": "npx",
				"args":    []any{"-y", "server"},
				"env": map[string]any{
					"KEY": "value",
				},
			},
			"test-remote": map[string]any{
				"url": "https://example.com/mcp",
			},
		},
	}

	result := TranslateMCPToGoose(input)
	if result == nil {
		t.Fatal("TranslateMCPToGoose returned nil")
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(result))
	}

	// Find the local server
	var local, remote map[string]any
	for _, extRaw := range result {
		ext := extRaw.(map[string]any)
		if ext["name"] == "test-local" {
			local = ext
		} else if ext["name"] == "test-remote" {
			remote = ext
		}
	}

	// Verify local server
	if local == nil {
		t.Fatal("test-local extension not found")
	}
	if local["type"] != "local" {
		t.Errorf("expected type 'local', got %v", local["type"])
	}
	command := local["command"].([]any)
	if len(command) != 3 {
		t.Errorf("expected 3 command parts, got %d", len(command))
	}
	if command[0] != "npx" || command[1] != "-y" || command[2] != "server" {
		t.Errorf("command array incorrect: %v", command)
	}

	// Verify remote server
	if remote == nil {
		t.Fatal("test-remote extension not found")
	}
	if remote["type"] != "remote" {
		t.Errorf("expected type 'remote', got %v", remote["type"])
	}
	if remote["url"] != "https://example.com/mcp" {
		t.Errorf("expected url 'https://example.com/mcp', got %v", remote["url"])
	}
}

// TestTranslateMCPFromGoose verifies the reverse MCP translation.
func TestTranslateMCPFromGoose(t *testing.T) {
	input := []any{
		map[string]any{
			"name":    "test-server",
			"type":    "local",
			"command": []any{"node", "server.js"},
			"environment": map[string]any{
				"API_KEY": "secret",
			},
			"enabled": true,
		},
	}

	result := TranslateMCPFromGoose(input)
	if result == nil {
		t.Fatal("TranslateMCPFromGoose returned nil")
	}

	servers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found in result")
	}

	server, ok := servers["test-server"].(map[string]any)
	if !ok {
		t.Fatal("test-server not found in mcpServers")
	}

	if server["command"] != "node" {
		t.Errorf("expected command 'node', got %v", server["command"])
	}

	args, ok := server["args"].([]any)
	if !ok {
		t.Fatal("args not found or not an array")
	}
	if len(args) != 1 || args[0] != "server.js" {
		t.Errorf("expected args ['server.js'], got %v", args)
	}

	env, ok := server["env"].(map[string]any)
	if !ok {
		t.Fatal("env not found or not a map")
	}
	if env["API_KEY"] != "secret" {
		t.Errorf("expected API_KEY 'secret', got %v", env["API_KEY"])
	}
}

// TestCompile_Goose_UserScope verifies that user scope compilation produces
// output in the correct location (.config/goose/ instead of .goose/).
func TestCompile_Goose_UserScope(t *testing.T) {
	t.Parallel()

	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Write keel.yml
	keelYML := `schema: v1
name: test-user-layer

instructions:
  - source: instructions.md
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write instructions
	if err := os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# User Instructions"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create layer
	layer := config.Layer{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}

	// Compile with user scope
	_, err := Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeUser,
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Verify output goes to .config/goose/
	userInstructions := filepath.Join(outputDir, ".config", "goose", "AGENTS.md")
	if _, err := os.Stat(userInstructions); err != nil {
		t.Errorf(".config/goose/AGENTS.md not created: %v", err)
	}

	// Verify .goosehints does NOT exist at root (user scope uses different path)
	goosehints := filepath.Join(outputDir, ".goosehints")
	if _, err := os.Stat(goosehints); err == nil {
		t.Error(".goosehints should not exist at root for user scope")
	}
}

// TestCompile_Goose_Categories verifies that commands and skills are written
// to .goose/commands/ and .goose/skills/.
func TestCompile_Goose_Categories(t *testing.T) {
	t.Parallel()

	layerDir := t.TempDir()
	outputDir := t.TempDir()

	// Write keel.yml
	keelYML := `schema: v1
name: test-categories

instructions:
  - source: instructions.md

commands:
  - source: commands/

skills:
  - source: skills/
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644); err != nil {
		t.Fatal(err)
	}

	// Write instructions
	if err := os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a command
	commandsDir := filepath.Join(layerDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "test-cmd.md"), []byte("# Test Command"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a skill
	skillsDir := filepath.Join(layerDir, "skills", "test-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create layer
	layer := config.Layer{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}

	// Compile
	_, err := Compile([]config.Layer{layer}, outputDir, compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Verify command appears in .goose/commands/
	cmdPath := filepath.Join(outputDir, ".goose", "commands", "test-cmd.md")
	if _, err := os.Stat(cmdPath); err != nil {
		t.Errorf(".goose/commands/test-cmd.md not created: %v", err)
	}

	// Verify skill appears in .goose/skills/
	skillPath := filepath.Join(outputDir, ".goose", "skills", "test-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf(".goose/skills/test-skill/SKILL.md not created: %v", err)
	}
}
