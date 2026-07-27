package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func TestLayerIR_Clone_Independence(t *testing.T) {
	t.Parallel()

	// Helper to create a bool pointer
	boolPtr := func(b bool) *bool { return &b }

	// Create an IR with ALL mutable fields populated
	original := &LayerIR{
		Name:        "original",
		Description: "original description",
		Instructions: []InstructionBlock{
			{Filename: "a.md", Content: "original content", LayerLabel: "base"},
		},
		Permissions: map[string]any{
			"allow": []any{"Read"},
			"deny":  []any{"Write"},
		},
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"s1": map[string]any{"command": "cmd"},
			},
		},
		Settings: map[string]any{"model": "sonnet"},
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "command", "command": "echo test"},
			},
		},
		ConfigBlocks: map[string]map[string]any{"environment": {"key": "val"}},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "cmd.md", Content: []byte("original"), IsDir: false, SourceAgent: "claude"},
			},
		},
		ForceLocks: []ForceLockEntry{
			{Category: "hook", Key: "PreToolUse"},
		},
		AgentSpecific: map[string]map[string]any{
			"claude": {"effortLevel": "high"},
		},
		Variables: map[string]string{"myvar": "myval"},
		Snippets: map[string][]byte{
			"snippet1": []byte("snippet content"),
		},
		Deps: map[string]schema.DepSpec{
			"tool": {Install: "go install ...", Verify: "tool --version", Global: true},
		},
		Bin: []BinFile{
			{RelPath: "mytool", Content: []byte("binary"), Mode: 0755},
		},
		ContentBlocks: map[string][]InstructionBlock{"loop": {
			{Content: "loop content"},
		}},
		Attribution: &CanonicalAttribution{
			CommitEnabled:  true,
			CommitTemplate: "tmpl",
			PREnabled:      true,
			PRTemplate:     "pr tmpl",
		},
		Sandbox: &schema.SandboxConfig{
			Enabled: boolPtr(true),
			Network: &schema.SandboxNetConfig{
				AllowedDomains: []string{"example.com"},
				DeniedDomains:  []string{"evil.com"},
			},
			AdditionalReadwritePaths: []string{"/rw"},
			AdditionalReadonlyPaths:  []string{"/ro"},
			DisableTmpWrite:          boolPtr(false),
			EnableSharedBuildCache:   boolPtr(true),
			Type:                     "test-type",
		},
		FilePolicies: map[string]schema.FilePolicy{
			"cmd.md": {Force: true},
		},
		MCPPolicy: &schema.MCPPolicy{
			Allowed:   []string{"s1"},
			Denied:    []string{"s2"},
			EnableAll: boolPtr(true),
		},
		PermissionsPolicy: &schema.PermissionsPolicy{
			Merge: "additive",
		},
		Repos: map[string]schema.Repo{
			"repo1": {URL: "https://example.com", Path: "repo1", Branch: "main"},
		},
		SourceAgent: "claude",
		LayerPath:   "/tmp/layer",
		DSLConfig:   &schema.KeelConfig{Name: "config"},
	}

	clone, err := original.Clone()
	require.NoError(t, err)

	// 1. Verify deep equality for all scalar fields
	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.Description, clone.Description)
	assert.Equal(t, original.SourceAgent, clone.SourceAgent)
	assert.Equal(t, original.LayerPath, clone.LayerPath)

	// Instructions
	require.Len(t, clone.Instructions, len(original.Instructions))
	assert.Equal(t, original.Instructions[0].Filename, clone.Instructions[0].Filename)
	assert.Equal(t, original.Instructions[0].Content, clone.Instructions[0].Content)
	assert.Equal(t, original.Instructions[0].LayerLabel, clone.Instructions[0].LayerLabel)

	// Maps: check structure
	assert.Equal(t, original.Permissions["allow"], clone.Permissions["allow"])
	assert.Equal(t, original.Settings["model"], clone.Settings["model"])
	assert.Equal(t, original.ConfigBlocks["environment"]["key"], clone.ConfigBlocks["environment"]["key"])

	// Categories
	require.Contains(t, clone.Categories, "commands")
	require.Len(t, clone.Categories["commands"], 1)
	assert.Equal(t, original.Categories["commands"][0].RelPath, clone.Categories["commands"][0].RelPath)
	assert.Equal(t, original.Categories["commands"][0].Content, clone.Categories["commands"][0].Content)

	// ForceLocks
	require.Len(t, clone.ForceLocks, 1)
	assert.Equal(t, original.ForceLocks[0].Category, clone.ForceLocks[0].Category)

	// AgentSpecific
	assert.Equal(t, original.AgentSpecific["claude"]["effortLevel"], clone.AgentSpecific["claude"]["effortLevel"])

	// Variables
	assert.Equal(t, original.Variables["myvar"], clone.Variables["myvar"])

	// Snippets
	assert.Equal(t, original.Snippets["snippet1"], clone.Snippets["snippet1"])

	// Deps
	assert.Equal(t, original.Deps["tool"].Install, clone.Deps["tool"].Install)

	// Bin
	require.Len(t, clone.Bin, 1)
	assert.Equal(t, original.Bin[0].RelPath, clone.Bin[0].RelPath)
	assert.Equal(t, original.Bin[0].Content, clone.Bin[0].Content)

	// ContentBlocks (loop, worktree-include)
	require.Len(t, clone.ContentBlocks["loop"], 1)
	assert.Equal(t, original.ContentBlocks["loop"][0].Content, clone.ContentBlocks["loop"][0].Content)

	// Attribution
	require.NotNil(t, clone.Attribution)
	assert.Equal(t, original.Attribution.CommitEnabled, clone.Attribution.CommitEnabled)
	assert.Equal(t, original.Attribution.CommitTemplate, clone.Attribution.CommitTemplate)

	// Sandbox
	require.NotNil(t, clone.Sandbox)
	assert.Equal(t, *original.Sandbox.Enabled, *clone.Sandbox.Enabled)
	require.NotNil(t, clone.Sandbox.Network)
	assert.Equal(t, original.Sandbox.Network.AllowedDomains, clone.Sandbox.Network.AllowedDomains)

	// FilePolicies
	assert.True(t, clone.FilePolicies["cmd.md"].Force)

	// MCPPolicy
	require.NotNil(t, clone.MCPPolicy)
	assert.Equal(t, original.MCPPolicy.Allowed, clone.MCPPolicy.Allowed)

	// PermissionsPolicy
	require.NotNil(t, clone.PermissionsPolicy)
	assert.Equal(t, original.PermissionsPolicy.Merge, clone.PermissionsPolicy.Merge)

	// Repos
	assert.Equal(t, original.Repos["repo1"].URL, clone.Repos["repo1"].URL)

	// DSLConfig: pointer should be the same (not deep copied)
	assert.Same(t, original.DSLConfig, clone.DSLConfig)

	// 2. Mutate the clone and verify original is unchanged
	clone.Name = "mutated"
	clone.Description = "mutated description"
	clone.Instructions[0].Content = "mutated"
	clone.Instructions[0].Filename = "mutated.md"
	clone.Permissions["allow"] = []any{"Write"}
	clone.MCP["mcpServers"] = "mutated"
	clone.Settings["model"] = "opus"
	clone.Hooks["PreToolUse"] = "mutated"
	clone.ConfigBlocks["environment"]["key"] = "mutated"
	clone.Categories["commands"][0].Content = []byte("mutated")
	clone.Categories["commands"][0].RelPath = "mutated.md"
	clone.ForceLocks[0].Key = "mutated"
	clone.AgentSpecific["claude"]["effortLevel"] = "mutated"
	clone.Variables["myvar"] = "mutated"
	clone.Snippets["snippet1"] = []byte("mutated")
	clone.Deps["tool"] = schema.DepSpec{Install: "mutated"}
	clone.Bin[0].Content = []byte("mutated")
	clone.ContentBlocks["loop"][0].Content = "mutated"
	clone.Attribution.CommitTemplate = "mutated"
	clone.Sandbox.Network.AllowedDomains[0] = "mutated.com"
	clone.Sandbox.AdditionalReadwritePaths[0] = "mutated"
	clone.FilePolicies["cmd.md"] = schema.FilePolicy{Force: false}
	clone.MCPPolicy.Allowed[0] = "mutated"
	clone.PermissionsPolicy.Merge = "mutated"
	clone.Repos["repo1"] = schema.Repo{URL: "mutated"}
	clone.SourceAgent = "mutated"
	clone.LayerPath = "mutated"

	// 3. Assert original is unchanged
	assert.Equal(t, "original", original.Name)
	assert.Equal(t, "original description", original.Description)
	assert.Equal(t, "original content", original.Instructions[0].Content)
	assert.Equal(t, "a.md", original.Instructions[0].Filename)
	assert.Equal(t, []any{"Read"}, original.Permissions["allow"])
	assert.Equal(t, "sonnet", original.Settings["model"])
	assert.Equal(t, "val", original.ConfigBlocks["environment"]["key"])
	assert.Equal(t, []byte("original"), original.Categories["commands"][0].Content)
	assert.Equal(t, "cmd.md", original.Categories["commands"][0].RelPath)
	assert.Equal(t, "PreToolUse", original.ForceLocks[0].Key)
	assert.Equal(t, "high", original.AgentSpecific["claude"]["effortLevel"])
	assert.Equal(t, "myval", original.Variables["myvar"])
	assert.Equal(t, []byte("snippet content"), original.Snippets["snippet1"])
	assert.Equal(t, "go install ...", original.Deps["tool"].Install)
	assert.Equal(t, []byte("binary"), original.Bin[0].Content)
	assert.Equal(t, "loop content", original.ContentBlocks["loop"][0].Content)
	assert.Equal(t, "tmpl", original.Attribution.CommitTemplate)
	assert.Equal(t, "example.com", original.Sandbox.Network.AllowedDomains[0])
	assert.Equal(t, "/rw", original.Sandbox.AdditionalReadwritePaths[0])
	assert.True(t, original.FilePolicies["cmd.md"].Force)
	assert.Equal(t, "s1", original.MCPPolicy.Allowed[0])
	assert.Equal(t, "additive", original.PermissionsPolicy.Merge)
	assert.Equal(t, "https://example.com", original.Repos["repo1"].URL)
	assert.Equal(t, "claude", original.SourceAgent)
	assert.Equal(t, "/tmp/layer", original.LayerPath)

	// MCP nested structure
	mcpServers, ok := original.MCP["mcpServers"].(map[string]any)
	require.True(t, ok)
	s1, ok := mcpServers["s1"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cmd", s1["command"])
}

