package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// --- Format detection tests (table-driven) ---

func TestFormatDetection(t *testing.T) {
	t.Parallel()
	td := testdataDir(t)
	tests := []struct {
		name     string
		path     string
		expected config.LayerFormat
	}{
		{"V1Claude", filepath.Join(td, "layers-native-policy", "company-claude"), config.LayerFormat("claude")},
		{"V1Cursor", filepath.Join(td, "layers-native-policy", "company-cursor"), config.LayerFormat("cursor")},
		{"V1DSL", filepath.Join(td, "layers-force", "company"), config.FormatKeel},
		{"NoKeelYml", filepath.Join(td, "layers-native-policy", "project"), config.FormatKeel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			format, _ := config.DetectLayerFormat(tt.path)
			assert.Equal(t, tt.expected, format)
		})
	}
}

func TestCompileClaude_NativeWithPolicy(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-native-policy", "company-claude", "project")
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings), "parsing settings.json")
	perms := settings["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 2, "expected 2 permissions from force-locked company settings")

	scanContent, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "security-scan.md"))
	require.NoError(t, err, "reading security-scan.md")
	assert.Contains(t, string(scanContent), "force-locked")
	assert.NotContains(t, string(scanContent), "should be blocked")

	buildPath := filepath.Join(outputDir, ".claude", "commands", "build.md")
	_, err = os.Stat(buildPath)
	assert.NoError(t, err, "expected build.md from project layer to exist")

	claudeMD, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(claudeMD), "Company Standards")

	foundSettings := false
	foundCommand := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "settings" && fl.Key == "settings.json" {
			foundSettings = true
		}
		if fl.Category == "command" && fl.Key == "security-scan.md" {
			foundCommand = true
		}
	}
	assert.True(t, foundSettings, "expected force-lock manifest entry for settings:settings.json")
	assert.True(t, foundCommand, "expected force-lock manifest entry for command:security-scan.md")
}

func TestCompileCursor_TranspilesClaudeLayer(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-native-policy", "company-claude", "project")
	outputDir := t.TempDir()

	_, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	mdcPath := filepath.Join(outputDir, ".cursor", "rules", "00-keel.mdc")
	content, err := os.ReadFile(mdcPath)
	require.NoError(t, err, "expected 00-keel.mdc to exist after transpiling claude layer")
	assert.Contains(t, string(content), "Company Standards")
}

func TestCompileClaude_SkipsCursorOnlyContent(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-native-policy", "company-cursor", "project")
	outputDir := t.TempDir()

	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Cursor-only file: cli-config.json should NOT appear in Claude output.
	cliConfigPath := filepath.Join(outputDir, ".claude", "cli-config.json")
	_, err = os.Stat(cliConfigPath)
	assert.True(t, os.IsNotExist(err), "claude compiler should not produce cli-config.json from v1-cursor layer")

	// Rules: both Claude and Cursor support rules. Cross-compiled rules from
	// a Cursor layer should appear in Claude output (portable category).
	rulesDir := filepath.Join(outputDir, ".claude", "rules")
	_, err = os.Stat(rulesDir)
	assert.False(t, os.IsNotExist(err), "rules should transfer from cursor layer to claude (both support rules)")
}

func TestCompile_MixedFormats(t *testing.T) {
	t.Parallel()
	td := testdataDir(t)
	layer1 := config.Layer{
		Path:   filepath.Join(td, "layers-native-policy", "company-claude"),
		Type:   "company-claude",
		Format: config.LayerFormat("claude"),
	}
	layer2 := config.Layer{
		Path:   filepath.Join(td, "layers-force", "company"),
		Type:   "company-dsl",
		Format: config.FormatKeel,
	}
	layer3 := config.Layer{
		Path:   filepath.Join(td, "layers-native-policy", "project"),
		Type:   "project",
		Format: config.FormatKeel,
	}
	layers := []config.Layer{layer1, layer2, layer3}
	outputDir := t.TempDir()

	result, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	claudeMD, err := os.ReadFile(filepath.Join(outputDir, "CLAUDE.md"))
	require.NoError(t, err, "reading CLAUDE.md")
	assert.Contains(t, string(claudeMD), "Company Standards")

	settingsData, err := os.ReadFile(filepath.Join(outputDir, ".claude", "settings.json"))
	require.NoError(t, err, "reading settings.json")
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings), "parsing settings.json")

	scanContent, err := os.ReadFile(filepath.Join(outputDir, ".claude", "commands", "security-scan.md"))
	require.NoError(t, err, "reading security-scan.md")
	assert.Contains(t, string(scanContent), "force-locked")

	buildPath := filepath.Join(outputDir, ".claude", "commands", "build.md")
	_, err = os.Stat(buildPath)
	assert.NoError(t, err, "expected build.md from native layer")

	assert.NotEmpty(t, result.Governance.ForceLocked, "expected force-locked entries in compile result")
}

func TestCompileCursor_NativeWithPolicy(t *testing.T) {
	t.Parallel()
	layers := buildTestLayers(t, "layers-native-policy", "company-cursor", "project")
	outputDir := t.TempDir()

	result, err := Compile("cursor", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	cliConfigData, err := os.ReadFile(filepath.Join(outputDir, ".cursor", "cli-config.json"))
	require.NoError(t, err, "reading cli-config.json")
	var cliConfig map[string]any
	require.NoError(t, json.Unmarshal(cliConfigData, &cliConfig), "parsing cli-config.json")
	permsAllow, ok := cliConfig["permissions.allow"].([]any)
	require.True(t, ok, "expected permissions.allow in cli-config.json")
	assert.Len(t, permsAllow, 2, "expected 2 permissions from force-locked company cli-config")

	pciPath := filepath.Join(outputDir, ".cursor", "rules", "pci-compliance.mdc")
	pciContent, err := os.ReadFile(pciPath)
	require.NoError(t, err, "reading pci-compliance.mdc")
	assert.Contains(t, string(pciContent), "force-locked")

	foundCLI := false
	foundRule := false
	for _, fl := range result.Governance.ForceLocked {
		if fl.Category == "settings" && fl.Key == "cli-config.json" {
			foundCLI = true
		}
		if fl.Category == "rule" && fl.Key == "pci-compliance.mdc" {
			foundRule = true
		}
	}
	assert.True(t, foundCLI, "expected force-lock manifest entry for settings:cli-config.json")
	assert.True(t, foundRule, "expected force-lock manifest entry for rule:pci-compliance.mdc")
}

func TestCategorizeFilePath(t *testing.T) {
	t.Parallel()
	categories := []CategoryDef{
		{"commands", "command"},
		{"agents", "agent"},
		{"skills", "skill"},
		{"rules", "rule"},
	}
	tests := []struct {
		path     string
		expected string
	}{
		{"commands/deploy.md", "command"},
		{"commands/security-scan.md", "command"},
		{"agents/code-reviewer.md", "agent"},
		{"skills/api-design/SKILL.md", "skill"},
		{"rules/pci-compliance.mdc", "rule"},
		{"settings.json", "settings"},
		{"settings.local.json", "settings"},
		{"mcp.json", "mcp-file"},
		{".mcp.json", "mcp-file"},
		{"hooks.json", "hooks-file"},
		{"random-file.txt", "file"},
		{"instructions.md", "file"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := categorizeFilePath(tt.path, categories)
			assert.Equal(t, tt.expected, got)
		})
	}
}
