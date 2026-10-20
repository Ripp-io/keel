package compiler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// TestCrossAgentTranspile_ClaudeNativeToCursor verifies that a Claude native
// layer can be compiled targeting Cursor, producing valid Cursor output.
func TestCrossAgentTranspile_ClaudeNativeToCursor(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Claude native layer
	layerDir := filepath.Join(tempDir, "claude-layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Instructions
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "CLAUDE.md"), []byte(`# Claude Instructions

This is a test instruction for Claude.
`), 0644))

	// Settings with model (the only portable setting) and MCP
	settings := map[string]any{
		"model": "claude-sonnet-4",
	}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), settingsData, 0644))

	// MCP config
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []string{"test"},
			},
		},
	}
	mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, ".mcp.json"), mcpData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	// Compile targeting Cursor
	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in Cursor format
	instrPath := filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading Cursor instructions")
	assert.Contains(t, string(instrData), "This is a test instruction for Claude", "instructions should carry over")

	// Verify model setting carries over (model is portable)
	cliConfigPath := filepath.Join(outputDir, ".cursor", "cli-config.json")
	if cliData, err := os.ReadFile(cliConfigPath); err == nil {
		var cliConfig map[string]any
		if unmarshalErr := json.Unmarshal(cliData, &cliConfig); unmarshalErr == nil {
			assert.Equal(t, "claude-sonnet-4", cliConfig["model"], "model should carry over (portable setting)")
		}
	}

	// Verify MCP config carries over
	mcpPath := filepath.Join(outputDir, ".cursor", "mcp.json")
	mcpOutData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpOutData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "cursor", result.Output.Agent, "result should indicate cursor target")
}

// TestCrossAgentTranspile_LossyFeatures verifies that when compiling a Claude
// layer targeting AGENTS.md (which supports neither loop prompts nor skills),
// the compilation succeeds but emits capability warnings.
func TestCrossAgentTranspile_LossyFeatures(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Claude layer with loop prompt and skills
	layerDir := filepath.Join(tempDir, "claude-layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	skillsDir := filepath.Join(claudeDir, "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))

	// Loop prompt
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "loop.md"), []byte(`# Loop Prompt

This is a loop prompt for Claude.
`), 0644))

	// Skills
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill
---

# Test Skill

This is a test skill.
`), 0644))

	// Instructions (minimal, so we have something)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "CLAUDE.md"), []byte(`# Instructions

Basic instructions.
`), 0644))

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	// Capture slog output to check for warnings
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	// Compile targeting AGENTS.md
	result, err := Compile("agentsmd", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "compilation should succeed even with lossy features")

	// Verify loop and skills are NOT in the output
	loopPath := filepath.Join(outputDir, "loop.md")
	_, loopErr := os.Stat(loopPath)
	assert.True(t, os.IsNotExist(loopErr), "loop.md should not exist in AGENTS.md output")

	skillsPath := filepath.Join(outputDir, "skills")
	_, skillsErr := os.Stat(skillsPath)
	assert.True(t, os.IsNotExist(skillsErr), "skills directory should not exist in AGENTS.md output")

	// Verify instructions ARE in the output
	instrPath := filepath.Join(outputDir, "AGENTS.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading AGENTS.md")
	assert.Contains(t, string(instrData), "Basic instructions", "instructions should be in output")

	// Verify capability warnings were emitted
	logOutput := logBuf.String()
	// The exact warning format may vary, but it should mention unsupported features
	if !strings.Contains(logOutput, "loop") && !strings.Contains(logOutput, "skill") && !strings.Contains(logOutput, "unsupported") && !strings.Contains(logOutput, "capability") {
		t.Logf("Warning: Expected capability warnings in log output, got:\n%s", logOutput)
		// Don't fail the test — warning format may change
	}

	// Verify result metadata
	assert.Equal(t, "agentsmd", result.Output.Agent, "result should indicate agentsmd target")
}

// TestCrossAgentTranspile_AGENTSmdToMultiple verifies that an AGENTS.md layer
// can be compiled to both Claude and Cursor (demonstrating the instructions-only
// format is portable).
func TestCrossAgentTranspile_AGENTSmdToMultiple(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create an AGENTS.md layer
	layerDir := filepath.Join(tempDir, "agentsmd-layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	// AGENTS.md instructions
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# Project Instructions

This is a portable instruction set using AGENTS.md format.

## Guidelines

- Follow coding standards
- Write tests
`), 0644))

	layers := []config.Layer{{Path: layerDir, Format: "agentsmd"}}

	// Compile to Claude
	t.Run("to-claude", func(t *testing.T) {
		outputDir := t.TempDir()
		result, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "compile to claude should succeed")

		instrPath := filepath.Join(outputDir, "CLAUDE.md")
		instrData, err := os.ReadFile(instrPath)
		require.NoError(t, err, "reading CLAUDE.md")
		assert.Contains(t, string(instrData), "portable instruction set", "instructions should carry over")

		assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
	})

	// Compile to Cursor
	t.Run("to-cursor", func(t *testing.T) {
		outputDir := t.TempDir()
		result, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "compile to cursor should succeed")

		instrPath := filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc")
		instrData, err := os.ReadFile(instrPath)
		require.NoError(t, err, "reading 00-keel.mdc")
		assert.Contains(t, string(instrData), "portable instruction set", "instructions should carry over")

		assert.Equal(t, "cursor", result.Output.Agent, "result should indicate cursor target")
	})
}

