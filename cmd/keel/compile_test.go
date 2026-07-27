package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestMergeConfigFile_CLIFlagTakesPrecedence(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.CompileConfig{}
	config.RegisterFlags(cmd, cfg)
	require.NoError(t, cmd.Flags().Set("output", "claude=/from-cli"))
	fileCfg := &config.CompileConfig{Output: map[string]string{"cursor": "/from-file"}}
	mergeConfigFile(cmd, cfg, fileCfg)
	assert.Equal(t, "/from-cli", cfg.Output["claude"])
	_, hasCursor := cfg.Output["cursor"]
	assert.False(t, hasCursor)
}

func TestMergeConfigFile_FileConfigUsedWhenFlagNotSet(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.CompileConfig{}
	config.RegisterFlags(cmd, cfg)
	fileCfg := &config.CompileConfig{Output: map[string]string{"cursor": "/custom-output"}, UserPriority: "low"}
	mergeConfigFile(cmd, cfg, fileCfg)
	assert.Equal(t, "/custom-output", cfg.Output["cursor"])
	assert.Equal(t, "low", cfg.UserPriority)
}

func TestMergeConfigFile_ZeroValueInFileNotApplied(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.CompileConfig{}
	config.RegisterFlags(cmd, cfg)
	require.NoError(t, cmd.Flags().Set("output", "claude=/cli-out"))
	mergeConfigFile(cmd, cfg, &config.CompileConfig{Output: nil})
	assert.Equal(t, "/cli-out", cfg.Output["claude"])
}

func TestMergeConfigFile_AllFieldsFromFile(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.CompileConfig{}
	config.RegisterFlags(cmd, cfg)
	fileCfg := &config.CompileConfig{Output: map[string]string{"cursor": "/file-output"}, Layers: []string{"/a", "/b"}, Distributable: true, Name: "my-dist", UserConfig: "/file-user", UserPriority: "low"}
	mergeConfigFile(cmd, cfg, fileCfg)
	assert.Equal(t, "/file-output", cfg.Output["cursor"])
	assert.Equal(t, []string{"/a", "/b"}, cfg.Layers)
	assert.True(t, cfg.Distributable)
	assert.Equal(t, "my-dist", cfg.Name)
	assert.Equal(t, "/file-user", cfg.UserConfig)
	assert.Equal(t, "low", cfg.UserPriority)
}

func TestMergeConfigFile_MultipleCLIFlagsOverride(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.CompileConfig{}
	config.RegisterFlags(cmd, cfg)
	require.NoError(t, cmd.Flags().Set("output", "claude=/cli-output"))
	fileCfg := &config.CompileConfig{Output: map[string]string{"cursor": "/file-output"}, Layers: []string{"/file-layers"}, UserPriority: "low"}
	mergeConfigFile(cmd, cfg, fileCfg)
	assert.Equal(t, "/cli-output", cfg.Output["claude"])
	assert.Equal(t, []string{"/file-layers"}, cfg.Layers)
	assert.Equal(t, "low", cfg.UserPriority)
}

func TestMergeConfigFile_EmptyFileConfig(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &config.CompileConfig{}
	config.RegisterFlags(cmd, cfg)
	require.NoError(t, cmd.Flags().Set("output", "claude=/cli-out"))
	mergeConfigFile(cmd, cfg, &config.CompileConfig{})
	assert.Equal(t, "/cli-out", cfg.Output["claude"])
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{99, "99"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{1000000, "1,000,000"},
		{9999999, "9,999,999"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintCompileSummary(t *testing.T) {
	// Create minimal test data
	layers := []config.Layer{
		{Path: "/layer1", Type: "base"},
		{Path: "/layer2", Type: "team"},
	}

	result := &compiler.CompileResult{
		Output: compiler.CompileOutput{
			Files: []manifest.FileRecord{
				{Output: ".claude/settings.json", Strategy: "deep-merge"},
				{Output: "CLAUDE.md", Strategy: "user-editable"},
				{Output: ".claude/commands/deploy.md", Strategy: "overlay"},
			},
		},
	}

	// Capture output
	var buf bytes.Buffer
	printCompileSummary(&buf, "claude", layers, result, false)

	output := buf.String()

	// Verify output contains expected elements
	assert.Contains(t, output, "Compiled claude with 2 layers")
	assert.Contains(t, output, ".claude/settings.json")
	assert.Contains(t, output, "CLAUDE.md")
	assert.Contains(t, output, ".claude/commands/deploy.md")
	assert.Contains(t, output, "deep-merge")
	assert.Contains(t, output, "user-editable")
	assert.Contains(t, output, "overlay")
}

func TestStrictMode_ZeroLayersError(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.CompileConfig{
		Output: map[string]string{"claude": t.TempDir()},
		Layers: []string{},
	}
	flags := compileFlags{Strict: true}
	err := runCompile(&buf, cfg, make(map[string]string), flags)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--strict: no config layers found")
}

func TestStrictMode_ZeroLayersWarnsWithoutStrict(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.CompileConfig{
		Output: map[string]string{"claude": t.TempDir()},
		Layers: []string{},
	}
	flags := compileFlags{Strict: false}
	err := runCompile(&buf, cfg, make(map[string]string), flags)
	// Should not error without --strict
	assert.NoError(t, err)
}
