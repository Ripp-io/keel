package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestInit_NonInteractive(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "my-project", "claude"))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelData), "my-project")
	assert.Contains(t, string(keelData), "schema: v1")
	instrData, err := os.ReadFile(filepath.Join(dir, "instructions.md"))
	require.NoError(t, err)
	assert.Contains(t, string(instrData), "my-project")
	output := stdout.String()
	assert.Contains(t, output, "keel.yml")
	assert.Contains(t, output, "instructions.md")
}

func TestInit_NonInteractive_CursorAgent(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "cursor-project", "cursor"))
	assert.Contains(t, stdout.String(), "cursor")
}

func TestInit_NonInteractive_BothAgent(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "dual-project", "both"))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelData), "dual-project")
	output := stdout.String()
	assert.Contains(t, output, "--output claude=. --output cursor=.", "next-step message should show both outputs")
}

func TestInit_InvalidAgent(t *testing.T) {
	var stdout bytes.Buffer
	err := runInit(strings.NewReader(""), &stdout, t.TempDir(), "my-project", "invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid agent")
}

func TestInit_DefaultName(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader("\n\nN\nN\n"), &stdout, dir, "", ""))
	dirBase := filepath.Base(dir)
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelData), dirBase)
}

func TestInit_DefaultAgent(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader("test-proj\n\nN\nN\n"), &stdout, dir, "", ""))
	assert.Contains(t, stdout.String(), "claude")
}

func TestInit_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "valid-project", "claude"))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	var cfg schema.KeelConfig
	require.NoError(t, yaml.Unmarshal(keelData, &cfg))
	assert.Equal(t, "v1", cfg.Schema)
	assert.Equal(t, "valid-project", cfg.Name)
	require.Len(t, cfg.Instructions, 1)
	assert.Equal(t, "instructions.md", cfg.Instructions[0].Source)
}

func TestInit_InstructionsMD(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "Test App", "claude"))
	data, err := os.ReadFile(filepath.Join(dir, "instructions.md"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "# Test App")
	assert.Contains(t, content, "## Project Context")
	assert.Contains(t, content, "## Tech Stack")
	assert.Contains(t, content, "## Build Commands")
	assert.Contains(t, content, "## Code Conventions")
}

func TestInit_WithRepos(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader("my-app\nclaude\ny\ngateway\ngit@example.com:org/gateway.git\n\nN\n"), &stdout, dir, "", ""))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	content := string(keelData)
	assert.Contains(t, content, "repos:")
	assert.Contains(t, content, "gateway:")
	assert.Contains(t, content, "git@example.com:org/gateway.git")
	assert.NotContains(t, content, "# repos:")
}

func TestInit_WithoutRepos_ShowsCommented(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "no-repos", "claude"))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelData), "# repos:")
}

func TestInit_WithMCP(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader("my-app\nclaude\nN\ny\nmy-tool\nnpx my-tool-server\n\n"), &stdout, dir, "", ""))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	content := string(keelData)
	assert.Contains(t, content, "mcp:")
	assert.Contains(t, content, "my-tool.json")
	mcpData, err := os.ReadFile(filepath.Join(dir, "mcp", "my-tool.json"))
	require.NoError(t, err)
	assert.Contains(t, string(mcpData), "npx my-tool-server")
}

func TestInit_MultipleReposAndMCP(t *testing.T) {
	dir := t.TempDir()
	input := strings.Join([]string{"multi-app", "both", "y", "frontend", "git@ex.com:fe", "backend", "git@ex.com:be", "", "y", "linter", "npx lint-server", "formatter", "npx fmt-server", ""}, "\n")
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(input+"\n"), &stdout, dir, "", ""))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	content := string(keelData)
	assert.Contains(t, content, "frontend:")
	assert.Contains(t, content, "backend:")
	assert.Contains(t, content, "linter.json")
	assert.Contains(t, content, "formatter.json")
	for _, name := range []string{"linter", "formatter"} {
		assert.FileExists(t, filepath.Join(dir, "mcp", name+".json"))
	}
}

func TestInit_PermissionsFileRef(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "perms-test", "claude"))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	content := string(keelData)
	assert.Contains(t, content, "permissions:")
	assert.Contains(t, content, "permissions.json")
	permData, err := os.ReadFile(filepath.Join(dir, "permissions.json"))
	require.NoError(t, err)
	permContent := string(permData)
	assert.Contains(t, permContent, "Read(*)")
	assert.Contains(t, permContent, "Edit(*)")
	assert.Contains(t, permContent, "Write(*)")
	assert.Contains(t, permContent, "Bash(git:*)")
}

func TestInit_CreatesOutputDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "sub", "dir")
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, nested, "nested-project", "claude"))
	assert.FileExists(t, filepath.Join(nested, "keel.yml"))
}

