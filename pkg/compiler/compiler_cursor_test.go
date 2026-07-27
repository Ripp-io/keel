// compiler_cursor_test.go — Cursor-specific compilation tests covering
// full enterprise stack compilation, MDC frontmatter, CLI config merge,
// hooks handling, rules/commands/skills/agents overlay, MCP copying,
// .cursorrules fallback, and native settings detection for the Cursor adapter.
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
)

// --- Cursor compiler tests ---

func TestCompileCursor_FullEnterpriseStack(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, "cursor", result.Output.Agent)

	// Check 00-keel.mdc instructions
	assertFileMatchesExpected(t, outputDir, ".cursor/rules/00-keel.mdc",
		filepath.Join(td, "expected", "cursor", ".cursor", "rules", "00-keel.mdc"))

	// Check mcp.json
	assertFileMatchesExpected(t, outputDir, ".cursor/mcp.json",
		filepath.Join(td, "expected", "cursor", ".cursor", "mcp.json"))

	// Check commands
	assertFileMatchesExpected(t, outputDir, ".cursor/commands/deploy-staging.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "commands", "deploy-staging.md"))
	assertFileMatchesExpected(t, outputDir, ".cursor/commands/run-tests.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "commands", "run-tests.md"))

	// Check skills
	assertFileMatchesExpected(t, outputDir, ".cursor/skills/security-review/SKILL.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "skills", "security-review", "SKILL.md"))
	assertFileMatchesExpected(t, outputDir, ".cursor/skills/api-design/SKILL.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "skills", "api-design", "SKILL.md"))

	// Check agents
	assertFileMatchesExpected(t, outputDir, ".cursor/agents/compliance-checker.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "agents", "compliance-checker.md"))
	assertFileMatchesExpected(t, outputDir, ".cursor/agents/integration-tester.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "agents", "integration-tester.md"))
}

func TestCompileCursor_MdcFrontmatter(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
	require.NoError(t, err)
	text := string(content)

	assert.True(t, strings.HasPrefix(text, "---\n"), "expected .mdc file to start with frontmatter")
	assert.Contains(t, text, "alwaysApply: true")
	assert.Contains(t, text, `description: "Keel workspace instructions`)
}

func TestCompileCursor_NoCLIConfig(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base", "org")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Cursor always writes cli-config.json with at least version: 1
	// When layers have only Claude-specific settings (settings.json),
	// Cursor compilation produces only the version field.
	cliConfig := filepath.Join(outputDir, ".cursor", "cli-config.json")
	data, err := os.ReadFile(cliConfig)
	require.NoError(t, err, "cli-config.json should always be written for Cursor")

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.Equal(t, float64(1), cfg["version"], "version should always be present")
	assert.Len(t, cfg, 1, "only version should be present when no Cursor-specific data")
}

func TestCompileCursor_McpCopied(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))

	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers key in mcp.json")

	for _, server := range []string{"sonarqube", "jira", "gitlab", "obsidian"} {
		assert.Contains(t, servers, server, "expected %s server in mcp.json", server)
	}
}

func TestCompileCursor_NoHooksFile(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base", "org")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	hooksPath := filepath.Join(outputDir, ".cursor", "hooks.json")
	_, err = os.Stat(hooksPath)
	assert.True(t, os.IsNotExist(err), "expected no hooks.json when no layer provides one")
}

// --- Cursor feature coverage tests ---

func TestCompileCursor_CLIConfigMerged(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")

	var cliCfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cliCfg))

	// Version should be 1
	v, ok := cliCfg["version"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(1), v)

	// permissions.allow should have project's array
	permsAllow := cliCfg["permissions.allow"].([]any)
	expectedPerms := []string{"Read(*)", "Edit(*)", "Write(*)", "Bash(git:*)", "Bash(npm:*)"}
	require.Len(t, permsAllow, len(expectedPerms))
	for i, expected := range expectedPerms {
		assert.Equal(t, expected, permsAllow[i], "permission[%d]", i)
	}

	// Verify against golden file
	assertJSONMatchesExpected(t, outputDir, ".cursor/cli-config.json",
		filepath.Join(td, "expected", "cursor", ".cursor", "cli-config.json"))
}

func TestCompileCursor_HooksDeepMerged(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "hooks.json"))
	require.NoError(t, err, "reading hooks.json")

	var hooks map[string]any
	require.NoError(t, json.Unmarshal(data, &hooks))

	// Additive merge deduplicates identical entries -> 1
	preToolUse, ok := hooks["preToolUse"]
	require.True(t, ok, "expected preToolUse in hooks.json")
	preArr := preToolUse.([]any)
	assert.Len(t, preArr, 1, "expected 1 preToolUse hook (deduplicated)")

	// postToolUse from solution
	postToolUse, ok := hooks["postToolUse"]
	require.True(t, ok, "expected postToolUse in hooks.json")
	postArr := postToolUse.([]any)
	assert.Len(t, postArr, 1)

	// Verify strategy is deep-merge in file records
	foundDeepMerge := false
	for _, f := range result.Output.Files {
		if f.Output == ".cursor/hooks.json" {
			assert.Equal(t, "deep-merge", f.Strategy)
			foundDeepMerge = true
		}
	}
	assert.True(t, foundDeepMerge, "expected file record for .cursor/hooks.json")

	assertJSONMatchesExpected(t, outputDir, ".cursor/hooks.json",
		filepath.Join(td, "expected", "cursor", ".cursor", "hooks.json"))
}

