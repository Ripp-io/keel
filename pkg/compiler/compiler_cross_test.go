// compiler_cross_test.go — cross-adapter tests that exercise behavior
// common to multiple agents: single/empty/zero-layer compilation, manifest
// generation, native passthrough, unknown adapter errors, instruction
// candidate ordering, AGENTS.md fallback, skill companion files, and
// cross-agent transpilation of portable concepts (attribution, sandbox).
package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// --- AGENTS.md fallback tests ---

func TestCompile_AGENTSmdInstructionFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "agents_only")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte("agents-based instructions"), 0644))

	// A layer with only AGENTS.md is auto-detected as agentsmd format.
	// Cross-compile it to Claude and Cursor to verify instructions transfer.
	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		layers := []config.Layer{{Path: layerDir, Type: "project", Format: "agentsmd"}}
		outputDir := t.TempDir()

		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err, "reading CLAUDE.md")
		assert.Contains(t, string(content), "agents-based instructions")
	})

	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		layers := []config.Layer{{Path: layerDir, Type: "project", Format: "agentsmd"}}
		outputDir := t.TempDir()

		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
		require.NoError(t, err, "reading 00-keel.mdc")
		assert.Contains(t, string(content), "agents-based instructions")
	})
}

func TestCompile_AGENTSmdLowerPriorityThanNative(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Explicit keel.yml declares only instructions.md — AGENTS.md excluded.
	// Without a keel.yml, defaults include both instructions.md and instructions/,
	// but not AGENTS.md (that's a separate format with its own reader).
	layerDir := filepath.Join(dir, "both")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("native instructions"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "AGENTS.md"), []byte("agents fallback"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\ninstructions:\n  - source: instructions.md\n"), 0644))

	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
		outputDir := t.TempDir()

		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "native instructions")
		assert.NotContains(t, string(content), "agents fallback")
	})

	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
		outputDir := t.TempDir()

		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "native instructions")
		assert.NotContains(t, string(content), "agents fallback")
	})
}

// --- Skill companion file tests ---

func TestCompile_SkillCompanionFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	skillDir := filepath.Join(layerDir, "skills", "security-review")
	scriptsDir := filepath.Join(skillDir, "scripts")
	refsDir := filepath.Join(skillDir, "references")

	for _, d := range []string{scriptsDir, refsDir} {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	keelYML := []byte("schema: v1\nskills:\n  - source: skills/\n")
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), keelYML, 0644))

	skillMD := []byte("---\nname: security-review\ndescription: Review code for security vulnerabilities\nforce: true\n---\nPerform a thorough security review.\n")
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skillMD, 0644))

	scriptContent := []byte("#!/bin/bash\necho 'Scanning for vulnerabilities...'\n")
	require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "check-vulnerabilities.sh"), scriptContent, 0644))

	refContent := []byte("# OWASP Top 10\n\nReference guide for security review.\n")
	require.NoError(t, os.WriteFile(filepath.Join(refsDir, "OWASP-TOP10.md"), refContent, 0644))

	templateContent := []byte("# Security report template\nstatus: pending\nfindings: []\n")
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "template.yaml"), templateContent, 0644))

	for _, agent := range []string{"claude", "cursor"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			layers := []config.Layer{{Path: layerDir, Type: "team", Format: config.FormatKeel}}
			outputDir := t.TempDir()

			_, err := Compile(agent, layers, outputDir, CompileOptions{})
			require.NoError(t, err)

			var agentDir string
			switch agent {
			case "claude":
				agentDir = ".claude"
			case "cursor":
				agentDir = ".cursor"
			}

			skillsBase := filepath.Join(outputDir, agentDir, "skills", "security-review")

			// 1. SKILL.md exists with frontmatter stripped of Keel governance fields
			skillData, err := os.ReadFile(filepath.Join(skillsBase, "SKILL.md"))
			require.NoError(t, err, "SKILL.md not found")
			skillText := string(skillData)
			assert.Contains(t, skillText, "name: security-review")
			assert.Contains(t, skillText, "description: Review code for security vulnerabilities")
			assert.NotContains(t, skillText, "force:")
			assert.Contains(t, skillText, "Perform a thorough security review.")

			// 2. scripts/check-vulnerabilities.sh
			scriptData, err := os.ReadFile(filepath.Join(skillsBase, "scripts", "check-vulnerabilities.sh"))
			require.NoError(t, err, "scripts/check-vulnerabilities.sh not found")
			assert.Equal(t, string(scriptContent), string(scriptData))

			// 3. references/OWASP-TOP10.md
			refData, err := os.ReadFile(filepath.Join(skillsBase, "references", "OWASP-TOP10.md"))
			require.NoError(t, err, "references/OWASP-TOP10.md not found")
			assert.Equal(t, string(refContent), string(refData))

			// 4. template.yaml
			tmplData, err := os.ReadFile(filepath.Join(skillsBase, "template.yaml"))
			require.NoError(t, err, "template.yaml not found")
			assert.Equal(t, string(templateContent), string(tmplData))

			// 5. Verify directory structure
			for _, subdir := range []string{"scripts", "references"} {
				info, err := os.Stat(filepath.Join(skillsBase, subdir))
				require.NoError(t, err, "expected subdirectory %q to exist", subdir)
				assert.True(t, info.IsDir(), "expected %q to be a directory", subdir)
			}
		})
	}
}

