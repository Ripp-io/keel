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
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

// --- Claude DSL tests ---

func TestCompileClaude_DSLLayer(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-dsl", "company")
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)
	assert.Equal(t, "claude", result.Output.Agent)

	// Check CLAUDE.md has company instructions
	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Company Standards")
	assert.Contains(t, string(content), "company coding standards")

	// Check settings.json has permissions
	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))
	perms := settings["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 2, "expected 2 permissions (Read, Edit)")

	// Check hooks in settings.json
	hooks := settings["hooks"].(map[string]any)
	preToolUse, ok := hooks["PreToolUse"]
	require.True(t, ok, "expected PreToolUse hook in settings.json")
	preArr := preToolUse.([]any)
	assert.Len(t, preArr, 1)

	// Check .mcp.json
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "sonarqube", "expected sonarqube server in .mcp.json")
	sonarqube := servers["sonarqube"].(map[string]any)
	env := sonarqube["env"].(map[string]any)
	assert.Equal(t, "${SONAR_TOKEN}", env["SONAR_TOKEN"])

	// Check commands
	commandPath := filepath.Join(outputDir, ".claude", "commands", "security-scan.md")
	_, err = os.Stat(commandPath)
	assert.NoError(t, err, "expected security-scan.md command to exist")
}

func TestCompileClaude_DSLMultiLayer(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-dsl", "company", "org", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// 1. Instructions concatenated from all 3 layers
	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	text := string(content)
	assert.Contains(t, text, "Company Standards")
	assert.Contains(t, text, "Payments Org Standards")
	assert.Contains(t, text, "Demo Project")

	// 2. Permissions from project (last-writer-wins on the array)
	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))
	perms := settings["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	// Project has: Read, Edit, Write, Bash(git:*), Bash(npm:*)
	assert.Len(t, allow, 5, "expected 5 permissions (project's array)")

	// 3. MCP servers accumulated (company: sonarqube, org: sonarqube+jira)
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "sonarqube")
	assert.Contains(t, servers, "jira")

	// 4. Commands accumulated
	for _, cmd := range []string{"security-scan.md", "compliance-check.md", "build-all.md"} {
		path := filepath.Join(outputDir, ".claude", "commands", cmd)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected command %q to exist", cmd)
	}

	// 5. Agents accumulated
	for _, agent := range []string{"code-reviewer.md", "qa-engineer.md"} {
		path := filepath.Join(outputDir, ".claude", "agents", agent)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected agent %q to exist", agent)
	}

	// 6. Hooks from company survive (project has no hooks, so company's persist via deep-merge)
	hooks, ok := settings["hooks"]
	require.True(t, ok, "expected hooks to survive from company layer")
	hooksMap := hooks.(map[string]any)
	assert.Contains(t, hooksMap, "PreToolUse", "expected PreToolUse hook from company layer")

	// 7. Settings from project layer (claude.theme: dark)
	assert.Equal(t, "dark", settings["theme"])
}

// --- Cursor DSL tests ---

func TestCompileCursor_DSLLayer(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-dsl", "company")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Check instructions in .mdc
	mdcPath := filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc")
	content, err := os.ReadFile(mdcPath)
	require.NoError(t, err, "reading 00-keel.mdc")
	text := string(content)
	assert.True(t, strings.HasPrefix(text, "---\n"), "expected .mdc to start with frontmatter")
	assert.Contains(t, text, "alwaysApply: true")
	assert.Contains(t, text, "Company Standards")

	// Check permission translation in cli-config.json
	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))
	permsAllow := cliConfig["permissions.allow"].([]any)
	// Company has Read, Edit -> Read(*), Edit(*)
	require.Len(t, permsAllow, 2)
	assert.Equal(t, "Read(*)", permsAllow[0])
	assert.Equal(t, "Edit(*)", permsAllow[1])

	// Check hook casing in hooks.json (PreToolUse -> preToolUse)
	hooksData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "hooks.json"))
	require.NoError(t, err, "reading hooks.json")
	var hooks map[string]any
	require.NoError(t, json.Unmarshal(hooksData, &hooks))
	assert.Contains(t, hooks, "preToolUse", "expected camelCase 'preToolUse' in hooks.json")
	assert.NotContains(t, hooks, "PreToolUse", "PascalCase 'PreToolUse' should NOT appear in Cursor hooks")

	// Check MCP
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "sonarqube", "expected sonarqube server in mcp.json")
}

