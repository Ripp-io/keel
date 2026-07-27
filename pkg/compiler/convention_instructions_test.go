package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// TestConventionLayer_InstructionsDirectory verifies that a convention layer
// (FormatKeel without keel.yml) discovers instructions from an instructions/
// directory. This is the regression test for the .keel/local instructions/ bug.
func TestConventionLayer_InstructionsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "local")
	instrDir := filepath.Join(layerDir, "instructions")
	require.NoError(t, os.MkdirAll(instrDir, 0755))
	testutil.WriteFile(t, instrDir, "backlog-management.md", []byte("# Backlog Management\n\nTrack work items."))
	testutil.WriteFile(t, instrDir, "research-upkeep.md", []byte("# Research Upkeep\n\nKeep docs current."))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "project"},
	}
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "CLAUDE.md should be written from instructions/ directory")

	claudeMD := string(content)
	assert.Contains(t, claudeMD, "Backlog Management")
	assert.Contains(t, claudeMD, "Research Upkeep")
}

// TestConventionLayer_InstructionsFileTakesPriority verifies that when a
// convention layer has both instructions.md (file) and instructions/ (directory),
// both are included (matching DSL defaults behavior).
func TestConventionLayer_InstructionsBothIncluded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "local")
	instrDir := filepath.Join(layerDir, "instructions")
	require.NoError(t, os.MkdirAll(instrDir, 0755))

	testutil.WriteFile(t, layerDir, "instructions.md", []byte("# From File\n\nSingle file instructions."))
	testutil.WriteFile(t, instrDir, "extra.md", []byte("# From Directory\n\nDirectory instructions."))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "project"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)

	claudeMD := string(content)
	// DSL defaults include both instructions/ and instructions.md —
	// when both exist, both are compiled (same as an explicit keel.yml).
	assert.Contains(t, claudeMD, "From File")
	assert.Contains(t, claudeMD, "From Directory")
}

// TestConventionLayer_FullPattern verifies the full convention layer pattern:
// instructions/ + settings.json + skills/ all discovered without a keel.yml.
func TestConventionLayer_FullPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "local")
	instrDir := filepath.Join(layerDir, "instructions")
	skillDir := filepath.Join(layerDir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(instrDir, 0755))
	require.NoError(t, os.MkdirAll(skillDir, 0755))

	testutil.WriteFile(t, instrDir, "guide.md", []byte("# Guide\n\nConvention layer guide."))
	testutil.WriteFile(t, layerDir, "settings.json", []byte(`{"model":"claude-sonnet-4-20250514"}`))
	testutil.WriteFile(t, skillDir, "SKILL.md", []byte("---\nname: my-skill\n---\nDo the thing."))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "project"},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Instructions from directory.
	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Convention layer guide")

	// Settings picked up via fallback.
	settings, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(settings), "claude-sonnet-4-20250514")

	// Skills picked up.
	skill, err := os.ReadFile(filepath.Join(outputDir, ".claude", "skills", "my-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skill), "Do the thing")
}
