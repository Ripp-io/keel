package compiler

import (
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func TestComputeStats_AllSections(t *testing.T) {
	result := &AccumulatedResult{
		Content: CompilationContent{
			Instructions: "This is a test instruction with some content here.",
			Categories: map[string][]CategoryFile{
				"skills": {
					{RelPath: "skill1.md", Content: []byte("skill content one"), IsDir: false},
					{RelPath: "skill2.md", Content: []byte("skill content two"), IsDir: false},
					{RelPath: "subdir", IsDir: true}, // directories don't count
				},
				"agents": {
					{RelPath: "agent1.md", Content: []byte("agent content"), IsDir: false},
				},
				"commands": {
					{RelPath: "cmd.md", Content: []byte("command"), IsDir: false},
				},
			},
			MCP: map[string]any{
				"mcpServers": map[string]any{
					"server1": map[string]any{"command": "test1"},
					"server2": map[string]any{"command": "test2"},
					"server3": map[string]any{"command": "test3"},
				},
			},
			Hooks: map[string]any{
				"PreToolUse":  []any{},
				"PostToolUse": []any{},
			},
			ContentBlocks: map[string]string{
				"loop":             "Loop prompt content goes here.",
				"worktree-include": ".git\n.keel\n",
			},
		},
	}

	effectiveState := BuildEffectiveState(result)
	stats := computeStats(result, effectiveState)

	// Check that all sections exist
	sectionNames := make(map[string]SectionStat)
	for _, s := range stats.Sections {
		sectionNames[s.Name] = s
	}

	// Verify instructions
	if instr, ok := sectionNames["instructions"]; !ok {
		t.Error("instructions section missing")
	} else {
		if instr.Count != 0 {
			t.Errorf("instructions Count = %d, want 0", instr.Count)
		}
		if instr.Bytes != len(result.Content.Instructions) {
			t.Errorf("instructions Bytes = %d, want %d", instr.Bytes, len(result.Content.Instructions))
		}
		expectedTokens := (len(result.Content.Instructions) + 3) / 4
		if instr.Tokens != expectedTokens {
			t.Errorf("instructions Tokens = %d, want %d", instr.Tokens, expectedTokens)
		}
	}

	// Verify skills (2 files, excluding the directory)
	if skills, ok := sectionNames["skills"]; !ok {
		t.Error("skills section missing")
	} else {
		if skills.Count != 2 {
			t.Errorf("skills Count = %d, want 2", skills.Count)
		}
		expectedBytes := len("skill content one") + len("skill content two")
		if skills.Bytes != expectedBytes {
			t.Errorf("skills Bytes = %d, want %d", skills.Bytes, expectedBytes)
		}
	}

	// Verify agents (1 file)
	if agents, ok := sectionNames["agents"]; !ok {
		t.Error("agents section missing")
	} else {
		if agents.Count != 1 {
			t.Errorf("agents Count = %d, want 1", agents.Count)
		}
	}

	// Verify commands (1 file)
	if commands, ok := sectionNames["commands"]; !ok {
		t.Error("commands section missing")
	} else {
		if commands.Count != 1 {
			t.Errorf("commands Count = %d, want 1", commands.Count)
		}
	}

	// Verify loop
	if loop, ok := sectionNames["loop"]; !ok {
		t.Error("loop section missing")
	} else {
		if loop.Count != 0 {
			t.Errorf("loop Count = %d, want 0", loop.Count)
		}
		if loop.Bytes != len(result.Content.ContentBlocks["loop"]) {
			t.Errorf("loop Bytes = %d, want %d", loop.Bytes, len(result.Content.ContentBlocks["loop"]))
		}
	}

	// Verify worktree-include
	if wti, ok := sectionNames["worktree-include"]; !ok {
		t.Error("worktree-include section missing")
	} else {
		if wti.Count != 0 {
			t.Errorf("worktree-include Count = %d, want 0", wti.Count)
		}
		if wti.Bytes != len(result.Content.ContentBlocks["worktree-include"]) {
			t.Errorf("worktree-include Bytes = %d, want %d", wti.Bytes, len(result.Content.ContentBlocks["worktree-include"]))
		}
	}

	// Verify MCP servers
	if stats.MCPServers != 3 {
		t.Errorf("MCPServers = %d, want 3", stats.MCPServers)
	}

	// Verify hook events
	if stats.HookEvents != 2 {
		t.Errorf("HookEvents = %d, want 2", stats.HookEvents)
	}

	// Verify totals
	expectedTotalBytes := len(result.Content.Instructions) +
		len("skill content one") + len("skill content two") +
		len("agent content") +
		len("command") +
		len(result.Content.ContentBlocks["loop"]) +
		len(result.Content.ContentBlocks["worktree-include"])
	if stats.TotalBytes != expectedTotalBytes {
		t.Errorf("TotalBytes = %d, want %d", stats.TotalBytes, expectedTotalBytes)
	}

	// TotalTokens is the sum of individual section tokens (not total bytes / 4)
	// Each section rounds independently, so we must sum section tokens
	expectedTotalTokens := 0
	for _, section := range stats.Sections {
		expectedTotalTokens += section.Tokens
	}
	if stats.TotalTokens != expectedTotalTokens {
		t.Errorf("TotalTokens = %d, want %d", stats.TotalTokens, expectedTotalTokens)
	}
}

func TestComputeStats_EmptyResult(t *testing.T) {
	result := &AccumulatedResult{}
	effectiveState := BuildEffectiveState(result)
	stats := computeStats(result, effectiveState)

	if len(stats.Sections) != 0 {
		t.Errorf("Sections = %d, want 0", len(stats.Sections))
	}
	if stats.MCPServers != 0 {
		t.Errorf("MCPServers = %d, want 0", stats.MCPServers)
	}
	if stats.HookEvents != 0 {
		t.Errorf("HookEvents = %d, want 0", stats.HookEvents)
	}
	if stats.TotalBytes != 0 {
		t.Errorf("TotalBytes = %d, want 0", stats.TotalBytes)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", stats.TotalTokens)
	}
}

func TestComputeStats_InstructionsOnly(t *testing.T) {
	content := "Just instructions, nothing else."
	result := &AccumulatedResult{
		Content: CompilationContent{
			Instructions: content,
		},
	}
	effectiveState := BuildEffectiveState(result)
	stats := computeStats(result, effectiveState)

	if len(stats.Sections) != 1 {
		t.Fatalf("Sections = %d, want 1", len(stats.Sections))
	}

	section := stats.Sections[0]
	if section.Name != "instructions" {
		t.Errorf("Section name = %q, want %q", section.Name, "instructions")
	}
	if section.Count != 0 {
		t.Errorf("Section Count = %d, want 0", section.Count)
	}
	if section.Bytes != len(content) {
		t.Errorf("Section Bytes = %d, want %d", section.Bytes, len(content))
	}

	expectedTokens := (len(content) + 3) / 4
	if section.Tokens != expectedTokens {
		t.Errorf("Section Tokens = %d, want %d", section.Tokens, expectedTokens)
	}

	if stats.TotalBytes != len(content) {
		t.Errorf("TotalBytes = %d, want %d", stats.TotalBytes, len(content))
	}
	if stats.TotalTokens != expectedTokens {
		t.Errorf("TotalTokens = %d, want %d", stats.TotalTokens, expectedTokens)
	}
}

func TestComputeStats_MCPServerCount(t *testing.T) {
	tests := []struct {
		name string
		mcp  map[string]any
		want int
	}{
		{
			name: "three servers",
			mcp: map[string]any{
				"mcpServers": map[string]any{
					"server1": map[string]any{"command": "cmd1"},
					"server2": map[string]any{"command": "cmd2"},
					"server3": map[string]any{"command": "cmd3"},
				},
			},
			want: 3,
		},
		{
			name: "no servers",
			mcp: map[string]any{
				"mcpServers": map[string]any{},
			},
			want: 0,
		},
		{
			name: "nil mcp",
			mcp:  nil,
			want: 0,
		},
		{
			name: "missing mcpServers key",
			mcp: map[string]any{
				"otherKey": "value",
			},
			want: 0,
		},
		{
			name: "mcpServers not a map",
			mcp: map[string]any{
				"mcpServers": "not a map",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &AccumulatedResult{
				Content: CompilationContent{
					MCP: tt.mcp,
				},
			}
			effectiveState := BuildEffectiveState(result)
			stats := computeStats(result, effectiveState)
			if stats.MCPServers != tt.want {
				t.Errorf("MCPServers = %d, want %d", stats.MCPServers, tt.want)
			}
		})
	}
}

func TestComputeStats_HookEventCount(t *testing.T) {
	tests := []struct {
		name  string
		hooks map[string]any
		want  int
	}{
		{
			name: "multiple events",
			hooks: map[string]any{
				"PreToolUse":  []any{},
				"PostToolUse": []any{},
				"OnError":     []any{},
			},
			want: 3,
		},
		{
			name:  "no events",
			hooks: map[string]any{},
			want:  0,
		},
		{
			name:  "nil hooks",
			hooks: nil,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &AccumulatedResult{
				Content: CompilationContent{
					Hooks: tt.hooks,
				},
			}
			effectiveState := BuildEffectiveState(result)
			stats := computeStats(result, effectiveState)
			if stats.HookEvents != tt.want {
				t.Errorf("HookEvents = %d, want %d", stats.HookEvents, tt.want)
			}
		})
	}
}