func TestCompile_DSLPermissionTranslation(t *testing.T) {
	t.Parallel()
	// DSL permissions: ["Read", "Edit", "Bash(git:*)"]
	// Claude output: same
	// Cursor output: ["Read(*)", "Edit(*)", "Bash(git:*)"]

	// Create a temporary DSL layer with external permissions.json
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "perm_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	keelYML := `
schema: v1
name: Perm Test
permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	permsJSON := `{"allow": ["Read", "Edit", "Bash(git:*)"]}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"), []byte(permsJSON), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))
		perms := settings["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
		// Claude keeps original format
		expected := []string{"Read", "Edit", "Bash(git:*)"}
		require.Len(t, allow, len(expected))
		for i, exp := range expected {
			assert.Equal(t, exp, allow[i], "allow[%d]", i)
		}
	})

	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
		require.NoError(t, err)
		var cliConfig map[string]any
		require.NoError(t, json.Unmarshal(data, &cliConfig))
		permsAllow := cliConfig["permissions.allow"].([]any)
		// Cursor translates simple names
		expected := []string{"Read(*)", "Edit(*)", "Bash(git:*)"}
		require.Len(t, permsAllow, len(expected))
		for i, exp := range expected {
			assert.Equal(t, exp, permsAllow[i], "permissions.allow[%d]", i)
		}
	})
}

func TestCompileClaude_MixedLayers(t *testing.T) {
	t.Parallel()
	// Layer 1: native (from existing testdata)
	// Layer 2: DSL (from layers-dsl)
	td := testdataDir(t)

	nativeLayer := config.Layer{
		Path:   filepath.Join(td, "layers", "base"),
		Type:   "base",
		Format: config.FormatKeel,
	}
	dslLayer := config.Layer{
		Path:   filepath.Join(td, "layers-dsl", "project"),
		Type:   "project",
		Format: config.FormatKeel,
	}

	layers := []config.Layer{nativeLayer, dslLayer}
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Native base layer contributes instructions
	content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err)
	text := string(content)
	assert.Contains(t, text, "containerized AI development environment")
	assert.Contains(t, text, "Demo Project")

	// Native base layer contributes commands
	baseCmdPath := filepath.Join(outputDir, ".claude", "commands", "project-status.md")
	_, err = os.Stat(baseCmdPath)
	assert.NoError(t, err, "expected native base command project-status.md to exist")

	// DSL project layer contributes commands
	projectCmdPath := filepath.Join(outputDir, ".claude", "commands", "build-all.md")
	_, err = os.Stat(projectCmdPath)
	assert.NoError(t, err, "expected DSL project command build-all.md to exist")

	// DSL project layer contributes agents
	agentPath := filepath.Join(outputDir, ".claude", "agents", "qa-engineer.md")
	_, err = os.Stat(agentPath)
	assert.NoError(t, err, "expected DSL project agent qa-engineer.md to exist")

	// DSL project settings appear
	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))
	assert.Equal(t, "dark", settings["theme"])
}

func TestCompileCursor_DSLMultiLayer(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-dsl", "company", "org", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Instructions from all 3 layers
	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
	require.NoError(t, err)
	text := string(content)
	assert.Contains(t, text, "Company Standards")
	assert.Contains(t, text, "Payments Org Standards")
	assert.Contains(t, text, "Demo Project")

	// Permission translation from project layer
	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))
	permsAllow := cliConfig["permissions.allow"].([]any)
	expectedPerms := []string{"Read(*)", "Edit(*)", "Write(*)", "Bash(git:*)", "Bash(npm:*)"}
	require.Len(t, permsAllow, len(expectedPerms))
	for i, exp := range expectedPerms {
		assert.Equal(t, exp, permsAllow[i], "permissions.allow[%d]", i)
	}

	// MCP servers accumulated
	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "mcp.json"))
	require.NoError(t, err, "reading mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))
	servers := mcp["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "sonarqube")
	assert.Contains(t, servers, "jira")

	// Hooks from company (camelCase)
	hooksData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "hooks.json"))
	require.NoError(t, err, "reading hooks.json")
	var hooks map[string]any
	require.NoError(t, json.Unmarshal(hooksData, &hooks))
	assert.Contains(t, hooks, "preToolUse", "expected camelCase preToolUse in hooks.json")

	// Cursor-specific settings
	assert.Equal(t, false, cliConfig["editor.vimMode"])

	// Commands accumulated
	for _, cmd := range []string{"security-scan.md", "compliance-check.md", "build-all.md"} {
		path := filepath.Join(outputDir, ".cursor", "commands", cmd)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected command %q to exist", cmd)
	}

	// Agents accumulated
	for _, agent := range []string{"code-reviewer.md", "qa-engineer.md"} {
		path := filepath.Join(outputDir, ".cursor", "agents", agent)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected agent %q to exist", agent)
	}
}

func TestCompile_DSLAutoDiscover(t *testing.T) {
	t.Parallel()
	// Layer has keel.yml but doesn't list commands/agents explicitly.
	// Layer has commands/ and agents/ directories.
	// Verify they're auto-discovered via defaults merge.

	dir := t.TempDir()
	layerDir := filepath.Join(dir, "autodiscover")
	require.NoError(t, os.MkdirAll(filepath.Join(layerDir, "commands"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(layerDir, "agents"), 0755))

	// Minimal keel.yml — no commands or agents specified
	keelYML := `
schema: v1
name: Auto Discover Test
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "commands", "auto-cmd.md"), []byte("auto-discovered command"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "agents", "auto-agent.md"), []byte("auto-discovered agent"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte("auto instructions"), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(outputDir, ".claude", "commands", "auto-cmd.md"))
		assert.NoError(t, err, "expected auto-discovered command to exist")

		_, err = os.Stat(filepath.Join(outputDir, ".claude", "agents", "auto-agent.md"))
		assert.NoError(t, err, "expected auto-discovered agent to exist")

		content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "auto instructions")
	})

	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(outputDir, ".cursor", "commands", "auto-cmd.md"))
		assert.NoError(t, err, "expected auto-discovered command to exist")

		_, err = os.Stat(filepath.Join(outputDir, ".cursor", "agents", "auto-agent.md"))
		assert.NoError(t, err, "expected auto-discovered agent to exist")
	})
}