// --- Cross-adapter tests ---

func TestCompile_SingleLayer(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "cursor"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			layers := buildTestLayers(t, "layers", "base")
			outputDir := t.TempDir()

			result, err := Compile(agent, layers, outputDir, CompileOptions{})
			require.NoError(t, err)
			assert.Equal(t, agent, result.Output.Agent)
			assert.NotEmpty(t, result.Output.Files, "expected at least one file record")
		})
	}
}

func TestCompile_EmptyLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	emptyLayer := filepath.Join(dir, "empty")
	require.NoError(t, os.MkdirAll(emptyLayer, 0755))

	for _, agent := range []string{"claude", "cursor"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			layers := []config.Layer{{Path: emptyLayer, Type: "project", Format: config.FormatKeel}}
			outputDir := t.TempDir()

			_, err := Compile(agent, layers, outputDir, CompileOptions{})
			assert.NoError(t, err, "Compile(%s) with empty layer should not error", agent)
		})
	}
}

func TestCompile_ManifestWritten(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base", "org")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err, "reading manifest")

	content := string(data)
	for _, key := range []string{`"agent"`, `"claude"`, `"layers"`, `"file_ops"`, `"compiled_at"`, `"keel_version"`} {
		assert.Contains(t, content, key, "expected manifest to contain %s", key)
	}
}

func TestCompile_ManifestLayerPaths(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base", "org")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)

	content := string(data)
	for _, layer := range layers {
		assert.Contains(t, content, layer.Path, "expected manifest to reference layer path")
	}
}

func TestCompile_ToolNativePassthrough(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "native_layer")
	nativeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(nativeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nativeDir, "custom.json"), []byte(`{"custom": true}`), 0644))

	// Layer has .claude/ directory — auto-detected as claude format.
	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "claude"}}
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "custom.json"))
	require.NoError(t, err, "expected native .claude/custom.json to be copied")
	assert.Contains(t, string(data), `"custom": true`)

	// Now expects per-file record, not directory-level record
	found := false
	for _, f := range result.Output.Files {
		if f.Output == ".claude/custom.json" && f.Strategy == "overlay" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected file record for .claude/custom.json")
}

func TestCompile_CursorNativePassthrough(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "native_layer")
	nativeDir := filepath.Join(layerDir, ".cursor")
	require.NoError(t, os.MkdirAll(nativeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nativeDir, "settings.json"), []byte(`{"key": "value"}`), 0644))

	// Layer has .cursor/ directory — auto-detected as cursor format.
	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "cursor"}}
	outputDir := t.TempDir()

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "settings.json"))
	require.NoError(t, err, "expected native .cursor/settings.json to be copied")
	assert.Contains(t, string(data), `"key": "value"`)

	// Now expects per-file record, not directory-level record
	found := false
	for _, f := range result.Output.Files {
		if f.Output == ".cursor/settings.json" && f.Strategy == "overlay" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected file record for .cursor/settings.json")
}

func TestCompile_UnknownAdapter(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base")
	outputDir := t.TempDir()

	_, err := Compile("unknown", layers, outputDir, CompileOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown agent")
}

func TestCompile_NoLayers(t *testing.T) {
	t.Parallel()
	for _, agent := range []string{"claude", "cursor"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			outputDir := t.TempDir()
			_, err := Compile(agent, nil, outputDir, CompileOptions{})
			assert.NoError(t, err, "Compile(%s) with no layers should not error", agent)
		})
	}
}

