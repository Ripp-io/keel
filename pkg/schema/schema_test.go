package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBytes_FullConfig(t *testing.T) {
	t.Parallel()
	yaml := `
schema: v1
name: Test Project
version: v1.0.0
description: A test project

instructions:
  - source: overview.md
  - source: security/
  - source: team-conventions.md
    force: true

permissions:
  rules:
    - source: permissions.json
    - source: extra-permissions.json
      force: true

mcp:
  servers:
    - source: mcp.json

commands:
  - source: commands/
  - source: extra/deploy.md
    force: true

agents:
  - source: agents/
  - source: agents/code-reviewer.md
    force: true

skills:
  - source: skills/security-review/
    force: true

hooks:
  - source: hooks.json

init:
  - source: init.d/

settings:
  model: sonnet
  claude: settings.claude.json
  cursor: settings.cursor.json

repos:
  party-service:
    url: git@example.com:party-service.git
    branch: main
    required: true
    force: true
  shared-libs:
    url: git@example.com:shared-libs.git
    branch: develop
    required: false
`

	cfg, err := ParseBytes([]byte(yaml))
	require.NoError(t, err)

	// Top-level fields
	assert.Equal(t, "v1", cfg.Schema)
	assert.Equal(t, "Test Project", cfg.Name)
	assert.Equal(t, "v1.0.0", cfg.Version)
	assert.Equal(t, "A test project", cfg.Description)

	// Instructions
	require.Len(t, cfg.Instructions, 3)
	assert.Equal(t, "overview.md", cfg.Instructions[0].Source)
	assert.Equal(t, "security/", cfg.Instructions[1].Source)
	assert.True(t, cfg.Instructions[2].Force)

	// Permissions
	require.NotNil(t, cfg.Permissions)
	require.Len(t, cfg.Permissions.Rules, 2)
	assert.Equal(t, "permissions.json", cfg.Permissions.Rules[0].Source)
	assert.True(t, cfg.Permissions.Rules[1].Force)

	// MCP
	require.NotNil(t, cfg.MCP)
	require.Len(t, cfg.MCP.Servers, 1)
	assert.Equal(t, "mcp.json", cfg.MCP.Servers[0].Source)

	// Commands
	require.Len(t, cfg.Commands, 2)
	assert.Equal(t, "commands/", cfg.Commands[0].Source)
	assert.True(t, cfg.Commands[1].Force)

	// Agents
	assert.Len(t, cfg.Agents, 2)

	// Skills
	require.Len(t, cfg.Skills, 1)
	assert.True(t, cfg.Skills[0].Force)

	// Hooks
	require.Len(t, cfg.Hooks, 1)
	assert.Equal(t, "hooks.json", cfg.Hooks[0].Source)

	// Init
	require.Len(t, cfg.Init, 1)
	assert.Equal(t, "init.d/", cfg.Init[0].Source)

	// Settings
	require.NotNil(t, cfg.Settings)
	assert.Equal(t, "sonnet", cfg.Settings.Model)
	assert.Equal(t, "settings.claude.json", cfg.Settings.Claude)
	assert.Equal(t, "settings.cursor.json", cfg.Settings.Cursor)

	// Repos
	require.Len(t, cfg.Repos, 2)
	ps, ok := cfg.Repos["party-service"]
	require.True(t, ok, "Repos missing key 'party-service'")
	require.NotNil(t, ps.Required)
	assert.True(t, *ps.Required)
	assert.True(t, ps.Force)
}

func TestParseBytes_MinimalConfig(t *testing.T) {
	t.Parallel()
	yaml := `
schema: v1
name: Minimal
`
	cfg, err := ParseBytes([]byte(yaml))
	require.NoError(t, err)

	assert.Equal(t, "v1", cfg.Schema)
	assert.Equal(t, "Minimal", cfg.Name)
	assert.Nil(t, cfg.Instructions)
	assert.Nil(t, cfg.Permissions)
	assert.Nil(t, cfg.MCP)
	assert.Nil(t, cfg.Hooks)
}