func TestLayerIR_Clone_Nil(t *testing.T) {
	t.Parallel()
	var ir *LayerIR
	cloned, err := ir.Clone()
	assert.NoError(t, err)
	assert.Nil(t, cloned)
}

func TestLayerIR_Clone_EmptyFields(t *testing.T) {
	t.Parallel()

	// An IR with no mutable fields set should clone to an equivalent empty IR
	original := &LayerIR{
		Name:        "empty-test",
		SourceAgent: "keel",
		LayerPath:   "/tmp/empty",
	}

	clone, err := original.Clone()
	require.NoError(t, err)

	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.SourceAgent, clone.SourceAgent)
	assert.Equal(t, original.LayerPath, clone.LayerPath)
	assert.Nil(t, clone.Instructions)
	assert.Nil(t, clone.Permissions)
	assert.Nil(t, clone.MCP)
	assert.Nil(t, clone.Settings)
	assert.Nil(t, clone.Hooks)
	assert.Nil(t, clone.ConfigBlocks["environment"])
	assert.Nil(t, clone.Categories)
	assert.Nil(t, clone.ForceLocks)
	assert.Nil(t, clone.AgentSpecific)
	assert.Nil(t, clone.Variables)
	assert.Nil(t, clone.Snippets)
	assert.Nil(t, clone.Deps)
	assert.Nil(t, clone.Bin)
	assert.Nil(t, clone.ContentBlocks)
	assert.Nil(t, clone.ConfigBlocks)
	assert.Nil(t, clone.Attribution)
	assert.Nil(t, clone.Sandbox)
	assert.Nil(t, clone.FilePolicies)
	assert.Nil(t, clone.MCPPolicy)
	assert.Nil(t, clone.PermissionsPolicy)
	assert.Nil(t, clone.Repos)
}

