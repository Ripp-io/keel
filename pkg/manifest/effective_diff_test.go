package manifest

import (
	"testing"
)

func TestDiffEffectiveState(t *testing.T) {
	tests := []struct {
		name     string
		compiled *EffectiveState
		current  *EffectiveState
		want     *EffectiveStateDiff
	}{
		{
			name:     "identical states",
			compiled: &EffectiveState{},
			current:  &EffectiveState{},
			want:     nil,
		},
		{
			name: "permission added to allow",
			compiled: &EffectiveState{
				Permissions: &EffectivePermissions{
					Allow: []string{"Read"},
				},
			},
			current: &EffectiveState{
				Permissions: &EffectivePermissions{
					Allow: []string{"Bash", "Read"},
				},
			},
			want: &EffectiveStateDiff{
				Permissions: &PermissionsDiff{
					AllowAdded: []string{"Bash"},
				},
			},
		},
		{
			name: "MCP server added",
			compiled: &EffectiveState{
				MCPServers: []EffectiveMCPServer{
					{Name: "server1", Type: "stdio"},
				},
			},
			current: &EffectiveState{
				MCPServers: []EffectiveMCPServer{
					{Name: "server1", Type: "stdio"},
					{Name: "server2", Type: "stdio"},
				},
			},
			want: &EffectiveStateDiff{
				MCPServers: &ListDiff{
					Added: []string{"server2"},
				},
			},
		},
		{
			name: "setting changed",
			compiled: &EffectiveState{
				Settings: map[string]any{
					"model": "claude-sonnet-4",
				},
			},
			current: &EffectiveState{
				Settings: map[string]any{
					"model": "claude-opus-4",
				},
			},
			want: &EffectiveStateDiff{
				Settings: []SettingChange{
					{
						Key:  "model",
						From: "claude-sonnet-4",
						To:   "claude-opus-4",
					},
				},
			},
		},
		{
			name: "instruction size changed",
			compiled: &EffectiveState{
				Instructions: &EffectiveInstructions{
					Bytes:         100,
					TokenEstimate: 25,
				},
			},
			current: &EffectiveState{
				Instructions: &EffectiveInstructions{
					Bytes:         120,
					TokenEstimate: 30,
				},
			},
			want: &EffectiveStateDiff{
				Instructions: &InstructionsDiff{
					ByteDelta:  20,
					TokenDelta: 5,
				},
			},
		},
		{
			name: "category file count changed",
			compiled: &EffectiveState{
				Categories: []EffectiveCategory{
					{Name: "commands", FileCount: 5},
				},
			},
			current: &EffectiveState{
				Categories: []EffectiveCategory{
					{Name: "commands", FileCount: 7},
				},
			},
			want: &EffectiveStateDiff{
				Categories: []CategoryDiff{
					{Name: "commands", CountDelta: 2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiffEffectiveState(tt.compiled, tt.current)
			if !deepEqualDiff(got, tt.want) {
				t.Errorf("DiffEffectiveState() failed")
				if got == nil || tt.want == nil {
					t.Errorf("  got nil=%v, want nil=%v", got == nil, tt.want == nil)
				} else {
					t.Errorf("  got=%+v", got)
					t.Errorf("  want=%+v", tt.want)
				}
			}
		})
	}
}

func TestHasChanges(t *testing.T) {
	tests := []struct {
		name string
		diff *EffectiveStateDiff
		want bool
	}{
		{
			name: "nil diff",
			diff: nil,
			want: false,
		},
		{
			name: "empty diff",
			diff: &EffectiveStateDiff{},
			want: false,
		},
		{
			name: "has permission changes",
			diff: &EffectiveStateDiff{
				Permissions: &PermissionsDiff{
					AllowAdded: []string{"Bash"},
				},
			},
			want: true,
		},
		{
			name: "has MCP changes",
			diff: &EffectiveStateDiff{
				MCPServers: &ListDiff{
					Added: []string{"server1"},
				},
			},
			want: true,
		},
		{
			name: "has setting changes",
			diff: &EffectiveStateDiff{
				Settings: []SettingChange{
					{Key: "model", From: "a", To: "b"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.diff.HasChanges()
			if got != tt.want {
				t.Errorf("HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiffStringLists(t *testing.T) {
	tests := []struct {
		name        string
		compiled    []string
		current     []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:     "identical lists",
			compiled: []string{"a", "b"},
			current:  []string{"a", "b"},
		},
		{
			name:      "item added",
			compiled:  []string{"a"},
			current:   []string{"a", "b"},
			wantAdded: []string{"b"},
		},
		{
			name:        "item removed",
			compiled:    []string{"a", "b"},
			current:     []string{"a"},
			wantRemoved: []string{"b"},
		},
		{
			name:        "items added and removed",
			compiled:    []string{"a", "b"},
			current:     []string{"b", "c"},
			wantAdded:   []string{"c"},
			wantRemoved: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := diffStringLists(tt.compiled, tt.current)
			if !stringSliceEqual(added, tt.wantAdded) {
				t.Errorf("added = %v, want %v", added, tt.wantAdded)
			}
			if !stringSliceEqual(removed, tt.wantRemoved) {
				t.Errorf("removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}
}

// Helper to compare diffs
func deepEqualDiff(a, b *EffectiveStateDiff) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if !deepEqualPermissionsDiff(a.Permissions, b.Permissions) {
		return false
	}
	if !deepEqualListDiff(a.MCPServers, b.MCPServers) {
		return false
	}
	if !deepEqualListDiff(a.Hooks, b.Hooks) {
		return false
	}
	if !deepEqualSettingChanges(a.Settings, b.Settings) {
		return false
	}
	if !deepEqualInstructionsDiff(a.Instructions, b.Instructions) {
		return false
	}
	if !deepEqualCategoryDiffs(a.Categories, b.Categories) {
		return false
	}

	return true
}

func deepEqualPermissionsDiff(a, b *PermissionsDiff) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return stringSliceEqual(a.AllowAdded, b.AllowAdded) &&
		stringSliceEqual(a.AllowRemoved, b.AllowRemoved) &&
		stringSliceEqual(a.DenyAdded, b.DenyAdded) &&
		stringSliceEqual(a.DenyRemoved, b.DenyRemoved)
}

func deepEqualListDiff(a, b *ListDiff) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return stringSliceEqual(a.Added, b.Added) && stringSliceEqual(a.Removed, b.Removed)
}

func deepEqualSettingChanges(a, b []SettingChange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].From != b[i].From || a[i].To != b[i].To {
			return false
		}
	}
	return true
}

func deepEqualInstructionsDiff(a, b *InstructionsDiff) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.ByteDelta == b.ByteDelta && a.TokenDelta == b.TokenDelta
}

func deepEqualCategoryDiffs(a, b []CategoryDiff) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].CountDelta != b[i].CountDelta {
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

func TestSettingValuesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "one nil",
			a:    nil,
			b:    "value",
			want: false,
		},
		{
			name: "equal strings",
			a:    "test",
			b:    "test",
			want: true,
		},
		{
			name: "different strings",
			a:    "test1",
			b:    "test2",
			want: false,
		},
		{
			name: "equal ints",
			a:    42,
			b:    42,
			want: true,
		},
		{
			name: "different ints",
			a:    42,
			b:    43,
			want: false,
		},
		{
			name: "equal bools",
			a:    true,
			b:    true,
			want: true,
		},
		{
			name: "different bools",
			a:    true,
			b:    false,
			want: false,
		},
		{
			name: "equal float64s",
			a:    3.14,
			b:    3.14,
			want: true,
		},
		{
			name: "different float64s",
			a:    3.14,
			b:    2.71,
			want: false,
		},
		{
			name: "equal empty slices",
			a:    []any{},
			b:    []any{},
			want: true,
		},
		{
			name: "equal slices",
			a:    []any{"a", "b", "c"},
			b:    []any{"a", "b", "c"},
			want: true,
		},
		{
			name: "different length slices",
			a:    []any{"a", "b"},
			b:    []any{"a"},
			want: false,
		},
		{
			name: "different slice elements",
			a:    []any{"a", "b"},
			b:    []any{"a", "c"},
			want: false,
		},
		{
			name: "slice vs non-slice",
			a:    []any{"a"},
			b:    "a",
			want: false,
		},
		{
			name: "nested equal slices",
			a:    []any{[]any{"nested"}},
			b:    []any{[]any{"nested"}},
			want: true,
		},
		{
			name: "nested different slices",
			a:    []any{[]any{"a"}},
			b:    []any{[]any{"b"}},
			want: false,
		},
		{
			name: "equal empty maps",
			a:    map[string]any{},
			b:    map[string]any{},
			want: true,
		},
		{
			name: "equal maps",
			a:    map[string]any{"key1": "value1", "key2": 42},
			b:    map[string]any{"key1": "value1", "key2": 42},
			want: true,
		},
		{
			name: "different map sizes",
			a:    map[string]any{"key1": "value1"},
			b:    map[string]any{"key1": "value1", "key2": "value2"},
			want: false,
		},
		{
			name: "different map values",
			a:    map[string]any{"key1": "value1"},
			b:    map[string]any{"key1": "value2"},
			want: false,
		},
		{
			name: "missing map key",
			a:    map[string]any{"key1": "value1"},
			b:    map[string]any{"key2": "value1"},
			want: false,
		},
		{
			name: "map vs non-map",
			a:    map[string]any{"key": "value"},
			b:    "value",
			want: false,
		},
		{
			name: "nested equal maps",
			a:    map[string]any{"outer": map[string]any{"inner": "value"}},
			b:    map[string]any{"outer": map[string]any{"inner": "value"}},
			want: true,
		},
		{
			name: "nested different maps",
			a:    map[string]any{"outer": map[string]any{"inner": "value1"}},
			b:    map[string]any{"outer": map[string]any{"inner": "value2"}},
			want: false,
		},
		{
			name: "complex nested structure equal",
			a: map[string]any{
				"list": []any{
					map[string]any{"key": "value"},
					"string",
					42,
				},
			},
			b: map[string]any{
				"list": []any{
					map[string]any{"key": "value"},
					"string",
					42,
				},
			},
			want: true,
		},
		{
			name: "complex nested structure different",
			a: map[string]any{
				"list": []any{
					map[string]any{"key": "value1"},
				},
			},
			b: map[string]any{
				"list": []any{
					map[string]any{"key": "value2"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := settingValuesEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("settingValuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