func TestCompileCursor_HooksConcatenated(t *testing.T) {
	t.Parallel()
	tmpLayers := t.TempDir()

	companyDir := filepath.Join(tmpLayers, "company")
	os.MkdirAll(companyDir, 0755)
	os.WriteFile(filepath.Join(companyDir, "hooks.json"), []byte(`{
		"preToolUse": [
			{"matcher": "Write(**/secrets/**)", "hooks": [{"type": "command", "command": "bash coordination-start.sh"}]}
		]
	}`), 0644)

	solutionDir := filepath.Join(tmpLayers, "solution")
	os.MkdirAll(solutionDir, 0755)
	os.WriteFile(filepath.Join(solutionDir, "hooks.json"), []byte(`{
		"preToolUse": [
			{"matcher": "Write(**/logs/**)", "hooks": [{"type": "command", "command": "bash evolution-start.sh"}]}
		]
	}`), 0644)

	layers := []config.Layer{
		{Path: companyDir, Format: "cursor"},
		{Path: solutionDir, Format: "cursor"},
	}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "hooks.json"))
	require.NoError(t, err, "reading hooks.json")

	var hooks map[string]any
	require.NoError(t, json.Unmarshal(data, &hooks))

	preToolUse, ok := hooks["preToolUse"]
	require.True(t, ok, "expected preToolUse in hooks.json")
	preArr, ok := preToolUse.([]any)
	require.True(t, ok, "expected preToolUse to be an array")
	assert.Len(t, preArr, 2, "expected 2 preToolUse hooks (concatenated)")
}

func TestCompileCursor_RulesOverlay(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	rulesDir := filepath.Join(outputDir, ".cursor", "rules")

	expectedRules := map[string]string{
		"pci-compliance.mdc":       "org",
		"conventional-commits.mdc": "team",
	}

	for rule, layer := range expectedRules {
		_, err := os.Stat(filepath.Join(rulesDir, rule))
		assert.NoError(t, err, "expected rule file %q (from %s layer) to exist", rule, layer)
	}

	assertFileMatchesExpected(t, outputDir, ".cursor/rules/pci-compliance.mdc",
		filepath.Join(td, "expected", "cursor", ".cursor", "rules", "pci-compliance.mdc"))
	assertFileMatchesExpected(t, outputDir, ".cursor/rules/conventional-commits.mdc",
		filepath.Join(td, "expected", "cursor", ".cursor", "rules", "conventional-commits.mdc"))

	_, err = os.Stat(filepath.Join(rulesDir, "00-keel.mdc"))
	assert.NoError(t, err, "expected 00-keel.mdc to exist")
}