func TestComputeStats_SkipEmptySections(t *testing.T) {
	result := &AccumulatedResult{
		Content: CompilationContent{
			Instructions: "Some instructions",
			Categories: map[string][]CategoryFile{
				"skills": {
					{RelPath: "dir", IsDir: true}, // only a directory, no content
				},
				"agents": {
					{RelPath: "agent.md", Content: []byte("content"), IsDir: false},
				},
				"commands": {}, // empty array
			},
		},
	}

	effectiveState := BuildEffectiveState(result)
	stats := computeStats(result, effectiveState)

	sectionNames := make(map[string]bool)
	for _, s := range stats.Sections {
		sectionNames[s.Name] = true
	}

	// Should have instructions and agents, but NOT skills (only dirs) or commands (empty)
	if !sectionNames["instructions"] {
		t.Error("expected instructions section")
	}
	if !sectionNames["agents"] {
		t.Error("expected agents section")
	}
	if sectionNames["skills"] {
		t.Error("skills section should be skipped (only dirs)")
	}
	if sectionNames["commands"] {
		t.Error("commands section should be skipped (empty)")
	}
}

func TestComputeStats_TokenEstimation(t *testing.T) {
	tests := []struct {
		bytes  int
		tokens int
	}{
		{bytes: 0, tokens: 0},
		{bytes: 1, tokens: 1},    // (1 + 3) / 4 = 1
		{bytes: 4, tokens: 1},    // (4 + 3) / 4 = 1
		{bytes: 5, tokens: 2},    // (5 + 3) / 4 = 2
		{bytes: 100, tokens: 25}, // (100 + 3) / 4 = 25
		{bytes: 101, tokens: 26}, // (101 + 3) / 4 = 26
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := estimateTokens(tt.bytes)
			if got != tt.tokens {
				t.Errorf("estimateTokens(%d) = %d, want %d", tt.bytes, got, tt.tokens)
			}
		})
	}
}

