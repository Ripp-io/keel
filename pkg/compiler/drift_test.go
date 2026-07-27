package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/drift"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestDetectDrift_NoDrift(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a layer and compile.
	layerDir := filepath.Join(tmpDir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	testutil.WriteFile(t, layerDir, "keel.yml", []byte("schema: v1\n"))
	testutil.WriteFile(t, layerDir, "instructions.md", []byte("# Test\nHello world"))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "keel"},
	}
	result, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Load manifest and check drift — should be clean.
	cm, err := manifest.LoadKeelState(filepath.Join(tmpDir, ".claude"))
	require.NoError(t, err)
	require.NotNil(t, cm)

	drift, err := drift.Detect(tmpDir, cm)
	require.NoError(t, err)
	assert.False(t, drift.HasDrift(), "expected no drift after clean compile")
	assert.Empty(t, drift.Modified)
	assert.Empty(t, drift.Missing)
}

func TestDetectDrift_ModifiedFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Compile.
	layerDir := filepath.Join(tmpDir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	testutil.WriteFile(t, layerDir, "keel.yml", []byte("schema: v1\n"))
	testutil.WriteFile(t, layerDir, "instructions.md", []byte("# Original\nContent"))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "keel"},
	}
	_, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)

	// Manually edit CLAUDE.md.
	claudeMD := filepath.Join(tmpDir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(claudeMD, []byte("# Modified by user"), 0644))

	// Detect drift.
	cm, err := manifest.LoadKeelState(filepath.Join(tmpDir, ".claude"))
	require.NoError(t, err)

	drift, err := drift.Detect(tmpDir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "expected drift after manual edit")
	assert.Len(t, drift.Modified, 1)
	assert.Equal(t, "CLAUDE.md", drift.Modified[0].Path)
	assert.NotEmpty(t, drift.Modified[0].Expected)
	assert.NotEmpty(t, drift.Modified[0].Actual)
	assert.NotEqual(t, drift.Modified[0].Expected, drift.Modified[0].Actual)
}

func TestDetectDrift_MissingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Compile.
	layerDir := filepath.Join(tmpDir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	testutil.WriteFile(t, layerDir, "keel.yml", []byte("schema: v1\n"))
	testutil.WriteFile(t, layerDir, "instructions.md", []byte("# Instructions"))

	layers := []config.Layer{
		{Path: layerDir, Format: config.FormatKeel, Type: "keel"},
	}
	_, err := Compile("claude", layers, tmpDir, CompileOptions{})
	require.NoError(t, err)

	// Delete CLAUDE.md.
	require.NoError(t, os.Remove(filepath.Join(tmpDir, "CLAUDE.md")))

	// Detect drift.
	cm, err := manifest.LoadKeelState(filepath.Join(tmpDir, ".claude"))
	require.NoError(t, err)

	drift, err := drift.Detect(tmpDir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "expected drift after file deletion")
	assert.Len(t, drift.Missing, 1)
	assert.Equal(t, "CLAUDE.md", drift.Missing[0].Path)
}

func TestDetectDrift_OverlayMissing(t *testing.T) {
	t.Parallel()

	// Build a manifest with per-file overlay records and matching overlay entries.
	// The new WriteCategory returns per-file records with hashes.
	cm := &manifest.CompiledManifest{
		Agent: "claude",
		FileOps: []manifest.FileRecord{
			{Output: ".claude/commands/deploy.md", Strategy: "overlay", Hash: "sha256:dummy"},
		},
		Overlays: map[string][]string{
			".claude/commands/deploy.md": {"layer-1"},
		},
	}

	tmpDir := t.TempDir()
	// Don't create the overlay file — it should be detected as missing.

	drift, err := drift.Detect(tmpDir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift())
	assert.Len(t, drift.Missing, 1)
	assert.Equal(t, ".claude/commands/deploy.md", drift.Missing[0].Path)
}

func TestDetectDrift_EmptyManifest(t *testing.T) {
	t.Parallel()

	cm := &manifest.CompiledManifest{Agent: "claude"}
	drift, err := drift.Detect(t.TempDir(), cm)
	require.NoError(t, err)
	assert.False(t, drift.HasDrift())
}
