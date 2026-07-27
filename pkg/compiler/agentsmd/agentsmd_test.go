package agentsmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentsmd"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cursor"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// --- Reader tests ---

func TestReader_ReadsAGENTSmd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, "AGENTS.md", []byte("# Project\n\nBuild with `make`."))

	reader, err := compiler.GetReader("agentsmd")
	require.NoError(t, err)

	ir, err := reader.Read(dir)
	require.NoError(t, err)
	require.Len(t, ir.Instructions, 1)
	assert.Equal(t, "AGENTS.md", ir.Instructions[0].Filename)
	assert.Contains(t, ir.Instructions[0].Content, "Build with `make`")
	assert.Equal(t, "agentsmd", ir.SourceAgent)
}

func TestReader_NoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	reader, err := compiler.GetReader("agentsmd")
	require.NoError(t, err)

	ir, err := reader.Read(dir)
	require.NoError(t, err)
	assert.Empty(t, ir.Instructions, "empty dir should produce empty instructions")
}

func TestReader_CaseInsensitive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	testutil.WriteFile(t, dir, "agents.md", []byte("lowercase variant"))

	reader, err := compiler.GetReader("agentsmd")
	require.NoError(t, err)

	ir, err := reader.Read(dir)
	require.NoError(t, err)
	// agents.md is second priority; only found if AGENTS.md doesn't exist.
	require.Len(t, ir.Instructions, 1)
	assert.Contains(t, ir.Instructions[0].Content, "lowercase variant")
}

// --- Writer tests ---

func TestWriter_WritesAGENTSmd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a DSL layer with instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	testutil.WriteFile(t, layerDir, "keel.yml", []byte("schema: v1\ninstructions:\n  - source: instructions.md\n"))
	testutil.WriteFile(t, layerDir, "instructions.md", []byte("# Build Guide\n\nRun `npm install`."))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "keel"},
	}
	outputDir := t.TempDir()

	result, err := compiler.Compile("agentsmd", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "agentsmd", result.Output.Agent)

	// Verify AGENTS.md written.
	content, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	require.NoError(t, err, "AGENTS.md should exist")
	assert.Contains(t, string(content), "Build Guide")
	assert.Contains(t, string(content), "npm install")

	// Verify manifest written.
	data, err := os.ReadFile(filepath.Join(outputDir, ".agents", ".keel.json"))
	require.NoError(t, err, ".agents/.keel.json should exist")
	assert.Contains(t, string(data), `"agent": "agentsmd"`)

	// Verify file record.
	var found bool
	for _, f := range result.Output.Files {
		if f.Output == "AGENTS.md" {
			found = true
			assert.Equal(t, "user-editable", f.Strategy)
			assert.NotEmpty(t, f.Hash)
		}
	}
	assert.True(t, found, "expected AGENTS.md in file records")
}

func TestWriter_EmptyInstructions(t *testing.T) {
	t.Parallel()

	// Compile with no instruction content.
	var layers []config.Layer
	outputDir := t.TempDir()

	result, err := compiler.Compile("agentsmd", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, "agentsmd", result.Output.Agent)

	// AGENTS.md should NOT exist (no instructions to write).
	_, err = os.Stat(filepath.Join(outputDir, "AGENTS.md"))
	assert.True(t, os.IsNotExist(err), "AGENTS.md should not be written when there are no instructions")
}

// --- Cross-agent transpilation ---

func TestTranspile_ClaudeToAGENTSmd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a Claude native layer with instructions and settings.
	layerDir := filepath.Join(dir, "claude-layer")
	testutil.WriteFile(t, layerDir, "CLAUDE.md", []byte("# Claude Instructions\n\nFollow these rules."))

	// Claude native layer (has CLAUDE.md marker).
	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	result, err := compiler.Compile("agentsmd", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Instructions should transfer.
	content, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Claude Instructions")
	assert.Contains(t, string(content), "Follow these rules")
}

func TestTranspile_AGENTSmdToClaude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create an AGENTS.md native layer.
	layerDir := filepath.Join(dir, "agentsmd-layer")
	testutil.WriteFile(t, layerDir, "AGENTS.md", []byte("# Universal Rules\n\nAlways test before committing."))

	layers := []config.Layer{{Path: layerDir, Format: "agentsmd"}}
	outputDir := t.TempDir()

	result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Instructions should transfer to CLAUDE.md.
	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Universal Rules")
	assert.Contains(t, string(content), "Always test before committing")
}

func TestTranspile_AGENTSmdToCursor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "agentsmd-layer")
	testutil.WriteFile(t, layerDir, "AGENTS.md", []byte("# Cross-Agent Config\n\nUse strict mode."))

	layers := []config.Layer{{Path: layerDir, Format: "agentsmd"}}
	outputDir := t.TempDir()

	result, err := compiler.Compile("cursor", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Instructions should transfer to Cursor's rule file.
	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Cross-Agent Config")
	assert.Contains(t, string(content), "Use strict mode")
}

// --- Capability warnings ---

func TestCapabilityWarnings_UnsupportedFeatures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a DSL layer with hooks and MCP — not supported by AGENTS.md.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	testutil.WriteFile(t, layerDir, "keel.yml", []byte(`schema: v1
instructions:
  - source: instructions.md
hooks:
  - source: hooks.json
mcp:
  servers:
    - source: mcp.json
`))
	testutil.WriteFile(t, layerDir, "instructions.md", []byte("# Test"))
	testutil.WriteFile(t, layerDir, "hooks.json", []byte(`{"PreToolUse": [{"type": "command", "command": "echo hi"}]}`))
	testutil.WriteFile(t, layerDir, "mcp.json", []byte(`{"mcpServers": {"test": {"command": "test"}}}`))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "keel"},
	}
	outputDir := t.TempDir()

	// Should compile without error — unsupported features are warned, not errored.
	result, err := compiler.Compile("agentsmd", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Instructions should still be written.
	content, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "# Test")
}
