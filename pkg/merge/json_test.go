package merge

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

func Test_deepMergeJSON_ScalarOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "dst.json")
	testutil.WriteJSON(t, dst, map[string]any{"a": float64(1), "b": "keep"})

	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{"a": float64(2)})

	require.NoError(t, deepMergeJSON(src, dst))

	got := testutil.ReadJSONMap(t, dst)
	assert.Equal(t, float64(2), got["a"])
	assert.Equal(t, "keep", got["b"])
}

func Test_deepMergeJSON_NestedMerge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "dst.json")
	testutil.WriteJSON(t, dst, map[string]any{
		"a": map[string]any{"b": float64(1)},
	})

	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{
		"a": map[string]any{"c": float64(2)},
	})

	require.NoError(t, deepMergeJSON(src, dst))

	got := testutil.ReadJSONMap(t, dst)
	nested, ok := got["a"].(map[string]any)
	require.True(t, ok, "expected a to be a map, got %T", got["a"])
	assert.Equal(t, float64(1), nested["b"])
	assert.Equal(t, float64(2), nested["c"])
}

func Test_deepMergeJSON_ArrayReplace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "dst.json")
	testutil.WriteJSON(t, dst, map[string]any{
		"a": []any{float64(1), float64(2)},
	})

	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{
		"a": []any{float64(3), float64(4)},
	})

	require.NoError(t, deepMergeJSON(src, dst))

	got := testutil.ReadJSONMap(t, dst)
	arr, ok := got["a"].([]any)
	require.True(t, ok, "expected a to be an array, got %T", got["a"])
	require.Len(t, arr, 2)
	assert.Equal(t, float64(3), arr[0])
	assert.Equal(t, float64(4), arr[1])
}

func Test_deepMergeJSON_CreateNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "sub", "dst.json")
	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{"x": "hello"})

	require.NoError(t, deepMergeJSON(src, dst))

	got := testutil.ReadJSONMap(t, dst)
	assert.Equal(t, "hello", got["x"])
}

func Test_deepMergeJSON_SrcMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "dst.json")
	testutil.WriteJSON(t, dst, map[string]any{"a": float64(1)})

	src := filepath.Join(dir, "nonexistent.json")
	require.NoError(t, deepMergeJSON(src, dst))

	got := testutil.ReadJSONMap(t, dst)
	assert.Equal(t, float64(1), got["a"], "dst should be unchanged")
}

func Test_deepMergeJSON_BothMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "nonexistent_src.json")
	dst := filepath.Join(dir, "nonexistent_dst.json")

	require.NoError(t, deepMergeJSON(src, dst), "should be no-op when src missing")

	_, err := os.Stat(dst)
	assert.True(t, os.IsNotExist(err), "dst should not exist when src is missing")
}

func Test_deepMergeJSON_InvalidSrcJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "src.json")
	require.NoError(t, os.WriteFile(src, []byte("not json"), 0644))

	dst := filepath.Join(dir, "dst.json")

	err := deepMergeJSON(src, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), src)
	assert.Contains(t, err.Error(), "source")
}

func Test_deepMergeJSON_InvalidDstJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{"a": float64(1)})

	dst := filepath.Join(dir, "dst.json")
	require.NoError(t, os.WriteFile(dst, []byte("not json"), 0644))

	err := deepMergeJSON(src, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), dst)
	assert.Contains(t, err.Error(), "destination")
}

func Test_deepMergeJSON_NestedOverrideScalarWithMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "dst.json")
	testutil.WriteJSON(t, dst, map[string]any{"a": "scalar"})

	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{
		"a": map[string]any{"nested": true},
	})

	require.NoError(t, deepMergeJSON(src, dst))

	got := testutil.ReadJSONMap(t, dst)
	nested, ok := got["a"].(map[string]any)
	require.True(t, ok, "expected a to be a map after override, got %T", got["a"])
	assert.Equal(t, true, nested["nested"])
}