func TestCompileCursor_MixedLayers(t *testing.T) {
	t.Parallel()
	// Mix native and DSL layers for Cursor
	td := testdataDir(t)

	nativeLayer := config.Layer{
		Path:   filepath.Join(td, "layers", "base"),
		Type:   "base",
		Format: config.FormatKeel,
	}
	dslLayer := config.Layer{
		Path:   filepath.Join(td, "layers-dsl", "project"),
		Type:   "project",
		Format: config.FormatKeel,
	}

	layers := []config.Layer{nativeLayer, dslLayer}
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Instructions from both layers
	content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
	require.NoError(t, err)
	text := string(content)
	assert.Contains(t, text, "containerized AI development environment")
	assert.Contains(t, text, "Demo Project")

	// Commands from both layers
	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "commands", "project-status.md"))
	assert.NoError(t, err, "expected native base command project-status.md")
	_, err = os.Stat(filepath.Join(outputDir, ".cursor", "commands", "build-all.md"))
	assert.NoError(t, err, "expected DSL project command build-all.md")
}

func TestCompileClaude_DSLMCPDeepMerge(t *testing.T) {
	t.Parallel()
	// Company adds sonarqube, org adds sonarqube+jira.
	// Output should have both, with org's sonarqube overwriting company's.
	layers := buildTestLayers(t, "layers-dsl", "company", "org")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	mcpData, err := os.ReadFile(filepath.Join(outputDir, ".mcp.json"))
	require.NoError(t, err, "reading .mcp.json")
	var mcp map[string]any
	require.NoError(t, json.Unmarshal(mcpData, &mcp))

	servers := mcp["mcpServers"].(map[string]any)
	assert.Len(t, servers, 2, "expected 2 MCP servers (sonarqube + jira)")
	assert.Contains(t, servers, "sonarqube")
	assert.Contains(t, servers, "jira")
}

func TestCompileClaude_DSLSettingsDeepMerge(t *testing.T) {
	t.Parallel()
	// Company has permissions + hooks, project has permissions + claude settings.
	// Verify deep-merge: hooks from company survive, permissions from project win.
	layers := buildTestLayers(t, "layers-dsl", "company", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))

	// Project permissions win (last-writer-wins on the array via deep-merge)
	perms := settings["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 5, "expected 5 permissions from project")

	// Company hooks survive (different key path)
	hooks, ok := settings["hooks"]
	require.True(t, ok, "expected hooks from company to survive deep-merge")
	hooksMap := hooks.(map[string]any)
	assert.Contains(t, hooksMap, "PreToolUse")

	// Project's claude theme setting
	assert.Equal(t, "dark", settings["theme"])
}