func TestLayerIR_Clone_NestedSliceIndependence(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	// Test that nested slices in complex structures are properly cloned
	original := &LayerIR{
		Name: "nested-test",
		Sandbox: &schema.SandboxConfig{
			Enabled: boolPtr(true),
			Network: &schema.SandboxNetConfig{
				AllowedDomains: []string{"a.com", "b.com"},
				DeniedDomains:  []string{"x.com", "y.com"},
			},
			AdditionalReadwritePaths: []string{"/path1", "/path2"},
			AdditionalReadonlyPaths:  []string{"/ro1", "/ro2"},
		},
		MCPPolicy: &schema.MCPPolicy{
			Allowed: []string{"server1", "server2"},
			Denied:  []string{"bad1", "bad2"},
		},
	}

	clone, err := original.Clone()
	require.NoError(t, err)

	// Mutate clone's slices
	clone.Sandbox.Network.AllowedDomains = append(clone.Sandbox.Network.AllowedDomains, "c.com")
	clone.Sandbox.Network.DeniedDomains[0] = "mutated.com"
	clone.Sandbox.AdditionalReadwritePaths = append(clone.Sandbox.AdditionalReadwritePaths, "/path3")
	clone.Sandbox.AdditionalReadonlyPaths[0] = "/mutated"
	clone.MCPPolicy.Allowed = append(clone.MCPPolicy.Allowed, "server3")
	clone.MCPPolicy.Denied[0] = "mutated"

	// Original should be unchanged
	assert.Equal(t, []string{"a.com", "b.com"}, original.Sandbox.Network.AllowedDomains)
	assert.Equal(t, "x.com", original.Sandbox.Network.DeniedDomains[0])
	assert.Equal(t, []string{"/path1", "/path2"}, original.Sandbox.AdditionalReadwritePaths)
	assert.Equal(t, "/ro1", original.Sandbox.AdditionalReadonlyPaths[0])
	assert.Equal(t, []string{"server1", "server2"}, original.MCPPolicy.Allowed)
	assert.Equal(t, "bad1", original.MCPPolicy.Denied[0])
}

