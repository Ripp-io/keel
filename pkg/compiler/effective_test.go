package compiler

import (
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

func TestBuildEffectiveState(t *testing.T) {
	tests := []struct {
		name   string
		result *AccumulatedResult
		want   *manifest.EffectiveState
	}{
		{
			name:   "nil result",
			result: nil,
			want:   nil,
		},
		{
			name: "fully populated result",
			result: &AccumulatedResult{
				Content: CompilationContent{
					Instructions: "# Test\nSome instruc",
					Permissions: map[string]any{
						"allow": []any{"Read", "Edit", "Write"},
						"deny":  []any{"Bash"},
					},
					MCP: map[string]any{
						"mcpServers": map[string]any{
							"server1": map[string]any{"command": "echo"},
							"server2": map[string]any{"url": "http://example.com"},
						},
					},
					Hooks: map[string]any{
						"PreToolUse":  []any{map[string]any{"name": "hook1"}, map[string]any{"name": "hook2"}},
						"PostToolUse": []any{map[string]any{"name": "hook3"}},
					},
					Settings: map[string]any{
						"model":       "claude-sonnet-4",
						"effortLevel": "high",
					},
					Categories: map[string][]CategoryFile{
						"commands": {
							{RelPath: "deploy.md", Content: []byte("# Deploy"), IsDir: false},
							{RelPath: "subdir", Content: nil, IsDir: true},
						},
						"skills": {
							{RelPath: "skill1.md", Content: []byte("# Skill"), IsDir: false},
						},
					},
				},
			},
			want: &manifest.EffectiveState{
				Permissions: &manifest.EffectivePermissions{
					Allow: []string{"Edit", "Read", "Write"},
					Deny:  []string{"Bash"},
				},
				MCPServers: []manifest.EffectiveMCPServer{
					{Name: "server1", Type: "stdio"},
					{Name: "server2", Type: "sse"},
				},
				Hooks: []manifest.EffectiveHook{
					{Event: "PostToolUse", Handlers: 1},
					{Event: "PreToolUse", Handlers: 2},
				},
				Settings: map[string]any{
					"model":       "claude-sonnet-4",
					"effortLevel": "high",
				},
				Instructions: &manifest.EffectiveInstructions{
					Bytes:         19,
					TokenEstimate: 5, // (19 + 3) / 4 = 22 / 4 = 5
				},
				Categories: []manifest.EffectiveCategory{
					{Name: "commands", FileCount: 1},
					{Name: "skills", FileCount: 1},
				},
			},
		},
		{
			name: "minimal result with only settings",
			result: &AccumulatedResult{
				Content: CompilationContent{
					Settings: map[string]any{
						"model": "claude-opus-4",
					},
				},
			},
			want: &manifest.EffectiveState{
				Settings: map[string]any{
					"model": "claude-opus-4",
				},
			},
		},
		{
			name: "empty result",
			result: &AccumulatedResult{
				Content: CompilationContent{
					Settings:    map[string]any{},
					Permissions: map[string]any{},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildEffectiveState(tt.result)
			if !deepEqualEffectiveState(got, tt.want) {
				t.Errorf("BuildEffectiveState() failed")
				if got == nil || tt.want == nil {
					t.Errorf("  got nil=%v, want nil=%v", got == nil, tt.want == nil)
				} else {
					if !deepEqualPermissions(got.Permissions, tt.want.Permissions) {
						t.Errorf("  Permissions differ: got=%+v, want=%+v", got.Permissions, tt.want.Permissions)
					}
					if !deepEqualMCPServers(got.MCPServers, tt.want.MCPServers) {
						t.Errorf("  MCPServers differ: got=%+v, want=%+v", got.MCPServers, tt.want.MCPServers)
					}
					if !deepEqualHooks(got.Hooks, tt.want.Hooks) {
						t.Errorf("  Hooks differ: got=%+v, want=%+v", got.Hooks, tt.want.Hooks)
					}
					if !deepEqualMap(got.Settings, tt.want.Settings) {
						t.Errorf("  Settings differ: got=%+v, want=%+v", got.Settings, tt.want.Settings)
					}
					if !deepEqualInstructions(got.Instructions, tt.want.Instructions) {
						t.Errorf("  Instructions differ: got=%+v, want=%+v", got.Instructions, tt.want.Instructions)
					}
					if !deepEqualCategories(got.Categories, tt.want.Categories) {
						t.Errorf("  Categories differ: got=%+v, want=%+v", got.Categories, tt.want.Categories)
					}
				}
			}
		})
	}
}

func TestBuildEffectiveStateFromIR(t *testing.T) {
	tests := []struct {
		name string
		ir   *LayerIR
		want *manifest.EffectiveState
	}{
		{
			name: "nil IR",
			ir:   nil,
			want: nil,
		},
		{
			name: "IR with instruction blocks",
			ir: &LayerIR{
				Instructions: []InstructionBlock{
					{Content: "# Part 1\n"},
					{Content: "# Part 2\n"},
				},
				Permissions: map[string]any{
					"allow": []any{"Read"},
				},
			},
			want: &manifest.EffectiveState{
				Permissions: &manifest.EffectivePermissions{
					Allow: []string{"Read"},
				},
				Instructions: &manifest.EffectiveInstructions{
					Bytes:         18,
					TokenEstimate: 5, // (18 + 3) / 4
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildEffectiveStateFromIR(tt.ir)
			if !deepEqualEffectiveState(got, tt.want) {
				t.Errorf("BuildEffectiveStateFromIR() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestExtractPermissions(t *testing.T) {
	tests := []struct {
		name  string
		perms map[string]any
		want  *manifest.EffectivePermissions
	}{
		{
			name:  "nil permissions",
			perms: nil,
			want:  nil,
		},
		{
			name:  "empty permissions",
			perms: map[string]any{},
			want:  nil,
		},
		{
			name: "allow only",
			perms: map[string]any{
				"allow": []any{"Write", "Read", "Edit"},
			},
			want: &manifest.EffectivePermissions{
				Allow: []string{"Edit", "Read", "Write"},
			},
		},
		{
			name: "deny only",
			perms: map[string]any{
				"deny": []any{"Bash", "WebSearch"},
			},
			want: &manifest.EffectivePermissions{
				Deny: []string{"Bash", "WebSearch"},
			},
		},
		{
			name: "both allow and deny",
			perms: map[string]any{
				"allow": []any{"Read"},
				"deny":  []any{"Bash"},
			},
			want: &manifest.EffectivePermissions{
				Allow: []string{"Read"},
				Deny:  []string{"Bash"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPermissions(tt.perms)
			if !deepEqualPermissions(got, tt.want) {
				t.Errorf("extractPermissions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractMCPServers(t *testing.T) {
	tests := []struct {
		name string
		mcp  map[string]any
		want []manifest.EffectiveMCPServer
	}{
		{
			name: "nil MCP",
			mcp:  nil,
			want: nil,
		},
		{
			name: "no mcpServers key",
			mcp:  map[string]any{},
			want: nil,
		},
		{
			name: "stdio servers",
			mcp: map[string]any{
				"mcpServers": map[string]any{
					"server1": map[string]any{"command": "echo"},
					"server2": map[string]any{"command": "cat"},
				},
			},
			want: []manifest.EffectiveMCPServer{
				{Name: "server1", Type: "stdio"},
				{Name: "server2", Type: "stdio"},
			},
		},
		{
			name: "SSE server",
			mcp: map[string]any{
				"mcpServers": map[string]any{
					"sse-srv": map[string]any{"url": "http://example.com"},
				},
			},
			want: []manifest.EffectiveMCPServer{
				{Name: "sse-srv", Type: "sse"},
			},
		},
		{
			name: "streamable-http server",
			mcp: map[string]any{
				"mcpServers": map[string]any{
					"http-srv": map[string]any{
						"url":       "http://example.com",
						"transport": "streamable-http",
					},
				},
			},
			want: []manifest.EffectiveMCPServer{
				{Name: "http-srv", Type: "streamable-http"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMCPServers(tt.mcp)
			if !deepEqualMCPServers(got, tt.want) {
				t.Errorf("extractMCPServers() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractHooks(t *testing.T) {
	tests := []struct {
		name  string
		hooks map[string]any
		want  []manifest.EffectiveHook
	}{
		{
			name:  "nil hooks",
			hooks: nil,
			want:  nil,
		},
		{
			name:  "empty hooks",
			hooks: map[string]any{},
			want:  nil,
		},
		{
			name: "single hook",
			hooks: map[string]any{
				"PreToolUse": []any{
					map[string]any{"name": "hook1"},
				},
			},
			want: []manifest.EffectiveHook{
				{Event: "PreToolUse", Handlers: 1},
			},
		},
		{
			name: "multiple hooks sorted",
			hooks: map[string]any{
				"PostToolUse": []any{
					map[string]any{"name": "hook1"},
				},
				"PreToolUse": []any{
					map[string]any{"name": "hook2"},
					map[string]any{"name": "hook3"},
				},
			},
			want: []manifest.EffectiveHook{
				{Event: "PostToolUse", Handlers: 1},
				{Event: "PreToolUse", Handlers: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHooks(tt.hooks)
			if !deepEqualHooks(got, tt.want) {
				t.Errorf("extractHooks() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractInstructions(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *manifest.EffectiveInstructions
	}{
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "small content",
			content: "test",
			want: &manifest.EffectiveInstructions{
				Bytes:         4,
				TokenEstimate: 1, // (4 + 3) / 4
			},
		},
		{
			name:    "larger content",
			content: "This is a longer test string for token estimation",
			want: &manifest.EffectiveInstructions{
				Bytes:         49,
				TokenEstimate: 13, // (49 + 3) / 4 = 52 / 4 = 13
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInstructions(tt.content)
			if !deepEqualInstructions(got, tt.want) {
				t.Errorf("extractInstructions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractCategories(t *testing.T) {
	tests := []struct {
		name string
		cats map[string][]CategoryFile
		want []manifest.EffectiveCategory
	}{
		{
			name: "nil categories",
			cats: nil,
			want: nil,
		},
		{
			name: "empty categories",
			cats: map[string][]CategoryFile{},
			want: nil,
		},
		{
			name: "single category with files",
			cats: map[string][]CategoryFile{
				"commands": {
					{RelPath: "deploy.md", IsDir: false},
					{RelPath: "build.md", IsDir: false},
				},
			},
			want: []manifest.EffectiveCategory{
				{Name: "commands", FileCount: 2},
			},
		},
		{
			name: "multiple categories sorted, directories excluded",
			cats: map[string][]CategoryFile{
				"skills": {
					{RelPath: "skill1.md", IsDir: false},
					{RelPath: "subdir", IsDir: true},
				},
				"commands": {
					{RelPath: "deploy.md", IsDir: false},
				},
			},
			want: []manifest.EffectiveCategory{
				{Name: "commands", FileCount: 1},
				{Name: "skills", FileCount: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCategories(tt.cats)
			if !deepEqualCategories(got, tt.want) {
				t.Errorf("extractCategories() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Helper functions for deep equality checks

func deepEqualEffectiveState(a, b *manifest.EffectiveState) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !deepEqualPermissions(a.Permissions, b.Permissions) {
		return false
	}
	if !deepEqualMCPServers(a.MCPServers, b.MCPServers) {
		return false
	}
	if !deepEqualHooks(a.Hooks, b.Hooks) {
		return false
	}
	if !deepEqualMap(a.Settings, b.Settings) {
		return false
	}
	if !deepEqualInstructions(a.Instructions, b.Instructions) {
		return false
	}
	if !deepEqualCategories(a.Categories, b.Categories) {
		return false
	}
	return true
}

func deepEqualPermissions(a, b *manifest.EffectivePermissions) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !stringSliceEqual(a.Allow, b.Allow) {
		return false
	}
	if !stringSliceEqual(a.Deny, b.Deny) {
		return false
	}
	return true
}

func deepEqualMCPServers(a, b []manifest.EffectiveMCPServer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Type != b[i].Type {
			return false
		}
	}
	return true
}

func deepEqualHooks(a, b []manifest.EffectiveHook) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Event != b[i].Event || a[i].Handlers != b[i].Handlers {
			return false
		}
	}
	return true
}

func deepEqualInstructions(a, b *manifest.EffectiveInstructions) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Bytes == b.Bytes && a.TokenEstimate == b.TokenEstimate
}

func deepEqualCategories(a, b []manifest.EffectiveCategory) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].FileCount != b[i].FileCount {
			return false
		}
	}
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func deepEqualMap(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}