func TestCompileCursor_CommandsOverlay(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	expectedCommands := map[string]string{
		"deploy-staging.md": "team",
		"run-tests.md":      "project",
	}

	commandsDir := filepath.Join(outputDir, ".cursor", "commands")
	for cmd, layer := range expectedCommands {
		_, err := os.Stat(filepath.Join(commandsDir, cmd))
		assert.NoError(t, err, "expected command %q (from %s layer) to exist", cmd, layer)
	}

	assertFileMatchesExpected(t, outputDir, ".cursor/commands/deploy-staging.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "commands", "deploy-staging.md"))
	assertFileMatchesExpected(t, outputDir, ".cursor/commands/run-tests.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "commands", "run-tests.md"))
}

func TestCompileCursor_SkillsOverlay(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	expectedSkills := map[string]string{
		"security-review/SKILL.md": "team",
		"api-design/SKILL.md":      "project",
	}

	skillsDir := filepath.Join(outputDir, ".cursor", "skills")
	for skill, layer := range expectedSkills {
		_, err := os.Stat(filepath.Join(skillsDir, skill))
		assert.NoError(t, err, "expected skill %q (from %s layer) to exist", skill, layer)
	}

	assertFileMatchesExpected(t, outputDir, ".cursor/skills/security-review/SKILL.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "skills", "security-review", "SKILL.md"))
	assertFileMatchesExpected(t, outputDir, ".cursor/skills/api-design/SKILL.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "skills", "api-design", "SKILL.md"))

	companionFiles := []string{
		".cursor/skills/security-review/scripts/check-vulnerabilities.sh",
		".cursor/skills/security-review/references/OWASP-TOP10.md",
		".cursor/skills/security-review/template.yaml",
	}
	for _, f := range companionFiles {
		assertFileMatchesExpected(t, outputDir, f,
			filepath.Join(td, "expected", "cursor", f))
	}
}

func TestCompileCursor_AgentsOverlay(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	expectedAgents := map[string]string{
		"compliance-checker.md": "org",
		"integration-tester.md": "project",
	}

	agentsDir := filepath.Join(outputDir, ".cursor", "agents")
	for agent, layer := range expectedAgents {
		_, err := os.Stat(filepath.Join(agentsDir, agent))
		assert.NoError(t, err, "expected agent %q (from %s layer) to exist", agent, layer)
	}

	assertFileMatchesExpected(t, outputDir, ".cursor/agents/compliance-checker.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "agents", "compliance-checker.md"))
	assertFileMatchesExpected(t, outputDir, ".cursor/agents/integration-tester.md",
		filepath.Join(td, "expected", "cursor", ".cursor", "agents", "integration-tester.md"))
}

func TestCompileCursor_NoEmptyDirectories(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", "base")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	for _, dir := range []string{"skills", "agents"} {
		dirPath := filepath.Join(outputDir, ".cursor", dir)
		_, err := os.Stat(dirPath)
		assert.True(t, os.IsNotExist(err),
			"expected .cursor/%s to not exist when no layer provides %s", dir, dir)
	}
}

// --- Cursor native settings detection ---

func TestCompileCursor_NativeSettingsCursorJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	cursorDir := filepath.Join(layerDir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))

	settingsContent := `{"version":1,"custom_key":"from-convention-name"}`
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "settings.cursor.json"), []byte(settingsContent), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "cursor"}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "expected .cursor/cli-config.json to exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "from-convention-name", settings["custom_key"])
}

func TestCompileCursor_NativeCLIConfigTakesPriority(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	cursorDir := filepath.Join(layerDir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "cli-config.json"),
		[]byte(`{"version":1,"source":"tool-native"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "settings.cursor.json"),
		[]byte(`{"version":1,"source":"convention"}`), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "cursor"}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "tool-native", settings["source"],
		"cli-config.json should take priority")
}

func TestCompileCursor_NativeSettingsConventionNotLeakedViaOverlay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	cursorDir := filepath.Join(layerDir, ".cursor")
	require.NoError(t, os.MkdirAll(cursorDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cursorDir, "settings.cursor.json"),
		[]byte(`{"version":1,"model":"sonnet"}`), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "cursor"}}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "settings.cursor.json"))
	assert.True(t, os.IsNotExist(err),
		"settings.cursor.json should NOT leak through to output via overlay")
}
