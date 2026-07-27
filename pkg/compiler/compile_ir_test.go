package compiler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func TestCompileIR_DropsUnsupportedContent(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()
	claudeDir := filepath.Join(layerDir, ".claude")
	testutil.WriteFile(t, layerDir, "CLAUDE.md", []byte("# Cross-compile test"))
	settings := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read", "Edit"}},
		"model":       "claude-sonnet-4-20250514",
		"sandbox":     map[string]any{"enabled": true},
	}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	testutil.WriteFile(t, claudeDir, "settings.json", settingsData)
	testutil.WriteFile(t, claudeDir, filepath.Join("commands", "deploy.md"), []byte("# Deploy"))
	testutil.WriteFile(t, claudeDir, filepath.Join("plugins", "my-plugin.md"), []byte("# Plugin"))
	testutil.WriteFile(t, claudeDir, filepath.Join("agent-memory", "context.md"), []byte("# Memory"))
	layers := []config.Layer{{Path: layerDir, Format: "claude"}}

	t.Run("claude-to-cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		result, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		assert.Equal(t, "cursor", result.Output.Agent, "result agent")
		cmdPath := filepath.Join(outputDir, ".cursor", "commands", "deploy.md")
		_, err = os.Stat(cmdPath)
		assert.NoError(t, err, "commands/deploy.md should be in Cursor output")
		instrPath := filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc")
		_, err = os.Stat(instrPath)
		assert.NoError(t, err, "instructions should be in Cursor output")
		pluginPath := filepath.Join(outputDir, ".cursor", "plugins")
		_, err = os.Stat(pluginPath)
		assert.True(t, os.IsNotExist(err), "plugins/ should not exist in Cursor output")
		memoryPath := filepath.Join(outputDir, ".cursor", "agent-memory")
		_, err = os.Stat(memoryPath)
		assert.True(t, os.IsNotExist(err), "agent-memory/ should not exist in Cursor output")
		cliConfigPath := filepath.Join(outputDir, ".cursor", "cli-config.json")
		if data, readErr := os.ReadFile(cliConfigPath); readErr == nil {
			var cliConfig map[string]any
			if jsonErr := json.Unmarshal(data, &cliConfig); jsonErr == nil {
				assert.NotContains(t, cliConfig, "sandbox", "sandbox should not be in Cursor cli-config.json")
			}
		}
	})

	t.Run("claude-to-claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		pluginPath := filepath.Join(outputDir, ".claude", "plugins", "my-plugin.md")
		_, err = os.Stat(pluginPath)
		assert.NoError(t, err, "plugins/my-plugin.md should be in Claude output")
		memoryPath := filepath.Join(outputDir, ".claude", "agent-memory", "context.md")
		_, err = os.Stat(memoryPath)
		assert.NoError(t, err, "agent-memory/context.md should be in Claude output")
	})
}

