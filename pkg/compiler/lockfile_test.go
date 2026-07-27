package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestHashLayerDir_Deterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("beta"), 0644))
	hash1, err := manifest.HashLayerDir(dir)
	require.NoError(t, err, "manifest.HashLayerDir() first call")
	hash2, err := manifest.HashLayerDir(dir)
	require.NoError(t, err, "manifest.HashLayerDir() second call")
	assert.Equal(t, hash1, hash2, "hash should be deterministic")
	require.True(t, len(hash1) >= len("sha256:"), "hash too short: %q", hash1)
	assert.Equal(t, "sha256:", hash1[:7], "hash prefix")
}

func TestHashLayerDir_ContentChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0644))
	hash1, err := manifest.HashLayerDir(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644))
	hash2, err := manifest.HashLayerDir(dir)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "hash should change when file content changes")
}

func TestHashLayerDir_NewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644))
	hash1, err := manifest.HashLayerDir(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644))
	hash2, err := manifest.HashLayerDir(dir)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "hash should change when a new file is added")
}

func TestHashLayerDir_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hash1, err := manifest.HashLayerDir(dir)
	require.NoError(t, err, "first call")
	hash2, err := manifest.HashLayerDir(dir)
	require.NoError(t, err, "second call")
	assert.Equal(t, hash1, hash2, "deterministic hash for empty dir")
	assert.Equal(t, "sha256:", hash1[:7], "hash prefix")
}

func TestHashLayerDir_SkipsGitDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644))
	hash1, err := manifest.HashLayerDir(dir)
	require.NoError(t, err)
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "objects"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0644))
	hash2, err := manifest.HashLayerDir(dir)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2, ".git/ directory should be excluded from hash computation")
}

func TestHashLayerDir_PathMatters(t *testing.T) {
	t.Parallel()
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "fileA.txt"), []byte("content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "fileB.txt"), []byte("content"), 0644))
	hash1, err := manifest.HashLayerDir(dir1)
	require.NoError(t, err, "manifest.HashLayerDir(dir1)")
	hash2, err := manifest.HashLayerDir(dir2)
	require.NoError(t, err, "manifest.HashLayerDir(dir2)")
	assert.NotEqual(t, hash1, hash2, "different file paths with same content should produce different hashes")
}

func TestKeelState_WriteAndVerify(t *testing.T) {
	t.Parallel()
	layerDir1 := t.TempDir()
	layerDir2 := t.TempDir()
	outputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(layerDir1, "a.txt"), []byte("alpha"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir2, "b.txt"), []byte("beta"), 0644))
	layers := []config.Layer{
		{Path: layerDir1, Type: "base", Format: config.FormatKeel},
		{Path: layerDir2, Type: "project", Format: config.FormatKeel, Source: "https://git.example.com/org/policy.git#main"},
	}
	result := &CompileResult{Output: CompileOutput{Agent: "claude", OutputRoot: ".claude"}}
	opts := CompileOptions{Scope: ScopeProject}
	agentDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	require.NoError(t, writeKeelState(result, layers, outputDir, opts), "writeKeelState()")
	statePath := filepath.Join(agentDir, ".keel.json")
	data, err := os.ReadFile(statePath)
	require.NoError(t, err, "reading .keel.json")
	var cm manifest.CompiledManifest
	require.NoError(t, json.Unmarshal(data, &cm), "parsing .keel.json")
	assert.Equal(t, "claude", cm.Agent, "agent")
	assert.Equal(t, "project", cm.Scope, "scope")
	require.Len(t, cm.LayerStack, 2, "layers")
	assert.Equal(t, layerDir1, cm.LayerStack[0].Path, "layer 0 path")
	assert.Equal(t, "https://git.example.com/org/policy.git#main", cm.LayerStack[1].Source, "layer 1 source")
	assert.True(t, strings.HasPrefix(cm.LayerStack[0].Hash, "sha256:"), "layer 0 hash prefix, got %q", cm.LayerStack[0].Hash)
	assert.NotEmpty(t, cm.KeelVersion, "keel_version should be set")
	assert.NotEmpty(t, cm.CompiledAt, "compiled_at should be set")
	require.NoError(t, VerifyKeelState(agentDir, layers), "VerifyKeelState()")
}