func TestCompile_DSLEmptyLayer(t *testing.T) {
	t.Parallel()
	// A DSL layer with only schema/name (no files, no dirs) should not error.
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "empty_dsl")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\nname: Empty\n"), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	for _, adapter := range []string{"claude", "cursor"} {
		t.Run(adapter, func(t *testing.T) {
			t.Parallel()
			outputDir := t.TempDir()
			_, err := Compile(adapter, layers, outputDir, CompileOptions{})
			assert.NoError(t, err, "Compile(%s) with empty DSL layer should not error", adapter)
		})
	}
}

func TestCompileCursor_DSLSettingsModel(t *testing.T) {
	t.Parallel()
	// Verify that settings.model is written to cli-config.json
	layers := buildTestLayers(t, "layers-dsl", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err)
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))
	assert.Equal(t, "sonnet", cliConfig["model"])
}

func TestCompile_DSLInstructionPathRef(t *testing.T) {
	t.Parallel()
	// Test that source: references to directories resolve their contents
	// sorted by filename.
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "path_ref")
	instrDir := filepath.Join(layerDir, "docs")
	require.NoError(t, os.MkdirAll(instrDir, 0755))

	// Create files in non-alphabetical order to verify sorting
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "02-security.md"), []byte("security rules"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "01-overview.md"), []byte("project overview"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "03-testing.md"), []byte("testing guidelines"), 0644))
	// Non-.md file should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "readme.txt"), []byte("not markdown"), 0644))

	keelYML := `
schema: v1
name: Path Ref Test
instructions:
  - source: docs/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("claude", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
		require.NoError(t, err)
		text := string(content)

		// All 3 .md files should be included
		assert.Contains(t, text, "project overview")
		assert.Contains(t, text, "security rules")
		assert.Contains(t, text, "testing guidelines")
		// .txt file should NOT be included
		assert.NotContains(t, text, "not markdown")

		// Verify order: overview before security before testing
		overviewIdx := strings.Index(text, "project overview")
		securityIdx := strings.Index(text, "security rules")
		testingIdx := strings.Index(text, "testing guidelines")
		assert.Less(t, overviewIdx, securityIdx, "expected files sorted alphabetically")
		assert.Less(t, securityIdx, testingIdx, "expected files sorted alphabetically")
	})

	t.Run("cursor", func(t *testing.T) {
		t.Parallel()
		outputDir := t.TempDir()
		_, err := Compile("cursor", layers, outputDir, CompileOptions{})
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc"))
		require.NoError(t, err)
		text := string(content)

		assert.Contains(t, text, "project overview")
		assert.Contains(t, text, "security rules")
	})
}

func TestCompile_DSLDenyPermissions(t *testing.T) {
	t.Parallel()
	// Verify deny permissions are handled
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "deny_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	keelYML := `
schema: v1
name: Deny Test
permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	permsJSON := `{"allow": ["Read"], "deny": ["Bash(rm:*)", "Bash(sudo:*)"]}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"), []byte(permsJSON), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	perms := settings["permissions"].(map[string]any)
	deny := perms["deny"].([]any)
	require.Len(t, deny, 2)
	assert.Equal(t, "Bash(rm:*)", deny[0])
}

func TestProcessDSLLayer_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "invalid")
	require.NoError(t, os.MkdirAll(layerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(":::bad yaml"), 0644))

	_, err := processDSLLayer(layerDir)
	require.Error(t, err, "expected error for invalid keel.yml")
}

func TestProcessDSLLayer_MissingKeelYML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg, err := processDSLLayer(dir)
	require.NoError(t, err, "unexpected error for missing keel.yml")
	require.NotNil(t, cfg, "expected defaults config, got nil")
}

func TestCompile_DSLLayerWithSkills(t *testing.T) {
	t.Parallel()
	// Test that skills with source: directory refs are overlaid correctly.
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "skills_test")
	skillDir := filepath.Join(layerDir, "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("my skill content"), 0644))

	keelYML := `
schema: v1
name: Skills Test
skills:
  - source: skills/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	for _, adapter := range []string{"claude", "cursor"} {
		t.Run(adapter, func(t *testing.T) {
			t.Parallel()
			outputDir := t.TempDir()
			_, err := Compile(adapter, layers, outputDir, CompileOptions{})
			require.NoError(t, err)

			var skillPath string
			if adapter == "claude" {
				skillPath = filepath.Join(outputDir, ".claude", "skills", "my-skill", "SKILL.md")
			} else {
				skillPath = filepath.Join(outputDir, ".cursor", "skills", "my-skill", "SKILL.md")
			}
			data, err := os.ReadFile(skillPath)
			require.NoError(t, err, "expected skill to exist")
			assert.Contains(t, string(data), "my skill content")
		})
	}
}

