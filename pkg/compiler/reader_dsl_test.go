package compiler

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// examplesLayerDir returns the absolute path to a named layer under examples/compiler/layers/.
func examplesLayerDir(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "unable to determine test file location")
	// pkg/compiler/ -> repo root -> examples/compiler/layers/<name>
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	dir := filepath.Join(repoRoot, "examples", "compiler", "layers", name)
	_, err := os.Stat(dir)
	require.NoError(t, err, "fixture layer %q not found at %s", name, dir)
	return dir
}

func TestKeelDSLReader_AgentName(t *testing.T) {
	t.Parallel()
	reader := &KeelDSLReader{}
	assert.Equal(t, "keel", reader.AgentName())
}

func TestKeelDSLReader_01KeelBase(t *testing.T) {
	t.Parallel()
	reader := &KeelDSLReader{Offline: true}
	layerPath := examplesLayerDir(t, "01-keel-base")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	// Metadata
	assert.Equal(t, "keel", ir.SourceAgent)
	assert.Equal(t, "keel-base", ir.Name)
	assert.Equal(t, "Comprehensive Keel DSL test fixture", ir.Description)
	assert.Equal(t, layerPath, ir.LayerPath)
	require.NotNil(t, ir.DSLConfig)

	// Instructions: keel.yml references three instruction files
	require.Len(t, ir.Instructions, 3)
	assert.Equal(t, "instructions.md", ir.Instructions[0].Filename)
	assert.Contains(t, ir.Instructions[0].Content, "Keel Base Instructions")
	assert.Equal(t, "instructions-carveout.md", ir.Instructions[1].Filename)
	assert.Equal(t, "instructions-template.md", ir.Instructions[2].Filename)

	// MCP
	require.NotNil(t, ir.MCP)
	assert.Contains(t, ir.MCP, "mcpServers")
	mcpServers, ok := ir.MCP["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers should be a map")
	assert.Contains(t, mcpServers, "sonarqube")
	assert.Contains(t, mcpServers, "gitlab")

	// MCP Policy
	require.NotNil(t, ir.MCPPolicy)
	assert.Contains(t, ir.MCPPolicy.Allowed, "sonarqube")
	assert.Contains(t, ir.MCPPolicy.Denied, "unsafe-server")
	require.NotNil(t, ir.MCPPolicy.EnableAll)
	assert.True(t, *ir.MCPPolicy.EnableAll)

	// Permissions
	require.NotNil(t, ir.Permissions)
	assert.Contains(t, ir.Permissions, "allow")
	assert.Contains(t, ir.Permissions, "deny")

	// Permissions Policy
	require.NotNil(t, ir.PermissionsPolicy)
	assert.Equal(t, "additive", ir.PermissionsPolicy.Merge)

	// Hooks
	require.NotNil(t, ir.Hooks)
	assert.Contains(t, ir.Hooks, "PreToolUse")
	assert.Contains(t, ir.Hooks, "PostToolUse")

	// Categories
	require.NotNil(t, ir.Categories)
	assert.Contains(t, ir.Categories, "commands")
	assert.Contains(t, ir.Categories, "agents")
	assert.Contains(t, ir.Categories, "skills")
	assert.Contains(t, ir.Categories, "plugins")
	assert.Contains(t, ir.Categories, "agent-memory")
	assert.Contains(t, ir.Categories, "rules")
	assert.Contains(t, ir.Categories, "init")

	// Verify commands include specific files
	commands := ir.Categories["commands"]
	var cmdNames []string
	for _, f := range commands {
		if !f.IsDir {
			cmdNames = append(cmdNames, f.RelPath)
		}
	}
	assert.Contains(t, cmdNames, "shared-cmd.md")
	assert.Contains(t, cmdNames, "keel-only-cmd.md")

	// Variables
	require.NotNil(t, ir.Variables)
	assert.Equal(t, "https://api.example.com", ir.Variables["api_url"])
	assert.Equal(t, "production", ir.Variables["environment"])

	// Snippets
	require.NotNil(t, ir.Snippets)
	assert.Contains(t, ir.Snippets, "deployment-note")
	assert.Contains(t, ir.Snippets, "reasoning-protocol")
	assert.Contains(t, ir.Snippets, "domain-rule")
	assert.Contains(t, ir.Snippets, "parallel-dispatch")

	// Settings (model is portable)
	require.NotNil(t, ir.Settings)
	assert.Equal(t, "sonnet", ir.Settings["model"])

	// Agent-specific settings from external JSON files
	require.NotNil(t, ir.AgentSpecific)
	assert.Contains(t, ir.AgentSpecific, "claude")
	assert.Contains(t, ir.AgentSpecific, "cursor")

	// All Settings struct fields are now portable and go to ir.Settings
	// AgentSpecific["claude"] only has settings from agent-keyed file references (settings.claude.json)
	claudeSpecific := ir.AgentSpecific["claude"]
	assert.NotContains(t, claudeSpecific, "sandbox", "sandbox is now portable on ir.Sandbox")
	assert.NotContains(t, claudeSpecific, "env", "env is now portable on ir.Settings")
	assert.Contains(t, ir.Settings, "env", "env should be in ir.Settings")

	// Portable sandbox and attribution
	require.NotNil(t, ir.Sandbox, "sandbox should be set on ir.Sandbox")
	require.NotNil(t, ir.Sandbox.Enabled)
	assert.True(t, *ir.Sandbox.Enabled)

	// Environment
	require.NotNil(t, ir.ConfigBlocks["environment"])
	assert.Equal(t, "https://staging.example.com", ir.ConfigBlocks["environment"]["STAGING_URL"])

	// Repos
	require.NotNil(t, ir.Repos)
	assert.Contains(t, ir.Repos, "party-service")
	assert.Contains(t, ir.Repos, "lens-common")

	// FilePolicies
	require.NotNil(t, ir.FilePolicies)
	assert.Contains(t, ir.FilePolicies, "commands/force-locked-cmd.md")
}

