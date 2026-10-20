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

// T1: TestCompileMulti_WithMCP verifies that two agents compile with MCP servers
// and that the shared .mcp.json is written to each output directory.
func TestCompileMulti_WithMCP(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	// Create keel.yml with instructions and MCP servers
	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: MCP test

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	// Create instructions
	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# MCP Instructions\n\nThese instructions include MCP servers.\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	// Create MCP config
	mcpPath := filepath.Join(layerPath, "mcp.json")
	mcpContent := `{
  "mcpServers": {
    "test-server": {
      "command": "npx",
      "args": ["test-mcp"]
    }
  }
}`
	require.NoError(t, os.WriteFile(mcpPath, []byte(mcpContent), 0644), "writing mcp.json")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	// Separate output directories
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

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 2, "expected 2 results")

	// Verify AGENTS.md exists in both output dirs
	claudeAgentsMd := filepath.Join(claudeOut, "AGENTS.md")
	agentsMdData, err := os.ReadFile(claudeAgentsMd)
	require.NoError(t, err, "claude AGENTS.md not written")
	assert.Contains(t, string(agentsMdData), "MCP Instructions", "AGENTS.md should contain instructions")

	cursorAgentsMd := filepath.Join(cursorOut, "AGENTS.md")
	_, err = os.Stat(cursorAgentsMd)
	assert.NoError(t, err, "cursor AGENTS.md not written")

	// Verify .mcp.json exists in both output dirs
	claudeMcp := filepath.Join(claudeOut, ".mcp.json")
	mcpData, err := os.ReadFile(claudeMcp)
	require.NoError(t, err, "claude .mcp.json not written")
	assert.Contains(t, string(mcpData), "test-server", ".mcp.json should contain test-server")

	cursorMcp := filepath.Join(cursorOut, ".mcp.json")
	_, err = os.Stat(cursorMcp)
	assert.NoError(t, err, "cursor .mcp.json not written")
}

// T2: TestCompileMulti_SharedPhaseSkip_AgentsMdOnly verifies that the
// shared phase runs for agentsmd (which has SharedMCPOutput but no shim).
func TestCompileMulti_SharedPhaseSkip_AgentsMdOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: AgentsMD test

instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Test Instructions\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	outputDir := filepath.Join(tmpDir, "agentsmd-out")
	agents := map[string]string{
		"agentsmd": outputDir,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 1, "expected 1 result")

	// Verify AGENTS.md exists (agentsmd IS the shared format)
	agentsMd := filepath.Join(outputDir, "AGENTS.md")
	_, err = os.Stat(agentsMd)
	assert.NoError(t, err, "AGENTS.md should exist for agentsmd")
}

// T3: TestCompileMulti_ClaudeAndAgentsMd_SameDir verifies that when
// claude and agentsmd target the same directory, both compile successfully.
func TestCompileMulti_ClaudeAndAgentsMd_SameDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: Mixed agents test

instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Mixed Instructions\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	// Both agents target the same output directory
	outputDir := filepath.Join(tmpDir, "mixed-out")
	agents := map[string]string{
		"claude":   outputDir,
		"agentsmd": outputDir,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 2, "expected 2 results")

	// Shared phase should run (both need it)
	agentsMd := filepath.Join(outputDir, "AGENTS.md")
	_, err = os.Stat(agentsMd)
	assert.NoError(t, err, "AGENTS.md should exist when claude and agentsmd are in the group")

	// Claude gets CLAUDE.md shim (if it was written - may not be for same dir)
	// AgentsMD writes AGENTS.md directly (no shim)
}