func TestCompileCursor_DSLCursorSpecificSettings(t *testing.T) {
	t.Parallel()
	// Verify that settings.cursor fields are written to cli-config.json
	layers := buildTestLayers(t, "layers-dsl", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err)
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))
	assert.Equal(t, false, cliConfig["editor.vimMode"])
}

func TestForceLockKeyForFile(t *testing.T) {
	t.Parallel()
	categories := []CategoryDef{
		{"commands", "command"},
		{"agents", "agent"},
		{"skills", "skill"},
		{"rules", "rule"},
	}
	tests := []struct {
		input string
		want  string
	}{
		{"commands/deploy.md", "deploy.md"},
		{"commands/sub/review.md", "sub/review.md"},
		{"agents/code-reviewer.md", "code-reviewer.md"},
		{"skills/deep/nested/file.mdc", "deep/nested/file.mdc"},
		{"rules/lint.md", "lint.md"},
		{"settings.json", "settings.json"},
		{".mcp.json", ".mcp.json"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, forceLockKeyForFile(tt.input, categories))
		})
	}
}

func TestContainsPath(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	inside := filepath.Join(base, "sub", "file.txt")
	os.MkdirAll(filepath.Join(base, "sub"), 0755)
	os.WriteFile(inside, []byte("test"), 0644)

	tests := []struct {
		name string
		base string
		path string
		want bool
	}{
		{"inside base", base, inside, true},
		{"equals base", base, base, true},
		{"outside via parent", base, filepath.Join(base, "..", "outside"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, pathutil.ContainsPath(tt.base, tt.path))
		})
	}
}

func TestCompileClaude_DSLPlugins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "plugins_test")
	pluginDir := filepath.Join(layerDir, "plugins", "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(`{"name":"my-plugin"}`), 0644))

	keelYML := `
schema: v1
name: Plugins Test
plugins:
  - source: plugins/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	pluginPath := filepath.Join(outputDir, ".claude", "plugins", "my-plugin", "manifest.json")
	data, err := os.ReadFile(pluginPath)
	require.NoError(t, err, "expected plugin to exist at %s", pluginPath)
	assert.Contains(t, string(data), "my-plugin")
}

func TestCompileClaude_DSLAgentMemory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "memory_test")
	memDir := filepath.Join(layerDir, "memory")
	require.NoError(t, os.MkdirAll(memDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(memDir, "project-context.md"), []byte("project context"), 0644))

	keelYML := `
schema: v1
name: Memory Test
agent-memory:
  - source: memory/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	memPath := filepath.Join(outputDir, ".claude", "agent-memory", "project-context.md")
	data, err := os.ReadFile(memPath)
	require.NoError(t, err, "expected agent-memory file to exist")
	assert.Contains(t, string(data), "project context")
}

func TestCompileCursor_DSLPluginsWarning(t *testing.T) {
	t.Parallel()
	// Cursor does not support plugins. Compilation should succeed
	// (skipping the unsupported category) without erroring.
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "cursor_plugins_test")
	pluginDir := filepath.Join(layerDir, "plugins", "my-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(`{"name":"my-plugin"}`), 0644))

	keelYML := `
schema: v1
name: Cursor Plugins Test
plugins:
  - source: plugins/
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	// Should not error — unsupported categories are just warned, not overlaid.
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	assert.NoError(t, err, "Compile() should succeed even with unsupported plugins")

	// Plugins dir should NOT exist in cursor output
	pluginPath := filepath.Join(outputDir, ".cursor", "plugins")
	_, err = os.Stat(pluginPath)
	assert.True(t, os.IsNotExist(err), "plugins directory should not exist in cursor output")
}

func TestCompileCursor_DSLEnvironment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "env_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	envJSON := `{
  "install": "npm install",
  "start": "npm run dev",
  "env": {"NODE_ENV": "development"}
}`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "environment.json"), []byte(envJSON), 0644))

	keelYML := `
schema: v1
name: Environment Test
environment:
  - source: environment.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	envPath := filepath.Join(outputDir, ".cursor", "environment.json")
	data, err := os.ReadFile(envPath)
	require.NoError(t, err, "expected environment.json to exist")

	var envConfig map[string]any
	require.NoError(t, json.Unmarshal(data, &envConfig))
	assert.Equal(t, "npm install", envConfig["install"])
	assert.Equal(t, "npm run dev", envConfig["start"])
	envVars, ok := envConfig["env"].(map[string]any)
	require.True(t, ok, "expected env to be a map")
	assert.Equal(t, "development", envVars["NODE_ENV"])
}