func TestParseBytes_InvalidYAML(t *testing.T) {
	t.Parallel()
	_, err := ParseBytes([]byte(":::invalid"))
	assert.Error(t, err)
}

func TestParse_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := Parse("/nonexistent/keel.yml")
	assert.Error(t, err)
}

func TestParse_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keelPath := filepath.Join(dir, "keel.yml")
	require.NoError(t, os.WriteFile(keelPath, []byte("schema: v1\nname: FromFile\n"), 0644))

	cfg, err := Parse(keelPath)
	require.NoError(t, err)
	assert.Equal(t, "FromFile", cfg.Name)
}

func TestParseDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := ParseDefaults()
	require.NoError(t, err)

	assert.Equal(t, "v1", cfg.Schema)
	require.Len(t, cfg.Instructions, 2)
	assert.Equal(t, "instructions/", cfg.Instructions[0].Source)
	assert.Equal(t, "instructions.md", cfg.Instructions[1].Source)
	require.Len(t, cfg.Commands, 1)
	assert.Equal(t, "commands/", cfg.Commands[0].Source)
}

func TestMergeWithDefaults_UserOverridesInstructions(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{
		Schema: "v1",
		Name:   "My Project",
		Instructions: []FileRef{
			{Source: "custom.md"},
		},
	}

	merged := MergeWithDefaults(user)

	assert.Equal(t, "v1", merged.Schema)
	assert.Equal(t, "My Project", merged.Name)
	// User overrides instructions
	require.Len(t, merged.Instructions, 1)
	assert.Equal(t, "custom.md", merged.Instructions[0].Source)
	// Defaults preserved for unspecified fields
	require.Len(t, merged.Commands, 1)
	assert.Equal(t, "commands/", merged.Commands[0].Source)
}

func TestMergeWithDefaults_EmptyUserGetsDefaults(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{}

	merged := MergeWithDefaults(user)

	assert.Equal(t, "v1", merged.Schema)
	assert.Len(t, merged.Instructions, 2)
	assert.Len(t, merged.Commands, 1)
	assert.Len(t, merged.Agents, 1)
	assert.Len(t, merged.Skills, 1)
	assert.Len(t, merged.Init, 1)
}

func TestMergeWithDefaults_UserPermissionsOverrideDefaults(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{
		Permissions: &PermissionsConfig{
			Rules: []FileRef{
				{Source: "permissions.json"},
				{Source: "extra.json"},
			},
		},
	}

	merged := MergeWithDefaults(user)

	require.NotNil(t, merged.Permissions)
	require.Len(t, merged.Permissions.Rules, 2)
	assert.Equal(t, "permissions.json", merged.Permissions.Rules[0].Source)
}

func TestMergeWithDefaults_UserMCPOverridesDefaults(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{
		MCP: &MCPConfig{Servers: []FileRef{{Source: "mcp.json"}}},
	}

	merged := MergeWithDefaults(user)

	require.NotNil(t, merged.MCP)
	require.Len(t, merged.MCP.Servers, 1)
	assert.Equal(t, "mcp.json", merged.MCP.Servers[0].Source)
}

func TestMergeWithDefaults_UserSettingsOverrideDefaults(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{
		Settings: &Settings{
			Model:  "opus",
			Claude: "settings.claude.json",
		},
	}

	merged := MergeWithDefaults(user)

	require.NotNil(t, merged.Settings)
	assert.Equal(t, "opus", merged.Settings.Model)
	assert.Equal(t, "settings.claude.json", merged.Settings.Claude)
}

func TestParseBytes_MCPStructuredConfig(t *testing.T) {
	t.Parallel()
	yaml := `
mcp:
  servers:
    - source: mcp.json
`
	cfg, err := ParseBytes([]byte(yaml))
	require.NoError(t, err)

	require.NotNil(t, cfg.MCP)
	require.Len(t, cfg.MCP.Servers, 1)
	assert.Equal(t, "mcp.json", cfg.MCP.Servers[0].Source)
}

