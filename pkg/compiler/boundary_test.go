package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestBoundary_EmptySettingsMap(t *testing.T) {
	t.Parallel()

	// settings: {} in keel.yml — should not produce a settings file (SkipSettings logic).

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keel := `schema: v1
name: empty-settings
settings: {}
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// settings.json should NOT exist.
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	_, err = os.Stat(settingsPath)
	assert.True(t, os.IsNotExist(err), "settings.json should not exist for empty settings map")
}

func TestBoundary_EmptyInstructionsList(t *testing.T) {
	t.Parallel()

	// No instruction files in any layer — should not produce instruction output.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keel := `schema: v1
name: no-instructions
settings:
  model: claude-sonnet-4
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// CLAUDE.md should NOT exist.
	instructionPath := filepath.Join(outputDir, "CLAUDE.md")
	_, err = os.Stat(instructionPath)
	assert.True(t, os.IsNotExist(err), "CLAUDE.md should not exist when no instructions provided")

	// settings.json SHOULD exist (has model).
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, "settings.json should exist")
}

func TestBoundary_LargeNumberOfCategoryFiles(t *testing.T) {
	t.Parallel()

	// 50+ files in a single category — should compile successfully.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	commandsDir := filepath.Join(layerDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	const numFiles = 60
	for i := 0; i < numFiles; i++ {
		filename := filepath.Join(commandsDir, "cmd"+string(rune('a'+i%26))+"-"+string(rune('0'+i/26))+".md")
		content := "Command " + filename
		require.NoError(t, os.WriteFile(filename, []byte(content), 0644))
	}

	keel := `schema: v1
name: large-category
commands:
  - source: commands/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify all files were written.
	outputCommandsDir := filepath.Join(outputDir, ".claude", "commands")
	entries, err := os.ReadDir(outputCommandsDir)
	require.NoError(t, err)
	assert.Len(t, entries, numFiles, "all command files should be written")
}

func TestBoundary_DeeplyNestedCategoryFiles(t *testing.T) {
	t.Parallel()

	// skills/a/b/c/deep.md — should preserve relative path.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	deepDir := filepath.Join(layerDir, "skills", "a", "b", "c")
	require.NoError(t, os.MkdirAll(deepDir, 0755))

	deepContent := "Deep skill content"
	deepFile := filepath.Join(deepDir, "deep.md")
	require.NoError(t, os.WriteFile(deepFile, []byte(deepContent), 0644))

	keel := `schema: v1
name: deep-nested
skills:
  - source: skills/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify deep file exists at correct path.
	outputDeepFile := filepath.Join(outputDir, ".claude", "skills", "a", "b", "c", "deep.md")
	content, err := os.ReadFile(outputDeepFile)
	require.NoError(t, err)
	assert.Equal(t, deepContent, string(content), "deep file content should match")
}

func TestBoundary_ZeroLengthInstructionFile(t *testing.T) {
	t.Parallel()

	// Empty instruction file — should produce empty CLAUDE.md (not skip).

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keel := `schema: v1
name: empty-instruction
instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(""), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// CLAUDE.md should exist (even if mostly empty, it has layer headers).
	instructionPath := filepath.Join(outputDir, "CLAUDE.md")
	_, err = os.Stat(instructionPath)
	assert.NoError(t, err, "CLAUDE.md should exist even with empty instruction content")
}

func TestBoundary_OnlyPortableSettings(t *testing.T) {
	t.Parallel()

	// Layer with only portable settings (model) — should produce settings file.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keel := `schema: v1
name: portable-only
settings:
  model: claude-sonnet-4
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// settings.json should exist.
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, "settings.json should exist for portable settings")
}

func TestBoundary_OnlyPermissions(t *testing.T) {
	t.Parallel()

	// Layer with only permissions (no settings) — should produce settings file.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keel := `schema: v1
name: perm-only
permissions:
  rules:
    - source: permissions.json
`
	perm := `{"allow": ["Read", "Write"]}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keel), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"), []byte(perm), 0644))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// settings.json should exist (contains permissions).
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	_, err = os.Stat(settingsPath)
	assert.NoError(t, err, "settings.json should exist when permissions are provided")
}
