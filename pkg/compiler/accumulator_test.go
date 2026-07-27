package compiler

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// testProfile returns a minimal AgentProfile suitable for accumulator tests.
// It mirrors the Claude profile structure without requiring the full adapter.
func testProfile() AgentProfile {
	return AgentProfile{
		Name:              "claude",
		OutputRoot:        ".claude",
		InstructionOutput: "CLAUDE.md",
		SettingsFile:      "settings.json",
		SettingsLockKey:   "settings.json",
		MCPOutput:         ".mcp.json",
		HooksInSettings:   true,
		Categories: []CategoryDef{
			{"commands", "command"},
			{"agents", "agent"},
			{"skills", "skill"},
			{"plugins", "plugin"},
			{"agent-memory", "agent-memory"},
			{"rules", "rule"},
		},
		HookKeyTransform:  func(s string) string { return s },
		ToolNameTransform: func(s string) string { return s },
		Capabilities: Capabilities{
			Hooks:       true,
			MCP:         true,
			Skills:      true,
			Permissions: true,
			Commands:    true,
			Agents:      true,
			Plugins:     true,
			AgentMemory: true,
			Rules:       true,
			Bin:         true,
			Deps:        true,
		},
	}
}

func testOpts() CompileOptions {
	return CompileOptions{
		Scope: ScopeProject,
	}
}

func newTestState() *AccumulationState {
	p := testProfile()
	foldConfig := FoldConfig{
		Categories: p.Categories,
	}
	return NewAccumulationState(foldConfig, testOpts(), "/tmp/test-output", 3)
}

func TestFold_Instructions_Concatenation(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "base.md", Content: "Base instructions"},
		},
	}
	ir2 := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "team.md", Content: "Team instructions"},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	// Before finalization, raw blocks are accumulated.
	assert.Len(t, s.instructions, 2)
	assert.Equal(t, "Base instructions", s.instructions[0].Content)
	assert.Equal(t, "Team instructions", s.instructions[1].Content)

	// After finalization, instructions are concatenated.
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	assert.Contains(t, result.Content.Instructions, "Base instructions")
	assert.Contains(t, result.Content.Instructions, "Team instructions")
}

func TestFold_Settings_DeepMerge(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Settings: map[string]any{
			"model": "opus",
		},
		Sandbox: &schema.SandboxConfig{
			Enabled: new(bool),
		},
	}
	*ir1.Sandbox.Enabled = true
	ir2 := &LayerIR{
		Settings: map[string]any{
			"model": "sonnet",
		},
		Sandbox: &schema.SandboxConfig{
			AutoAllowBashIfSandboxed: new(bool),
		},
	}
	*ir2.Sandbox.AutoAllowBashIfSandboxed = true

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// model: last-writer-wins
	assert.Equal(t, "sonnet", result.Content.Settings["model"])

	// sandbox: deep-merged via portable Sandbox field
	require.NotNil(t, result.Content.Sandbox)
	require.NotNil(t, result.Content.Sandbox.Enabled)
	assert.True(t, *result.Content.Sandbox.Enabled)
	require.NotNil(t, result.Content.Sandbox.AutoAllowBashIfSandboxed)
	assert.True(t, *result.Content.Sandbox.AutoAllowBashIfSandboxed)
}

func TestFold_Hooks_ForceLock(t *testing.T) {
	s := newTestState()

	// Layer 1: defines a hook with force: true
	ir1 := &LayerIR{
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Read",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo base"},
					},
					"force": true,
				},
			},
		},
	}
	// Layer 2: tries to override the same hook — should be skipped
	ir2 := &LayerIR{
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Read",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo override"},
					},
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	// Apply hooks strategy before finalize (mimics what profile.go does).
	s.ApplyHooksStrategy(true) // Claude uses last-writer-wins (hooksInSettings=true)

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Only the base layer's hook should be present (force-locked).
	hookEntries, ok := result.Content.Hooks["PreToolUse"].([]any)
	require.True(t, ok, "PreToolUse should be a slice")

	// The base layer's entry should be there (force field stripped).
	assert.Len(t, hookEntries, 1)
	entry := hookEntries[0].(map[string]any)
	assert.Equal(t, "Read", entry["matcher"])
	// Force field should be stripped.
	_, hasForce := entry["force"]
	assert.False(t, hasForce, "force field should be stripped")
}

func TestFold_Categories_LastWriterWins(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Base deploy")},
			},
		},
	}
	ir2 := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Team deploy")},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Equal(t, "deploy.md", cmdFiles[0].RelPath)
	assert.Equal(t, "Team deploy", string(cmdFiles[0].Content))

	// Overlay provenance should show both layers wrote to this path.
	assert.Equal(t, []string{"base", "team"}, result.Governance.Overlays[".claude/commands/deploy.md"])
}