func TestCompileIR_FrontmatterWarnings(t *testing.T) {
	layerDir := t.TempDir()
	claudeDir := filepath.Join(layerDir, ".claude")
	testutil.WriteFile(t, layerDir, "CLAUDE.md", []byte("# Frontmatter portability test"))
	settings := map[string]any{"permissions": map[string]any{"allow": []string{"Read"}}}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	testutil.WriteFile(t, claudeDir, "settings.json", settingsData)
	testutil.WriteFile(t, claudeDir, filepath.Join("agents", "portable-agent.md"),
		[]byte("---\nname: portable-agent\ndescription: Fully portable agent\nmodel: sonnet\n---\nPortable agent body.\n"))
	testutil.WriteFile(t, claudeDir, filepath.Join("skills", "deploy", "SKILL.md"),
		[]byte("---\nname: deploy\ndescription: Deploy skill\ncontext: fork\narguments:\n  - env\n  - region\n---\nDeploy skill body.\n"))
	testutil.WriteFile(t, claudeDir, filepath.Join("commands", "build.md"),
		[]byte("---\ndescription: Build the project\nallowed-tools:\n  - Bash\n---\nBuild command body.\n"))
	testutil.WriteFile(t, claudeDir, filepath.Join("commands", "lint.md"),
		[]byte("---\nforce: true\n---\nLint command body.\n"))
	testutil.WriteFile(t, claudeDir, filepath.Join("commands", "config.json"), []byte("{\"key\": \"value\"}"))
	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	var logBuf bytes.Buffer
	origLogger := slog.Default()
	testHandler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(testHandler))
	defer slog.SetDefault(origLogger)

	t.Run("claude-to-cursor-warns", func(t *testing.T) {
		logBuf.Reset()
		outputDir := t.TempDir()
		result, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		assert.Equal(t, "cursor", result.Output.Agent, "result agent")
		logOutput := logBuf.String()
		assert.True(t, strings.Contains(logOutput, "context") && strings.Contains(logOutput, "arguments"),
			"expected warning about non-portable skill fields (context, arguments), got logs:\n%s", logOutput)
		assert.Contains(t, logOutput, "build.md", "expected warning about command frontmatter for build.md")
		assert.NotContains(t, logOutput, "lint.md", "lint.md has only Keel governance fields, should not trigger portability warning")
		assert.NotContains(t, logOutput, "portable-agent.md", "portable-agent.md has only portable fields, should not trigger warning")
		skillPath := filepath.Join(outputDir, ".cursor", "skills", "deploy", "SKILL.md")
		skillData, err := os.ReadFile(skillPath)
		if assert.NoError(t, err, "skill file should be in output") {
			assert.Contains(t, string(skillData), "Deploy skill body.", "skill file body content should be preserved")
		}
		cmdPath := filepath.Join(outputDir, ".cursor", "commands", "build.md")
		cmdData, err := os.ReadFile(cmdPath)
		if assert.NoError(t, err, "command file should be in output") {
			assert.Contains(t, string(cmdData), "Build command body.", "command file body content should be preserved")
		}
	})

	t.Run("claude-to-claude-no-warnings", func(t *testing.T) {
		logBuf.Reset()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		logOutput := logBuf.String()
		assert.NotContains(t, logOutput, "frontmatter", "same-agent compilation should not emit frontmatter portability warnings")
	})
}

func TestCompileIR_HookEventPortabilityWarnings(t *testing.T) {
	layerDir := t.TempDir()
	testutil.WriteFile(t, layerDir, ".cursorrules", []byte("# Hook portability test"))
	hooks := map[string]any{
		"preToolUse":           []any{map[string]any{"matcher": "Write", "hooks": []string{"echo pre"}}},
		"beforeShellExecution": []any{map[string]any{"hooks": []string{"echo shell"}}},
	}
	hooksData, _ := json.MarshalIndent(hooks, "", "  ")
	testutil.WriteFile(t, layerDir, "hooks.json", hooksData)
	layers := []config.Layer{{Path: layerDir, Format: "cursor"}}
	var logBuf bytes.Buffer
	origLogger := slog.Default()
	testHandler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(testHandler))
	defer slog.SetDefault(origLogger)

	t.Run("cursor-to-claude-warns-nonportable-hooks", func(t *testing.T) {
		logBuf.Reset()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "hook event may not be supported", "expected warning about non-portable hook event")
		assert.Contains(t, logOutput, "BeforeShellExecution", "expected warning to mention BeforeShellExecution")
	})

	t.Run("cursor-to-cursor-no-hook-warnings", func(t *testing.T) {
		logBuf.Reset()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		logOutput := logBuf.String()
		assert.NotContains(t, logOutput, "hook event may not be supported", "same-agent compilation should not emit hook portability warnings")
	})
}

func TestIsPortableHookEvent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		event    string
		portable bool
	}{
		{"PreToolUse", true}, {"PostToolUse", true}, {"UserPromptSubmit", true},
		{"Stop", true}, {"SessionStart", true},
		{"BeforeShellExecution", false}, {"AfterShellExecution", false}, {"SomeUnknownEvent", false},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.portable, isPortableHookEvent(tt.event), "isPortableHookEvent(%q)", tt.event)
		})
	}
}