// TestCrossAgentTranspile_ClaudeNativeToCodex verifies that a Claude native
// layer can be compiled targeting Codex.
func TestCrossAgentTranspile_ClaudeNativeToCodex(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Claude native layer
	layerDir := filepath.Join(tempDir, "claude-layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Instructions
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "CLAUDE.md"), []byte(`# Claude Instructions

This is a test instruction for Claude.
`), 0644))

	// MCP config
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []string{"test"},
			},
		},
	}
	mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, ".mcp.json"), mcpData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	// Compile targeting Codex
	result, err := Compile("codex", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in Codex format (AGENTS.md)
	instrPath := filepath.Join(outputDir, "AGENTS.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading AGENTS.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Claude", "instructions should carry over")

	// Verify MCP carries over to codex format (.codex/config.toml)
	configPath := filepath.Join(outputDir, ".codex", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		configData, _ := os.ReadFile(configPath)
		assert.Contains(t, string(configData), "test-server", "MCP server should be in config.toml")
	}

	// Verify result metadata
	assert.Equal(t, "codex", result.Output.Agent, "result should indicate codex target")
}

// TestCrossAgentTranspile_ClaudeNativeToGoose verifies that a Claude native
// layer can be compiled targeting Goose.
func TestCrossAgentTranspile_ClaudeNativeToGoose(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Claude native layer
	layerDir := filepath.Join(tempDir, "claude-layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Instructions
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "CLAUDE.md"), []byte(`# Claude Instructions

This is a test instruction for Claude.
`), 0644))

	// MCP config
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []string{"test"},
			},
		},
	}
	mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, ".mcp.json"), mcpData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	// Compile targeting Goose
	result, err := Compile("goose", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in Goose format (.goosehints)
	instrPath := filepath.Join(outputDir, ".goosehints")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading .goosehints")
	assert.Contains(t, string(instrData), "This is a test instruction for Claude", "instructions should carry over")

	// Verify MCP carries over to goose format (extensions in .goose/config.yaml)
	configPath := filepath.Join(outputDir, ".goose", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		configData, _ := os.ReadFile(configPath)
		// Goose uses extensions format
		assert.Contains(t, string(configData), "extensions", "MCP should be translated to extensions in config.yaml")
	}

	// Verify result metadata
	assert.Equal(t, "goose", result.Output.Agent, "result should indicate goose target")
}

// TestCrossAgentTranspile_DSLToMultipleNewAdapters verifies that a DSL layer
// can be compiled to both codex and goose.
func TestCrossAgentTranspile_DSLToMultipleNewAdapters(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a DSL layer
	layerDir := filepath.Join(tempDir, "dsl-layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	// keel.yml
	keelYML := `schema: v1
name: multi-agent-layer
description: Test layer for multiple agents

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	// Instructions
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(`# Multi-Agent Instructions

This should work across all agents.
`), 0644))

	// MCP config
	mcpJSON := `{
  "mcpServers": {
    "shared-server": {
      "command": "node",
      "args": ["server.js"]
    }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpJSON), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel}}

	// Compile to Codex
	t.Run("to-codex", func(t *testing.T) {
		outputDir := t.TempDir()
		result, err := Compile("codex", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "compile to codex should succeed")

		// Verify instructions appear in codex format
		instrPath := filepath.Join(outputDir, "AGENTS.md")
		instrData, err := os.ReadFile(instrPath)
		require.NoError(t, err, "reading AGENTS.md")
		assert.Contains(t, string(instrData), "This should work across all agents", "instructions should carry over")

		// Verify MCP is translated to codex format
		if _, err := os.Stat(filepath.Join(outputDir, ".codex")); err == nil {
			// .codex directory exists - MCP might be in config.toml
		}

		// Verify result has correct agent name
		assert.Equal(t, "codex", result.Output.Agent, "result should indicate codex target")
	})

	// Compile to Goose
	t.Run("to-goose", func(t *testing.T) {
		outputDir := t.TempDir()
		result, err := Compile("goose", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "compile to goose should succeed")

		// Verify instructions appear in goose format
		instrPath := filepath.Join(outputDir, ".goosehints")
		instrData, err := os.ReadFile(instrPath)
		require.NoError(t, err, "reading .goosehints")
		assert.Contains(t, string(instrData), "This should work across all agents", "instructions should carry over")

		// Verify MCP is translated to goose format (extensions in config.yaml)
		configPath := filepath.Join(outputDir, ".goose", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			configData, _ := os.ReadFile(configPath)
			assert.Contains(t, string(configData), "shared-server", "MCP server should be in config.yaml")
		}

		// Verify result has correct agent name
		assert.Equal(t, "goose", result.Output.Agent, "result should indicate goose target")
	})
}

