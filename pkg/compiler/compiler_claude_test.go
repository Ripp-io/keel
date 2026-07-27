// compiler_claude_test.go — Claude-specific compilation tests covering
// full enterprise stack compilation, instruction rendering, settings deep
// merge, hooks, permissions, agents/skills/commands overlay, MCP handling,
// and native settings detection for the Claude adapter.
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

// --- Claude compiler tests ---

func TestCompileClaude_FullEnterpriseStack(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, "claude", result.Output.Agent)

	// Check CLAUDE.md matches expected
	assertFileMatchesExpected(t, outputDir, "CLAUDE.md",
		filepath.Join(td, "expected", "claude", "CLAUDE.md"))

	// Check settings.json (deep-merged)
	assertJSONMatchesExpected(t, outputDir, ".claude/settings.json",
		filepath.Join(td, "expected", "claude", ".claude", "settings.json"))

	// Check all 9 command files
	expectedCommands := []string{
		"project-status.md", "security-scan.md", "compliance-check.md",
		"data-validate.md", "deploy.md", "deploy-staging.md",
		"build-all.md", "run-tests.md", "my-shortcuts.md",
	}
	for _, cmd := range expectedCommands {
		assertFileMatchesExpected(t, outputDir, ".claude/commands/"+cmd,
			filepath.Join(td, "expected", "claude", ".claude", "commands", cmd))
	}

	// Check all 5 agent files
	expectedAgents := []string{
		"code-reviewer.md", "compliance-checker.md", "builder.md",
		"qa-engineer.md", "integration-tester.md",
	}
	for _, agent := range expectedAgents {
		assertFileMatchesExpected(t, outputDir, ".claude/agents/"+agent,
			filepath.Join(td, "expected", "claude", ".claude", "agents", agent))
	}
}

func TestCompileClaude_InstructionSectionHeaders(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	text := string(content)

	// Verify section headers for layers with instructions (base through project)
	for _, layer := range []string{"base", "company", "org", "solution", "team", "project"} {
		header := "# --- " + layer + " ---"
		assert.Contains(t, text, header, "expected CLAUDE.md to contain header %q", header)
	}

	// User has no instructions.md, should not have a header
	assert.NotContains(t, text, "# --- user ---")
}

func TestCompileClaude_EachLayerContributes(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Each layer contributes command files — verify all 9 exist
	expectedCommands := map[string]string{
		"project-status.md":   "base",
		"security-scan.md":    "company",
		"compliance-check.md": "org",
		"data-validate.md":    "solution",
		"deploy.md":           "team",
		"deploy-staging.md":   "team",
		"build-all.md":        "project",
		"run-tests.md":        "project",
		"my-shortcuts.md":     "user",
	}

	commandsDir := filepath.Join(outputDir, ".claude", "commands")
	for cmd, layer := range expectedCommands {
		path := filepath.Join(commandsDir, cmd)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected command %q (from %s layer) to exist", cmd, layer)
	}

	// Verify no extra command files
	entries, err := os.ReadDir(commandsDir)
	require.NoError(t, err, "reading commands dir")
	assert.Len(t, entries, 9, "expected exactly 9 command files")
}

func TestCompileClaude_SettingsDeepMerge(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify user's permissions.allow array wins (last-writer-wins for arrays)
	perms := settings["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	// User has 7 entries
	assert.Len(t, allow, 7, "expected 7 permissions (user's array)")

	// Verify solution's hooks survive (different key path from permissions)
	hooks, ok := settings["hooks"]
	require.True(t, ok, "expected hooks to survive merge (from solution layer)")
	hooksMap := hooks.(map[string]any)

	// PreToolUse from solution (overwrote company's)
	preToolUse, ok := hooksMap["PreToolUse"]
	require.True(t, ok, "expected PreToolUse hook to survive merge")
	preArr := preToolUse.([]any)
	assert.Len(t, preArr, 1)

	// PostToolUse from solution
	postToolUse, ok := hooksMap["PostToolUse"]
	require.True(t, ok, "expected PostToolUse hook from solution layer")
	postArr := postToolUse.([]any)
	assert.Len(t, postArr, 1)
}

func TestCompileClaude_AgentsOverlay(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	expectedAgents := []string{
		"code-reviewer.md", "compliance-checker.md", "builder.md",
		"qa-engineer.md", "integration-tester.md",
	}

	agentsDir := filepath.Join(outputDir, ".claude", "agents")
	for _, agent := range expectedAgents {
		_, err := os.Stat(filepath.Join(agentsDir, agent))
		assert.NoError(t, err, "expected agent file %q to exist", agent)
	}

	// Verify no extra agent files
	entries, err := os.ReadDir(agentsDir)
	require.NoError(t, err, "reading agents dir")
	assert.Len(t, entries, 5, "expected exactly 5 agent files")
}

func TestCompileClaude_FileRecords(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Output.Files, "expected file records in result")

	// Check for expected outputs
	outputs := make(map[string]bool)
	for _, f := range result.Output.Files {
		outputs[f.Output] = true
	}

	// Check for key files (not directory-level records anymore)
	assert.True(t, outputs["CLAUDE.md"], "expected file record for CLAUDE.md")
	assert.True(t, outputs[".claude/settings.json"], "expected file record for settings.json")

	// Check that we have SOME command and agent files (per-file records now)
	hasCommand := false
	hasAgent := false
	for path := range outputs {
		if strings.HasPrefix(path, ".claude/commands/") {
			hasCommand = true
		}
		if strings.HasPrefix(path, ".claude/agents/") {
			hasAgent = true
		}
	}
	assert.True(t, hasCommand, "expected at least one command file record")
	assert.True(t, hasAgent, "expected at least one agent file record")
}