func TestExtractNonPortableHandlerTypes(t *testing.T) {
	t.Parallel()
	nestedEntries := []any{
		map[string]any{"matcher": "Write", "hooks": []any{
			map[string]any{"type": "command", "command": "echo ok"},
			map[string]any{"type": "http", "url": "https://example.com"},
		}},
		map[string]any{"matcher": "Read", "hooks": []any{
			map[string]any{"type": "mcp_tool", "server": "gitlab"},
		}},
	}
	got := extractNonPortableHandlerTypes(nestedEntries)
	require.Len(t, got, 2, "non-portable types for nested entries")
	assert.Equal(t, []string{"http", "mcp_tool"}, got, "non-portable handler types")
	flatEntries := []any{
		map[string]any{"type": "command", "command": "echo ok"},
		map[string]any{"type": "prompt", "prompt": "check this"},
	}
	got = extractNonPortableHandlerTypes(flatEntries)
	assert.Empty(t, got, "no non-portable types for command+prompt")
	agentEntries := []any{
		map[string]any{"hooks": []any{map[string]any{"type": "agent", "prompt": "review this"}}},
	}
	got = extractNonPortableHandlerTypes(agentEntries)
	assert.Equal(t, []string{"agent"}, got, "agent handler type")
	got = extractNonPortableHandlerTypes(nil)
	assert.Empty(t, got, "nil input")
}

func TestCompileIR_HandlerTypeWarnings(t *testing.T) {
	layerDir := t.TempDir()
	claudeDir := filepath.Join(layerDir, ".claude")
	testutil.WriteFile(t, layerDir, "CLAUDE.md", []byte("# Handler type test"))
	settings := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read"}},
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"matcher": "Write", "hooks": []any{
					map[string]any{"type": "http", "url": "https://webhook.example.com"},
				}},
			},
		},
	}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	testutil.WriteFile(t, claudeDir, "settings.json", settingsData)
	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	var logBuf bytes.Buffer
	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(origLogger)
	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err, "Compile()")
	logOutput := logBuf.String()
	assert.True(t, strings.Contains(logOutput, "http") && strings.Contains(logOutput, "handler type"),
		"expected warning about non-portable handler type http, got:\n%s", logOutput)
}

func TestReadBinDir(t *testing.T) {
	t.Parallel()

	t.Run("with files", func(t *testing.T) {
		t.Parallel()
		layerDir := t.TempDir()

		testutil.WriteFile(t, layerDir, "bin/script1.sh", []byte("#!/bin/bash\necho one\n"))
		testutil.WriteFile(t, layerDir, "bin/subdir/script2.sh", []byte("#!/bin/bash\necho two\n"))

		ir := &LayerIR{}
		ReadBinDir(layerDir, ir)

		require.Len(t, ir.Bin, 2, "expected 2 bin entries")

		// Build a map for order-independent assertions.
		binMap := make(map[string]BinFile)
		for _, b := range ir.Bin {
			binMap[b.RelPath] = b
		}

		script1, ok := binMap["script1.sh"]
		require.True(t, ok, "expected bin entry for script1.sh")
		assert.Equal(t, []byte("#!/bin/bash\necho one\n"), script1.Content)
		assert.Zero(t, script1.Mode, "Mode should be 0 (writer defaults to 0755)")

		script2, ok := binMap["subdir/script2.sh"]
		require.True(t, ok, "expected bin entry for subdir/script2.sh")
		assert.Equal(t, []byte("#!/bin/bash\necho two\n"), script2.Content)
		assert.Zero(t, script2.Mode, "Mode should be 0 (writer defaults to 0755)")
	})

	t.Run("missing bin dir", func(t *testing.T) {
		t.Parallel()
		layerDir := t.TempDir()

		ir := &LayerIR{}
		ReadBinDir(layerDir, ir)

		assert.Nil(t, ir.Bin, "missing bin/ should leave ir.Bin nil")
	})

	t.Run("empty bin dir", func(t *testing.T) {
		t.Parallel()
		layerDir := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(layerDir, "bin"), 0755))

		ir := &LayerIR{}
		ReadBinDir(layerDir, ir)

		assert.Nil(t, ir.Bin, "empty bin/ should leave ir.Bin nil")
	})

	t.Run("bin is a file not dir", func(t *testing.T) {
		t.Parallel()
		layerDir := t.TempDir()

		// Create bin as a regular file, not a directory.
		testutil.WriteFile(t, layerDir, "bin", []byte("not a directory"))

		ir := &LayerIR{}
		ReadBinDir(layerDir, ir)

		assert.Nil(t, ir.Bin, "bin as a file (not dir) should leave ir.Bin nil")
	})
}

