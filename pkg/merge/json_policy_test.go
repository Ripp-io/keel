package merge

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
)

func TestStripKeelPolicies_Basic(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"mcpServers": map[string]any{
			"gitlab": map[string]any{"command": "npx", "args": []any{"gitlab-mcp"}, "_keel": map[string]any{"force": true}},
			"sonar":  map[string]any{"command": "npx"},
		},
	}
	cleaned, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	gitlab := cleaned["mcpServers"].(map[string]any)["gitlab"].(map[string]any)
	assert.NotContains(t, gitlab, "_keel")
	assert.Equal(t, "npx", gitlab["command"])
	assert.Len(t, gitlab["args"].([]any), 1)
	sonar := cleaned["mcpServers"].(map[string]any)["sonar"].(map[string]any)
	assert.Equal(t, "npx", sonar["command"])
	kp, ok := policies["mcpServers.gitlab"]
	require.True(t, ok)
	require.NotNil(t, kp.Force)
	assert.True(t, *kp.Force)
	assert.NotContains(t, policies, "mcpServers.sonar")
}

func TestStripKeelPolicies_NoKeel(t *testing.T) {
	t.Parallel()
	data := map[string]any{"mcpServers": map[string]any{"gitlab": map[string]any{"command": "npx", "args": []any{"gitlab-mcp"}}}, "version": float64(1)}
	cleaned, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	assert.Equal(t, "npx", cleaned["mcpServers"].(map[string]any)["gitlab"].(map[string]any)["command"])
	assert.Equal(t, float64(1), cleaned["version"])
	assert.Empty(t, policies)
}

func TestStripKeelPolicies_Nested(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"_keel":      map[string]any{"force": true},
		"mcpServers": map[string]any{"gitlab": map[string]any{"command": "npx", "_keel": map[string]any{"force": false, "merge": "replace"}}},
	}
	cleaned, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	assert.NotContains(t, cleaned, "_keel")
	assert.NotContains(t, cleaned["mcpServers"].(map[string]any)["gitlab"].(map[string]any), "_keel")
	assert.Contains(t, cleaned, "mcpServers")
	rootPolicy := policies[""]
	require.NotNil(t, rootPolicy.Force)
	assert.True(t, *rootPolicy.Force)
	gitlabPolicy := policies["mcpServers.gitlab"]
	require.NotNil(t, gitlabPolicy.Force)
	assert.False(t, *gitlabPolicy.Force)
	assert.Equal(t, policy.MergeStrategyReplace, gitlabPolicy.Merge)
}

func TestStripKeelPolicies_ForceExplicitFalse(t *testing.T) {
	t.Parallel()
	data := map[string]any{"mcpServers": map[string]any{"sonar": map[string]any{"command": "npx", "_keel": map[string]any{"force": false}}}}
	cleaned, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	assert.NotContains(t, cleaned["mcpServers"].(map[string]any)["sonar"].(map[string]any), "_keel")
	kp := policies["mcpServers.sonar"]
	require.NotNil(t, kp.Force)
	assert.False(t, *kp.Force)
}

func TestStripKeelPolicies_KeelNotMap(t *testing.T) {
	t.Parallel()
	data := map[string]any{"mcpServers": map[string]any{"gitlab": map[string]any{"command": "npx", "_keel": "not-a-map"}}}
	cleaned, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	assert.NotContains(t, cleaned["mcpServers"].(map[string]any)["gitlab"].(map[string]any), "_keel")
	assert.Empty(t, policies)
}

func TestStripKeelPolicies_EmptyMap(t *testing.T) {
	t.Parallel()
	cleaned, policies, err := StripKeelPolicies(map[string]any{})
	require.NoError(t, err)
	assert.Empty(t, cleaned)
	assert.Empty(t, policies)
}

func TestStripKeelPolicies_OnlyMergeField(t *testing.T) {
	t.Parallel()
	data := map[string]any{"mcpServers": map[string]any{"gitlab": map[string]any{"command": "npx", "_keel": map[string]any{"merge": "deep-merge"}}}}
	_, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	kp := policies["mcpServers.gitlab"]
	assert.Nil(t, kp.Force)
	assert.Equal(t, policy.MergeStrategyDeepMerge, kp.Merge)
}

func TestStripKeelPolicies_PreservesNonMapValues(t *testing.T) {
	t.Parallel()
	data := map[string]any{"version": float64(1), "name": "test", "enabled": true, "items": []any{"a", "b"}}
	cleaned, policies, err := StripKeelPolicies(data)
	require.NoError(t, err)
	assert.Equal(t, float64(1), cleaned["version"])
	assert.Equal(t, "test", cleaned["name"])
	assert.Equal(t, true, cleaned["enabled"])
	assert.Len(t, cleaned["items"].([]any), 2)
	assert.Empty(t, policies)
}

func TestStripKeelPolicies_DepthLimit(t *testing.T) {
	t.Parallel()
	inner := map[string]any{"_keel": map[string]any{"policy": "force-lock"}, "value": "test"}
	for i := 0; i < 140; i++ {
		inner = map[string]any{"nested": inner}
	}
	data, err := json.Marshal(inner)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	_, _, err = StripKeelPolicies(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nesting depth")
}