func TestComputeStats_Integration(t *testing.T) {
	// Realistic compilation result
	result := &AccumulatedResult{
		Content: CompilationContent{
			Instructions: string(make([]byte, 49600)), // ~12,400 tokens
			Categories: map[string][]CategoryFile{
				"skills": {
					{RelPath: "skill1.md", Content: make([]byte, 56400), IsDir: false}, // ~14,100 tokens each
					{RelPath: "skill2.md", Content: make([]byte, 56400), IsDir: false},
				},
				"agents": {
					{RelPath: "agent1.md", Content: make([]byte, 25600), IsDir: false}, // ~6,400 tokens
				},
				"commands": {
					{RelPath: "cmd1.md", Content: make([]byte, 12800), IsDir: false}, // ~3,200 tokens
				},
			},
			MCP: map[string]any{
				"mcpServers": map[string]any{
					"server1": map[string]any{},
					"server2": map[string]any{},
					"server3": map[string]any{},
					"server4": map[string]any{},
				},
			},
			Hooks: map[string]any{
				"PreToolUse":  []any{},
				"PostToolUse": []any{},
				"OnError":     []any{},
			},
		},
	}

	effectiveState := BuildEffectiveState(result)
	stats := computeStats(result, effectiveState)

	// Verify rough totals match expectations
	expectedTotalBytes := 49600 + 56400 + 56400 + 25600 + 12800
	if stats.TotalBytes != expectedTotalBytes {
		t.Errorf("TotalBytes = %d, want %d", stats.TotalBytes, expectedTotalBytes)
	}

	expectedTotalTokens := (expectedTotalBytes + 3) / 4
	if stats.TotalTokens != expectedTotalTokens {
		t.Errorf("TotalTokens = %d, want %d", stats.TotalTokens, expectedTotalTokens)
	}

	if stats.MCPServers != 4 {
		t.Errorf("MCPServers = %d, want 4", stats.MCPServers)
	}

	if stats.HookEvents != 3 {
		t.Errorf("HookEvents = %d, want 3", stats.HookEvents)
	}

	// Verify section count
	if len(stats.Sections) != 4 {
		t.Errorf("Sections count = %d, want 4 (instructions, skills, agents, commands)", len(stats.Sections))
	}
}

// TestComputeStats_WithSchema verifies stats work with schema types
func TestComputeStats_WithSchema(t *testing.T) {
	result := &AccumulatedResult{
		Content: CompilationContent{
			Instructions: "Test",
			Settings:     map[string]any{"model": "claude-sonnet-4"},
			Permissions:  map[string]any{"allow": []string{"Read", "Write"}},
			ConfigBlocks: map[string]map[string]any{"environment": {"NODE_ENV": "production"}},
			Sandbox:      &schema.SandboxConfig{Enabled: new(bool)},
		},
	}
	*result.Content.Sandbox.Enabled = true

	effectiveState := BuildEffectiveState(result)
	stats := computeStats(result, effectiveState)

	// Settings, permissions, environment, sandbox are not counted in stats
	// Only Instructions should be counted
	if len(stats.Sections) != 1 {
		t.Errorf("Sections = %d, want 1 (only instructions)", len(stats.Sections))
	}
	if stats.Sections[0].Name != "instructions" {
		t.Errorf("Section name = %q, want %q", stats.Sections[0].Name, "instructions")
	}
}