func TestCompile_InstructionCandidateOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "multi_instructions")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("primary"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "CLAUDE.md"), []byte("secondary"), 0644))

	// CLAUDE.md is a marker file — auto-detected as claude format.
	// Claude native reader picks instructions.md first (higher priority candidate).
	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "primary")
	assert.NotContains(t, string(content), "secondary")
}

// --- Manifest version tests ---

func TestCompile_ManifestVersionFromEnv(t *testing.T) {
	layers := buildTestLayers(t, "layers", "base")
	outputDir := t.TempDir()

	t.Setenv("KEEL_BUILD", "1.2.3")

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"keel_version": "1.2.3"`)
}

func TestCompile_ManifestVersionDefault(t *testing.T) {
	t.Setenv("KEEL_BUILD", "")

	layers := buildTestLayers(t, "layers", "base")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", ".keel.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"keel_version": "dev"`)
}

func TestWriteKeelState_Minimal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	agentDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(agentDir, 0755))

	result := &CompileResult{
		Output: CompileOutput{
			Agent:      "claude",
			OutputRoot: ".claude",
		},
	}
	err := writeKeelState(result, nil, dir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(agentDir, ".keel.json"))
	require.NoError(t, err, "reading .keel.json")
	assert.Contains(t, string(data), `"agent": "claude"`)
}

// --- Zero layers compilation ---

func TestCompile_ZeroLayers(t *testing.T) {
	t.Parallel()
	t.Run("nil layers", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()

		existingContent := "# Existing Config\nDo not overwrite me."
		require.NoError(t, os.WriteFile(filepath.Join(outputDir, "CLAUDE.md"), []byte(existingContent), 0644))

		result, err := Compile("claude", nil, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile() with nil layers should not error")
		assert.Empty(t, result.Output.Files, "expected 0 file records from nil layers")

		got, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err, "pre-existing CLAUDE.md was deleted")
		assert.Equal(t, existingContent, string(got), "CLAUDE.md was modified by zero-layer compile")

		for _, file := range []string{".claude/settings.json", ".mcp.json"} {
			_, err := os.Stat(filepath.Join(outputDir, file))
			assert.True(t, os.IsNotExist(err), "zero-layer compile should not produce %s", file)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()

		existingContent := "# Keep me intact"
		require.NoError(t, os.WriteFile(filepath.Join(outputDir, "CLAUDE.md"), []byte(existingContent), 0644))

		result, err := Compile("claude", []config.Layer{}, outputDir, CompileOptions{})
		require.NoError(t, err, "Compile() with empty slice should not error")
		assert.Empty(t, result.Output.Files, "expected 0 file records from empty layers")

		got, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err, "pre-existing CLAUDE.md was deleted")
		assert.Equal(t, existingContent, string(got), "CLAUDE.md was modified by zero-layer compile")
	})

	t.Run("result metadata", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()

		result, err := Compile("claude", nil, outputDir, CompileOptions{})
		require.NoError(t, err)
		assert.Equal(t, "claude", result.Output.Agent)
		assert.Equal(t, ".claude", result.Output.OutputRoot)
	})
}

// --- Cross-agent transpilation: portable attribution and sandbox ---

func TestCrossCompile_Claude_To_Cursor_Attribution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a Claude native layer with attribution in settings.json.
	layerDir := filepath.Join(dir, "claude-layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	settings := map[string]any{
		"attribution": map[string]any{
			"commit": "Co-Authored-By: Claude",
			"pr":     "Generated by Claude Code",
		},
	}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	testutil.WriteFile(t, claudeDir, "settings.json", settingsData)

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Cursor output: attribution should be in cli-config.json as boolean flags.
	cliConfigPath := filepath.Join(outputDir, ".cursor", "cli-config.json")
	data, err := os.ReadFile(cliConfigPath)
	require.NoError(t, err, "reading cli-config.json")

	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(data, &cliConfig))

	attr, ok := cliConfig["attribution"].(map[string]any)
	require.True(t, ok, "expected attribution to be a map in Cursor output, got %T", cliConfig["attribution"])
	assert.Equal(t, true, attr["attributeCommitsToAgent"])
	assert.Equal(t, true, attr["attributePRsToAgent"])
}