func TestFold_Categories_ForceLock(t *testing.T) {
	s := newTestState()

	// Layer 1: deploy.md with force: true in frontmatter.
	ir1 := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("---\nforce: true\n---\nBase deploy")},
			},
		},
	}
	// Layer 2: tries to override deploy.md — should be skipped.
	ir2 := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Team deploy")},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Equal(t, "deploy.md", cmdFiles[0].RelPath)
	// After finalization, frontmatter is stripped.
	assert.Equal(t, "Base deploy", string(cmdFiles[0].Content))

	// Only base wrote the file — team was force-locked.
	assert.Equal(t, []string{"base"}, result.Governance.Overlays[".claude/commands/deploy.md"])
}

func TestFold_Variables_KeelAutoProtected(t *testing.T) {
	s := newTestState()

	originalAgent := s.variables[varKeelAgent]
	originalScope := s.variables[varKeelScope]

	ir := &LayerIR{
		Variables: map[string]string{
			"keel.agent":  "evil-agent",
			"keel.scope":  "evil-scope",
			"keel.output": "evil-output",
			"custom.var":  "custom-value",
		},
	}

	require.NoError(t, s.Fold(ir, "untrusted"))

	// Auto-injected vars should be unchanged.
	assert.Equal(t, originalAgent, s.variables[varKeelAgent])
	assert.Equal(t, originalScope, s.variables[varKeelScope])

	// Custom vars should be set.
	assert.Equal(t, "custom-value", s.variables["custom.var"])

	// External vars also cannot override auto-vars.
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(map[string]string{
		"keel.agent":      "external-agent",
		"keel.version":    "external-version",
		"external.custom": "ext-value",
	}))

	assert.Equal(t, originalAgent, s.variables[varKeelAgent])
	assert.Equal(t, "ext-value", s.variables["external.custom"])
}

func TestFold_Deps_FirstOccurrenceWins(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Deps: map[string]schema.DepSpec{
			"mytool": {Install: "curl -L https://example.com/v1", Verify: "mytool --version"},
		},
	}
	ir2 := &LayerIR{
		Deps: map[string]schema.DepSpec{
			"mytool": {Install: "curl -L https://example.com/v2", Verify: "mytool --version"},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	// First occurrence wins — base's dep spec should be kept.
	assert.Equal(t, "curl -L https://example.com/v1", s.deps["mytool"].Install)

	// Dep path variables are injected by ProcessDeps, not Fold.
	// Call ProcessDeps with allowInstall=false to inject variables without installing.
	require.NoError(t, s.ProcessDeps("/tmp/test-output", false, false, nil))

	// Dep path variables should now be injected.
	assert.Contains(t, s.variables, "keel.deps.mytool.path")
	assert.Contains(t, s.variables, "keel.deps.mytool.dir")
}

func TestFinalize_VariableSubstitution(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Variables: map[string]string{
			"project.name": "my-project",
		},
		Instructions: []InstructionBlock{
			{Filename: "base.md", Content: "Project: ${keel:project.name}, Agent: ${keel:keel.agent}"},
		},
		Settings: map[string]any{
			"description": "Built by ${keel:keel.agent}",
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Deploy ${keel:project.name}")},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p) // Must call AgentBuild to set keel.agent
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Instructions should have variables substituted.
	assert.Contains(t, result.Content.Instructions, "Project: my-project")
	assert.Contains(t, result.Content.Instructions, "Agent: claude")

	// Settings should have variables substituted.
	assert.Equal(t, "Built by claude", result.Content.Settings["description"])

	// Category files should have variables substituted.
	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Equal(t, "Deploy my-project", string(cmdFiles[0].Content))
}

func TestFinalize_FrontmatterStripping(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("---\nname: deployer\nforce: true\n---\nDeploy content")},
				{RelPath: "review.md", Content: []byte("---\nname: reviewer\n---\nReview content")},
				{RelPath: "build.md", Content: []byte("No frontmatter content")},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 3)

	// Find each file by RelPath.
	fileMap := make(map[string]string)
	for _, f := range cmdFiles {
		fileMap[f.RelPath] = string(f.Content)
	}

	// deploy.md: force stripped, name preserved.
	deployContent := fileMap["deploy.md"]
	assert.NotContains(t, deployContent, "force: true")
	assert.Contains(t, deployContent, "name: deployer")
	assert.Contains(t, deployContent, "Deploy content")

	// review.md: no Keel fields — content should be unchanged.
	reviewContent := fileMap["review.md"]
	assert.Contains(t, reviewContent, "name: reviewer")
	assert.Contains(t, reviewContent, "Review content")

	// build.md: no frontmatter — content should be unchanged.
	assert.Equal(t, "No frontmatter content", fileMap["build.md"])
}