func TestValidate_Errors(t *testing.T) {
	t.Parallel()
	longURL := string(make([]byte, 2049))

	manyInstructions := make([]FileRef, 1001)
	manyPermRules := make([]FileRef, 101)
	manyMCPServers := make([]FileRef, 101)

	tests := []struct {
		name    string
		config  KeelConfig
		wantErr string
	}{
		{
			name: "MCP servers list exceeds max",
			config: KeelConfig{
				MCP: &MCPConfig{Servers: manyMCPServers},
			},
			wantErr: "exceeding maximum of 100",
		},
		{
			name: "hooks list exceeds max",
			config: KeelConfig{
				Hooks: make([]FileRef, 101),
			},
			wantErr: "exceeding maximum of 100",
		},
		{
			name: "instructions exceed max",
			config: KeelConfig{
				Instructions: manyInstructions,
			},
			wantErr: "exceeding maximum of 1000",
		},
		{
			name: "permissions rules exceed max",
			config: KeelConfig{
				Permissions: &PermissionsConfig{Rules: manyPermRules},
			},
			wantErr: "exceeding maximum of 100",
		},
		{
			name: "repo URL too long",
			config: KeelConfig{
				Repos: map[string]Repo{"bad": {URL: longURL}},
			},
			wantErr: "exceeds maximum length of 2048",
		},
		{
			name: "plugins list exceeds max",
			config: KeelConfig{
				Plugins: make([]FileRef, 501),
			},
			wantErr: "exceeding maximum of 500",
		},
		{
			name: "agent-memory list exceeds max",
			config: KeelConfig{
				AgentMemory: make([]FileRef, 501),
			},
			wantErr: "exceeding maximum of 500",
		},
		{
			name: "environment list exceeds max",
			config: KeelConfig{
				Environment: make([]FileRef, 101),
			},
			wantErr: "exceeding maximum of 100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidate_RepoPathTraversal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{name: "simple relative", path: "services/party", wantErr: false},
		{name: "single segment", path: "myrepo", wantErr: false},
		{name: "deep nesting", path: "a/b/c/d", wantErr: false},
		{name: "trailing slash", path: "myrepo/", wantErr: false},
		{name: "empty path (unset)", path: "", wantErr: false},

		// Invalid: absolute paths
		{name: "absolute path", path: "/etc/passwd", wantErr: true},
		{name: "absolute root", path: "/", wantErr: true},

		// Invalid: dotdot traversal
		{name: "dotdot at start", path: "../outside", wantErr: true},
		{name: "dotdot in middle", path: "a/../b", wantErr: true},
		{name: "dotdot at end", path: "a/b/..", wantErr: true},
		{name: "bare dotdot", path: "..", wantErr: true},

		// Invalid: single dot segments
		{name: "dot at start", path: "./here", wantErr: true},
		{name: "dot in middle", path: "a/./b", wantErr: true},
		{name: "dot at end", path: "a/b/.", wantErr: true},
		{name: "bare dot", path: ".", wantErr: true},

		// Invalid: empty segments (consecutive slashes)
		{name: "double slash", path: "a//b", wantErr: true},
		{name: "leading double slash", path: "//a", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KeelConfig{
				Repos: map[string]Repo{
					"test-repo": {URL: "git@example.com:test.git", Path: tt.path},
				},
			}
			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err, "expected error for path %q", tt.path)
				assert.Contains(t, err.Error(), "invalid path")
				assert.Contains(t, err.Error(), "test-repo")
			} else {
				assert.NoError(t, err, "unexpected error for path %q", tt.path)
			}
		})
	}
}

func TestValidate_InvalidMergeStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		merge   string
		wantErr bool
	}{
		{name: "valid deep-merge", merge: "deep-merge", wantErr: false},
		{name: "valid replace", merge: "replace", wantErr: false},
		{name: "valid additive", merge: "additive", wantErr: false},
		{name: "empty is valid (omitted)", merge: "", wantErr: false},
		{name: "invalid yolo", merge: "yolo", wantErr: true},
		{name: "invalid REPLACE (case)", merge: "REPLACE", wantErr: true},
		{name: "invalid unknown", merge: "unknown", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KeelConfig{
				Files: map[string]FilePolicy{
					"settings.json": {Merge: tt.merge},
				},
			}
			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err, "expected error for merge strategy %q", tt.merge)
				assert.Contains(t, err.Error(), "invalid merge strategy")
			} else {
				assert.NoError(t, err, "unexpected error for merge strategy %q", tt.merge)
			}
		})
	}
}

