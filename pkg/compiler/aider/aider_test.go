package aider

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

	assert.Equal(t, "aider", profile.Name)
	assert.Equal(t, ".aider", profile.OutputRoot)
	assert.Equal(t, "CONVENTIONS.md", profile.InstructionOutput)
	assert.Equal(t, "", profile.SettingsFile)
	assert.Equal(t, "", profile.MCPOutput)
	assert.False(t, profile.HooksInSettings)
	assert.Nil(t, profile.Categories)
	assert.Equal(t, []string{"CONVENTIONS.md", "conventions.md"}, profile.InstructionCandidates)
}

func TestCompile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-aider-compile
description: Minimal test layer for Aider
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	instContent := "# Aider Instructions\n\nAlways write tests."
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "aider", result.Output.Agent)

	// Verify CONVENTIONS.md was written at the output root.
	conventionsPath := filepath.Join(dir, "CONVENTIONS.md")
	content, err := os.ReadFile(conventionsPath)
	require.NoError(t, err, "CONVENTIONS.md should exist")
	assert.Contains(t, string(content), "Aider Instructions")
	assert.Contains(t, string(content), "Always write tests")
}

func TestCompile_EmptyLayers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a layer with no instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-aider-empty
description: Layer with no instructions
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "aider", result.Output.Agent)

	// CONVENTIONS.md should NOT exist when there are no instructions.
	conventionsPath := filepath.Join(dir, "CONVENTIONS.md")
	_, err = os.Stat(conventionsPath)
	assert.True(t, os.IsNotExist(err), "CONVENTIONS.md should not exist without instructions")
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
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "inst1.md"), []byte("# Layer 1\n\nFirst layer."), 0644))

	layer2 := filepath.Join(dir, "layer2")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "keel.yml"), []byte(`schema: v1
name: layer2
instructions:
  - source: inst2.md
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "inst2.md"), []byte("# Layer 2\n\nSecond layer."), 0644))

	layers := []config.Layer{
		{Path: layer1, Format: config.FormatKeel, Type: "test"},
		{Path: layer2, Format: config.FormatKeel, Type: "test"},
	}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "aider", result.Output.Agent)

	// Verify both instructions are concatenated.
	conventionsPath := filepath.Join(dir, "CONVENTIONS.md")
	content, err := os.ReadFile(conventionsPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Layer 1")
	assert.Contains(t, string(content), "First layer")
	assert.Contains(t, string(content), "Layer 2")
	assert.Contains(t, string(content), "Second layer")
}