func TestFold_Metadata_FirstWriterWins(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Name:        "base-project",
		Description: "Base description",
		LayerPath:   "/layers/base",
		Repos: map[string]schema.Repo{
			"core": {URL: "https://example.com/core.git", Path: "core"},
		},
	}
	ir2 := &LayerIR{
		Name:        "team-project",
		Description: "Team description",
		LayerPath:   "/layers/team",
		Repos: map[string]schema.Repo{
			"extra": {URL: "https://example.com/extra.git", Path: "extra"},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// First-writer-wins for name/description (project identity from first layer,
	// included layers contribute content, not identity).
	assert.Equal(t, "base-project", result.Metadata.Name)
	assert.Equal(t, "Base description", result.Metadata.Description)

	// Repos are merged (additive).
	assert.Len(t, result.Metadata.Repos, 2)
	assert.Contains(t, result.Metadata.Repos, "core")
	assert.Contains(t, result.Metadata.Repos, "extra")

	// Layer names recorded for both.
	assert.Equal(t, "base-project", result.Metadata.LayerNames["/layers/base"])
	assert.Equal(t, "team-project", result.Metadata.LayerNames["/layers/team"])
}

func TestFold_MCP_DeepMerge(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"gitlab": map[string]any{
					"command": "npx",
					"args":    []any{"gitlab-mcp"},
				},
			},
		},
	}
	ir2 := &LayerIR{
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"jira": map[string]any{
					"command": "npx",
					"args":    []any{"jira-mcp"},
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Both MCP servers should be present.
	servers, ok := result.Content.MCP["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "gitlab")
	assert.Contains(t, servers, "jira")
}

func TestFold_MCP_ForceLock(t *testing.T) {
	s := newTestState()

	// Layer 1: gitlab server with force: true.
	ir1 := &LayerIR{
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"gitlab": map[string]any{
					"command": "npx",
					"args":    []any{"gitlab-mcp"},
					"force":   true,
				},
			},
		},
	}
	// Layer 2: tries to override gitlab — should be dropped.
	ir2 := &LayerIR{
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"gitlab": map[string]any{
					"command": "npx",
					"args":    []any{"evil-mcp"},
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	servers, ok := result.Content.MCP["mcpServers"].(map[string]any)
	require.True(t, ok)
	gitlabServer, ok := servers["gitlab"].(map[string]any)
	require.True(t, ok)

	// Should retain base layer's config, not team's override.
	args, ok := gitlabServer["args"].([]any)
	require.True(t, ok)
	assert.Equal(t, "gitlab-mcp", args[0])
}

func TestFold_Permissions_DeepMerge(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Permissions: map[string]any{
			"allow": []any{"Bash(git *)"},
		},
	}
	ir2 := &LayerIR{
		Permissions: map[string]any{
			"allow": []any{"Read"},
			"deny":  []any{"Write(~/.ssh/*)"},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// allow: last-writer-wins (default merge for arrays without additive policy).
	allow, ok := result.Content.Permissions["allow"].([]any)
	require.True(t, ok)
	assert.Contains(t, allow, "Read")

	// deny: only from team layer.
	deny, ok := result.Content.Permissions["deny"].([]any)
	require.True(t, ok)
	assert.Contains(t, deny, "Write(~/.ssh/*)")
}

func TestFold_Environment_DeepMerge(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		ConfigBlocks: map[string]map[string]any{
			"environment": {
				"NODE_ENV": "production",
			},
		},
	}
	ir2 := &LayerIR{
		ConfigBlocks: map[string]map[string]any{
			"environment": {
				"DEBUG": "true",
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	assert.Equal(t, "production", result.Content.ConfigBlocks["environment"]["NODE_ENV"])
	assert.Equal(t, "true", result.Content.ConfigBlocks["environment"]["DEBUG"])
}

func TestFold_AgentSpecific_DeepMerge(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		AgentSpecific: map[string]map[string]any{
			"claude": {"customField": "base-value"},
		},
	}
	ir2 := &LayerIR{
		AgentSpecific: map[string]map[string]any{
			"claude": {"anotherField": "team-value"},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	assert.Equal(t, "base-value", result.Content.AgentSpecific["claude"]["customField"])
	assert.Equal(t, "team-value", result.Content.AgentSpecific["claude"]["anotherField"])
}

func TestFinalize_AgentSpecific_VariableSubstitution(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Variables: map[string]string{"org": "acme"},
		AgentSpecific: map[string]map[string]any{
			"claude": {
				"title": "Config for ${keel:org}",
				"nested": map[string]any{
					"value": "${keel:org}-team",
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	assert.Equal(t, "Config for acme", result.Content.AgentSpecific["claude"]["title"])
	nested := result.Content.AgentSpecific["claude"]["nested"].(map[string]any)
	assert.Equal(t, "acme-team", nested["value"])
}

func TestFold_Snippets_LastWriterWins(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Snippets: map[string][]byte{
			"header": []byte("Base header"),
		},
	}
	ir2 := &LayerIR{
		Snippets: map[string][]byte{
			"header": []byte("Team header"),
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	assert.Equal(t, "Team header", string(s.snippets["header"]))
}

func TestFold_Bin_LastWriterWins(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		Bin: []BinFile{
			{RelPath: "mytool", Content: []byte("v1"), Mode: 0755},
		},
	}
	ir2 := &LayerIR{
		Bin: []BinFile{
			{RelPath: "mytool", Content: []byte("v2"), Mode: 0755},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	require.Len(t, result.Content.Bin, 1)
	assert.Equal(t, "v2", string(result.Content.Bin[0].Content))
}

func TestFold_FilePolicies_ForceRegistered(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		FilePolicies: map[string]schema.FilePolicy{
			"commands/deploy.md": {Force: true},
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Locked deploy")},
			},
		},
	}
	ir2 := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Override attempt")},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Equal(t, "Locked deploy", string(cmdFiles[0].Content))
}

func TestNewAccumulationState_AutoVars(t *testing.T) {
	s := newTestState()

	// keel.agent is not set yet — it's set by AgentBuild
	_, hasAgent := s.variables[varKeelAgent]
	assert.False(t, hasAgent, "keel.agent should not be set until AgentBuild")

	assert.Equal(t, "project", s.variables[varKeelScope])
	assert.Equal(t, "3", s.variables[varKeelLayersCount])
	assert.Contains(t, s.variables[varKeelBinProject], ".keel/bin")
	assert.NotEmpty(t, s.variables[varKeelVersion])
	assert.Equal(t, "/tmp/test-output", s.variables[varKeelOutput])

	// After AgentBuild, keel.agent should be set
	p := testProfile()
	s.ApplyGovernance()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	assert.Equal(t, "claude", s.variables[varKeelAgent])
}

func TestFinalize_SnippetResolution(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Snippets: map[string][]byte{
			"warning": []byte("WARNING: Do not modify this file."),
		},
		Instructions: []InstructionBlock{
			{Filename: "base.md", Content: "<!-- keel:include warning -->\n\nInstructions here."},
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("<!-- keel:include warning -->\n\nDeploy steps.")},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	assert.Contains(t, result.Content.Instructions, "WARNING: Do not modify this file.")
	assert.Contains(t, result.Content.Instructions, "Instructions here.")

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Contains(t, string(cmdFiles[0].Content), "WARNING: Do not modify this file.")
}

func TestFinalize_Conditionals(t *testing.T) {
	s := newTestState()

	content := "<!-- keel-if: keel.agent == 'claude' -->\nClaude-specific content\n<!-- /keel-if -->\n<!-- keel-if: keel.agent == 'cursor' -->\nCursor-specific content\n<!-- /keel-if -->"

	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte(content)},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p) // Must call AgentBuild to set keel.agent for conditionals
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	output := string(cmdFiles[0].Content)
	assert.Contains(t, output, "Claude-specific content")
	assert.NotContains(t, output, "Cursor-specific content")
}

func TestResult_ForceLocked(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Read",
					"hooks":   []any{map[string]any{"type": "command", "command": "echo test"}},
					"force":   true,
				},
			},
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("---\nforce: true\n---\nDeploy content")},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	// Apply hooks strategy before finalize (mimics what profile.go does).
	s.ApplyHooksStrategy(true) // Claude uses last-writer-wins
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Should have force-lock records for both the hook and the command file.
	assert.GreaterOrEqual(t, len(result.Governance.ForceLocked), 2)

	lockMap := make(map[string]string)
	for _, lock := range result.Governance.ForceLocked {
		lockMap[lock.Category+"/"+lock.Key] = lock.LockedBy
	}
	assert.Equal(t, "base", lockMap["hook/PreToolUse"])
	assert.Equal(t, "base", lockMap["command/deploy.md"])
}

func TestFold_MCPPolicy_Accumulated(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		MCPPolicy: &schema.MCPPolicy{
			Allowed: []string{"gitlab"},
		},
	}
	ir2 := &LayerIR{
		MCPPolicy: &schema.MCPPolicy{
			Denied:    []string{"untrusted"},
			EnableAll: new(bool),
		},
	}
	*ir2.MCPPolicy.EnableAll = true

	require.NoError(t, s.Fold(ir1, "base"))
	require.NoError(t, s.Fold(ir2, "team"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	require.NotNil(t, result.Governance.MCPPolicy)
	assert.Contains(t, result.Governance.MCPPolicy.Allowed, "gitlab")
	assert.Contains(t, result.Governance.MCPPolicy.Denied, "untrusted")
	assert.NotNil(t, result.Governance.MCPPolicy.EnableAll)
	assert.True(t, *result.Governance.MCPPolicy.EnableAll)
}

func TestFinalize_MCPCondition_WithLayerVariables(t *testing.T) {
	s := newTestState()

	// Layer defines a variable and uses it in MCP condition
	ir := &LayerIR{
		Variables: map[string]string{
			"env": "production",
		},
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"prod-server": map[string]any{
					"command": "server-prod",
					"_keel": map[string]any{
						"condition": `env == "production"`,
					},
				},
				"staging-server": map[string]any{
					"command": "server-staging",
					"_keel": map[string]any{
						"condition": `env == "staging"`,
					},
				},
				"always-on": map[string]any{
					"command": "server-base",
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p) // Must call AgentBuild to set keel.agent
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// MCP should have prod-server (condition matches) and always-on,
	// but NOT staging-server (condition failed)
	require.NotNil(t, result.Content.MCP)
	servers, ok := result.Content.MCP["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers should be a map")
	assert.Contains(t, servers, "prod-server", "prod-server should be included (env == production)")
	assert.Contains(t, servers, "always-on", "always-on should be included (no condition)")
	assert.NotContains(t, servers, "staging-server", "staging-server should be filtered out (env != staging)")
}

func TestFinalize_MCPCondition_WithCLIVariables(t *testing.T) {
	s := newTestState()

	// Layer has MCP servers with conditions
	ir := &LayerIR{
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"dev-server": map[string]any{
					"command": "dev",
					"_keel": map[string]any{
						"condition": `mode == "dev"`,
					},
				},
				"prod-server": map[string]any{
					"command": "prod",
					"_keel": map[string]any{
						"condition": `mode == "prod"`,
					},
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	p := testProfile()
	s.ApplyGovernance()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	// Pass CLI variable via externalVars to Finalize
	require.NoError(t, s.Finalize(map[string]string{"mode": "dev"}))
	result := s.Result()

	require.NotNil(t, result.Content.MCP)
	servers, ok := result.Content.MCP["mcpServers"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, servers, "dev-server", "dev-server should be included (mode == dev from CLI)")
	assert.NotContains(t, servers, "prod-server", "prod-server should be filtered out (mode != prod)")
}

func TestFinalize_Distributable_PreservesFrontmatter(t *testing.T) {
	p := testProfile()
	foldConfig := FoldConfig{
		Categories: p.Categories,
	}
	s := NewAccumulationState(foldConfig, CompileOptions{
		Scope:         ScopeProject,
		Distributable: true,
	}, "/tmp/test-output", 1)

	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("---\nforce: true\nname: deployer\n---\nDeploy content")},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	// In distributable mode, frontmatter should NOT be stripped.
	content := string(cmdFiles[0].Content)
	assert.Contains(t, content, "force: true")
	assert.Contains(t, content, "name: deployer")
}

// TestDeepMergeSandboxPreservesCursorFields tests that deepMergeSandbox correctly
// preserves Cursor-only fields (those with json:"-" tags) during merge.
func TestDeepMergeSandboxPreservesCursorFields(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		base     *schema.SandboxConfig
		overlay  *schema.SandboxConfig
		expected *schema.SandboxConfig
	}{
		{
			name: "overlay overwrites base cursor fields",
			base: &schema.SandboxConfig{
				Type:                     "base-type",
				AdditionalReadwritePaths: []string{"/base/rw"},
				AdditionalReadonlyPaths:  []string{"/base/ro"},
				DisableTmpWrite:          &trueVal,
				EnableSharedBuildCache:   &falseVal,
			},
			overlay: &schema.SandboxConfig{
				Type:                     "overlay-type",
				AdditionalReadwritePaths: []string{"/overlay/rw"},
				AdditionalReadonlyPaths:  []string{"/overlay/ro"},
				DisableTmpWrite:          &falseVal,
				EnableSharedBuildCache:   &trueVal,
			},
			expected: &schema.SandboxConfig{
				Type:                     "overlay-type",
				AdditionalReadwritePaths: []string{"/overlay/rw"},
				AdditionalReadonlyPaths:  []string{"/overlay/ro"},
				DisableTmpWrite:          &falseVal,
				EnableSharedBuildCache:   &trueVal,
			},
		},
		{
			name: "base fields preserved when overlay is empty",
			base: &schema.SandboxConfig{
				Type:                     "base-type",
				AdditionalReadwritePaths: []string{"/base/rw"},
				AdditionalReadonlyPaths:  []string{"/base/ro"},
				DisableTmpWrite:          &trueVal,
				EnableSharedBuildCache:   &trueVal,
			},
			overlay: &schema.SandboxConfig{},
			expected: &schema.SandboxConfig{
				Type:                     "base-type",
				AdditionalReadwritePaths: []string{"/base/rw"},
				AdditionalReadonlyPaths:  []string{"/base/ro"},
				DisableTmpWrite:          &trueVal,
				EnableSharedBuildCache:   &trueVal,
			},
		},
		{
			name: "overlay fields used when base is empty",
			base: &schema.SandboxConfig{},
			overlay: &schema.SandboxConfig{
				Type:                     "overlay-type",
				AdditionalReadwritePaths: []string{"/overlay/rw"},
				AdditionalReadonlyPaths:  []string{"/overlay/ro"},
				DisableTmpWrite:          &falseVal,
				EnableSharedBuildCache:   &trueVal,
			},
			expected: &schema.SandboxConfig{
				Type:                     "overlay-type",
				AdditionalReadwritePaths: []string{"/overlay/rw"},
				AdditionalReadonlyPaths:  []string{"/overlay/ro"},
				DisableTmpWrite:          &falseVal,
				EnableSharedBuildCache:   &trueVal,
			},
		},
		{
			name: "network default policy preserved from overlay",
			base: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{
					DefaultPolicy: "base-policy",
				},
			},
			overlay: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{
					DefaultPolicy: "overlay-policy",
				},
			},
			expected: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{
					DefaultPolicy: "overlay-policy",
				},
			},
		},
		{
			name: "network default policy preserved from base when overlay empty",
			base: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{
					DefaultPolicy: "base-policy",
				},
			},
			overlay: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{},
			},
			expected: &schema.SandboxConfig{
				Network: &schema.SandboxNetConfig{
					DefaultPolicy: "base-policy",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deepMergeSandbox(tt.base, tt.overlay)

			assert.Equal(t, tt.expected.Type, result.Type)
			assert.Equal(t, tt.expected.AdditionalReadwritePaths, result.AdditionalReadwritePaths)
			assert.Equal(t, tt.expected.AdditionalReadonlyPaths, result.AdditionalReadonlyPaths)
			assert.Equal(t, tt.expected.DisableTmpWrite, result.DisableTmpWrite)
			assert.Equal(t, tt.expected.EnableSharedBuildCache, result.EnableSharedBuildCache)

			if tt.expected.Network != nil {
				require.NotNil(t, result.Network)
				assert.Equal(t, tt.expected.Network.DefaultPolicy, result.Network.DefaultPolicy)
			}
		})
	}
}

// TestSandboxConfigCursorFieldsCovered is a guard test that ensures all
// Cursor-only fields (json:"-") in SandboxConfig and SandboxNetConfig are
// explicitly handled in deepMergeSandbox. If a new json:"-" field is added,
// this test will fail until deepMergeSandbox is updated.
func TestSandboxConfigCursorFieldsCovered(t *testing.T) {
	// Known Cursor-only fields that deepMergeSandbox handles.
	knownSandboxFields := map[string]bool{
		"Type":                     true,
		"AdditionalReadwritePaths": true,
		"AdditionalReadonlyPaths":  true,
		"DisableTmpWrite":          true,
		"EnableSharedBuildCache":   true,
	}

	knownNetConfigFields := map[string]bool{
		"DefaultPolicy": true,
	}

	// Use reflection to find all fields with json:"-" tags.
	// Check SandboxConfig
	sbType := reflect.TypeOf(schema.SandboxConfig{})
	for i := 0; i < sbType.NumField(); i++ {
		field := sbType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			assert.True(t, knownSandboxFields[field.Name],
				"SandboxConfig field %q has json:\"-\" but is not handled in deepMergeSandbox", field.Name)
		}
	}

	// Check SandboxNetConfig
	netType := reflect.TypeOf(schema.SandboxNetConfig{})
	for i := 0; i < netType.NumField(); i++ {
		field := netType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			assert.True(t, knownNetConfigFields[field.Name],
				"SandboxNetConfig field %q has json:\"-\" but is not handled in deepMergeSandbox", field.Name)
		}
	}
}