func Test_deepMergeJSON_MultipleSequentialMerges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	dst := filepath.Join(dir, "dst.json")
	testutil.WriteJSON(t, dst, map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read"},
		},
	})

	// First merge: add hooks, replace permissions.allow
	src1 := filepath.Join(dir, "src1.json")
	testutil.WriteJSON(t, src1, map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read", "Write"},
		},
		"hooks": map[string]any{
			"pre": "check",
		},
	})
	require.NoError(t, deepMergeJSON(src1, dst))

	// Second merge: replace permissions.allow again
	src2 := filepath.Join(dir, "src2.json")
	testutil.WriteJSON(t, src2, map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read", "Write", "Edit"},
		},
	})
	require.NoError(t, deepMergeJSON(src2, dst))

	got := testutil.ReadJSONMap(t, dst)

	hooks, ok := got["hooks"].(map[string]any)
	require.True(t, ok, "hooks should survive sequential merges")
	assert.Equal(t, "check", hooks["pre"])

	perms := got["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 3, "permissions.allow should be src2's value")
}

func Test_deepMergeJSON_OutputTrailingNewline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := filepath.Join(dir, "src.json")
	testutil.WriteJSON(t, src, map[string]any{"a": float64(1)})

	dst := filepath.Join(dir, "dst.json")
	require.NoError(t, deepMergeJSON(src, dst))

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	assert.Equal(t, byte('\n'), data[len(data)-1], "output should end with newline")
}

// --- DeepMergeMapsWithPolicies tests ---

func TestDeepMergeMapsWithPolicies_Additive(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"allow": []any{"Read", "Edit"}}
	src := map[string]any{"allow": []any{"Read", "Write"}}
	policies := map[string]policy.KeelPolicy{
		"allow": {Merge: policy.MergeStrategyAdditive},
	}

	result := DeepMergeMapsWithPolicies(dst, src, policies)

	arr, ok := result["allow"].([]any)
	require.True(t, ok, "expected allow to be []any, got %T", result["allow"])
	require.Len(t, arr, 3, "should be union: [Read, Edit, Write]")
	want := []string{"Read", "Edit", "Write"}
	for i, w := range want {
		assert.Equal(t, w, fmt.Sprintf("%v", arr[i]))
	}
}

func TestDeepMergeMapsWithPolicies_Replace(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"allow": []any{"Read", "Edit"}}
	src := map[string]any{"allow": []any{"Write"}}
	policies := map[string]policy.KeelPolicy{
		"allow": {Merge: policy.MergeStrategyReplace},
	}

	result := DeepMergeMapsWithPolicies(dst, src, policies)

	arr, ok := result["allow"].([]any)
	require.True(t, ok, "expected allow to be []any, got %T", result["allow"])
	require.Len(t, arr, 1, "replace: src wins entirely")
	assert.Equal(t, "Write", arr[0])
}

func TestDeepMergeMapsWithPolicies_NoPolicy(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"a": map[string]any{"b": float64(1)},
		"c": []any{"x"},
	}
	src := map[string]any{
		"a": map[string]any{"d": float64(2)},
		"c": []any{"y"},
	}

	result := DeepMergeMapsWithPolicies(dst, src, map[string]policy.KeelPolicy{})

	nested, ok := result["a"].(map[string]any)
	require.True(t, ok, "expected a to be map, got %T", result["a"])
	assert.Equal(t, float64(1), nested["b"])
	assert.Equal(t, float64(2), nested["d"])

	arr, ok := result["c"].([]any)
	require.True(t, ok, "expected c to be []any, got %T", result["c"])
	assert.Equal(t, []any{"y"}, arr, "arrays: LWW (default, no additive policy)")
}

func TestDeepMergeMapsWithPolicies_NilPolicies(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"a": float64(1), "b": "keep"}
	src := map[string]any{"a": float64(2)}

	result := DeepMergeMapsWithPolicies(dst, src, nil)

	assert.Equal(t, float64(2), result["a"])
	assert.Equal(t, "keep", result["b"])
}

func TestDeepMergeMapsWithPolicies_NestedPath_DirectPolicy(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read", "Edit"},
		},
	}
	src := map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read", "Write"},
		},
	}
	policies := map[string]policy.KeelPolicy{
		"permissions.allow": {Merge: policy.MergeStrategyAdditive},
	}

	result := DeepMergeMapsWithPolicies(dst, src, policies)

	perms, ok := result["permissions"].(map[string]any)
	require.True(t, ok)
	arr, ok := perms["allow"].([]any)
	require.True(t, ok)
	require.Len(t, arr, 3)
	want := []string{"Read", "Edit", "Write"}
	for i, w := range want {
		assert.Equal(t, w, fmt.Sprintf("%v", arr[i]))
	}
}

