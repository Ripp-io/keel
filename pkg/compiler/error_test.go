package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// TestFold_NilLayerIR tests that Fold panics with a nil LayerIR (current behavior).
// This test documents the current behavior — callers must not pass nil.
func TestFold_NilLayerIR(t *testing.T) {
	s := newTestState()

	// Folding a nil IR currently panics (nil pointer dereference).
	// This test documents that behavior.
	assert.Panics(t, func() {
		_ = s.Fold(nil, "nil-layer")
	})
}

// TestApplyGovernance_EmptyContributions tests that ApplyGovernance handles
// empty contributions gracefully without panicking.
func TestApplyGovernance_EmptyContributions(t *testing.T) {
	s := newTestState()

	// No contributions accumulated — should be a no-op.
	s.ApplyGovernance()

	// State should remain empty (no categories, no settings).
	assert.Empty(t, s.categories)
	assert.Empty(t, s.settings)
	// The force registry is initialized but has no locks.
	assert.NotNil(t, s.force)
}

// TestFinalize_UndefinedVariable tests that variable substitution handles
// undefined variables gracefully (leaves them unchanged).
func TestFinalize_UndefinedVariable(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "base.md", Content: "Var: ${keel:undefined.var}, Agent: ${keel:keel.agent}"},
		},
		Settings: map[string]any{
			"description": "Using ${keel:missing.var}",
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Undefined variables should remain as-is (not substituted).
	assert.Contains(t, result.Content.Instructions, "${keel:undefined.var}")
	assert.Contains(t, result.Content.Instructions, "Agent: claude")
	assert.Equal(t, "Using ${keel:missing.var}", result.Content.Settings["description"])
}