func TestSortedMapKeys(t *testing.T) {
	t.Parallel()

	m := map[string]any{"charlie": 1, "alpha": 2, "bravo": 3}
	keys := sortedMapKeys(m)
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, keys)
}

func TestSortedMapKeys_Empty(t *testing.T) {
	t.Parallel()

	keys := sortedMapKeys(map[string]any{})
	assert.Empty(t, keys)
}

// TestDeepMergeSandbox_PreservesAllCursorFields tests that all Cursor-only fields
// (with json:"-" tags) are correctly preserved through the deepMergeSandbox merge.
// This test catches regressions if new json:"-" fields are added to SandboxConfig
// without updating deepMergeSandbox.
func TestDeepMergeSandbox_PreservesAllCursorFields(t *testing.T) {
	trueVal := true
	falseVal := false

	// Create base and overlay with ALL Cursor-only fields set
	base := &schema.SandboxConfig{
		// Cursor-only fields
		Type:                     "base-type",
		AdditionalReadwritePaths: []string{"/base/rw1", "/base/rw2"},
		AdditionalReadonlyPaths:  []string{"/base/ro1", "/base/ro2"},
		DisableTmpWrite:          &trueVal,
		EnableSharedBuildCache:   &falseVal,
		Network: &schema.SandboxNetConfig{
			DefaultPolicy: "base-policy",
		},
		// Also include a portable field to ensure the merge works
		Enabled: &trueVal,
	}

	overlay := &schema.SandboxConfig{
		// Cursor-only fields
		Type:                     "overlay-type",
		AdditionalReadwritePaths: []string{"/overlay/rw1", "/overlay/rw2"},
		AdditionalReadonlyPaths:  []string{"/overlay/ro1", "/overlay/ro2"},
		DisableTmpWrite:          &falseVal,
		EnableSharedBuildCache:   &trueVal,
		Network: &schema.SandboxNetConfig{
			DefaultPolicy: "overlay-policy",
		},
		// Portable field
		AutoAllowBashIfSandboxed: &trueVal,
	}

	result := deepMergeSandbox(base, overlay)

	// Verify ALL Cursor-only fields from overlay are preserved
	assert.Equal(t, "overlay-type", result.Type, "Type field should be preserved")
	assert.Equal(t, []string{"/overlay/rw1", "/overlay/rw2"}, result.AdditionalReadwritePaths, "AdditionalReadwritePaths should be preserved")
	assert.Equal(t, []string{"/overlay/ro1", "/overlay/ro2"}, result.AdditionalReadonlyPaths, "AdditionalReadonlyPaths should be preserved")
	require.NotNil(t, result.DisableTmpWrite, "DisableTmpWrite should not be nil")
	assert.False(t, *result.DisableTmpWrite, "DisableTmpWrite should be preserved")
	require.NotNil(t, result.EnableSharedBuildCache, "EnableSharedBuildCache should not be nil")
	assert.True(t, *result.EnableSharedBuildCache, "EnableSharedBuildCache should be preserved")
	require.NotNil(t, result.Network, "Network should not be nil")
	assert.Equal(t, "overlay-policy", result.Network.DefaultPolicy, "Network.DefaultPolicy should be preserved")

	// Verify portable fields were also deep-merged
	require.NotNil(t, result.Enabled, "Enabled should be preserved from base")
	assert.True(t, *result.Enabled)
	require.NotNil(t, result.AutoAllowBashIfSandboxed, "AutoAllowBashIfSandboxed should be preserved from overlay")
	assert.True(t, *result.AutoAllowBashIfSandboxed)
}