func TestValidate_FileRefBothSourceAndPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  KeelConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "source only is valid",
			config: KeelConfig{
				Instructions: []FileRef{{Source: "overview.md"}},
			},
		},
		{
			name: "both empty is valid",
			config: KeelConfig{
				Instructions: []FileRef{{}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_MergeStrategyFromYAML(t *testing.T) {
	t.Parallel()
	yamlInput := `
schema: v1
name: test
files:
  settings.json:
    merge: yolo
`
	_, err := ParseBytes([]byte(yamlInput))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid merge strategy")
}

func TestParse_FileSizeLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "keel.yml")

	// Create a sparse file that exceeds the 10MB limit.
	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = f.Seek(11*1024*1024, 0)
	require.NoError(t, err)
	_, err = f.Write([]byte{0})
	require.NoError(t, err)
	f.Close()

	_, err = Parse(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
}

func TestParseBytes_ExpandedSettings(t *testing.T) {
	t.Parallel()
	yaml := `
schema: v1
name: Settings Test
plugins:
  - source: plugins/
agent-memory:
  - source: memory/
environment:
  - source: environment.json
settings:
  model: opus
  claude: settings.claude.json
  sandbox:
    enabled: true
    auto-allow-bash-if-sandboxed: false
    excluded-commands:
      - rm
      - sudo
    filesystem:
      allow-write:
        - ./src
      deny-read:
        - .env
    network:
      allowed-domains:
        - api.example.com
  env:
    MY_VAR: "hello"
    DEBUG: "1"
  worktree:
    base-ref: main
    symlink-directories:
      - node_modules
    sparse-paths:
      - src/
      - tests/
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
	cfg, err := ParseBytes([]byte(yaml))
	require.NoError(t, err)

	require.Len(t, cfg.Plugins, 1)
	assert.Equal(t, "plugins/", cfg.Plugins[0].Source)
	assert.Len(t, cfg.AgentMemory, 1)
	assert.Len(t, cfg.Environment, 1)

	s := cfg.Settings
	require.NotNil(t, s)
	assert.Equal(t, "opus", s.Model)

	// Sandbox
	require.NotNil(t, s.Sandbox)
	require.NotNil(t, s.Sandbox.Enabled)
	assert.True(t, *s.Sandbox.Enabled)
	require.NotNil(t, s.Sandbox.AutoAllowBashIfSandboxed)
	assert.False(t, *s.Sandbox.AutoAllowBashIfSandboxed)
	assert.Len(t, s.Sandbox.ExcludedCommands, 2)
	require.NotNil(t, s.Sandbox.Filesystem)
	assert.Equal(t, []string{"./src"}, s.Sandbox.Filesystem.AllowWrite)
	require.NotNil(t, s.Sandbox.Network)
	assert.Len(t, s.Sandbox.Network.AllowedDomains, 1)

	// Env
	require.Len(t, s.Env, 2)
	assert.Equal(t, "hello", s.Env["MY_VAR"])

	// Worktree
	require.NotNil(t, s.Worktree)
	assert.Equal(t, "main", s.Worktree.BaseRef)
	assert.Len(t, s.Worktree.SymlinkDirectories, 1)
	assert.Len(t, s.Worktree.SparsePaths, 2)

	// SkillOverrides
	require.Len(t, s.SkillOverrides, 2)
	assert.Equal(t, "off", s.SkillOverrides["deploy"])

	// AutoMode
	require.NotNil(t, s.AutoMode)
	assert.Len(t, s.AutoMode.Environment, 1)
	assert.Len(t, s.AutoMode.Allow, 1)
	assert.Len(t, s.AutoMode.SoftDeny, 1)

	// Attribution
	require.NotNil(t, s.Attribution)
	assert.Equal(t, "Co-Authored-By: Claude", s.Attribution.Commit)
	assert.Equal(t, "Generated by Claude Code", s.Attribution.PR)
}

func TestValidate_NilConfig(t *testing.T) {
	t.Parallel()
	var cfg *KeelConfig
	assert.NoError(t, cfg.Validate())
}

func TestValidate_ValidConfig(t *testing.T) {
	t.Parallel()
	cfg := KeelConfig{
		MCP:          &MCPConfig{Servers: []FileRef{{Source: "mcp.json"}}},
		Instructions: []FileRef{{Source: "test.md"}},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_PermissionsPolicyMerge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		merge   string
		wantErr bool
	}{
		{name: "empty is valid", merge: "", wantErr: false},
		{name: "additive is valid", merge: "additive", wantErr: false},
		{name: "replace is valid", merge: "replace", wantErr: false},
		{name: "invalid value", merge: "override", wantErr: true},
		{name: "invalid ADDITIVE (case)", merge: "ADDITIVE", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KeelConfig{
				Permissions: &PermissionsConfig{
					Policy: &PermissionsPolicy{Merge: tt.merge},
				},
			}
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err, "expected error for merge value %q", tt.merge)
			} else {
				assert.NoError(t, err, "unexpected error for merge value %q", tt.merge)
			}
		})
	}
}

func TestValidate_Deps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		deps    map[string]DepSpec
		wantErr string
	}{
		{
			name:    "valid install dep",
			deps:    map[string]DepSpec{"mytool": {Install: "npm install -g mytool"}},
			wantErr: "",
		},
		{
			name:    "valid download dep",
			deps:    map[string]DepSpec{"mytool": {Download: "https://example.com/tool.tar.gz"}},
			wantErr: "",
		},
		{
			name:    "download with sha256",
			deps:    map[string]DepSpec{"mytool": {Download: "https://example.com/tool.tar.gz", SHA256: "abc123"}},
			wantErr: "",
		},
		{
			name:    "download with executable",
			deps:    map[string]DepSpec{"mytool": {Download: "https://example.com/tool.tar.gz", Executable: "tool"}},
			wantErr: "",
		},
		{
			name:    "empty name",
			deps:    map[string]DepSpec{"": {Install: "go install"}},
			wantErr: "empty dependency name",
		},
		{
			name:    "dot-prefixed name",
			deps:    map[string]DepSpec{".hidden": {Install: "go install"}},
			wantErr: "must not start with '.'",
		},
		{
			name:    "slash in name",
			deps:    map[string]DepSpec{"path/tool": {Install: "go install"}},
			wantErr: "must not contain path separators",
		},
		{
			name:    "backslash in name",
			deps:    map[string]DepSpec{"path\\tool": {Install: "go install"}},
			wantErr: "must not contain path separators",
		},
		{
			name:    "both install and download",
			deps:    map[string]DepSpec{"tool": {Install: "npm i", Download: "https://x.com"}},
			wantErr: "cannot set both install and download",
		},
		{
			name:    "neither install nor download",
			deps:    map[string]DepSpec{"tool": {}},
			wantErr: "must set either install or download",
		},
		{
			name:    "sha256 without download",
			deps:    map[string]DepSpec{"tool": {Install: "npm i", SHA256: "abc"}},
			wantErr: "sha256 requires download",
		},
		{
			name:    "executable without download",
			deps:    map[string]DepSpec{"tool": {Install: "npm i", Executable: "bin"}},
			wantErr: "executable requires download",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KeelConfig{Deps: tt.deps}
			err := cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err, "expected error containing %q", tt.wantErr)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestMergeWithDefaults_SettingsPartialOverride(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{
		Settings: &Settings{
			Model: "haiku",
			Env:   map[string]string{"A": "1"},
		},
	}
	merged := MergeWithDefaults(user)

	assert.Equal(t, "haiku", merged.Settings.Model)
	assert.Equal(t, "1", merged.Settings.Env["A"])
}

func TestMergeWithDefaults_NilUser(t *testing.T) {
	t.Parallel()
	merged := MergeWithDefaults(nil)
	require.NotNil(t, merged)
}

func TestMergeWithDefaults_SettingsStructFields(t *testing.T) {
	t.Parallel()
	user := &KeelConfig{
		Settings: &Settings{
			Sandbox:        &SandboxConfig{ExcludedCommands: []string{"rm"}},
			Worktree:       &WorktreeConfig{BaseRef: "head"},
			AutoMode:       &AutoModeConfig{Allow: []string{"bash"}},
			Attribution:    &AttributionConfig{Commit: "test"},
			SkillOverrides: map[string]string{"foo": "bar"},
		},
	}
	merged := MergeWithDefaults(user)

	assert.NotNil(t, merged.Settings.Sandbox)
	require.NotNil(t, merged.Settings.Worktree)
	assert.Equal(t, "head", merged.Settings.Worktree.BaseRef)
	assert.NotNil(t, merged.Settings.AutoMode)
	assert.NotNil(t, merged.Settings.Attribution)
	assert.NotNil(t, merged.Settings.SkillOverrides)
}

func TestMergeWithDefaults_CoversAllFields(t *testing.T) {
	t.Parallel()
	// Ensure MergeWithDefaults handles every field in KeelConfig.
	// If this test fails, a field was added to KeelConfig without
	// updating MergeWithDefaults. Update the merge logic in
	// defaults.go and then increment the expected count here.
	expectedFields := 26
	actualFields := reflect.TypeOf(KeelConfig{}).NumField()
	assert.Equal(t, expectedFields, actualFields,
		"KeelConfig field count changed — update MergeWithDefaults in defaults.go and this test")
}

func TestParseBytes_Unicode(t *testing.T) {
	t.Parallel()
	yaml := `
schema: v1
name: "层测试"
instructions:
  - source: "指令/说明.md"
`
	cfg, err := ParseBytes([]byte(yaml))
	require.NoError(t, err)

	assert.Equal(t, "层测试", cfg.Name)
	require.Len(t, cfg.Instructions, 1)
	assert.Equal(t, "指令/说明.md", cfg.Instructions[0].Source)
}

// TestSettings_AllFieldsHaveJSONTags verifies every field in Settings and its
// sub-structs has a non-empty json struct tag. This catches "forgot to add
// json tag" errors at test time, keeping the struct definition as the single
// source of truth for output key names.
func TestSettings_AllFieldsHaveJSONTags(t *testing.T) {
	t.Parallel()

	types := []struct {
		name string
		typ  reflect.Type
	}{
		{"Settings", reflect.TypeOf(Settings{})},
		{"SandboxConfig", reflect.TypeOf(SandboxConfig{})},
		{"SandboxFSConfig", reflect.TypeOf(SandboxFSConfig{})},
		{"SandboxNetConfig", reflect.TypeOf(SandboxNetConfig{})},
		{"SandboxRipgrepConfig", reflect.TypeOf(SandboxRipgrepConfig{})},
		{"WorktreeConfig", reflect.TypeOf(WorktreeConfig{})},
		{"AutoModeConfig", reflect.TypeOf(AutoModeConfig{})},
		{"AttributionConfig", reflect.TypeOf(AttributionConfig{})},
		{"VoiceConfig", reflect.TypeOf(VoiceConfig{})},
		{"SSHConfigEntry", reflect.TypeOf(SSHConfigEntry{})},
		{"FileSuggestionConfig", reflect.TypeOf(FileSuggestionConfig{})},
		{"StatusLineConfig", reflect.TypeOf(StatusLineConfig{})},
		{"SpinnerVerbsConfig", reflect.TypeOf(SpinnerVerbsConfig{})},
		{"SpinnerTipsOverrideConfig", reflect.TypeOf(SpinnerTipsOverrideConfig{})},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.typ.NumField(); i++ {
				field := tt.typ.Field(i)
				jsonTag := field.Tag.Get("json")
				assert.NotEmpty(t, jsonTag,
					"%s field %q is missing a json struct tag", tt.name, field.Name)
			}
		})
	}
}
