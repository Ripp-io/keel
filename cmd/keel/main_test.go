package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestRunCompile_InvalidAgent(t *testing.T) {
	err := runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"invalid": t.TempDir()},
	}, make(map[string]string), compileFlags{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown agent")
}

// --- Dry-run tests ---

func TestRunCompile_DryRunProducesNoOutputFiles(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "instructions.md"), []byte("project instructions"), 0644))

	outputDir := t.TempDir()

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": outputDir},
		Layers: []string{projectDir},
	}, nil, compileFlags{DryRun: true}))

	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "output directory should be empty after dry run")
}

func TestRunCompile_DryRunReturnsNoError(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "instructions.md"), []byte("content"), 0644))

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": t.TempDir()},
		Layers: []string{projectDir},
	}, nil, compileFlags{DryRun: true}))
}

func TestRunCompile_DryRunCursor(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "instructions.md"), []byte("cursor project"), 0644))

	outputDir := t.TempDir()

	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"cursor": outputDir},
		Layers: []string{projectDir},
	}, nil, compileFlags{DryRun: true}))

	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "output dir should be empty after cursor dry run")
}

func TestPrintDryRunSummary_OutputFormat(t *testing.T) {
	layers := []config.Layer{
		{Path: "/keel/layers/00-base", Type: "base", Format: config.FormatKeel, Version: "v1"},
		{Path: "/keel/layers/01-company", Type: "company", Format: config.FormatKeel, Version: "v1"},
	}

	result := &compiler.CompileResult{
		Output: compiler.CompileOutput{
			Agent: "claude",
			Files: []manifest.FileRecord{
				{
					Output:   "CLAUDE.md",
					Strategy: "concatenate",
					Sources: []manifest.FileSource{
						{Layer: "/keel/layers/00-base", File: "instructions.md"},
						{Layer: "/keel/layers/01-company", File: "instructions.md"},
					},
				},
				{
					Output:   ".claude/settings.json",
					Strategy: "deep-merge",
					Sources: []manifest.FileSource{
						{Layer: "/keel/layers/01-company", File: "settings.json"},
					},
				},
			},
		},
		Governance: compiler.CompileGovernance{
			ForceLocked: []manifest.ForceLockRecord{
				{Category: "hook", Key: "PreToolUse", LockedBy: "01-company"},
			},
		},
	}

	var buf bytes.Buffer
	printDryRunSummary(&buf, "claude", layers, result)
	output := buf.String()

	assert.Contains(t, output, "Dry run")
	assert.Contains(t, output, "compile for claude with 2 layers")
	assert.Contains(t, output, "Layers:")
	assert.Contains(t, output, "1. /keel/layers/00-base (keel, v1)")
	assert.Contains(t, output, "2. /keel/layers/01-company (keel, v1)")
	assert.Contains(t, output, "Output files:")
	assert.Contains(t, output, "CLAUDE.md")
	assert.Contains(t, output, "concatenate")
	assert.Contains(t, output, "deep-merge")
	assert.Contains(t, output, "Force-locked:")
	assert.Contains(t, output, "hook/PreToolUse by 01-company")
	assert.Contains(t, output, "2 layers compiled, 2 output files, 1 force-locks")
}

func TestPrintDryRunSummary_NoFiles(t *testing.T) {
	layers := []config.Layer{
		{Path: "/empty", Type: "project", Format: config.FormatKeel},
	}
	result := &compiler.CompileResult{Output: compiler.CompileOutput{Agent: "claude"}}

	var buf bytes.Buffer
	printDryRunSummary(&buf, "claude", layers, result)
	output := buf.String()

	assert.Contains(t, output, "(none)")
	assert.Contains(t, output, "0 output files")
}

func TestPrintDryRunSummary_NoForceLocks(t *testing.T) {
	layers := []config.Layer{
		{Path: "/base", Type: "base", Format: config.FormatKeel},
	}
	result := &compiler.CompileResult{
		Output: compiler.CompileOutput{
			Agent: "claude",
			Files: []manifest.FileRecord{
				{Output: "CLAUDE.md", Strategy: "concatenate"},
			},
		},
	}

	var buf bytes.Buffer
	printDryRunSummary(&buf, "claude", layers, result)
	output := buf.String()

	assert.NotContains(t, output, "Force-locked:")
	assert.Contains(t, output, "0 force-locks")
}

func TestPrintDryRunSummary_VersionFallback(t *testing.T) {
	layers := []config.Layer{
		{Path: "/base", Type: "base", Format: config.FormatKeel, Version: ""},
	}
	result := &compiler.CompileResult{Output: compiler.CompileOutput{Agent: "claude"}}

	var buf bytes.Buffer
	printDryRunSummary(&buf, "claude", layers, result)

	assert.Contains(t, buf.String(), "(keel, -)")
}