func TestLayerIR_Clone_DeepMapIndependence(t *testing.T) {
	t.Parallel()

	// Test deep map cloning with nested structures
	original := &LayerIR{
		Name: "deep-map-test",
		MCP: map[string]any{
			"mcpServers": map[string]any{
				"gitlab": map[string]any{
					"command": "npx",
					"args":    []any{"gitlab-mcp"},
					"env": map[string]any{
						"TOKEN": "secret",
					},
				},
			},
		},
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Read",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo test"},
					},
				},
			},
		},
	}

	clone, err := original.Clone()
	require.NoError(t, err)

	// Mutate nested structures in clone
	mcpServers := clone.MCP["mcpServers"].(map[string]any)
	gitlabServer := mcpServers["gitlab"].(map[string]any)
	gitlabServer["command"] = "mutated"
	gitlabEnv := gitlabServer["env"].(map[string]any)
	gitlabEnv["TOKEN"] = "mutated-secret"

	preToolUse := clone.Hooks["PreToolUse"].([]any)
	entry := preToolUse[0].(map[string]any)
	entry["matcher"] = "Write"

	// Verify original is unchanged
	origMCPServers := original.MCP["mcpServers"].(map[string]any)
	origGitlab := origMCPServers["gitlab"].(map[string]any)
	assert.Equal(t, "npx", origGitlab["command"])
	origEnv := origGitlab["env"].(map[string]any)
	assert.Equal(t, "secret", origEnv["TOKEN"])

	origPreToolUse := original.Hooks["PreToolUse"].([]any)
	origEntry := origPreToolUse[0].(map[string]any)
	assert.Equal(t, "Read", origEntry["matcher"])
}
