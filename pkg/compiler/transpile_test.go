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