func TestCompileClaude_InstructionContent(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	text := string(content)

	// Verify unique content from each layer is present
	layerContent := map[string]string{
		"base":     "containerized AI development environment",
		"company":  "company coding standards",
		"org":      "PCI-DSS compliance",
		"solution": "Customer Master Data Management",
		"team":     "platform engineering team",
		"project":  "microservices application with three components",
	}

	for layer, expected := range layerContent {
		assert.Contains(t, text, expected, "expected content from %s layer", layer)
	}
}

func TestCompileClaude_HookMergeSemantics(t *testing.T) {
	t.Parallel()
	// Verify that solution's PreToolUse overwrites company's PreToolUse
	// (same key path, array last-writer-wins) while PostToolUse is added
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	hooks := settings["hooks"].(map[string]any)

	// PreToolUse: solution's version
	pre := hooks["PreToolUse"].([]any)
	require.Len(t, pre, 1)
	preHook := pre[0].(map[string]any)
	assert.Equal(t, "Write(**/secrets/**)", preHook["matcher"])

	// PostToolUse: only solution has it
	post := hooks["PostToolUse"].([]any)
	require.Len(t, post, 1)
	postHook := post[0].(map[string]any)
	assert.Equal(t, "Write(**)", postHook["matcher"])
}

func TestCompileClaude_PermissionsLastWriterWins(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	perms := settings["permissions"].(map[string]any)
	allow := perms["allow"].([]any)

	expectedPerms := []string{
		"Read", "Edit", "Write", "Bash(git:*)", "Bash(rg:*)", "Bash(npm:*)", "Bash(find:*)",
	}
	require.Len(t, allow, len(expectedPerms))
	for i, expected := range expectedPerms {
		assert.Equal(t, expected, allow[i], "permission[%d]", i)
	}
}

// --- Claude feature coverage tests ---

func TestCompileClaude_SettingsLocalExcluded(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	localPath := filepath.Join(outputDir, ".claude", "settings.local.json")
	_, err = os.Stat(localPath)
	assert.True(t, os.IsNotExist(err),
		"settings.local.json should NOT be in compiled output")
}

func TestCompileClaude_SkillsOverlay(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()
	td := testdataDir(t)

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify skills from team and project layers exist
	expectedSkills := map[string]string{
		"security-review/SKILL.md": "team",
		"api-design/SKILL.md":      "project",
	}

	skillsDir := filepath.Join(outputDir, ".claude", "skills")
	for skill, layer := range expectedSkills {
		_, err := os.Stat(filepath.Join(skillsDir, skill))
		assert.NoError(t, err, "expected skill %q (from %s layer) to exist", skill, layer)
	}

	// Verify content matches expected golden files
	assertFileMatchesExpected(t, outputDir, ".claude/skills/security-review/SKILL.md",
		filepath.Join(td, "expected", "claude", ".claude", "skills", "security-review", "SKILL.md"))
	assertFileMatchesExpected(t, outputDir, ".claude/skills/api-design/SKILL.md",
		filepath.Join(td, "expected", "claude", ".claude", "skills", "api-design", "SKILL.md"))

	// Verify companion files are preserved
	companionFiles := []string{
		".claude/skills/security-review/scripts/check-vulnerabilities.sh",
		".claude/skills/security-review/references/OWASP-TOP10.md",
		".claude/skills/security-review/template.yaml",
	}
	for _, f := range companionFiles {
		assertFileMatchesExpected(t, outputDir, f,
			filepath.Join(td, "expected", "claude", f))
	}
}

func TestCompileClaude_McpInSettings(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify mcpServers from org layer survived deep merge
	mcpServers, ok := settings["mcpServers"]
	require.True(t, ok, "expected mcpServers key to survive deep merge")

	servers := mcpServers.(map[string]any)
	jira, ok := servers["jira"]
	require.True(t, ok, "expected jira server in mcpServers")

	jiraMap := jira.(map[string]any)
	assert.Equal(t, "npx", jiraMap["command"])

	args := jiraMap["args"].([]any)
	require.Len(t, args, 1)
	assert.Equal(t, "jira-mcp-server", args[0])
}

