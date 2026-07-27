package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunImport_ClaudeNativeConfig(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "project")
	claudeDir := filepath.Join(srcDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("# Project Instructions\n\nDo the thing."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"permissions":{"allow":["Read","Write"]},"model":"sonnet"}`), 0644))
	outputDir := filepath.Join(t.TempDir(), "output")
	require.NoError(t, runImport(io.Discard, srcDir, "", outputDir, ""))
	keelYml, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelYml), "schema: v1")
	assert.Contains(t, string(keelYml), "instructions.md")
	instructions, err := os.ReadFile(filepath.Join(outputDir, "instructions.md"))
	require.NoError(t, err)
	assert.Contains(t, string(instructions), "Do the thing")
	assert.FileExists(t, filepath.Join(outputDir, "permissions.json"))
}

func TestRunImport_CursorNativeConfig(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "project")
	cursorDir := filepath.Join(srcDir, ".cursor")
	require.NoError(t, os.MkdirAll(filepath.Join(cursorDir, "rules"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "instructions.md"), []byte("You are a helpful assistant."), 0644))
	outputDir := filepath.Join(t.TempDir(), "output")
	require.NoError(t, runImport(io.Discard, srcDir, "", outputDir, ""))
	keelYml, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelYml), "schema: v1")
	instructions, err := os.ReadFile(filepath.Join(outputDir, "instructions.md"))
	require.NoError(t, err)
	assert.Contains(t, string(instructions), "helpful assistant")
}

func TestRunImport_AutoDetectsClaude(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, ".claude"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("instructions"), 0644))
	outputDir := filepath.Join(t.TempDir(), "output")
	require.NoError(t, runImport(io.Discard, srcDir, "", outputDir, ""))
	keelYml, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelYml), "claude")
}

func TestRunImport_AutoDetectsCursor(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "project")
	cursorDir := filepath.Join(srcDir, ".cursor")
	require.NoError(t, os.MkdirAll(filepath.Join(cursorDir, "rules"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "instructions.md"), []byte("cursor instructions"), 0644))
	outputDir := filepath.Join(t.TempDir(), "output")
	require.NoError(t, runImport(io.Discard, srcDir, "", outputDir, ""))
	keelYml, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelYml), "cursor")
}

func TestRunImport_CustomLayerName(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, ".claude"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("instructions"), 0644))
	outputDir := filepath.Join(t.TempDir(), "output")
	require.NoError(t, runImport(io.Discard, srcDir, "claude", outputDir, "my-custom-layer"))
	keelYml, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelYml), "my-custom-layer")
}

func TestRunImport_InvalidPath(t *testing.T) {
	err := runImport(io.Discard, "/nonexistent/path/that/does/not/exist", "", t.TempDir(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source directory")
}

func TestRunImport_SourceIsFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("hello"), 0644))
	err := runImport(io.Discard, tmpFile, "", t.TempDir(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestRunImport_NoAgentDetected(t *testing.T) {
	err := runImport(io.Discard, t.TempDir(), "", filepath.Join(t.TempDir(), "out"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not detect")
}

func TestRunImport_InvalidAgent(t *testing.T) {
	err := runImport(io.Discard, t.TempDir(), "nonexistent-agent", filepath.Join(t.TempDir(), "out"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reader")
}

func TestImportCmd_MissingToFlag(t *testing.T) {
	cmd := newImportCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestRunImport_WithCategories(t *testing.T) {
	srcDir := filepath.Join(t.TempDir(), "project")
	commandsDir := filepath.Join(srcDir, ".claude", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("instructions"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "deploy.md"), []byte("deploy instructions"), 0644))
	outputDir := filepath.Join(t.TempDir(), "output")
	require.NoError(t, runImport(io.Discard, srcDir, "claude", outputDir, ""))
	keelYml, err := os.ReadFile(filepath.Join(outputDir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelYml), "commands/")
	deployCmd, err := os.ReadFile(filepath.Join(outputDir, "commands", "deploy.md"))
	require.NoError(t, err)
	assert.Contains(t, string(deployCmd), "deploy instructions")
}
