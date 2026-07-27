package cursor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompile_Cursor(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal layer with a keel.yml.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-cursor-compile
description: Minimal test layer for Cursor
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{})

	require.NoError(t, err)
	assert.Equal(t, "cursor", result.Output.Agent)
}

func TestCompile_Cursor_ProjectScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-project-cursor
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	instContent := "# Cursor Project Instructions\n\nTest content."
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instContent), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeProject})

	require.NoError(t, err)
	assert.Equal(t, "cursor", result.Output.Agent)

	// Project scope should produce .cursor/rules/00-keel.mdc.
	instructionPath := filepath.Join(dir, ".cursor", "rules", "00-keel.mdc")
	_, err = os.Stat(instructionPath)
	assert.NoError(t, err, ".cursor/rules/00-keel.mdc should exist for project scope")
}

func TestCompile_Cursor_UserScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `schema: v1
name: test-user-cursor
settings:
  model: gpt-4o
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}
	result, err := Compile(layers, dir, compiler.CompileOptions{Scope: compiler.ScopeUser})

	require.NoError(t, err)
	assert.Equal(t, "cursor", result.Output.Agent)

	// User scope should produce settings but NOT instructions (no rules for user scope).
	settingsPath := filepath.Join(dir, ".cursor", "cli-config.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, "settings should exist for user scope")

	// .cursor/rules/00-keel.mdc should NOT exist in user scope.
	instructionPath := filepath.Join(dir, ".cursor", "rules", "00-keel.mdc")
	_, err = os.Stat(instructionPath)
	assert.True(t, os.IsNotExist(err), "instruction file should not exist for user scope")
}
