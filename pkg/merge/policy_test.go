package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
)

func TestIsValidStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"deep-merge", true},
		{"replace", true},
		{"additive", true},
		{"", false},
		{"merge", false},
		{"REPLACE", false},
		{"Deep-Merge", false},
		{"unknown", false},
		{" deep-merge", false},
		{"deep-merge ", false},
		{"yolo", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.IsValidStrategy(tt.input))
		})
	}
}

func TestMergeStrategyConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, policy.MergeStrategy("deep-merge"), policy.MergeStrategyDeepMerge)
	assert.Equal(t, policy.MergeStrategy("replace"), policy.MergeStrategyReplace)
	assert.Equal(t, policy.MergeStrategy("additive"), policy.MergeStrategyAdditive)
}
