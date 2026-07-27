package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFold_Hooks_InjectsHooksDir(t *testing.T) {
	s := newTestState()

	// Layer with hooks should inject keel.hooks.dir
	ir := &LayerIR{
		LayerPath: "/path/to/layer",
		HooksDir:  "/path/to/layer/hooks",
		Hooks: map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash(*)",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "${keel:keel.hooks.dir}/guard-bash.sh",
						},
					},
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))

	// Verify keel.hooks.dir was injected into the variable map
	assert.Equal(t, "/path/to/layer/hooks", s.variables[varKeelHooksDir])

	// Verify the variable is available for substitution
	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))

	result := s.Result()
	// Verify the variable is in the tracking output
	assert.Equal(t, "/path/to/layer/hooks", result.Tracking.Variables[varKeelHooksDir])
}

func TestFold_Hooks_LastLayerWins(t *testing.T) {
	s := newTestState()

	// First layer with hooks
	ir1 := &LayerIR{
		LayerPath: "/path/to/layer1",
		HooksDir:  "/path/to/layer1/hooks",
		Hooks: map[string]any{
			"PreToolUse": []any{},
		},
	}

	// Second layer with hooks (should override keel.hooks.dir)
	ir2 := &LayerIR{
		LayerPath: "/path/to/layer2",
		HooksDir:  "/path/to/layer2/different-hooks",
		Hooks: map[string]any{
			"PostToolUse": []any{},
		},
	}

	require.NoError(t, s.Fold(ir1, "layer1"))
	assert.Equal(t, "/path/to/layer1/hooks", s.variables[varKeelHooksDir])

	require.NoError(t, s.Fold(ir2, "layer2"))
	// Last layer wins
	assert.Equal(t, "/path/to/layer2/different-hooks", s.variables[varKeelHooksDir])
}

func TestFold_Hooks_NoHooksDirIfNoHooks(t *testing.T) {
	s := newTestState()

	// Layer without hooks should not inject keel.hooks.dir
	ir := &LayerIR{
		LayerPath: "/path/to/layer",
		Settings: map[string]any{
			"model": "sonnet",
		},
	}

	require.NoError(t, s.Fold(ir, "layer1"))

	// Verify keel.hooks.dir was not injected
	_, exists := s.variables[varKeelHooksDir]
	assert.False(t, exists, "keel.hooks.dir should not be set when no hooks are present")
}
