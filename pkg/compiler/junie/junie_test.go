package junie

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestProjectProfile(t *testing.T) {
	t.Parallel()

	profile := ProjectProfile()

	assert.Equal(t, "junie", profile.Name)
	assert.Equal(t, ".junie", profile.OutputRoot)
	assert.Equal(t, ".junie/guidelines.md", profile.InstructionOutput)
	assert.Equal(t, "AGENTS.md", profile.SharedInstructionOutput)
	assert.Equal(t, "", profile.SettingsFile)
	assert.Equal(t, "", profile.MCPOutput)
	assert.False(t, profile.HooksInSettings)
	assert.Nil(t, profile.Categories)
	assert.Equal(t, []string{".junie/guidelines.md", "AGENTS.md"}, profile.InstructionCandidates)
}

func TestCompile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-junie-compile
description: Minimal test layer for Junie
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	instContent := "# Junie Instructions\n\nUse JetBrains IDE features effectively."
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "junie", result.Output.Agent)

	// Verify .junie/guidelines.md was written.
	guidelinesPath := filepath.Join(dir, ".junie", "guidelines.md")
	content, err := os.ReadFile(guidelinesPath)
	require.NoError(t, err, ".junie/guidelines.md should exist")
	assert.Contains(t, string(content), "Junie Instructions")
	assert.Contains(t, string(content), "Use JetBrains IDE features effectively")
}

func TestCompile_EmptyLayers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a layer with no instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-junie-empty
description: Layer with no instructions
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "junie", result.Output.Agent)

	// .junie/guidelines.md should NOT exist when there are no instructions.
	guidelinesPath := filepath.Join(dir, ".junie", "guidelines.md")
	_, err = os.Stat(guidelinesPath)
	assert.True(t, os.IsNotExist(err), ".junie/guidelines.md should not exist without instructions")
}

func TestCompile_MultipleInstructions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create two layers with instructions.
	layer1 := filepath.Join(dir, "layer1")
	require.NoError(t, os.MkdirAll(layer1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "keel.yml"), []byte(`schema: v1
name: layer1
instructions:
  - source: inst1.md
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "inst1.md"), []byte("# Layer 1\n\nFirst guideline."), 0644))

	layer2 := filepath.Join(dir, "layer2")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "keel.yml"), []byte(`schema: v1
name: layer2
instructions:
  - source: inst2.md
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "inst2.md"), []byte("# Layer 2\n\nSecond guideline."), 0644))

	layers := []config.Layer{
		{Path: layer1, Format: config.FormatKeel, Type: "test"},
		{Path: layer2, Format: config.FormatKeel, Type: "test"},
	}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "junie", result.Output.Agent)

	// Verify both instructions are concatenated.
	guidelinesPath := filepath.Join(dir, ".junie", "guidelines.md")
	content, err := os.ReadFile(guidelinesPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Layer 1")
	assert.Contains(t, string(content), "First guideline")
	assert.Contains(t, string(content), "Layer 2")
	assert.Contains(t, string(content), "Second guideline")
}