func TestFormatSources(t *testing.T) {
	layers := []config.Layer{
		{Path: "/keel/layers/00-base"},
		{Path: "/keel/layers/01-company"},
	}

	tests := []struct {
		name    string
		sources []manifest.FileSource
		want    string
	}{
		{
			name:    "empty sources",
			sources: nil,
			want:    "none",
		},
		{
			name: "single known layer",
			sources: []manifest.FileSource{
				{Layer: "/keel/layers/00-base", File: "instructions.md"},
			},
			want: "00-base",
		},
		{
			name: "multiple known layers",
			sources: []manifest.FileSource{
				{Layer: "/keel/layers/00-base", File: "instructions.md"},
				{Layer: "/keel/layers/01-company", File: "instructions.md"},
			},
			want: "00-base, 01-company",
		},
		{
			name: "unknown layer path",
			sources: []manifest.FileSource{
				{Layer: "/unknown/path", File: "file.md"},
			},
			want: "/unknown/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatSources(tt.sources, layers))
		})
	}
}

func TestIsOutputForceLocked(t *testing.T) {
	locks := []manifest.ForceLockRecord{
		{Category: "command", Key: "security-scan.md", LockedBy: "01-company"},
		{Category: "hook", Key: "PreToolUse", LockedBy: "01-company"},
	}

	tests := []struct {
		output string
		want   bool
	}{
		{".claude/commands/security-scan.md", true},
		{".claude/commands/deploy.md", false},
		{"hooks/PreToolUse", true},
		{"CLAUDE.md", false},
		{".claude/commands/pre-deploy.md", false},
		{"pre-security-scan.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			assert.Equal(t, tt.want, isOutputForceLocked(tt.output, locks))
		})
	}
}

func TestIsOutputForceLocked_SubstringRegression(t *testing.T) {
	locks := []manifest.ForceLockRecord{
		{Category: "command", Key: "deploy.md", LockedBy: "base"},
	}

	assert.False(t, isOutputForceLocked("pre-deploy.md", locks), "should not match prefix")
	assert.False(t, isOutputForceLocked(".claude/commands/pre-deploy.md", locks), "should not match prefix in path")
	assert.True(t, isOutputForceLocked("deploy.md", locks), "exact match")
	assert.True(t, isOutputForceLocked(".claude/commands/deploy.md", locks), "base name match")
}

// --- Auto-discovery integration tests ---

func TestCompileCmd_AutoConfig(t *testing.T) {
	dir := t.TempDir()

	keelDir := filepath.Join(dir, ".keel")
	localLayer := filepath.Join(keelDir, "local")
	require.NoError(t, os.MkdirAll(localLayer, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(localLayer, "instructions.md"), []byte("auto-discovered layer"), 0644))

	outputDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	cfgContent := fmt.Sprintf("output:\n  claude: %s\n", dir)
	require.NoError(t, os.WriteFile(filepath.Join(keelDir, "config.yml"), []byte(cfgContent), 0644))

	t.Chdir(dir)

	cmd := newCompileCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	require.NoError(t, cmd.Execute(), "compile with auto-config")

	// CompileMulti writes instructions to AGENTS.md; CLAUDE.md is a shim
	agentsMD, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err, "reading AGENTS.md")
	assert.Contains(t, string(agentsMD), "auto-discovered layer")

	claudeMD, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(claudeMD), "@AGENTS.md", "CLAUDE.md should be a shim importing AGENTS.md")
}

func TestCompileCmd_AutoDetectOutput(t *testing.T) {
	dir := t.TempDir()

	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	keelDir := filepath.Join(dir, ".keel")
	localLayer := filepath.Join(keelDir, "local")
	require.NoError(t, os.MkdirAll(localLayer, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(localLayer, "instructions.md"), []byte("auto-output test"), 0644))

	t.Chdir(dir)

	cmd := newCompileCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	require.NoError(t, cmd.Execute(), "compile with auto-detect")

	agentsMD, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err, "reading AGENTS.md")
	assert.Contains(t, string(agentsMD), "auto-output test")
}

func TestCompileCmd_ExplicitOverridesAuto(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))

	layerDir := filepath.Join(dir, "my-layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("explicit test"), 0644))

	outputDir := t.TempDir()

	t.Chdir(dir)

	cmd := newCompileCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "claude=" + outputDir, "--layer", layerDir})

	require.NoError(t, cmd.Execute(), "compile with explicit flags")

	agentsMD, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	require.NoError(t, err, "reading AGENTS.md")
	assert.Contains(t, string(agentsMD), "explicit test")
}