func TestCompileClaude_HooksConcatenated(t *testing.T) {
	t.Parallel()
	tmpLayers := t.TempDir()

	companyDir := filepath.Join(tmpLayers, "company")
	os.MkdirAll(filepath.Join(companyDir, "hooks"), 0755)
	os.WriteFile(filepath.Join(companyDir, "keel.yml"), []byte("hooks:\n  - source: hooks/\n"), 0644)
	os.WriteFile(filepath.Join(companyDir, "hooks", "coordination.json"), []byte(`{
		"SessionStart": [{"hooks": [{"type": "command", "command": "bash coordination-start.sh"}]}]
	}`), 0644)
	os.WriteFile(filepath.Join(companyDir, "hooks", "evolution.json"), []byte(`{
		"SessionStart": [{"hooks": [{"type": "command", "command": "bash evolution-start.sh"}]}]
	}`), 0644)

	layers := []config.Layer{
		{Path: companyDir, Format: config.FormatKeel},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	hooksMap, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "expected hooks map in settings")

	sessionStart, ok := hooksMap["SessionStart"]
	require.True(t, ok, "expected SessionStart in hooks")
	sessionArr, ok := sessionStart.([]any)
	require.True(t, ok, "expected SessionStart to be an array")
	assert.Len(t, sessionArr, 2, "expected 2 SessionStart hooks (concatenated)")
}

// --- Claude MCP separate file tests ---

func TestCompileClaude_McpSeparateFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "mcp_layer")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	mcpContent := `{"mcpServers":{"context7":{"command":"npx","args":["context7-mcp"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "mcp.json"), []byte(mcpContent), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: config.FormatKeel}}
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Verify .mcp.json exists at output root
	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "expected .mcp.json at output root")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok, "expected mcpServers key")
	assert.Contains(t, servers, "context7")

	// Verify it's NOT inside .claude/
	_, err = os.Stat(filepath.Join(outputDir, ".claude", "mcp.json"))
	assert.True(t, os.IsNotExist(err), "mcp.json should NOT be inside .claude/")

	found := false
	for _, f := range result.Output.Files {
		if f.Output == ".mcp.json" && f.Strategy == "deep-merge" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected file record for .mcp.json with strategy 'deep-merge'")
}

func TestCompileClaude_McpSeparateFileDeepMerged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layer1 := filepath.Join(dir, "layer1")
	require.NoError(t, os.MkdirAll(layer1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "mcp.json"),
		[]byte(`{"mcpServers":{"first":{"command":"first-cmd"}}}`), 0644))

	layer2 := filepath.Join(dir, "layer2")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "mcp.json"),
		[]byte(`{"mcpServers":{"second":{"command":"second-cmd"}}}`), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "base", Format: config.FormatKeel},
		{Path: layer2, Type: "project", Format: config.FormatKeel},
	}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "first", "expected first layer's server (deep-merge)")
	assert.Contains(t, servers, "second", "expected second layer's server (deep-merge)")
}

func TestCompileClaude_McpSeparateFileWithEnterprise(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers", allLayers...)
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "expected .mcp.json at output root")

	var mcp map[string]any
	require.NoError(t, json.Unmarshal(data, &mcp))
	servers := mcp["mcpServers"].(map[string]any)

	for _, s := range []string{"sonarqube", "jira", "gitlab", "obsidian"} {
		assert.Contains(t, servers, s, "expected %s server in .mcp.json", s)
	}
}

// --- Claude native settings detection ---

func TestCompileClaude_NativeSettingsClaudeJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	settingsContent := `{"model":"claude-sonnet-4-20250514","custom_key":"from-convention-name"}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.claude.json"), []byte(settingsContent), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "expected .claude/settings.json to exist")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "claude-sonnet-4-20250514", settings["model"])
	assert.Equal(t, "from-convention-name", settings["custom_key"])
}

func TestCompileClaude_NativeSettingsJSONTakesPriority(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"source":"tool-native"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.claude.json"),
		[]byte(`{"source":"convention"}`), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "tool-native", settings["source"],
		"settings.json (tool-native) should take priority")
}

func TestCompileClaude_NativeSettingsConventionNotLeakedViaOverlay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	layerDir := filepath.Join(dir, "layer")
	claudeDir := filepath.Join(layerDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.claude.json"),
		[]byte(`{"model":"sonnet"}`), 0644))

	layers := []config.Layer{{Path: layerDir, Type: "project", Format: "claude"}}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, ".claude", "settings.claude.json"))
	assert.True(t, os.IsNotExist(err),
		"settings.claude.json should NOT leak through to output via overlay")
}