func TestCrossCompile_Cursor_To_Claude_Attribution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a Cursor native layer with boolean attribution.
	layerDir := filepath.Join(dir, "cursor-layer")
	cursorDir := filepath.Join(layerDir, ".cursor")
	cliConfig := map[string]any{
		"version": float64(1),
		"attribution": map[string]any{
			"attributeCommitsToAgent": true,
			"attributePRsToAgent":     true,
		},
	}
	cliConfigData, _ := json.MarshalIndent(cliConfig, "", "  ")
	testutil.WriteFile(t, cursorDir, "cli-config.json", cliConfigData)

	layers := []config.Layer{{Path: layerDir, Format: "cursor"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Claude output: attribution should be in settings.json with template strings.
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	attr, ok := settings["attribution"].(map[string]any)
	require.True(t, ok, "expected attribution to be a map in Claude output, got %T", settings["attribution"])

	// Cursor booleans become empty template strings (enabled but no template).
	// The Claude writer always writes commit and pr keys.
	_, hasCommit := attr["commit"]
	assert.True(t, hasCommit, "expected 'commit' key in Claude attribution output")
	_, hasPR := attr["pr"]
	assert.True(t, hasPR, "expected 'pr' key in Claude attribution output")
}

func TestCrossCompile_Claude_To_Cursor_Sandbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a Claude native layer with sandbox in settings.json.
	layerDir := filepath.Join(dir, "claude-layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	settings := map[string]any{
		"sandbox": map[string]any{
			"enabled":                  true,
			"autoAllowBashIfSandboxed": true,
			"excludedCommands":         []string{"rm -rf /"},
			"filesystem": map[string]any{
				"allowWrite": []string{"/tmp"},
			},
			"network": map[string]any{
				"allowedDomains": []string{"api.example.com"},
				"deniedDomains":  []string{"evil.com"},
			},
		},
	}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	testutil.WriteFile(t, claudeDir, "settings.json", settingsData)

	layers := []config.Layer{{Path: layerDir, Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Cursor output: sandbox should be in sandbox.json, not in cli-config.json.
	sandboxPath := filepath.Join(outputDir, ".cursor", "sandbox.json")
	data, err := os.ReadFile(sandboxPath)
	require.NoError(t, err, "reading sandbox.json")

	var sandbox map[string]any
	require.NoError(t, json.Unmarshal(data, &sandbox))

	// filesystem.allowWrite maps to additionalReadwritePaths in Cursor.
	rwPaths, ok := sandbox["additionalReadwritePaths"].([]any)
	require.True(t, ok, "expected additionalReadwritePaths in Cursor sandbox output")
	assert.Contains(t, rwPaths, "/tmp")

	// network maps to networkPolicy in Cursor.
	np, ok := sandbox["networkPolicy"].(map[string]any)
	require.True(t, ok, "expected networkPolicy in Cursor sandbox output")
	allow, ok := np["allow"].([]any)
	require.True(t, ok)
	assert.Contains(t, allow, "api.example.com")
	deny, ok := np["deny"].([]any)
	require.True(t, ok)
	assert.Contains(t, deny, "evil.com")

	// Verify sandbox is NOT in cli-config.json.
	cliConfigPath := filepath.Join(outputDir, ".cursor", "cli-config.json")
	if cliData, readErr := os.ReadFile(cliConfigPath); readErr == nil {
		var cliConfig map[string]any
		if jsonErr := json.Unmarshal(cliData, &cliConfig); jsonErr == nil {
			assert.NotContains(t, cliConfig, "sandbox",
				"sandbox should be in sandbox.json, not cli-config.json")
		}
	}
}

func TestCrossCompile_Cursor_To_Claude_Sandbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a Cursor native layer with sandbox.json.
	layerDir := filepath.Join(dir, "cursor-layer")
	cursorDir := filepath.Join(layerDir, ".cursor")
	sandbox := map[string]any{
		"type":                     "docker",
		"additionalReadwritePaths": []string{"/var/data"},
		"additionalReadonlyPaths":  []string{"/etc/config"},
		"networkPolicy": map[string]any{
			"default": "deny",
			"allow":   []string{"registry.example.com"},
			"deny":    []string{"malware.example.com"},
		},
	}
	sandboxData, _ := json.MarshalIndent(sandbox, "", "  ")
	testutil.WriteFile(t, cursorDir, "sandbox.json", sandboxData)

	layers := []config.Layer{{Path: layerDir, Format: "cursor"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Claude output: sandbox should be embedded in settings.json.
	settingsPath := filepath.Join(outputDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	sb, ok := settings["sandbox"].(map[string]any)
	require.True(t, ok, "expected sandbox in Claude settings output")

	// Network policy should be translated to Claude format.
	net, ok := sb["network"].(map[string]any)
	require.True(t, ok, "expected sandbox.network in Claude output")
	allowDomains, ok := net["allowedDomains"].([]any)
	require.True(t, ok, "expected allowedDomains in sandbox.network")
	assert.Contains(t, allowDomains, "registry.example.com")
	denyDomains, ok := net["deniedDomains"].([]any)
	require.True(t, ok, "expected deniedDomains in sandbox.network")
	assert.Contains(t, denyDomains, "malware.example.com")
}

func TestCrossCompile_DSL_To_Both_Attribution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a DSL layer with attribution.
	layerDir := filepath.Join(dir, "dsl-layer")
	keelYML := `schema: v1
settings:
  attribution:
    commit: "Co-Authored-By: AI"
    pr: "PR by AI"
`
	testutil.WriteFile(t, layerDir, "keel.yml", []byte(keelYML))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel}}

	t.Run("to-claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))

		attr, ok := settings["attribution"].(map[string]any)
		require.True(t, ok, "expected attribution in Claude settings")
		assert.Equal(t, "Co-Authored-By: AI", attr["commit"])
		assert.Equal(t, "PR by AI", attr["pr"])
	})

	t.Run("to-cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
		require.NoError(t, err)
		var cliConfig map[string]any
		require.NoError(t, json.Unmarshal(data, &cliConfig))

		attr, ok := cliConfig["attribution"].(map[string]any)
		require.True(t, ok, "expected attribution in Cursor cli-config.json")
		assert.Equal(t, true, attr["attributeCommitsToAgent"])
		assert.Equal(t, true, attr["attributePRsToAgent"])
	})
}