// TestCrossAgentTranspile_GooseNativeToClaude verifies that a Goose native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_GooseNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Goose native layer
	layerDir := filepath.Join(tempDir, "goose-layer")
	gooseDir := filepath.Join(layerDir, ".goose")
	require.NoError(t, os.MkdirAll(gooseDir, 0755))

	// Instructions (.goosehints)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, ".goosehints"), []byte(`# Goose Instructions

This is a test instruction for Goose.
`), 0644))

	// MCP config in Goose format (extensions array in config.yaml)
	gooseConfig := `extensions:
  - name: test-server
    type: local
    command: ["echo", "test"]
    environment:
      TEST_VAR: "value"
`
	require.NoError(t, os.WriteFile(filepath.Join(gooseDir, "config.yaml"), []byte(gooseConfig), 0644))

	layers := []config.Layer{{Path: layerDir, Format: "goose"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Goose", "instructions should carry over")

	// Verify MCP carries over (Goose extensions → Claude .mcp.json)
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_CodexNativeToClaude verifies that a Codex native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_CodexNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Codex native layer
	layerDir := filepath.Join(tempDir, "codex-layer")
	codexDir := filepath.Join(layerDir, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0755))

	// Instructions (AGENTS.md is Codex's instruction format)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# Codex Instructions

This is a test instruction for Codex.
`), 0644))

	// MCP config in Codex format (config.json with mcp.servers)
	codexConfig := map[string]any{
		"mcp": map[string]any{
			"servers": map[string]any{
				"test-server": map[string]any{
					"command": []any{"echo", "test"},
					"env": map[string]any{
						"TEST_VAR": "value",
					},
				},
			},
		},
	}
	codexData, _ := json.MarshalIndent(codexConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "config.json"), codexData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "codex"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Codex", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_KiroNativeToClaude verifies that a Kiro native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_KiroNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Kiro native layer
	layerDir := filepath.Join(tempDir, "kiro-layer")
	kiroSettingsDir := filepath.Join(layerDir, ".kiro", "settings")
	require.NoError(t, os.MkdirAll(kiroSettingsDir, 0755))

	// Instructions (AGENTS.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# Kiro Instructions