func TestKeelDSLReader_04KeelOverrides(t *testing.T) {
	t.Parallel()
	reader := &KeelDSLReader{Offline: true}
	layerPath := examplesLayerDir(t, "04-keel-overrides")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	assert.Equal(t, "keel", ir.SourceAgent)
	assert.Equal(t, "keel-overrides", ir.Name)

	// Variables override: environment should be "staging"
	require.NotNil(t, ir.Variables)
	assert.Equal(t, "staging", ir.Variables["environment"])

	// Settings: model should be "opus"
	require.NotNil(t, ir.Settings)
	assert.Equal(t, "opus", ir.Settings["model"])

	// Instructions
	require.Len(t, ir.Instructions, 1)
	assert.Equal(t, "instructions.md", ir.Instructions[0].Filename)

	// Commands
	require.NotNil(t, ir.Categories)
	assert.Contains(t, ir.Categories, "commands")

	// MCP, hooks, permissions populated
	require.NotNil(t, ir.MCP)
	require.NotNil(t, ir.Hooks)
	require.NotNil(t, ir.Permissions)
}

func TestKeelDSLReader_07DepsAndBin(t *testing.T) {
	t.Parallel()
	reader := &KeelDSLReader{Offline: true}
	layerPath := examplesLayerDir(t, "07-deps-and-bin")

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)

	assert.Equal(t, "keel", ir.SourceAgent)
	assert.Equal(t, "deps-and-bin", ir.Name)

	// Deps
	require.NotNil(t, ir.Deps)
	assert.Contains(t, ir.Deps, "fixture-tool")
	dep := ir.Deps["fixture-tool"]
	assert.Equal(t, "echo 'would install fixture-tool'", dep.Install)
	assert.Equal(t, "fixture-tool --version", dep.Verify)

	// Bin
	require.NotEmpty(t, ir.Bin)
	foundCheckEnv := false
	for _, bin := range ir.Bin {
		if bin.RelPath == "check-env.sh" {
			foundCheckEnv = true
			assert.Contains(t, string(bin.Content), "check-env v1.0")
			assert.NotZero(t, bin.Mode, "bin file should have a mode")
		}
	}
	assert.True(t, foundCheckEnv, "expected bin to include check-env.sh")

	// Instructions
	require.Len(t, ir.Instructions, 1)
	assert.Contains(t, ir.Instructions[0].Content, "Tool Integration")

	// MCP
	require.NotNil(t, ir.MCP)
	assert.Contains(t, ir.MCP, "mcpServers")

	// Hooks
	require.NotNil(t, ir.Hooks)
	assert.Contains(t, ir.Hooks, "UserPromptSubmit")
}