// TestDeepMergeSandbox_BaseFieldsFallback tests that when overlay has empty
// Cursor-only fields, the base values are preserved (last-writer-wins fallback).
func TestDeepMergeSandbox_BaseFieldsFallback(t *testing.T) {
	trueVal := true

	base := &schema.SandboxConfig{
		Type:                     "base-type",
		AdditionalReadwritePaths: []string{"/base/rw"},
		AdditionalReadonlyPaths:  []string{"/base/ro"},
		DisableTmpWrite:          &trueVal,
		EnableSharedBuildCache:   &trueVal,
		Network: &schema.SandboxNetConfig{
			DefaultPolicy: "base-policy",
		},
	}

	overlay := &schema.SandboxConfig{
		// Empty overlay - should fall back to base values
		Network: &schema.SandboxNetConfig{},
	}

	result := deepMergeSandbox(base, overlay)

	// All base Cursor-only fields should be preserved when overlay is empty
	assert.Equal(t, "base-type", result.Type)
	assert.Equal(t, []string{"/base/rw"}, result.AdditionalReadwritePaths)
	assert.Equal(t, []string{"/base/ro"}, result.AdditionalReadonlyPaths)
	require.NotNil(t, result.DisableTmpWrite)
	assert.True(t, *result.DisableTmpWrite)
	require.NotNil(t, result.EnableSharedBuildCache)
	assert.True(t, *result.EnableSharedBuildCache)
	require.NotNil(t, result.Network)
	assert.Equal(t, "base-policy", result.Network.DefaultPolicy)
}