// T4: TestCompileMulti_ThreeAgents verifies compilation for three agents
// targeting separate output directories.
func TestCompileMulti_ThreeAgents(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: Three agents test

instructions:
  - source: instructions.md
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Three Agents Instructions\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	// Three agents with separate output directories
	claudeOut := filepath.Join(tmpDir, "claude-out")
	cursorOut := filepath.Join(tmpDir, "cursor-out")
	codexOut := filepath.Join(tmpDir, "codex-out")

	agents := map[string]string{
		"claude": claudeOut,
		"cursor": cursorOut,
		"codex":  codexOut,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 3, "expected 3 results")

	// Each output dir has its own AGENTS.md (separate shared phases)
	claudeAgentsMd := filepath.Join(claudeOut, "AGENTS.md")
	_, err = os.Stat(claudeAgentsMd)
	assert.NoError(t, err, "claude AGENTS.md should exist")

	cursorAgentsMd := filepath.Join(cursorOut, "AGENTS.md")
	_, err = os.Stat(cursorAgentsMd)
	assert.NoError(t, err, "cursor AGENTS.md should exist")

	codexAgentsMd := filepath.Join(codexOut, "AGENTS.md")
	_, err = os.Stat(codexAgentsMd)
	assert.NoError(t, err, "codex AGENTS.md should exist")

	// Verify each agent has its dot-directory
	claudeDotDir := filepath.Join(claudeOut, ".claude")
	_, err = os.Stat(claudeDotDir)
	assert.NoError(t, err, "claude .claude directory should exist")

	cursorDotDir := filepath.Join(cursorOut, ".cursor")
	_, err = os.Stat(cursorDotDir)
	assert.NoError(t, err, "cursor .cursor directory should exist")

	codexDotDir := filepath.Join(codexOut, ".codex")
	_, err = os.Stat(codexDotDir)
	assert.NoError(t, err, "codex .codex directory should exist")
}

// T5: TestCompileMulti_DepDeduplication verifies that dependencies are
// installed only once across multiple agents.
func TestCompileMulti_DepDeduplication(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	// Create a layer with a fake dep (using echo as a no-op install command)
	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: Dep deduplication test

deps:
  testdep:
    install: "echo 'installing testdep'"
    verify: "echo 'testdep installed'"
    global: false
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
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

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")

	claudeResult := results["claude"]
	cursorResult := results["cursor"]

	// Both results should track the dep
	assert.Len(t, claudeResult.Dependencies.InstalledDeps, 1, "claude should track 1 dep")
	assert.Len(t, cursorResult.Dependencies.InstalledDeps, 1, "cursor should track 1 dep")

	// First agent installs via command, second should skip (already in preInstalled)
	// We can't easily verify the install was called only once without mocking,
	// but we can verify both results have the dep with the right method
	assert.Equal(t, "testdep", claudeResult.Dependencies.InstalledDeps[0].Name)
	assert.Equal(t, "testdep", cursorResult.Dependencies.InstalledDeps[0].Name)
}

// T6: TestCompileMulti_WithMCPAndPermissions verifies that MCP servers and
// permissions are compiled correctly for multiple agents with different
// settings formats.
func TestCompileMulti_WithMCPAndPermissions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: MCP and permissions test

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json

permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Test Instructions\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	mcpPath := filepath.Join(layerPath, "mcp.json")
	mcpContent := `{
  "mcpServers": {
    "test-server": {
      "command": "npx",
      "args": ["test-mcp"]
    }
  }
}`
	require.NoError(t, os.WriteFile(mcpPath, []byte(mcpContent), 0644), "writing mcp.json")

	permPath := filepath.Join(layerPath, "permissions.json")
	permContent := `{
  "allow": ["Read", "Edit"]
}`
	require.NoError(t, os.WriteFile(permPath, []byte(permContent), 0644), "writing permissions.json")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
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

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 2, "expected 2 results")

	// Verify shared .mcp.json contains the servers (claude)
	claudeMcp := filepath.Join(claudeOut, ".mcp.json")
	claudeMcpData, err := os.ReadFile(claudeMcp)
	require.NoError(t, err, "claude .mcp.json not written")
	assert.Contains(t, string(claudeMcpData), "test-server", "claude .mcp.json should contain test-server")

	// Verify cursor .mcp.json
	cursorMcp := filepath.Join(cursorOut, ".mcp.json")
	cursorMcpData, err := os.ReadFile(cursorMcp)
	require.NoError(t, err, "cursor .mcp.json not written")
	assert.Contains(t, string(cursorMcpData), "test-server", "cursor .mcp.json should contain test-server")

	// Verify Claude settings.json has permissions
	claudeSettings := filepath.Join(claudeOut, ".claude", "settings.json")
	claudeSettingsData, err := os.ReadFile(claudeSettings)
	require.NoError(t, err, "claude settings.json not written")
	assert.Contains(t, string(claudeSettingsData), "permissions", "claude settings should contain permissions")

	// Verify Cursor cli-config.json has permissions
	cursorSettings := filepath.Join(cursorOut, ".cursor", "cli-config.json")
	cursorSettingsData, err := os.ReadFile(cursorSettings)
	require.NoError(t, err, "cursor cli-config.json not written")
	assert.Contains(t, string(cursorSettingsData), "permissions", "cursor settings should contain permissions")
}

