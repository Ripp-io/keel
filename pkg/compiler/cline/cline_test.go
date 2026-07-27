package cline

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

	assert.Equal(t, "cline", profile.Name)
	assert.Equal(t, ".cline", profile.OutputRoot)
	assert.Equal(t, ".clinerules", profile.InstructionOutput)
	assert.Equal(t, "", profile.SettingsFile)
	assert.Equal(t, "", profile.MCPOutput)
	assert.False(t, profile.HooksInSettings)
	assert.Nil(t, profile.Categories)
	assert.Equal(t, []string{".clinerules"}, profile.InstructionCandidates)
}

func TestCompile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-cline-compile
description: Minimal test layer for Cline
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	instContent := "# Cline Instructions\n\nUse VSCode extensions wisely."
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "cline", result.Output.Agent)

	// Verify .clinerules was written at the output root.
	clinerulesPath := filepath.Join(dir, ".clinerules")
	content, err := os.ReadFile(clinerulesPath)
	require.NoError(t, err, ".clinerules should exist")
	assert.Contains(t, string(content), "Cline Instructions")
	assert.Contains(t, string(content), "Use VSCode extensions wisely")
}

func TestCompile_EmptyLayers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a layer with no instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-cline-empty
description: Layer with no instructions
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "cline", result.Output.Agent)

	// .clinerules should NOT exist when there are no instructions.
	clinerulesPath := filepath.Join(dir, ".clinerules")
	_, err = os.Stat(clinerulesPath)
	assert.True(t, os.IsNotExist(err), ".clinerules should not exist without instructions")
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
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "inst1.md"), []byte("# Layer 1\n\nFirst rule."), 0644))

	layer2 := filepath.Join(dir, "layer2")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "keel.yml"), []byte(`schema: v1
name: layer2
instructions:
  - source: inst2.md
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "inst2.md"), []byte("# Layer 2\n\nSecond rule."), 0644))

	layers := []config.Layer{
		{Path: layer1, Format: config.FormatKeel, Type: "test"},
		{Path: layer2, Format: config.FormatKeel, Type: "test"},
	}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "cline", result.Output.Agent)

	// Verify both instructions are concatenated.
	clinerulesPath := filepath.Join(dir, ".clinerules")
	content, err := os.ReadFile(clinerulesPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Layer 1")
	assert.Contains(t, string(content), "First rule")
	assert.Contains(t, string(content), "Layer 2")
	assert.Contains(t, string(content), "Second rule")
}