func TestCompileCursor_DSLEnvironmentMultiLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Layer 1: base environment
	layer1 := filepath.Join(dir, "base")
	require.NoError(t, os.MkdirAll(layer1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "environment.json"),
		[]byte(`{"install": "npm install", "env": {"NODE_ENV": "production"}}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer1, "keel.yml"),
		[]byte("schema: v1\nname: Base\nenvironment:\n  - source: environment.json\n"), 0644))

	// Layer 2: project overrides
	layer2 := filepath.Join(dir, "project")
	require.NoError(t, os.MkdirAll(layer2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "environment.json"),
		[]byte(`{"start": "npm run dev", "env": {"DEBUG": "1"}}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layer2, "keel.yml"),
		[]byte("schema: v1\nname: Project\nenvironment:\n  - source: environment.json\n"), 0644))

	layers := []config.Layer{
		{Path: layer1, Type: "base", Format: config.FormatKeel},
		{Path: layer2, Type: "project", Format: config.FormatKeel},
	}

	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "environment.json"))
	require.NoError(t, err, "expected environment.json")

	var envConfig map[string]any
	require.NoError(t, json.Unmarshal(data, &envConfig))

	assert.Equal(t, "npm install", envConfig["install"], "install from base layer")
	assert.Equal(t, "npm run dev", envConfig["start"], "start from project layer")
	envVars, ok := envConfig["env"].(map[string]any)
	require.True(t, ok, "expected env to be a map")
	assert.Equal(t, "1", envVars["DEBUG"])
}

func TestCompileClaude_DSLExpandedSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "settings_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Settings Test
settings:
  model: opus
  sandbox:
    enabled: true
    excluded-commands:
      - rm
    filesystem:
      allow-write:
        - ./src
      deny-read:
        - .env
    network:
      allowed-domains:
        - api.example.com
  env:
    MY_VAR: hello
  worktree:
    base-ref: main
    symlink-directories:
      - node_modules
    sparse-paths:
      - src/
  skill-overrides:
    deploy: "off"
    legacy-context: "name-only"
  auto-mode:
    environment:
      - "Source control: github.example.com"
    allow:
      - Bash(npm:*)
    soft-deny:
      - "Never run terraform apply"
  attribution:
    commit: "Co-Authored-By: Claude"
    pr: "Generated by Claude Code"
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Model
	assert.Equal(t, "opus", settings["model"])

	// Sandbox
	sandbox, ok := settings["sandbox"].(map[string]any)
	require.True(t, ok, "expected sandbox to be a map")
	assert.Equal(t, true, sandbox["enabled"])
	excludedCmds, ok := sandbox["excludedCommands"].([]any)
	require.True(t, ok)
	require.Len(t, excludedCmds, 1)
	fs, ok := sandbox["filesystem"].(map[string]any)
	require.True(t, ok, "expected sandbox.filesystem to be a map")
	allowWrite, ok := fs["allowWrite"].([]any)
	require.True(t, ok)
	require.Len(t, allowWrite, 1)
	assert.Equal(t, "./src", allowWrite[0])
	net, ok := sandbox["network"].(map[string]any)
	require.True(t, ok, "expected sandbox.network to be a map")
	domains, ok := net["allowedDomains"].([]any)
	require.True(t, ok)
	require.Len(t, domains, 1)

	// Env
	env, ok := settings["env"].(map[string]any)
	require.True(t, ok, "expected env to be a map")
	assert.Equal(t, "hello", env["MY_VAR"])

	// Worktree
	wt, ok := settings["worktree"].(map[string]any)
	require.True(t, ok, "expected worktree to be a map")
	assert.Equal(t, "main", wt["baseRef"])
	symlinkDirs, ok := wt["symlinkDirectories"].([]any)
	require.True(t, ok)
	require.Len(t, symlinkDirs, 1)
	sparsePaths, ok := wt["sparsePaths"].([]any)
	require.True(t, ok)
	require.Len(t, sparsePaths, 1)

	// SkillOverrides
	so, ok := settings["skillOverrides"].(map[string]any)
	require.True(t, ok, "expected skillOverrides to be a map")
	assert.Equal(t, "off", so["deploy"])
	assert.Equal(t, "name-only", so["legacy-context"])

	// AutoMode
	am, ok := settings["autoMode"].(map[string]any)
	require.True(t, ok, "expected autoMode to be a map")
	amEnv, ok := am["environment"].([]any)
	require.True(t, ok)
	require.Len(t, amEnv, 1)
	amAllow, ok := am["allow"].([]any)
	require.True(t, ok)
	require.Len(t, amAllow, 1)
	amSoftDeny, ok := am["softDeny"].([]any)
	require.True(t, ok, "expected softDeny in autoMode")
	require.Len(t, amSoftDeny, 1)

	// Attribution
	attr, ok := settings["attribution"].(map[string]any)
	require.True(t, ok, "expected attribution to be a map")
	assert.Equal(t, "Co-Authored-By: Claude", attr["commit"])
	assert.Equal(t, "Generated by Claude Code", attr["pr"])
}

