package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestCompileMulti_TwoAgents(t *testing.T) {
	t.Parallel()

	// Create a test DSL layer
	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	content := `schema: v1
name: test-layer
description: Multi-agent test

instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(keelYml, []byte(content), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Test Instructions\n\nThis is a test.\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	// Create separate output directories for each agent
	claudeOut := filepath.Join(tmpDir, "claude-out")
	cursorOut := filepath.Join(tmpDir, "cursor-out")

	agents := map[string]string{
		"claude": claudeOut,
		"cursor": cursorOut,
	}

	opts := CompileOptions{
		Scope:         ScopeProject,
		Distributable: false,
		DryRun:        false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")

	// Verify both agents compiled successfully
	assert.Len(t, results, 2, "expected 2 results")

	claudeResult, ok := results["claude"]
	require.True(t, ok, "missing claude result")
	cursorResult, ok := results["cursor"]
	require.True(t, ok, "missing cursor result")

	// Verify shared AGENTS.md exists in both output dirs
	claudeAgentsMd := filepath.Join(claudeOut, "AGENTS.md")
	_, err = os.Stat(claudeAgentsMd)
	assert.NoError(t, err, "shared AGENTS.md not written to claude outputDir")

	cursorAgentsMd := filepath.Join(cursorOut, "AGENTS.md")
	_, err = os.Stat(cursorAgentsMd)
	assert.NoError(t, err, "shared AGENTS.md not written to cursor outputDir")

	// Verify Claude shim exists (Claude doesn't read AGENTS.md natively)
	claudeInstr := filepath.Join(claudeOut, "CLAUDE.md")
	claudeContent, err := os.ReadFile(claudeInstr)
	require.NoError(t, err, "claude shim CLAUDE.md not written")
	assert.Contains(t, string(claudeContent), "@AGENTS.md", "claude shim should import AGENTS.md")

	// Verify Cursor does NOT write instruction file when no agent-specific content
	// (Cursor reads AGENTS.md natively, so no shim needed)
	cursorInstr := filepath.Join(cursorOut, ".cursor", "rules", "00-keel.mdc")
	_, err = os.Stat(cursorInstr)
	assert.True(t, os.IsNotExist(err), "cursor should not write 00-keel.mdc when no cursor-specific content (reads AGENTS.md natively)")

	// Verify both results have the correct name
	assert.Equal(t, "test-layer", claudeResult.Tracking.Name, "expected claude result name 'test-layer'")
	assert.Equal(t, "test-layer", cursorResult.Tracking.Name, "expected cursor result name 'test-layer'")
}

func TestCompileMulti_CacheEfficiency(t *testing.T) {
	t.Parallel()

	// This test verifies that DSL layers are cached (read once) while
	// convention/native layers are read per-agent.
	tmpDir := t.TempDir()

	// Create a DSL layer
	dslPath := filepath.Join(tmpDir, "dsl-layer")
	require.NoError(t, os.MkdirAll(dslPath, 0755), "creating dsl layer")
	require.NoError(t, os.WriteFile(filepath.Join(dslPath, "keel.yml"), []byte("schema: v1\nname: dsl\n"), 0644), "writing keel.yml")

	// Create a convention layer (FormatKeel without keel.yml)
	convPath := filepath.Join(tmpDir, "conv-layer")
	require.NoError(t, os.MkdirAll(filepath.Join(convPath, ".claude"), 0755), "creating convention layer")

	layers := []config.Layer{
		{Path: dslPath, Format: config.FormatKeel},
		{Path: convPath, Format: config.FormatKeel},
	}

	claudeOut := filepath.Join(tmpDir, "claude-out")
	cursorOut := filepath.Join(tmpDir, "cursor-out")

	agents := map[string]string{
		"claude": claudeOut,
		"cursor": cursorOut,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	// Create a cache directly to inspect it after compilation
	cache := NewLayerCache()
	opts.layerCache = cache

	// Manually compile with the same cache to verify caching behavior
	preInstalled := make(map[string]string)
	for agent, outputDir := range agents {
		agentOpts := opts
		agentOpts.PreInstalledDeps = preInstalled
		agentOpts.layerCache = cache

		result, err := Compile(agent, layers, outputDir, agentOpts)
		require.NoError(t, err, "compiling for %s", agent)

		for _, dep := range result.Dependencies.InstalledDeps {
			preInstalled[dep.Name] = dep.Method
		}
	}

	// Verify DSL layer is in cache
	_, ok := cache.entries[dslPath]
	assert.True(t, ok, "DSL layer not cached")

	// Verify convention layer is NOT in cache
	_, ok = cache.entries[convPath]
	assert.False(t, ok, "convention layer should not be cached")

	// Verify cache has exactly 1 entry (the DSL layer)
	assert.Len(t, cache.entries, 1, "expected 1 cache entry")
}

func TestCompileMulti_SingleAgent(t *testing.T) {
	t.Parallel()

	// CompileMulti should work for a single agent (no cache benefit, but same code path)
	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	require.NoError(t, os.WriteFile(keelYml, []byte("schema: v1\nname: single\n"), 0644), "writing keel.yml")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	outputDir := filepath.Join(tmpDir, "output")
	agents := map[string]string{"claude": outputDir}

	opts := CompileOptions{Scope: ScopeProject}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")

	assert.Len(t, results, 1, "expected 1 result")

	result, ok := results["claude"]
	require.True(t, ok, "missing claude result")
	assert.Equal(t, "single", result.Tracking.Name, "expected name 'single'")
}