func TestInit_AllAgent(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "all-project", "all"))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(keelData), "all-project")
	output := stdout.String()
	// Should mention all registered agents
	agentNames := config.AgentNames()
	for _, agent := range agentNames {
		assert.Contains(t, output, agent, "should mention agent %s in output", agent)
	}
}

func TestInit_OtherAgents(t *testing.T) {
	agents := []string{"agentsmd", "cline", "copilot", "aider"}
	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			var stdout bytes.Buffer
			require.NoError(t, runInit(strings.NewReader(""), &stdout, dir, "test-project", agent))
			keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
			require.NoError(t, err)
			assert.Contains(t, string(keelData), "test-project")
			output := stdout.String()
			assert.Contains(t, output, agent)
		})
	}
}

func TestValidateAgent_AllRegistered(t *testing.T) {
	// Test all registered agents
	for _, agent := range config.AgentNames() {
		t.Run(agent, func(t *testing.T) {
			assert.NoError(t, validateAgent(agent))
		})
	}
}

func TestValidateAgent_SpecialValues(t *testing.T) {
	assert.NoError(t, validateAgent("both"))
	assert.NoError(t, validateAgent("all"))
}

func TestValidateAgent_Invalid(t *testing.T) {
	err := validateAgent("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid agent")
}

func TestSanitizeMCPName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "my-tool", false},
		{"valid with dots", "my.tool.v2", false},
		{"valid with underscores", "my_tool", false},
		{"valid alphanumeric", "tool123", false},
		{"valid single char", "t", false},
		{"empty name", "", true},
		{"path traversal", "../../etc/evil", true},
		{"double dot only", "..", true},
		{"double dot in middle", "foo..bar", true},
		{"forward slash", "my/tool", true},
		{"backslash", `my\tool`, true},
		{"starts with dot", ".hidden", true},
		{"starts with hyphen", "-tool", true},
		{"special chars", "my tool!", true},
		{"space in name", "my tool", true},
		{"colon in name", "my:tool", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizeMCPName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInit_MCPPathTraversal_Rejected(t *testing.T) {
	var stdout bytes.Buffer
	err := runInit(strings.NewReader("my-app\nclaude\nN\ny\n../../etc/evil\nnpx evil-server\n\n"), &stdout, t.TempDir(), "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MCP server name")
}

func TestInit_RepoWithYAMLSpecialChars(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader("my-app\nclaude\ny\nrepo: \"special\"\ngit@example.com:org/special.git\n\nN\n"), &stdout, dir, "", ""))
	keelData, err := os.ReadFile(filepath.Join(dir, "keel.yml"))
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(keelData, &parsed))
	repos, ok := parsed["repos"].(map[string]any)
	require.True(t, ok, "expected repos to be a map, got %T", parsed["repos"])
	_, exists := repos[`repo: "special"`]
	assert.True(t, exists, "repo name with special chars should be preserved, repos: %v", repos)
}

func TestInit_CobraCommand(t *testing.T) {
	cmd := newInitCmd()
	assert.Equal(t, "init [dir]", cmd.Use)
	for _, flag := range []string{"name", "agent"} {
		assert.NotNil(t, cmd.Flags().Lookup(flag), "expected flag %q", flag)
	}
}

func TestInit_EmptyStdin(t *testing.T) {
	var stdout bytes.Buffer
	require.NoError(t, runInit(strings.NewReader(""), &stdout, t.TempDir(), "eof-test", "claude"))
}

func TestInitOutputIsCompilable(t *testing.T) {
	tests := []struct {
		name          string
		agent         string
		expectedFiles []string
	}{
		{"claude", "claude", []string{"CLAUDE.md", ".claude/.keel.json"}},
		// Cursor reads AGENTS.md natively; no .cursor/rules/00-keel.mdc is written
		// when there's no Cursor-specific conditional content.
		{"cursor", "cursor", []string{"AGENTS.md", ".cursor/.keel.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initDir := t.TempDir()
			var initOut bytes.Buffer
			require.NoError(t, runInit(strings.NewReader(""), &initOut, initDir, "RoundTrip Test", tt.agent))
			for _, f := range []string{"keel.yml", "instructions.md", "permissions.json"} {
				assert.FileExists(t, filepath.Join(initDir, f))
			}
			compileDir := t.TempDir()
			require.NoError(t, runCompile(io.Discard, config.CompileConfig{
				Output: map[string]string{tt.agent: compileDir},
				Layers: []string{initDir},
			}, make(map[string]string), compileFlags{}))
			for _, f := range tt.expectedFiles {
				assert.FileExists(t, filepath.Join(compileDir, f))
			}
		})
	}
}