func TestCompileClaude_DSLSettingsMinimal(t *testing.T) {
	t.Parallel()
	// Verify that minimal settings (only model) still work correctly.
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "minimal_settings")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Minimal Settings
settings:
  model: sonnet
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "sonnet", settings["model"])

	// None of the new settings should be present
	for _, key := range []string{"sandbox", "env", "worktree", "skillOverrides", "autoMode", "attribution"} {
		assert.NotContains(t, settings, key, "settings should not contain %q when not specified", key)
	}
}

// --- Cursor sandbox support tests ---

func TestCompileCursor_DSLSandboxFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "sandbox_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Sandbox Test
settings:
  sandbox:
    enabled: true
    type: strict
    filesystem:
      allow-write:
        - ./src
        - ./build
      allow-read:
        - ./config
    additional-readwrite-paths:
      - /tmp/cache
    additional-readonly-paths:
      - /etc/ssl
    disable-tmp-write: true
    enable-shared-build-cache: false
    network:
      default-policy: deny
      allowed-domains:
        - api.example.com
      denied-domains:
        - evil.com
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// sandbox.json should exist
	sandboxPath := filepath.Join(outputDir, ".cursor", "sandbox.json")
	data, err := os.ReadFile(sandboxPath)
	require.NoError(t, err, "expected .cursor/sandbox.json to exist")

	var sandbox map[string]any
	require.NoError(t, json.Unmarshal(data, &sandbox))

	// Cursor-only fields
	assert.Equal(t, "strict", sandbox["type"])
	assert.Equal(t, true, sandbox["disableTmpWrite"])
	assert.Equal(t, false, sandbox["enableSharedBuildCache"])

	// Cursor-only + portable merged: additionalReadwritePaths = [/tmp/cache, ./src, ./build]
	rwPaths, ok := sandbox["additionalReadwritePaths"].([]any)
	require.True(t, ok, "expected additionalReadwritePaths to be an array")
	assert.Contains(t, rwPaths, "/tmp/cache")
	assert.Contains(t, rwPaths, "./src")
	assert.Contains(t, rwPaths, "./build")

	// additionalReadonlyPaths = [/etc/ssl, ./config]
	roPaths, ok := sandbox["additionalReadonlyPaths"].([]any)
	require.True(t, ok, "expected additionalReadonlyPaths to be an array")
	assert.Contains(t, roPaths, "/etc/ssl")
	assert.Contains(t, roPaths, "./config")

	// Network policy
	netPolicy, ok := sandbox["networkPolicy"].(map[string]any)
	require.True(t, ok, "expected networkPolicy to be a map")
	assert.Equal(t, "deny", netPolicy["default"])
	allowDomains, ok := netPolicy["allow"].([]any)
	require.True(t, ok)
	assert.Contains(t, allowDomains, "api.example.com")
	denyDomains, ok := netPolicy["deny"].([]any)
	require.True(t, ok)
	assert.Contains(t, denyDomains, "evil.com")
}

func TestCompileCursor_DSLSandboxNotInSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "sandbox_settings_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Sandbox Settings Test
settings:
  model: sonnet
  sandbox:
    type: strict
    filesystem:
      allow-write:
        - ./src
permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"),
		[]byte(`{"allow": ["Read"]}`), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// cli-config.json should NOT contain sandbox key
	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))
	assert.NotContains(t, cliConfig, "sandbox",
		"sandbox should be in sandbox.json, not cli-config.json")
	assert.Equal(t, "sonnet", cliConfig["model"])
}

