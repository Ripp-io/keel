package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteVariables_Basic(t *testing.T) {
	content := []byte("Hello ${keel:name}, welcome to ${keel:project}!")
	vars := map[string]string{"name": "Alice", "project": "Keel"}
	assert.Equal(t, "Hello Alice, welcome to Keel!", string(SubstituteVariables(content, vars)))
}

func TestSubstituteVariables_UnknownLeftAsIs(t *testing.T) {
	content := []byte("${keel:known} and ${keel:unknown}")
	vars := map[string]string{"known": "yes"}
	assert.Equal(t, "yes and ${keel:unknown}", string(SubstituteVariables(content, vars)))
}

func TestSubstituteVariables_EmptyVars(t *testing.T) {
	content := []byte("${keel:foo}")
	got := SubstituteVariables(content, nil)
	assert.Equal(t, string(content), string(got))
}

func TestSubstituteVariablesInMap_FlatStrings(t *testing.T) {
	data := map[string]any{
		"path":    "${keel:bin}/tool",
		"message": "agent is ${keel:keel.agent}",
	}
	vars := map[string]string{"bin": "/usr/local/bin", "keel.agent": "claude"}
	got := SubstituteVariablesInMap(data, vars)

	assert.Equal(t, "/usr/local/bin/tool", got["path"])
	assert.Equal(t, "agent is claude", got["message"])
	assert.Equal(t, "${keel:bin}/tool", data["path"], "original map should not be mutated")
}

func TestSubstituteVariablesInMap_NestedMap(t *testing.T) {
	data := map[string]any{
		"outer": map[string]any{
			"inner": "${keel:val}",
			"deep":  map[string]any{"leaf": "${keel:val}"},
		},
	}
	got := SubstituteVariablesInMap(data, map[string]string{"val": "resolved"})

	outer := got["outer"].(map[string]any)
	assert.Equal(t, "resolved", outer["inner"])
	assert.Equal(t, "resolved", outer["deep"].(map[string]any)["leaf"])
}

func TestSubstituteVariablesInMap_ArrayOfStrings(t *testing.T) {
	data := map[string]any{"args": []any{"${keel:bin}/run", "--config", "${keel:cfg}"}}
	vars := map[string]string{"bin": "/opt", "cfg": "prod.json"}
	got := SubstituteVariablesInMap(data, vars)

	args := got["args"].([]any)
	assert.Equal(t, "/opt/run", args[0])
	assert.Equal(t, "--config", args[1])
	assert.Equal(t, "prod.json", args[2])
}

func TestSubstituteVariablesInMap_NonStringValues(t *testing.T) {
	data := map[string]any{"enabled": true, "count": float64(42), "nothing": nil, "name": "${keel:who}"}
	got := SubstituteVariablesInMap(data, map[string]string{"who": "alice"})

	assert.Equal(t, true, got["enabled"])
	assert.Equal(t, float64(42), got["count"])
	assert.Nil(t, got["nothing"])
	assert.Equal(t, "alice", got["name"])
}

func TestSubstituteVariablesInMap_UnknownVarsLeftAsIs(t *testing.T) {
	data := map[string]any{"known": "${keel:a}", "unknown": "${keel:b}"}
	got := SubstituteVariablesInMap(data, map[string]string{"a": "yes"})

	assert.Equal(t, "yes", got["known"])
	assert.Equal(t, "${keel:b}", got["unknown"])
}

func TestSubstituteVariablesInMap_EmptyAndNilInputs(t *testing.T) {
	t.Run("empty vars is no-op", func(t *testing.T) {
		data := map[string]any{"key": "${keel:val}"}
		got := SubstituteVariablesInMap(data, map[string]string{})
		assert.Equal(t, data["key"], got["key"])
	})

	t.Run("empty data returns empty map", func(t *testing.T) {
		got := SubstituteVariablesInMap(map[string]any{}, map[string]string{"a": "b"})
		assert.Empty(t, got)
	})

	t.Run("nil data returns nil", func(t *testing.T) {
		got := SubstituteVariablesInMap(nil, map[string]string{"a": "b"})
		assert.Nil(t, got)
	})
}

func TestSubstituteVariablesInMap_DepthLimit(t *testing.T) {
	inner := map[string]any{"leaf": "${keel:val}"}
	for i := 0; i < 130; i++ {
		inner = map[string]any{"nested": inner}
	}
	got := SubstituteVariablesInMap(inner, map[string]string{"val": "replaced"})

	current := got
	for i := 0; i < 130; i++ {
		next, ok := current["nested"].(map[string]any)
		require.True(t, ok, "unexpected type at depth %d: %T", i, current["nested"])
		current = next
	}
	leaf, ok := current["leaf"].(string)
	require.True(t, ok, "leaf not a string: %T", current["leaf"])
	assert.Equal(t, "${keel:val}", leaf, "leaf at excessive depth should be unchanged")
}

func TestSubstituteVariablesInMap_MixedArrayTypes(t *testing.T) {
	data := map[string]any{
		"mixed": []any{"${keel:x}", float64(10), true, nil, map[string]any{"inner": "${keel:x}"}},
	}
	got := SubstituteVariablesInMap(data, map[string]string{"x": "val"})

	arr := got["mixed"].([]any)
	assert.Equal(t, "val", arr[0])
	assert.Equal(t, float64(10), arr[1])
	assert.Equal(t, true, arr[2])
	assert.Nil(t, arr[3])
	assert.Equal(t, "val", arr[4].(map[string]any)["inner"])
}

func TestSubstituteVariablesInMap_DoesNotMutateInput(t *testing.T) {
	inner := map[string]any{"key": "${keel:v}"}
	data := map[string]any{"nested": inner, "arr": []any{"${keel:v}"}}

	got := SubstituteVariablesInMap(data, map[string]string{"v": "new"})

	assert.Equal(t, "new", got["nested"].(map[string]any)["key"])
	assert.Equal(t, "${keel:v}", inner["key"], "inner map should not be mutated")
	assert.Equal(t, "${keel:v}", data["arr"].([]any)[0], "array in original data should not be mutated")
}