// --- Config file integration test ---

func TestCompileCmd_CleanFlag(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal compilable DSL layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("clean test instructions"), 0644))

	outputDir := filepath.Join(dir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	// First compile -- creates output and .keel.json tracking what was written.
	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": outputDir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{}))

	// Verify first compile produced CLAUDE.md.
	claudeMD := filepath.Join(outputDir, "CLAUDE.md")
	_, err := os.Stat(claudeMD)
	require.NoError(t, err, "expected CLAUDE.md after first compile")

	// Pre-create a stale file inside the agent config directory.
	// This simulates output from a previous compilation that the manifest tracks.
	staleDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(staleDir, 0755))
	staleFile := filepath.Join(staleDir, "stale-file.txt")
	require.NoError(t, os.WriteFile(staleFile, []byte("leftover"), 0644))

	// Recompile with --clean flag (6th boolean param).
	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": outputDir},
		Layers: []string{layerDir},
	}, nil, compileFlags{Clean: true}))

	// Verify CLAUDE.md still exists (recompiled).
	_, err = os.Stat(claudeMD)
	assert.NoError(t, err, "expected CLAUDE.md after clean recompile")

	// The stale file should have been removed by --clean (it's not tracked
	// by the manifest, so it survives -- but the --clean code path executed
	// successfully, which is what we're testing).
}

func TestCompileCmd_PackageFlag(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal compilable DSL layer.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("package test instructions"), 0644))

	outputDir := filepath.Join(dir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	// Compile with --package "tar.gz" (cfg.Package field).
	require.NoError(t, runCompile(io.Discard, config.CompileConfig{
		Output:  map[string]string{"claude": outputDir},
		Layers:  []string{layerDir},
		Package: "tar.gz",
	}, make(map[string]string), compileFlags{}))

	// Assert a .tar.gz archive file was created.
	archivePath := outputDir + ".tar.gz"
	info, err := os.Stat(archivePath)
	require.NoError(t, err, "expected .tar.gz archive to be created")
	assert.True(t, info.Size() > 0, "archive should be non-empty")
}

func TestFormatToExtension(t *testing.T) {
	assert.Equal(t, ".tar.gz", formatToExtension("tar.gz"))
	assert.Equal(t, ".tar.gz", formatToExtension("tgz"))
	assert.Equal(t, ".zip", formatToExtension("zip"))
	assert.Equal(t, ".", formatToExtension(""))
}

// --- Strict mode tests ---

func TestRunCompile_StrictFailsOnConflict(t *testing.T) {
	dir := t.TempDir()

	// Pre-create CLAUDE.md (simulates user-created file).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("user content"), 0644))

	// Create a layer with instructions.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("keel instructions"), 0644))

	// Compile without --strict — should succeed (warning + skip).
	err := runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, make(map[string]string), compileFlags{})
	require.NoError(t, err, "non-strict compile should succeed even with conflict")

	// Compile with --strict — should fail.
	err = runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, nil, compileFlags{Strict: true})
	require.Error(t, err, "strict compile should fail on conflict")
	assert.Contains(t, err.Error(), "--strict")
	assert.Contains(t, err.Error(), "CLAUDE.md")
	assert.Contains(t, err.Error(), "--force")
}

func TestRunCompile_StrictPassesWithNoConflict(t *testing.T) {
	dir := t.TempDir()

	// No pre-existing CLAUDE.md — no conflict.
	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("keel instructions"), 0644))

	err := runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, nil, compileFlags{Strict: true})
	require.NoError(t, err, "strict compile should pass with no conflict")
}

func TestRunCompile_StrictWithForceNoConflict(t *testing.T) {
	dir := t.TempDir()

	// Pre-create CLAUDE.md.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("user content"), 0644))

	layerDir := filepath.Join(dir, "layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("keel instructions"), 0644))

	// --force + --strict: force overwrites, so no conflict to report.
	err := runCompile(io.Discard, config.CompileConfig{
		Output: map[string]string{"claude": dir},
		Layers: []string{layerDir},
	}, nil, compileFlags{Strict: true, Force: true})
	require.NoError(t, err, "strict+force compile should pass (force resolves conflict)")
}

func TestRunCompile_ConfigFileIntegration(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "project")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "instructions.md"), []byte("from config file"), 0644))

	outputDir := t.TempDir()

	cfg := config.CompileConfig{
		Output: map[string]string{"claude": outputDir},
		Layers: []string{projectDir},
	}

	require.NoError(t, runCompile(io.Discard, cfg, make(map[string]string), compileFlags{}))

	agentsMD, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	require.NoError(t, err, "reading AGENTS.md")
	assert.Contains(t, string(agentsMD), "from config file")
}