func TestCompileClaude_CursorOnlySandboxFieldsExcluded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "cursor_fields_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Cursor Fields Excluded
settings:
  sandbox:
    enabled: true
    type: strict
    additional-readwrite-paths:
      - /tmp/cache
    disable-tmp-write: true
    enable-shared-build-cache: false
    network:
      default-policy: deny
      allowed-domains:
        - api.example.com
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	sandbox, ok := settings["sandbox"].(map[string]any)
	require.True(t, ok, "expected sandbox in Claude settings")
	assert.Equal(t, true, sandbox["enabled"])

	// Cursor-only fields should NOT appear in Claude output
	assert.NotContains(t, sandbox, "type",
		"Cursor-only 'type' should not leak into Claude settings")
	assert.NotContains(t, sandbox, "additionalReadwritePaths",
		"Cursor-only 'additionalReadwritePaths' should not leak into Claude settings")
	assert.NotContains(t, sandbox, "disableTmpWrite",
		"Cursor-only 'disableTmpWrite' should not leak into Claude settings")
	assert.NotContains(t, sandbox, "enableSharedBuildCache",
		"Cursor-only 'enableSharedBuildCache' should not leak into Claude settings")

	// Claude's own fields should be present
	net, ok := sandbox["network"].(map[string]any)
	require.True(t, ok, "expected network in Claude sandbox")
	domains, ok := net["allowedDomains"].([]any)
	require.True(t, ok)
	assert.Contains(t, domains, "api.example.com")
	// Cursor-only defaultPolicy should not leak
	assert.NotContains(t, net, "defaultPolicy",
		"Cursor-only 'defaultPolicy' should not leak into Claude settings")
}

// --- Cursor attribution support tests ---

func TestCompileCursor_DSLAttribution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "attribution_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Attribution Test
settings:
  attribution:
    commit: "Co-Authored-By: Claude"
    pr: "Generated by Claude Code"
permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"),
		[]byte(`{"allow": ["Read"]}`), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// cli-config.json should contain Cursor-format attribution
	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))

	attr, ok := cliConfig["attribution"].(map[string]any)
	require.True(t, ok, "expected attribution in cli-config.json")
	assert.Equal(t, true, attr["attributeCommitsToAgent"],
		"expected boolean flag for commit attribution")
	assert.Equal(t, true, attr["attributePRsToAgent"],
		"expected boolean flag for PR attribution")

	// Should NOT contain Claude's template strings
	_, hasCommit := attr["commit"]
	assert.False(t, hasCommit, "Claude commit template should not appear in Cursor attribution")
	_, hasPR := attr["pr"]
	assert.False(t, hasPR, "Claude PR template should not appear in Cursor attribution")
}

func TestCompileCursor_DSLAttributionCommitOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "attr_commit_test")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	keelYML := `
schema: v1
name: Attribution Commit Only
settings:
  attribution:
    commit: "Co-Authored-By: AI"
permissions:
  rules:
    - source: permissions.json
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "permissions.json"),
		[]byte(`{"allow": ["Read"]}`), 0644))

	layers := []config.Layer{{
		Path:   layerDir,
		Type:   "project",
		Format: config.FormatKeel,
	}}

	outputDir := t.TempDir()
	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	cliData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err)
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliData, &cliConfig))

	attr, ok := cliConfig["attribution"].(map[string]any)
	require.True(t, ok, "expected attribution in cli-config.json")
	assert.Equal(t, true, attr["attributeCommitsToAgent"])
	_, hasPR := attr["attributePRsToAgent"]
	assert.False(t, hasPR, "PR attribution should not be set when PR template is empty")
}

func TestDSL_EnvVarsPassThroughToCompiledOutput(t *testing.T) {
	t.Parallel()
	layers := []config.Layer{
		{Path: "testdata/layers-dsl/company", Type: "company", Format: config.FormatKeel},
	}

	// Claude
	claudeDir := t.TempDir()
	_, err := Compile("claude", layers, claudeDir, CompileOptions{})
	require.NoError(t, err, "Claude compile failed")

	mcpFile := filepath.Join(claudeDir, ".mcp.json")
	data, err := os.ReadFile(mcpFile)
	require.NoError(t, err, "Failed to read .mcp.json")
	assert.Contains(t, string(data), "${SONAR_TOKEN}",
		"Expected ${SONAR_TOKEN} to pass through in .mcp.json")

	// Cursor
	cursorDir := t.TempDir()
	_, err = Compile("cursor", layers, cursorDir, CompileOptions{})
	require.NoError(t, err, "Cursor compile failed")

	cursorMCP := filepath.Join(cursorDir, ".cursor", "mcp.json")
	data, err = os.ReadFile(cursorMCP)
	require.NoError(t, err, "Failed to read cursor mcp.json")
	assert.Contains(t, string(data), "${SONAR_TOKEN}",
		"Expected ${SONAR_TOKEN} to pass through in cursor mcp.json")
}
