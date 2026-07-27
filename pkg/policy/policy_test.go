package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		// Valid strategies.
		{"replace", true},
		{"additive", true},
		{"deep-merge", true},

		// Invalid strategies.
		{"", false},
		{"merge", false},
		{"REPLACE", false},
		{"shallow-merge", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsValidStrategy(tt.input), "IsValidStrategy(%q)", tt.input)
		})
	}
}