func TestCrossCompile_DSL_To_Both_Sandbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a DSL layer with sandbox config.
	layerDir := filepath.Join(dir, "dsl-layer")
	keelYML := `schema: v1
settings:
  sandbox:
    enabled: true
    filesystem:
      allow-write:
        - "/tmp"
    network:
      allowed-domains:
        - "api.example.com"
    type: docker
    additional-readwrite-paths:
      - "/var/data"
`
	testutil.WriteFile(t, layerDir, "keel.yml", []byte(keelYML))

	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel}}

	t.Run("to-claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))

		sb, ok := settings["sandbox"].(map[string]any)
		require.True(t, ok, "expected sandbox in Claude settings")
		assert.Equal(t, true, sb["enabled"])

		fs, ok := sb["filesystem"].(map[string]any)
		require.True(t, ok, "expected sandbox.filesystem")
		aw, ok := fs["allowWrite"].([]any)
		require.True(t, ok)
		assert.Contains(t, aw, "/tmp")

		// Cursor-only field "type" should NOT appear in Claude output
		// (json:"-" tag excludes it from json.Marshal).
		assert.NotContains(t, sb, "type", "Cursor-only 'type' should not leak into Claude output")
	})

	t.Run("to-cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "sandbox.json"))
		require.NoError(t, err)
		var sandbox map[string]any
		require.NoError(t, json.Unmarshal(data, &sandbox))

		// Cursor-only fields
		assert.Equal(t, "docker", sandbox["type"])

		// filesystem.allow-write -> additionalReadwritePaths (combined with direct paths)
		rwPaths, ok := sandbox["additionalReadwritePaths"].([]any)
		require.True(t, ok, "expected additionalReadwritePaths")
		assert.Contains(t, rwPaths, "/var/data")
		assert.Contains(t, rwPaths, "/tmp") // from filesystem.allow-write

		// network -> networkPolicy
		np, ok := sandbox["networkPolicy"].(map[string]any)
		require.True(t, ok, "expected networkPolicy")
		allow, ok := np["allow"].([]any)
		require.True(t, ok)
		assert.Contains(t, allow, "api.example.com")
	})
}

// TestCompile_DoubleNestingGuard tests that outputDir cannot be set to an
// agent's dot-directory (e.g., ".claude"), which would create nested output
// like ".claude/.claude/settings.json".
func TestCompile_DoubleNestingGuard(t *testing.T) {
	t.Parallel()

	layerDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\n"), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}

	// Attempt to compile with outputDir=".claude" — should error
	outputDir := t.TempDir()
	claudeDir := filepath.Join(outputDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	_, err := Compile("claude", layers, claudeDir, CompileOptions{})
	require.Error(t, err, "expected error when outputDir is .claude")
	assert.Contains(t, err.Error(), "nested", "error message should mention 'nested'")
}