func TestKeelDSLReader_NonexistentLayer(t *testing.T) {
	t.Parallel()
	reader := &KeelDSLReader{Offline: true}

	// A nonexistent layer directory should still succeed because
	// processDSLLayer returns defaults when keel.yml is absent.
	// But the result should be essentially empty.
	layerPath := filepath.Join(t.TempDir(), "nonexistent")
	require.NoError(t, os.MkdirAll(layerPath, 0755))

	ir, err := reader.Read(layerPath)
	require.NoError(t, err)
	assert.Equal(t, "keel", ir.SourceAgent)
	assert.Empty(t, ir.Instructions)
	assert.Nil(t, ir.MCP)
	assert.Nil(t, ir.Hooks)
	assert.Nil(t, ir.Permissions)
	assert.Empty(t, ir.Categories)
}

func TestKeelDSLReader_Registration(t *testing.T) {
	t.Parallel()
	reader, err := GetReader("keel")
	require.NoError(t, err)
	assert.Equal(t, "keel", reader.AgentName())
}

func TestKeelDSLReader_RegisterDirForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create files in subdirectories to simulate a category directory tree.
	testutil.WriteFile(t, dir, "deploy.md", []byte("# Deploy"))
	testutil.WriteFile(t, dir, "review.md", []byte("# Review"))
	testutil.WriteFile(t, dir, "sub/nested.md", []byte("# Nested"))

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.registerDirForce(dir, "commands", ir)

	require.NotNil(t, ir.FilePolicies, "FilePolicies should be populated")

	// All three files should have force: true entries keyed by category/relpath.
	expectedKeys := []string{
		"commands/deploy.md",
		"commands/review.md",
		"commands/sub/nested.md",
	}
	for _, key := range expectedKeys {
		fp, ok := ir.FilePolicies[key]
		assert.True(t, ok, "expected FilePolicies to contain %q", key)
		assert.True(t, fp.Force, "expected Force=true for %q", key)
	}

	assert.Len(t, ir.FilePolicies, len(expectedKeys),
		"FilePolicies should contain exactly the walked files")
}

func TestKeelDSLReader_RegisterDirForce_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.registerDirForce(dir, "commands", ir)

	assert.Nil(t, ir.FilePolicies, "empty dir should not create FilePolicies")
}

func TestKeelDSLReader_ReadBinFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := []byte("#!/bin/bash\necho hello\n")
	testutil.WriteFile(t, dir, "my-script.sh", content)
	filePath := filepath.Join(dir, "my-script.sh")

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readBinFile(filePath, "my-script.sh", ir)

	require.Len(t, ir.Bin, 1)
	assert.Equal(t, "my-script.sh", ir.Bin[0].RelPath)
	assert.Equal(t, content, ir.Bin[0].Content)
	assert.NotZero(t, ir.Bin[0].Mode, "Mode should be captured from stat")
}

func TestKeelDSLReader_ReadBinFile_ExecutePermission(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := []byte("#!/bin/bash\necho exec\n")
	filePath := filepath.Join(dir, "exec-script.sh")
	testutil.WriteFile(t, dir, "exec-script.sh", content)
	require.NoError(t, os.Chmod(filePath, 0755))

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readBinFile(filePath, "tools/exec-script.sh", ir)

	require.Len(t, ir.Bin, 1)
	assert.Equal(t, "tools/exec-script.sh", ir.Bin[0].RelPath)
	assert.Equal(t, content, ir.Bin[0].Content)
	// Verify execute bits are captured.
	assert.NotZero(t, ir.Bin[0].Mode&0111, "execute permission bits should be set")
}

func TestKeelDSLReader_ReadBinFile_Nonexistent(t *testing.T) {
	t.Parallel()
	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readBinFile("/nonexistent/path/script.sh", "script.sh", ir)

	assert.Nil(t, ir.Bin, "nonexistent file should not add to ir.Bin")
}