// T7: TestGroupNeedsSharedPhase is a direct unit test for the
// groupNeedsSharedPhase function.
func TestGroupNeedsSharedPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		agents   []string
		scope    Scope
		expected bool
	}{
		{
			name:     "claude and cursor - both have shared output",
			agents:   []string{"claude", "cursor"},
			scope:    ScopeProject,
			expected: true,
		},
		{
			name:     "agentsmd - has SharedMCPOutput",
			agents:   []string{"agentsmd"},
			scope:    ScopeProject,
			expected: true,
		},
		{
			name:     "claude and agentsmd - both have shared output",
			agents:   []string{"claude", "agentsmd"},
			scope:    ScopeProject,
			expected: true,
		},
		{
			name:     "empty slice",
			agents:   []string{},
			scope:    ScopeProject,
			expected: false,
		},
		{
			name:     "codex - has SharedMCPOutput",
			agents:   []string{"codex"},
			scope:    ScopeProject,
			expected: true,
		},
		{
			name:     "agentplugins only - no shared output",
			agents:   []string{"agentplugins"},
			scope:    ScopeProject,
			expected: false,
		},
		{
			name:     "claude and agentplugins - claude has shared output",
			agents:   []string{"claude", "agentplugins"},
			scope:    ScopeProject,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := groupNeedsSharedPhase(tt.agents, tt.scope)
			assert.Equal(t, tt.expected, result, "groupNeedsSharedPhase(%v, %v)", tt.agents, tt.scope)
		})
	}
}

// TestCompileMulti_SharedPhaseSkip_AgentPluginsOnly verifies that when
// compiling only agentplugins, the shared phase is skipped (no AGENTS.md or .mcp.json).
func TestCompileMulti_SharedPhaseSkip_AgentPluginsOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: AgentPlugins test

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# AgentPlugins Instructions\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	mcpPath := filepath.Join(layerPath, "mcp.json")
	mcpContent := `{
  "mcpServers": {
    "test-server": {
      "command": "npx",
      "args": ["test-mcp"]
    }
  }
}`
	require.NoError(t, os.WriteFile(mcpPath, []byte(mcpContent), 0644), "writing mcp.json")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	outputDir := filepath.Join(tmpDir, "output")
	agents := map[string]string{
		"agentplugins": outputDir,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 1, "expected 1 result")

	t.Run("NoAGENTSmd", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(outputDir, "AGENTS.md"))
		assert.True(t, os.IsNotExist(err), "AGENTS.md should not exist (shared phase skipped)")
	})

	t.Run("NoMCPjson", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(outputDir, ".mcp.json"))
		assert.True(t, os.IsNotExist(err), ".mcp.json should not exist (shared phase skipped)")
	})

	t.Run("AgentPluginsDir_Exists", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(outputDir, ".agentplugins"))
		assert.NoError(t, err, ".agentplugins directory should exist")
	})
}