func TestCompileIR_HookForceLockEnforced(t *testing.T) {
	t.Parallel()
	cursorLayerDir := t.TempDir()
	testutil.WriteFile(t, cursorLayerDir, ".cursorrules", []byte("# Force-lock hook test"))
	cursorHooks := map[string]any{
		"_keel":         map[string]any{"subagentStart": map[string]any{"force": true}},
		"subagentStart": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo locked-by-cursor"}}, "force": true}},
		"preToolUse":    []any{map[string]any{"matcher": "Write", "hooks": []any{map[string]any{"type": "command", "command": "echo pre-cursor"}}}},
	}
	cursorHooksData, _ := json.MarshalIndent(cursorHooks, "", "  ")
	testutil.WriteFile(t, cursorLayerDir, "hooks.json", cursorHooksData)
	claudeLayerDir := t.TempDir()
	claudeDir := filepath.Join(claudeLayerDir, ".claude")
	claudeSettings := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read"}},
		"hooks": map[string]any{
			"SubagentStart": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "echo override-attempt"}}}},
			"PostToolUse":   []any{map[string]any{"matcher": "Write(**)", "hooks": []any{map[string]any{"type": "command", "command": "echo post-claude"}}}},
		},
	}
	claudeSettingsData, _ := json.MarshalIndent(claudeSettings, "", "  ")
	testutil.WriteFile(t, claudeDir, "settings.json", claudeSettingsData)
	layers := []config.Layer{
		{Path: cursorLayerDir, Type: "base", Format: "cursor"},
		{Path: claudeLayerDir, Type: "project", Format: "claude"},
	}

	t.Run("cursor-force-lock-enforced-compiling-to-claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		result, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err, "reading settings.json")
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings), "parsing settings.json")
		hooks, ok := settings["hooks"].(map[string]any)
		require.True(t, ok, "expected hooks key in settings.json")
		subagentStart, ok := hooks["SubagentStart"]
		require.True(t, ok, "SubagentStart hook in settings.json")
		subArr := subagentStart.([]any)
		require.Len(t, subArr, 1, "SubagentStart entries")
		subEntry := subArr[0].(map[string]any)
		subHooks := subEntry["hooks"].([]any)
		subHook := subHooks[0].(map[string]any)
		assert.Equal(t, "echo locked-by-cursor", subHook["command"], "SubagentStart hook should have original command from force-locked layer")
		assert.NotContains(t, subEntry, "force", "force field should be stripped from hook entries in output")
		postToolUse, ok := hooks["PostToolUse"]
		require.True(t, ok, "PostToolUse hook from Layer 2")
		postArr := postToolUse.([]any)
		require.Len(t, postArr, 1, "PostToolUse entries")
		preToolUse, ok := hooks["PreToolUse"]
		require.True(t, ok, "PreToolUse hook from Layer 1")
		preArr := preToolUse.([]any)
		require.Len(t, preArr, 1, "PreToolUse entries")
		foundLock := false
		for _, lock := range result.Governance.ForceLocked {
			if lock.Category == "hook" && lock.Key == "SubagentStart" {
				foundLock = true
				break
			}
		}
		assert.True(t, foundLock, "force-lock record for hook SubagentStart")
	})

	t.Run("cursor-force-lock-enforced-compiling-to-cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile()")
		hooksPath := filepath.Join(outputDir, ".cursor", "hooks.json")
		data, err := os.ReadFile(hooksPath)
		require.NoError(t, err, "reading hooks.json")
		var hooks map[string]any
		require.NoError(t, json.Unmarshal(data, &hooks), "parsing hooks.json")
		subagentStart, ok := hooks["subagentStart"]
		require.True(t, ok, "subagentStart hook in hooks.json")
		subArr := subagentStart.([]any)
		require.Len(t, subArr, 1, "subagentStart entries")
		subEntry := subArr[0].(map[string]any)
		subHooks := subEntry["hooks"].([]any)
		subHook := subHooks[0].(map[string]any)
		assert.Equal(t, "echo locked-by-cursor", subHook["command"], "subagentStart hook should have original command from force-locked layer")
		postToolUse, ok := hooks["postToolUse"]
		require.True(t, ok, "postToolUse hook from Layer 2 (via IR path)")
		postArr := postToolUse.([]any)
		require.Len(t, postArr, 1, "postToolUse entries")
	})
}