func TestKeelDSLReader_ReadSnippets(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	// Create a snippets directory with .md files.
	snippetsDir := filepath.Join(layerDir, "snippets")
	testutil.WriteFile(t, snippetsDir, "greeting.md", []byte("Hello, World!"))
	testutil.WriteFile(t, snippetsDir, "farewell.md", []byte("Goodbye!"))
	// Non-.md file should be ignored by LoadSnippetsFromDir.
	testutil.WriteFile(t, snippetsDir, "notes.txt", []byte("ignored"))

	cfg := &schema.KeelConfig{
		Snippets: []schema.FileRef{
			{Source: "snippets"},
		},
	}

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readSnippets(layerDir, cfg, ir)

	require.NotNil(t, ir.Snippets)
	assert.Len(t, ir.Snippets, 2)
	assert.Equal(t, []byte("Hello, World!"), ir.Snippets["greeting"])
	assert.Equal(t, []byte("Goodbye!"), ir.Snippets["farewell"])
	_, hasNotes := ir.Snippets["notes"]
	assert.False(t, hasNotes, "non-.md files should not be loaded as snippets")
}

func TestKeelDSLReader_ReadSnippets_SourceFile(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	// Create a single snippet file referenced by source.
	testutil.WriteFile(t, layerDir, "my-snippet.md", []byte("snippet content"))

	cfg := &schema.KeelConfig{
		Snippets: []schema.FileRef{
			{Source: "my-snippet.md"},
		},
	}

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readSnippets(layerDir, cfg, ir)

	require.NotNil(t, ir.Snippets)
	assert.Equal(t, []byte("snippet content"), ir.Snippets["my-snippet"])
}

func TestKeelDSLReader_ReadSnippets_SourceDir(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	// Create a snippets directory referenced by source.
	testutil.WriteFile(t, layerDir, "snips/alpha.md", []byte("alpha"))
	testutil.WriteFile(t, layerDir, "snips/beta.md", []byte("beta"))

	cfg := &schema.KeelConfig{
		Snippets: []schema.FileRef{
			{Source: "snips"},
		},
	}

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readSnippets(layerDir, cfg, ir)

	require.NotNil(t, ir.Snippets)
	assert.Len(t, ir.Snippets, 2)
	assert.Equal(t, []byte("alpha"), ir.Snippets["alpha"])
	assert.Equal(t, []byte("beta"), ir.Snippets["beta"])
}

func TestKeelDSLReader_ReadSnippets_Empty(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	cfg := &schema.KeelConfig{} // no snippets

	reader := &KeelDSLReader{}
	ir := &LayerIR{}

	reader.readSnippets(layerDir, cfg, ir)

	assert.Nil(t, ir.Snippets, "no snippets config should leave ir.Snippets nil")
}