// TestFinalize_NestedSnippets tests that nested snippets (snippets that include
// other snippets) are resolved only one level deep.
func TestFinalize_NestedSnippets(t *testing.T) {
	s := newTestState()

	// Create a simple chain of 2 nested snippets.
	ir := &LayerIR{
		Snippets: map[string][]byte{
			"snippet-a": []byte("Level A <!-- keel:include snippet-b -->"),
			"snippet-b": []byte("Level B"),
		},
		Instructions: []InstructionBlock{{Filename: "base.md", Content: "Start\n<!-- keel:include snippet-a -->\nEnd"}},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	err := s.Finalize(nil)
	require.NoError(t, err)

	result := s.Result()
	// ResolveSnippets is called only once, so only the first level is resolved.
	// snippet-a is resolved to "Level A <!-- keel:include snippet-b -->", but
	// the nested snippet-b marker is NOT resolved (single-pass resolution).
	assert.Contains(t, result.Content.Instructions, "Level A")
	assert.Contains(t, result.Content.Instructions, "snippet-b", "nested snippet markers should remain unresolved")
}

// TestFinalize_MissingSnippet tests that missing snippet references are handled gracefully.
func TestFinalize_MissingSnippet(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Snippets: map[string][]byte{
			"existing": []byte("Content here"),
		},
		Instructions: []InstructionBlock{
			{Filename: "base.md", Content: "<!-- keel:include missing-snippet -->"},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	err := s.Finalize(nil)

	// Should return an error for missing snippet.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing-snippet")
}

// TestFold_TwoLayersForceLockingSameFile tests the behavior when two layers
// both try to force-lock the same file.
func TestFold_TwoLayersForceLockingSameFile(t *testing.T) {
	s := newTestState()

	// Layer 1: force-locks deploy.md.
	ir1 := &LayerIR{
		FilePolicies: map[string]schema.FilePolicy{
			"commands/deploy.md": {Force: true},
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Layer 1 deploy")},
			},
		},
	}

	// Layer 2: also tries to force-lock deploy.md.
	ir2 := &LayerIR{
		FilePolicies: map[string]schema.FilePolicy{
			"commands/deploy.md": {Force: true},
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Layer 2 deploy")},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "layer1"))
	require.NoError(t, s.Fold(ir2, "layer2"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Layer 1's content wins (first force-lock wins for content).
	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Equal(t, "deploy.md", cmdFiles[0].RelPath)
	assert.Equal(t, "Layer 1 deploy", string(cmdFiles[0].Content))

	// Force lock registry records layer2 (last-writer-wins for the lock record itself,
	// even though layer2's content was rejected).
	locks := result.Governance.ForceLocked
	var deployLock *manifest.ForceLockRecord
	for i := range locks {
		if locks[i].Category == "command" && locks[i].Key == "deploy.md" {
			deployLock = &locks[i]
			break
		}
	}
	require.NotNil(t, deployLock, "deploy.md should be force-locked")
	assert.Equal(t, "layer2", deployLock.LockedBy)
}

// TestFold_ForceLockInFrontmatter tests that force-lock in frontmatter takes
// precedence and blocks subsequent layers.
func TestFold_ForceLockInFrontmatter(t *testing.T) {
	s := newTestState()

	// Layer 1: deploy.md with force: true in frontmatter.
	ir1 := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("---\nforce: true\n---\nLayer 1 content")},
			},
		},
	}

	// Layer 2: tries to override with a new file policy force.
	ir2 := &LayerIR{
		FilePolicies: map[string]schema.FilePolicy{
			"commands/deploy.md": {Force: true},
		},
		Categories: map[string][]CategoryFile{
			"commands": {
				{RelPath: "deploy.md", Content: []byte("Layer 2 content")},
			},
		},
	}

	require.NoError(t, s.Fold(ir1, "layer1"))
	require.NoError(t, s.Fold(ir2, "layer2"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	// Layer 1's content should win (force-lock from frontmatter).
	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)
	assert.Equal(t, "deploy.md", cmdFiles[0].RelPath)
	// After finalization, frontmatter is stripped.
	assert.Equal(t, "Layer 1 content", string(cmdFiles[0].Content))

	// The lock record shows layer2 (last-writer-wins for the lock registry),
	// even though layer1's content won.
	locks := result.Governance.ForceLocked
	var deployLock *manifest.ForceLockRecord
	for i := range locks {
		if locks[i].Category == "command" && locks[i].Key == "deploy.md" {
			deployLock = &locks[i]
			break
		}
	}
	require.NotNil(t, deployLock)
	assert.Equal(t, "layer2", deployLock.LockedBy)
}

// TestFold_EmptyLayerIR tests that an empty LayerIR (all fields zero) is handled gracefully.
func TestFold_EmptyLayerIR(t *testing.T) {
	s := newTestState()

	emptyIR := &LayerIR{}

	err := s.Fold(emptyIR, "empty-layer")
	require.NoError(t, err)

	// State should remain empty.
	assert.Empty(t, s.instructions)
	assert.Empty(t, s.settings)
	assert.Empty(t, s.categories)
}

// TestFinalize_EmptyState tests that Finalize on an empty state doesn't panic.
func TestFinalize_EmptyState(t *testing.T) {
	s := newTestState()

	// No Fold calls — state is completely empty.
	// ApplyGovernance is required before Finalize (pipeline phase ordering).
	s.ApplyGovernance()
	err := s.Finalize(nil)
	require.NoError(t, err)

	result := s.Result()
	assert.Empty(t, result.Content.Instructions)
	assert.Empty(t, result.Content.Settings)
	assert.Empty(t, result.Content.Categories)
}

// TestFold_VariableSubstitution_ProtectedVarsFromExternalSource tests that
// auto-injected variables cannot be overridden by external variables.
func TestFold_VariableSubstitution_ProtectedVarsFromExternalSource(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Instructions: []InstructionBlock{
			{Filename: "base.md", Content: "Agent: ${keel:keel.agent}"},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	p := testProfile()
	s.ApplyAgentGovernance(p)
	s.TransformForAgent(p)
	s.EmitAgentWarnings(p)

	// Try to override keel.agent via external vars.
	err := s.Finalize(map[string]string{
		"keel.agent": "malicious-agent",
	})
	require.NoError(t, err)
	result := s.Result()

	// keel.agent should remain "claude", not "malicious-agent".
	assert.Contains(t, result.Content.Instructions, "Agent: claude")
	assert.NotContains(t, result.Content.Instructions, "malicious-agent")
}

// TestFinalize_ConditionalWithMissingVariable tests that conditionals with
// undefined variables are handled gracefully.
func TestFinalize_ConditionalWithMissingVariable(t *testing.T) {
	s := newTestState()

	ir := &LayerIR{
		Categories: map[string][]CategoryFile{
			"commands": {
				{
					RelPath: "deploy.md",
					Content: []byte(`<!-- keel-if: missing.var == "value" -->
Content when true
<!-- keel-endif -->`),
				},
			},
		},
	}

	require.NoError(t, s.Fold(ir, "base"))
	s.ApplyGovernance()
	// Don't set the variable — it's missing.
	err := s.Finalize(nil)
	require.NoError(t, err)

	result := s.Result()
	cmdFiles := result.Content.Categories["commands"]
	require.Len(t, cmdFiles, 1)

	// Conditional should evaluate to false (missing variable), so content is removed.
	content := string(cmdFiles[0].Content)
	assert.NotContains(t, content, "Content when true")
}

// TestFold_MCPPolicy_Accumulation tests that MCP policy is accumulated correctly
// across multiple layers without error.
func TestFold_MCPPolicy_Accumulation(t *testing.T) {
	s := newTestState()

	ir1 := &LayerIR{
		MCPPolicy: &schema.MCPPolicy{
			Allowed: []string{"server1"},
		},
	}
	ir2 := &LayerIR{
		MCPPolicy: &schema.MCPPolicy{
			Denied:    []string{"server2"},
			EnableAll: new(bool),
		},
	}
	*ir2.MCPPolicy.EnableAll = true

	require.NoError(t, s.Fold(ir1, "layer1"))
	require.NoError(t, s.Fold(ir2, "layer2"))

	s.ApplyGovernance()
	require.NoError(t, s.Finalize(nil))
	result := s.Result()

	require.NotNil(t, result.Governance.MCPPolicy)
	assert.Contains(t, result.Governance.MCPPolicy.Allowed, "server1")
	assert.Contains(t, result.Governance.MCPPolicy.Denied, "server2")
	assert.NotNil(t, result.Governance.MCPPolicy.EnableAll)
	assert.True(t, *result.Governance.MCPPolicy.EnableAll)
}

// TestFinalize_ZeroContributions tests that ApplyGovernance and Finalize
// handle zero contributions gracefully without panicking.
func TestFinalize_ZeroContributions(t *testing.T) {
	s := newTestState()

	// Fold a minimal IR with no contributions.
	ir := &LayerIR{
		Name: "test-layer",
	}

	require.NoError(t, s.Fold(ir, "test"))

	// ApplyGovernance with no contributions should not panic.
	s.ApplyGovernance()

	// Finalize with no contributions should not panic.
	err := s.Finalize(nil)
	require.NoError(t, err)

	result := s.Result()
	assert.Equal(t, "test-layer", result.Metadata.Name)
	assert.Empty(t, result.Content.Instructions)
	assert.Empty(t, result.Content.Categories)
}