// TestDeepMergeSandbox_CoversAllFields uses reflection to verify that deepMergeSandbox
// handles all fields in SandboxConfig and its nested structs. This catches new fields
// being added to the schema without updating the merge function.
func TestDeepMergeSandbox_CoversAllFields(t *testing.T) {
	t.Parallel()

	// Create a fully-populated overlay with all fields set to non-zero values.
	trueVal := true
	falseVal := false

	overlay := &schema.SandboxConfig{
		Enabled:                      &trueVal,
		AutoAllowBashIfSandboxed:     &trueVal,
		ExcludedCommands:             []string{"excluded-cmd"},
		FailIfUnavailable:            &trueVal,
		AllowUnsandboxedCommands:     &trueVal,
		EnableWeakerNestedSandbox:    &trueVal,
		EnableWeakerNetworkIsolation: &trueVal,
		IgnoreViolations:             map[string][]string{"key": {"val"}},
		BwrapPath:                    "/path/to/bwrap",
		SocatPath:                    "/path/to/socat",
		Type:                         "sandbox-type",
		AdditionalReadwritePaths:     []string{"/rw/path"},
		AdditionalReadonlyPaths:      []string{"/ro/path"},
		DisableTmpWrite:              &falseVal,
		EnableSharedBuildCache:       &falseVal,
		Filesystem: &schema.SandboxFSConfig{
			AllowWrite:                []string{"/allow/write"},
			DenyWrite:                 []string{"/deny/write"},
			DenyRead:                  []string{"/deny/read"},
			AllowRead:                 []string{"/allow/read"},
			AllowManagedReadPathsOnly: &trueVal,
		},
		Network: &schema.SandboxNetConfig{
			AllowedDomains:          []string{"allowed.com"},
			DeniedDomains:           []string{"denied.com"},
			AllowUnixSockets:        []string{"/socket/path"},
			AllowAllUnixSockets:     &trueVal,
			AllowLocalBinding:       &trueVal,
			AllowMachLookup:         []string{"mach-service"},
			HttpProxyPort:           new(int),
			SocksProxyPort:          new(int),
			AllowManagedDomainsOnly: &trueVal,
			DefaultPolicy:           "policy-name",
		},
		Ripgrep: &schema.SandboxRipgrepConfig{
			Command: "rg",
			Args:    []string{"--arg1", "--arg2"},
		},
	}
	*overlay.Network.HttpProxyPort = 8080
	*overlay.Network.SocksProxyPort = 1080

	// Merge with empty base — result should contain all overlay fields.
	result := deepMergeSandbox(nil, overlay)

	// Use reflection to verify all top-level fields are present.
	assertAllFieldsSet(t, reflect.ValueOf(overlay).Elem(), reflect.ValueOf(result).Elem(), "SandboxConfig")

	// Verify nested structs.
	require.NotNil(t, result.Filesystem, "Filesystem should be present")
	assertAllFieldsSet(t, reflect.ValueOf(overlay.Filesystem).Elem(), reflect.ValueOf(result.Filesystem).Elem(), "SandboxFSConfig")

	require.NotNil(t, result.Network, "Network should be present")
	assertAllFieldsSet(t, reflect.ValueOf(overlay.Network).Elem(), reflect.ValueOf(result.Network).Elem(), "SandboxNetConfig")

	require.NotNil(t, result.Ripgrep, "Ripgrep should be present")
	assertAllFieldsSet(t, reflect.ValueOf(overlay.Ripgrep).Elem(), reflect.ValueOf(result.Ripgrep).Elem(), "SandboxRipgrepConfig")
}