func TestDeepMergeMapsWithPolicies_NestedPath_ParentPolicy(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read", "Edit"},
			"deny":  []any{"Bash(rm:*)"},
		},
	}
	src := map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Read", "Write"},
			"deny":  []any{"Bash(rm:*)", "Bash(sudo:*)"},
		},
	}
	policies := map[string]policy.KeelPolicy{
		"permissions": {Merge: policy.MergeStrategyAdditive},
	}

	result := DeepMergeMapsWithPolicies(dst, src, policies)

	perms, ok := result["permissions"].(map[string]any)
	require.True(t, ok)

	allow, ok := perms["allow"].([]any)
	require.True(t, ok)
	assert.Len(t, allow, 3, "union of allow arrays")

	deny, ok := perms["deny"].([]any)
	require.True(t, ok)
	assert.Len(t, deny, 2, "union of deny arrays")
}

func TestDeepMergeMapsWithPolicies_AdditiveNonArray(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"x": "old"}
	src := map[string]any{"x": "new"}
	policies := map[string]policy.KeelPolicy{
		"x": {Merge: policy.MergeStrategyAdditive},
	}

	result := DeepMergeMapsWithPolicies(dst, src, policies)
	assert.Equal(t, "new", result["x"], "should LWW fallback for non-array additive")
}

func TestDeepMergeMapsWithPolicies_DepthLimit(t *testing.T) {
	t.Parallel()
	src := map[string]any{"key": "deep-value"}
	for i := 0; i < 140; i++ {
		src = map[string]any{"level": src}
	}
	dst := map[string]any{}

	result := DeepMergeMapsWithPolicies(dst, src, nil)
	require.NotNil(t, result)
}

// --- unionArrays tests ---

func TestUnionArrays(t *testing.T) {
	t.Parallel()
	dst := []any{"Read", "Edit"}
	src := []any{"Read", "Write", "Edit"}

	result := unionArrays(dst, src)

	require.Len(t, result, 3, "dst first, then new items from src")
	want := []string{"Read", "Edit", "Write"}
	for i, w := range want {
		assert.Equal(t, w, fmt.Sprintf("%v", result[i]))
	}
}

func TestUnionArrays_Empty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		dst, src []any
		wantLen  int
	}{
		{"both empty", []any{}, []any{}, 0},
		{"dst empty", []any{}, []any{"a", "b"}, 2},
		{"src empty", []any{"a", "b"}, []any{}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := unionArrays(tt.dst, tt.src)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestUnionArrays_MixedTypes(t *testing.T) {
	t.Parallel()
	dst := []any{float64(1), "hello", true}
	src := []any{float64(2), "hello", false, float64(1)}

	result := unionArrays(dst, src)
	// dst: [1, "hello", true] + new from src: [2, false]
	assert.Len(t, result, 5)
}

func TestUnionArrays_PrimitiveDedup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		dst     []any
		src     []any
		wantLen int
		want    []any
	}{
		{
			name:    "string dedup",
			dst:     []any{"a", "b", "c"},
			src:     []any{"b", "c", "d"},
			wantLen: 4,
			want:    []any{"a", "b", "c", "d"},
		},
		{
			name:    "number dedup",
			dst:     []any{float64(1), float64(2)},
			src:     []any{float64(2), float64(3)},
			wantLen: 3,
			want:    []any{float64(1), float64(2), float64(3)},
		},
		{
			name:    "bool dedup",
			dst:     []any{true, false},
			src:     []any{true},
			wantLen: 2,
			want:    []any{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := unionArrays(tt.dst, tt.src)
			assert.Len(t, result, tt.wantLen)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestUnionArrays_ComplexTypeLimitation(t *testing.T) {
	t.Parallel()
	// This test documents the known limitation: maps with the same %v
	// representation are treated as duplicates even when structurally
	// different maps could have the same string representation.
	//
	// Two identical maps should deduplicate (works correctly):
	dst := []any{
		map[string]any{"name": "server1"},
	}
	src := []any{
		map[string]any{"name": "server1"},
	}

	result := unionArrays(dst, src)
	// Both maps have the same %v output, so src entry is treated as a duplicate.
	assert.Len(t, result, 1, "identical maps should deduplicate via %%v")

	// Two different maps should NOT deduplicate (works when %v differs):
	dst2 := []any{
		map[string]any{"name": "server1"},
	}
	src2 := []any{
		map[string]any{"name": "server2"},
	}

	result2 := unionArrays(dst2, src2)
	assert.Len(t, result2, 2, "different maps should not deduplicate")
}
