package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripForceFromEntries_RemovesForceField(t *testing.T) {
	t.Parallel()
	entries := []any{
		map[string]any{"matcher": "Write(**)", "force": true},
		map[string]any{"matcher": "Read(**)", "action": "allow"},
	}
	result := stripForceFromEntries(entries)
	arr, ok := result.([]any)
	require.True(t, ok, "expected []any result")
	require.Len(t, arr, 2, "entries")
	first := arr[0].(map[string]any)
	assert.NotContains(t, first, "force", "force field should be stripped")
	assert.Equal(t, "Write(**)", first["matcher"], "matcher should be preserved")
	second := arr[1].(map[string]any)
	assert.Equal(t, "allow", second["action"], "action should be preserved")
}

func TestStripForceFromEntries_NonArrayInput(t *testing.T) {
	t.Parallel()
	input := "not an array"
	assert.Equal(t, input, stripForceFromEntries(input), "non-array input should be returned unchanged")
}

func TestHasForceEntry_True(t *testing.T) {
	t.Parallel()
	entries := []any{map[string]any{"matcher": "Write(**)", "force": true}}
	assert.True(t, hasForceEntry(entries), "hasForceEntry should return true")
}

func TestHasForceEntry_False(t *testing.T) {
	t.Parallel()
	entries := []any{map[string]any{"matcher": "Read(**)", "action": "allow"}}
	assert.False(t, hasForceEntry(entries), "hasForceEntry should return false when no force: true")
}

func TestHasForceEntry_NonArray(t *testing.T) {
	t.Parallel()
	assert.False(t, hasForceEntry("not an array"), "hasForceEntry should return false for non-array input")
}

func TestMergeHookEvent_ConcatenatesArrays(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"preToolUse": []any{map[string]any{"matcher": "Write(**)"}}}
	mergeHookEvent(dst, "preToolUse", []any{map[string]any{"matcher": "Read(**)"}})
	arr := dst["preToolUse"].([]any)
	require.Len(t, arr, 2, "entries")
}

func TestMergeHookEvent_NewKey(t *testing.T) {
	t.Parallel()
	dst := make(map[string]any)
	mergeHookEvent(dst, "postToolUse", []any{map[string]any{"matcher": "Write(**)"}})
	arr := dst["postToolUse"].([]any)
	require.Len(t, arr, 1, "entries")
}