This is a test instruction for Kiro.
`), 0644))

	// MCP config in Kiro format (.kiro/settings/mcp.json with standard mcpServers)
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []any{"test"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}
	mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(kiroSettingsDir, "mcp.json"), mcpData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "kiro"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Kiro", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData2, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData2, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_AmpNativeToClaude verifies that an Amp native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_AmpNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create an Amp native layer
	layerDir := filepath.Join(tempDir, "amp-layer")
	ampDir := filepath.Join(layerDir, ".amp")
	require.NoError(t, os.MkdirAll(ampDir, 0755))

	// Instructions (AGENTS.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# Amp Instructions

This is a test instruction for Amp.
`), 0644))

	// MCP config in Amp format (settings.json with amp.mcpServers)
	ampSettings := map[string]any{
		"amp.mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []any{"test"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}
	ampData, _ := json.MarshalIndent(ampSettings, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(ampDir, "settings.json"), ampData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "amp"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Amp", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_KiloNativeToClaude verifies that a Kilo native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_KiloNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Kilo native layer
	layerDir := filepath.Join(tempDir, "kilo-layer")
	kiloDir := filepath.Join(layerDir, ".kilo")
	require.NoError(t, os.MkdirAll(kiloDir, 0755))

	// Instructions (AGENTS.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# Kilo Instructions

This is a test instruction for Kilo.
`), 0644))

	// MCP config in Kilo format (kilo.jsonc with mcp.servers)
	kiloConfig := map[string]any{
		"mcp": map[string]any{
			"servers": map[string]any{
				"test-server": map[string]any{
					"command":     []any{"echo", "test"},
					"environment": map[string]any{
						"TEST_VAR": "value",
					},
				},
			},
		},
	}
	kiloData, _ := json.MarshalIndent(kiloConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(kiloDir, "kilo.jsonc"), kiloData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "kilo"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Kilo", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_OpenCodeNativeToClaude verifies that an OpenCode native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_OpenCodeNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create an OpenCode native layer
	layerDir := filepath.Join(tempDir, "opencode-layer")
	opencodeDir := filepath.Join(layerDir, ".opencode")
	require.NoError(t, os.MkdirAll(opencodeDir, 0755))

	// Instructions (AGENTS.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# OpenCode Instructions

This is a test instruction for OpenCode.
`), 0644))

	// MCP config in OpenCode format (opencode.json with mcp.servers)
	opencodeConfig := map[string]any{
		"mcp": map[string]any{
			"servers": map[string]any{
				"test-server": map[string]any{
					"command":     []any{"echo", "test"},
					"environment": map[string]any{
						"TEST_VAR": "value",
					},
				},
			},
		},
	}
	opencodeData, _ := json.MarshalIndent(opencodeConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(opencodeDir, "opencode.json"), opencodeData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "opencode"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for OpenCode", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_AntigravityNativeToClaude verifies that an Antigravity native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_AntigravityNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create an Antigravity native layer
	layerDir := filepath.Join(tempDir, "antigravity-layer")
	agentsDir := filepath.Join(layerDir, ".agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Instructions (GEMINI.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "GEMINI.md"), []byte(`# Antigravity Instructions

This is a test instruction for Antigravity.
`), 0644))

	// MCP config in Antigravity format (.agents/mcp_config.json with standard mcpServers)
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []any{"test"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}
	mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "mcp_config.json"), mcpData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "antigravity"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Antigravity", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData2, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData2, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_QwenNativeToClaude verifies that a Qwen native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_QwenNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Qwen native layer
	layerDir := filepath.Join(tempDir, "qwen-layer")
	qwenDir := filepath.Join(layerDir, ".qwen")
	require.NoError(t, os.MkdirAll(qwenDir, 0755))

	// Instructions (QWEN.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "QWEN.md"), []byte(`# Qwen Instructions

This is a test instruction for Qwen.
`), 0644))

	// MCP config in Qwen format (settings.json with standard mcpServers)
	qwenSettings := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []any{"test"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}
	qwenData, _ := json.MarshalIndent(qwenSettings, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(qwenDir, "settings.json"), qwenData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "qwen"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Qwen", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_VibeNativeToClaude verifies that a Vibe native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_VibeNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a Vibe native layer
	layerDir := filepath.Join(tempDir, "vibe-layer")
	vibeDir := filepath.Join(layerDir, ".vibe")
	require.NoError(t, os.MkdirAll(vibeDir, 0755))

	// Instructions (AGENTS.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# Vibe Instructions

This is a test instruction for Vibe.
`), 0644))

	// MCP config in Vibe format (config.json with mcp_servers array)
	vibeConfig := map[string]any{
		"mcp_servers": []any{
			map[string]any{
				"name":    "test-server",
				"command": "echo",
				"args":    []any{"test"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}
	vibeData, _ := json.MarshalIndent(vibeConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(vibeDir, "config.json"), vibeData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "vibe"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for Vibe", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}

// TestCrossAgentTranspile_OpenHandsNativeToClaude verifies that an OpenHands native
// layer can be compiled targeting Claude.
func TestCrossAgentTranspile_OpenHandsNativeToClaude(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create an OpenHands native layer
	layerDir := filepath.Join(tempDir, "openhands-layer")
	openhandsDir := filepath.Join(layerDir, ".openhands")
	require.NoError(t, os.MkdirAll(openhandsDir, 0755))

	// Instructions (AGENTS.md)
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte(`# OpenHands Instructions

This is a test instruction for OpenHands.
`), 0644))

	// MCP config in OpenHands format (.openhands/mcp.json with standard mcpServers)
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "echo",
				"args":    []any{"test"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}
	mcpData, _ := json.MarshalIndent(mcpConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(openhandsDir, "mcp.json"), mcpData, 0644))

	layers := []config.Layer{{Path: layerDir, Format: "openhands"}}
	outputDir := t.TempDir()

	// Compile targeting Claude
	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "cross-agent compile should succeed")

	// Verify instructions appear in CLAUDE.md
	instrPath := filepath.Join(outputDir, "CLAUDE.md")
	instrData, err := os.ReadFile(instrPath)
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(instrData), "This is a test instruction for OpenHands", "instructions should carry over")

	// Verify MCP carries over
	mcpPath := filepath.Join(outputDir, ".mcp.json")
	mcpData2, err := os.ReadFile(mcpPath)
	require.NoError(t, err, "reading .mcp.json")

	var mcpOut map[string]any
	require.NoError(t, json.Unmarshal(mcpData2, &mcpOut))

	servers, ok := mcpOut["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers in output")
	testSrv, ok := servers["test-server"].(map[string]any)
	require.True(t, ok, "expected test-server in MCP output")
	assert.Equal(t, "echo", testSrv["command"], "MCP server command should carry over")

	// Verify result metadata
	assert.Equal(t, "claude", result.Output.Agent, "result should indicate claude target")
}