func TestKeelDSLReader_FirstClassClaudeSettings(t *testing.T) {
	t.Parallel()
	layerDir := t.TempDir()

	keelYML := `
schema: v1
name: First-Class Settings Test
settings:
  model: opus
  effort-level: high
  always-thinking-enabled: true
  disable-all-hooks: false
  enabled-mcpjson-servers:
    - "context7"
  include-git-instructions: true
  pr-url-template: "https://gitlab.example.com/-/merge_requests"
  output-style: concise
  language: en
  show-turn-duration: true
  fast-mode: true
  auto-memory-enabled: true
  default-shell: /bin/zsh
  respect-gitignore: true
  teammate-mode: auto
  cleanup-period-days: 30
  voice:
    enabled: false
  status-line:
    type: command
    command: "echo ready"
  file-suggestion:
    type: command
    command: "find . -name '*.go'"
  auto-updates-channel: stable
  force-login-method: console
`
	testutil.WriteFile(t, layerDir, "keel.yml", []byte(keelYML))

	reader := &KeelDSLReader{Offline: true}
	ir, err := reader.Read(layerDir)
	require.NoError(t, err)

	// ALL Settings struct fields are now portable and go to ir.Settings
	require.NotNil(t, ir.Settings)
	assert.Equal(t, "opus", ir.Settings["model"])

	// Agent/Model
	assert.Equal(t, "high", ir.Settings["effortLevel"])
	assert.Equal(t, true, ir.Settings["alwaysThinkingEnabled"])

	// Hooks meta
	assert.Equal(t, false, ir.Settings["disableAllHooks"])

	// MCP controls
	servers, ok := ir.Settings["enabledMcpjsonServers"].([]any)
	require.True(t, ok, "expected enabledMcpjsonServers to be []any")
	assert.Contains(t, servers, "context7")

	// Git
	assert.Equal(t, true, ir.Settings["includeGitInstructions"])
	assert.Equal(t, "https://gitlab.example.com/-/merge_requests", ir.Settings["prUrlTemplate"])

	// UI
	assert.Equal(t, "concise", ir.Settings["outputStyle"])
	assert.Equal(t, "en", ir.Settings["language"])
	assert.Equal(t, true, ir.Settings["showTurnDuration"])

	// Features
	assert.Equal(t, true, ir.Settings["fastMode"])
	assert.Equal(t, true, ir.Settings["autoMemoryEnabled"])

	// Misc
	assert.Equal(t, "/bin/zsh", ir.Settings["defaultShell"])
	assert.Equal(t, true, ir.Settings["respectGitignore"])
	assert.Equal(t, "auto", ir.Settings["teammateMode"])
	assert.Equal(t, float64(30), ir.Settings["cleanupPeriodDays"])

	// Grouped: voice (nested map)
	voice, ok := ir.Settings["voice"].(map[string]any)
	require.True(t, ok, "expected voice to be a map")
	assert.Equal(t, false, voice["enabled"])

	// Grouped: statusLine
	sl, ok := ir.Settings["statusLine"].(map[string]any)
	require.True(t, ok, "expected statusLine to be a map")
	assert.Equal(t, "command", sl["type"])
	assert.Equal(t, "echo ready", sl["command"])

	// Grouped: fileSuggestion
	fs, ok := ir.Settings["fileSuggestion"].(map[string]any)
	require.True(t, ok, "expected fileSuggestion to be a map")
	assert.Equal(t, "command", fs["type"])

	// Updates (flat)
	assert.Equal(t, "stable", ir.Settings["autoUpdatesChannel"])
	_, hasUpdates := ir.Settings["updates"]
	assert.False(t, hasUpdates, "updates should be flattened, not nested")

	// Auth (flat)
	assert.Equal(t, "console", ir.Settings["forceLoginMethod"])
	_, hasAuth := ir.Settings["auth"]
	assert.False(t, hasAuth, "auth should be flattened, not nested")
}

func TestDecomposeSettingsIntoIR_MalformedHooks(t *testing.T) {
	t.Parallel()

	// When hooks is a string instead of a map, it should be silently dropped
	ir := &LayerIR{}
	data := map[string]any{
		"hooks": "not-a-map",
		"other": "value",
	}
	decomposeSettingsIntoIR(ir, "claude", data)
	assert.Nil(t, ir.Hooks, "malformed hooks should not be set on IR")
	assert.Equal(t, "value", ir.AgentSpecific["claude"]["other"])

	// When hooks is an array, it should also be silently dropped
	ir2 := &LayerIR{}
	data2 := map[string]any{
		"hooks": []any{"not", "a", "map"},
	}
	decomposeSettingsIntoIR(ir2, "claude", data2)
	assert.Nil(t, ir2.Hooks)
}

func TestDecomposeSettingsIntoIR_ValidHooks(t *testing.T) {
	t.Parallel()

	// When hooks is a valid map, it should be extracted to ir.Hooks
	ir := &LayerIR{}
	data := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "command", "command": "echo test"},
			},
		},
		"model": "sonnet",
	}
	decomposeSettingsIntoIR(ir, "claude", data)
	require.NotNil(t, ir.Hooks)
	assert.Contains(t, ir.Hooks, "PreToolUse")
	assert.Equal(t, "sonnet", ir.AgentSpecific["claude"]["model"])
	// hooks key should be removed from data
	_, hasHooks := ir.AgentSpecific["claude"]["hooks"]
	assert.False(t, hasHooks, "hooks should be removed from AgentSpecific after extraction")
}