// TestCompileMulti_ClaudeAndAgentPlugins_SameDir verifies that when
// claude and agentplugins target the same directory, the shared phase
// runs (because claude needs it) and both agents compile successfully.
func TestCompileMulti_ClaudeAndAgentPlugins_SameDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: Claude and AgentPlugins test

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Mixed Instructions\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	mcpPath := filepath.Join(layerPath, "mcp.json")
	mcpContent := `{
  "mcpServers": {
    "test-server": {
      "command": "npx",
      "args": ["test-mcp"]
    }
  }
}`
	require.NoError(t, os.WriteFile(mcpPath, []byte(mcpContent), 0644), "writing mcp.json")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	outputDir := filepath.Join(tmpDir, "output")
	agents := map[string]string{
		"claude":       outputDir,
		"agentplugins": outputDir,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 2, "expected 2 results")

	t.Run("AGENTSmd_Exists", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(outputDir, "AGENTS.md"))
		assert.NoError(t, err, "AGENTS.md should exist (shared phase runs for claude)")
	})

	t.Run("Claude_HasShim", func(t *testing.T) {
		claudeShim := filepath.Join(outputDir, "CLAUDE.md")
		claudeContent, err := os.ReadFile(claudeShim)
		require.NoError(t, err, "claude CLAUDE.md should exist")
		assert.Contains(t, string(claudeContent), "@AGENTS.md", "claude shim should reference AGENTS.md")
	})

	t.Run("AgentPluginsDir_Exists", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(outputDir, ".agentplugins"))
		assert.NoError(t, err, ".agentplugins directory should exist")
	})
}

// TestCompileMulti_SameOutputDir_ClaudeAndCursor verifies that when
// claude and cursor target the same directory, only one AGENTS.md and
// one .mcp.json are written (shared phase runs once per output group).
func TestCompileMulti_SameOutputDir_ClaudeAndCursor(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	require.NoError(t, os.MkdirAll(layerPath, 0755), "creating layer dir")

	keelYml := filepath.Join(layerPath, "keel.yml")
	keelContent := `schema: v1
name: test-layer
description: Claude and Cursor same dir test

instructions:
  - source: instructions.md

mcp:
  servers:
    - source: mcp.json
`
	require.NoError(t, os.WriteFile(keelYml, []byte(keelContent), 0644), "writing keel.yml")

	instrPath := filepath.Join(layerPath, "instructions.md")
	instrContent := "# Shared Instructions\n\nBoth agents read this.\n"
	require.NoError(t, os.WriteFile(instrPath, []byte(instrContent), 0644), "writing instructions.md")

	mcpPath := filepath.Join(layerPath, "mcp.json")
	mcpContent := `{
  "mcpServers": {
    "shared-server": {
      "command": "npx",
      "args": ["shared-mcp"]
    }
  }
}`
	require.NoError(t, os.WriteFile(mcpPath, []byte(mcpContent), 0644), "writing mcp.json")

	layers := []config.Layer{
		{Path: layerPath, Format: config.FormatKeel},
	}

	outputDir := filepath.Join(tmpDir, "output")
	agents := map[string]string{
		"claude": outputDir,
		"cursor": outputDir,
	}

	opts := CompileOptions{
		Scope:  ScopeProject,
		DryRun: false,
	}

	results, err := CompileMulti(agents, layers, opts)
	require.NoError(t, err, "CompileMulti failed")
	assert.Len(t, results, 2, "expected 2 results")

	t.Run("OneAGENTSmd", func(t *testing.T) {
		agentsMd := filepath.Join(outputDir, "AGENTS.md")
		_, err := os.Stat(agentsMd)
		assert.NoError(t, err, "AGENTS.md should exist")

		agentsMdData, err := os.ReadFile(agentsMd)
		require.NoError(t, err, "reading AGENTS.md")
		assert.Contains(t, string(agentsMdData), "Shared Instructions", "AGENTS.md should contain instructions")
	})

	t.Run("OneMCPjson", func(t *testing.T) {
		mcpJson := filepath.Join(outputDir, ".mcp.json")
		_, err := os.Stat(mcpJson)
		assert.NoError(t, err, ".mcp.json should exist at root")

		mcpData, err := os.ReadFile(mcpJson)
		require.NoError(t, err, "reading .mcp.json")
		assert.Contains(t, string(mcpData), "shared-server", ".mcp.json should contain shared-server")
	})

	t.Run("Claude_HasShim", func(t *testing.T) {
		claudeShim := filepath.Join(outputDir, "CLAUDE.md")
		claudeContent, err := os.ReadFile(claudeShim)
		require.NoError(t, err, "claude CLAUDE.md should exist")
		assert.Contains(t, string(claudeContent), "@AGENTS.md", "claude shim should reference AGENTS.md")
	})

	t.Run("CursorDir_Exists", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(outputDir, ".cursor"))
		assert.NoError(t, err, ".cursor directory should exist")
	})
}