// assertAllFieldsSet uses reflection to verify that all non-zero fields in expected
// are also non-zero in actual. This catches fields that deepMergeSandbox doesn't handle.
func assertAllFieldsSet(t *testing.T, expected, actual reflect.Value, structName string) {
	t.Helper()
	typ := expected.Type()
	for i := 0; i < expected.NumField(); i++ {
		field := typ.Field(i)
		expectedField := expected.Field(i)
		actualField := actual.Field(i)

		// Skip unexported fields.
		if !expectedField.CanInterface() {
			continue
		}

		// If the expected field is non-zero, the actual field should also be non-zero.
		if !expectedField.IsZero() {
			assert.False(t, actualField.IsZero(),
				"%s.%s: expected field to be set after merge, but it's zero", structName, field.Name)
		}
	}
}

// TestPhase_FoldAfterGovernance_Panics tests that calling Fold after ApplyGovernance panics.
func TestPhase_FoldAfterGovernance_Panics(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))
	s.ApplyGovernance()

	// Attempting to fold after governance should panic
	assert.PanicsWithValue(t, "Fold called in phase \"governed\" — pipeline has already advanced past folding", func() {
		_ = s.Fold(ir, "layer2")
	})
}

// TestPhase_FoldAfterTransform_Panics tests that calling Fold after TransformForAgent panics.
func TestPhase_FoldAfterTransform_Panics(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)

	// Attempting to fold after transform should panic
	assert.PanicsWithValue(t, "Fold called in phase \"transformed\" — pipeline has already advanced past folding", func() {
		_ = s.Fold(ir, "layer2")
	})
}