// TestExtractPortableSettings_AllFieldsPortable validates the settings portability model.
// It creates a Settings struct with known fields set, calls extractPortableSettings,
// and verifies that all struct fields serialize to the output map. This catches scenarios
// where a new field is added to Settings but doesn't serialize properly.
func TestExtractPortableSettings_AllFieldsPortable(t *testing.T) {
	t.Parallel()

	// Create a fully-populated Settings struct with non-zero values for all fields.
	trueVal := true
	falseVal := false

	settings := &schema.Settings{
		Model:                      "claude-sonnet-4",
		EffortLevel:                "high",
		Agent:                      "claude",
		AvailableModels:            []string{"model-a", "model-b"},
		ModelOverrides:             map[string]string{"key": "value"},
		AlwaysThinkingEnabled:      &trueVal,
		DisableAllHooks:            &falseVal,
		AllowManagedHooksOnly:      &trueVal,
		AllowedHttpHookUrls:        []string{"https://example.com"},
		HttpHookAllowedEnvVars:     []string{"VAR1"},
		EnabledMcpjsonServers:      []string{"server-a"},
		DisabledMcpjsonServers:     []string{"server-b"},
		AllowManagedMcpServersOnly: &trueVal,
		ApiKeyHelper:               "helper-script",
		AwsCredentialExport:        "aws-export",
		AwsAuthRefresh:             "aws-refresh",
		GcpAuthRefresh:             "gcp-refresh",
		OtelHeadersHelper:          "otel-helper",
		IncludeGitInstructions:     &trueVal,
		PrUrlTemplate:              "https://github.com/org/repo/pull/{number}",
		EditorMode:                 "normal",
		Tui:                        &trueVal,
		ViewMode:                   "split",
		OutputStyle:                "detailed",
		Language:                   "en",
		SpinnerTipsEnabled:         &trueVal,
		AutoScrollEnabled:          &falseVal,
		Env:                        map[string]string{"KEY": "value"},
		Worktree: &schema.WorktreeConfig{
			BaseRef: "main",
		},
		SkillOverrides: map[string]string{"skill": "path"},
		AutoMode: &schema.AutoModeConfig{
			Environment: []string{"prod"},
		},
		// Sandbox and Attribution are handled separately on IR
		Sandbox: &schema.SandboxConfig{
			Enabled: &trueVal,
		},
		Attribution: &schema.AttributionConfig{
			Commit: "commit-template",
			PR:     "pr-template",
		},
		// File references are not portable
		Claude: "settings.claude.json",
		Cursor: "settings.cursor.json",
	}

	reader := &KeelDSLReader{}
	result := reader.extractPortableSettings(settings)

	// Verify that result is non-nil and contains entries.
	require.NotNil(t, result, "extractPortableSettings should return a map")
	assert.NotEmpty(t, result, "extractPortableSettings should return non-empty map for populated settings")

	// Verify key fields are present (sampling a few to confirm serialization works).
	assert.Equal(t, "claude-sonnet-4", result["model"], "model should be present")
	assert.Equal(t, "high", result["effortLevel"], "effortLevel should be present (camelCase)")
	assert.Equal(t, "claude", result["agent"], "agent should be present")
	assert.Equal(t, trueVal, result["alwaysThinkingEnabled"], "alwaysThinkingEnabled should be present")
	assert.Equal(t, falseVal, result["disableAllHooks"], "disableAllHooks should be present")

	// Verify that file references are NOT in the result (they're agent-specific, not portable).
	_, hasClaudeRef := result["claude"]
	assert.False(t, hasClaudeRef, "claude file reference should not be in portable settings")
	_, hasCursorRef := result["cursor"]
	assert.False(t, hasCursorRef, "cursor file reference should not be in portable settings")

	// Verify that sandbox and attribution are NOT in the result (they're set on IR directly).
	_, hasSandbox := result["sandbox"]
	assert.False(t, hasSandbox, "sandbox should not be in portable settings (set on IR.Sandbox)")
	_, hasAttribution := result["attribution"]
	assert.False(t, hasAttribution, "attribution should not be in portable settings (set on IR.Attribution)")

	// Count non-zero fields in the Settings struct using reflection, excluding
	// the passthrough fields (Claude, Cursor, Sandbox, Attribution).
	expectedFieldCount := 0
	settingsVal := reflect.ValueOf(settings).Elem()
	settingsType := settingsVal.Type()
	for i := 0; i < settingsVal.NumField(); i++ {
		field := settingsVal.Field(i)
		fieldName := settingsType.Field(i).Name

		// Skip passthrough fields handled separately.
		if fieldName == "Claude" || fieldName == "Cursor" || fieldName == "Sandbox" || fieldName == "Attribution" {
			continue
		}

		if !field.IsZero() {
			expectedFieldCount++
		}
	}

	// The result map should have at least as many keys as non-zero portable fields.
	assert.GreaterOrEqual(t, len(result), expectedFieldCount,
		"extractPortableSettings should serialize all non-zero portable fields")
}