func TestVerifyKeelState_Mismatch(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()
	outputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "file.txt"), []byte("original"), 0644))
	layers := []config.Layer{{Path: layerDir, Type: "base", Format: config.FormatKeel}}
	result := &CompileResult{Output: CompileOutput{Agent: "claude", OutputRoot: ".claude"}}
	opts := CompileOptions{Scope: ScopeProject}
	agentDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	require.NoError(t, writeKeelState(result, layers, outputDir, opts))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "file.txt"), []byte("changed"), 0644))
	err := VerifyKeelState(agentDir, layers)
	require.Error(t, err, "VerifyKeelState() should fail when content changed")
	assert.Contains(t, err.Error(), "hash mismatch", "error message")
}

func TestVerifyKeelState_MissingState(t *testing.T) {
	t.Parallel()
	agentDir := t.TempDir()
	layers := []config.Layer{{Path: t.TempDir(), Type: "base", Format: config.FormatKeel}}
	err := VerifyKeelState(agentDir, layers)
	require.Error(t, err, "VerifyKeelState() should fail when state is missing")
	assert.Contains(t, err.Error(), "no compiled state found", "error message")
}

func TestVerifyKeelState_LayerCountMismatch(t *testing.T) {
	t.Parallel()
	layerDir1 := t.TempDir()
	layerDir2 := t.TempDir()
	outputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(layerDir1, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir2, "b.txt"), []byte("b"), 0644))
	layers := []config.Layer{
		{Path: layerDir1, Type: "base", Format: config.FormatKeel},
		{Path: layerDir2, Type: "project", Format: config.FormatKeel},
	}
	result := &CompileResult{Output: CompileOutput{Agent: "claude", OutputRoot: ".claude"}}
	opts := CompileOptions{Scope: ScopeProject}
	agentDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	require.NoError(t, writeKeelState(result, layers, outputDir, opts))
	singleLayer := []config.Layer{{Path: layerDir1, Type: "base", Format: config.FormatKeel}}
	err := VerifyKeelState(agentDir, singleLayer)
	require.Error(t, err, "VerifyKeelState() should fail with different layer count")
	assert.Contains(t, err.Error(), "layer count mismatch", "error message")
}

func TestKeelState_PrettyPrinted(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()
	outputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "file.txt"), []byte("content"), 0644))
	layers := []config.Layer{{Path: layerDir, Type: "base", Format: config.FormatKeel}}
	result := &CompileResult{Output: CompileOutput{Agent: "claude", OutputRoot: ".claude"}}
	opts := CompileOptions{Scope: ScopeProject}
	agentDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	require.NoError(t, writeKeelState(result, layers, outputDir, opts))
	data, err := os.ReadFile(filepath.Join(agentDir, ".keel.json"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "  ", "pretty-printed JSON should have indentation")
	assert.Equal(t, byte('\n'), content[len(content)-1], "state file should end with newline")
}

func TestKeelState_VersionFromEnv(t *testing.T) {
	layerDir := t.TempDir()
	outputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "f.txt"), []byte("x"), 0644))
	t.Setenv("KEEL_BUILD", "1.5.0")
	layers := []config.Layer{{Path: layerDir, Type: "base", Format: config.FormatKeel}}
	result := &CompileResult{Output: CompileOutput{Agent: "claude", OutputRoot: ".claude"}}
	opts := CompileOptions{Scope: ScopeProject}
	agentDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	require.NoError(t, writeKeelState(result, layers, outputDir, opts))
	data, err := os.ReadFile(filepath.Join(agentDir, ".keel.json"))
	require.NoError(t, err)
	var cm manifest.CompiledManifest
	require.NoError(t, json.Unmarshal(data, &cm))
	assert.Equal(t, "1.5.0", cm.KeelVersion, "keel_version from KEEL_BUILD env")
}