// TestPhase_TransformBeforeGovernance_Panics tests that calling TransformForAgent
// before ApplyGovernance panics.
func TestPhase_TransformBeforeGovernance_Panics(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))
	p := testProfile()

	// Attempting to transform before governance should panic
	assert.PanicsWithValue(t, "TransformForAgent called in phase \"folding\" — ApplyGovernance must be called first", func() {
		s.TransformForAgent(p)
	})
}

// TestPhase_FinalizeBeforeGovernance_Panics tests that calling Finalize
// before ApplyGovernance panics.
func TestPhase_FinalizeBeforeGovernance_Panics(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))

	// Attempting to finalize before governance should panic
	assert.PanicsWithValue(t, "Finalize called in phase \"folding\" — ApplyGovernance must be called first", func() {
		_ = s.Finalize(nil)
	})
}

// TestPhase_GovernanceAfterTransform_Panics tests that calling ApplyGovernance
// after TransformForAgent panics.
func TestPhase_GovernanceAfterTransform_Panics(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)

	// Attempting to apply governance after transform should panic
	assert.PanicsWithValue(t, "ApplyGovernance called in phase \"transformed\" — must be called before TransformForAgent/Finalize", func() {
		s.ApplyGovernance()
	})
}

// TestPhase_ValidPipeline_NoPanic tests that a valid pipeline sequence completes without panic.
func TestPhase_ValidPipeline_NoPanic(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	// Valid pipeline: Fold -> ApplyGovernance -> TransformForAgent -> Finalize
	require.NoError(t, s.Fold(ir, "layer1"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	require.NoError(t, s.Finalize(nil))

	// Verify we reached the finalized phase
	result := s.Result()
	assert.NotNil(t, result)
}

// TestPhase_SkipTransform_Allowed tests that skipping TransformForAgent is allowed.
// Fold -> ApplyGovernance -> Finalize (without TransformForAgent) should work.
func TestPhase_SkipTransform_Allowed(t *testing.T) {
	s := newTestState()
	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "Test content"},
		},
	}

	// Valid pipeline skipping transform: Fold -> ApplyGovernance -> Finalize
	require.NoError(t, s.Fold(ir, "layer1"))
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))

	// Verify we reached the finalized phase
	result := s.Result()
	assert.NotNil(t, result)
}
