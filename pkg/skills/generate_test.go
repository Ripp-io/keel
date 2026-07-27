package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSKILLMD_ValidSpec(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "code-review",
		Description: "Reviews code for quality.",
		License:     "MIT",
		Body:        "## Instructions\n\nReview code carefully.",
	}

	data, err := GenerateSKILLMD(spec)
	require.NoError(t, err)

	content := string(data)

	// Must start with frontmatter delimiter
	assert.True(t, strings.HasPrefix(content, "---\n"), "output must start with ---")

	// Must contain required fields
	assert.Contains(t, content, "name: code-review")
	assert.Contains(t, content, "description: Reviews code for quality.")
	assert.Contains(t, content, "license: MIT")

	// Must contain body after frontmatter
	assert.Contains(t, content, "## Instructions")

	// Must have closing frontmatter delimiter before body
	parts := strings.SplitN(content, "---\n", 3)
	require.Len(t, parts, 3, "expected at least two --- delimiters in output")
}

func TestGenerateSKILLMD_InvalidSpec(t *testing.T) {
	t.Parallel()
	spec := Spec{
		// Missing name
		Description: "A skill.",
	}

	_, err := GenerateSKILLMD(spec)
	require.Error(t, err, "expected error for invalid spec (missing name)")
}

func TestGenerateSKILLMD_OptionalFieldsOmitted(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "minimal-skill",
		Description: "A minimal skill.",
		Body:        "Body content.",
	}

	data, err := GenerateSKILLMD(spec)
	require.NoError(t, err)

	content := string(data)

	// Optional fields should not appear when empty
	assert.NotContains(t, content, "license:")
	assert.NotContains(t, content, "compatibility:")
	assert.NotContains(t, content, "allowed-tools:")
	assert.NotContains(t, content, "metadata:")
}

func TestGenerateSKILLMD_RoundTrip(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:          "round-trip",
		Description:   "A round-trip test skill.",
		License:       "Apache-2.0",
		Compatibility: "Requires Go 1.21+",
		AllowedTools:  "Bash(git:*)",
		Metadata:      map[string]string{"author": "test", "version": "1.0"},
		Body:          "## Usage\n\nRun the skill.",
	}

	data, err := GenerateSKILLMD(spec)
	require.NoError(t, err)

	// Write to temp dir and validate
	base := t.TempDir()
	dir := filepath.Join(base, "round-trip")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), data, 0o644))

	warnings := ValidateDir(dir)
	assert.Empty(t, warnings, "round-trip produced warnings")
	for i, w := range warnings {
		t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
	}
}

func TestGenerateSKILLMD_WithMetadata(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:        "meta-skill",
		Description: "Has metadata.",
		Metadata:    map[string]string{"author": "alice"},
		Body:        "Content.",
	}

	data, err := GenerateSKILLMD(spec)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "metadata:")
	assert.Contains(t, content, "author: alice")
}

func TestValidateSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		spec      Spec
		dirName   string
		wantN     int
		wantField string
	}{
		{
			name: "valid spec",
			spec: Spec{
				Name:        "test-skill",
				Description: "A test.",
			},
			dirName: "test-skill",
			wantN:   0,
		},
		{
			name: "missing name",
			spec: Spec{
				Description: "A test.",
			},
			dirName:   "test-skill",
			wantN:     1,
			wantField: "name",
		},
		{
			name: "missing description",
			spec: Spec{
				Name: "test-skill",
			},
			dirName:   "test-skill",
			wantN:     1,
			wantField: "description",
		},
		{
			name: "name mismatch",
			spec: Spec{
				Name:        "other-name",
				Description: "A test.",
			},
			dirName:   "test-skill",
			wantN:     1,
			wantField: "name",
		},
		{
			name: "name too long",
			spec: Spec{
				Name:        strings.Repeat("a", 65),
				Description: "A test.",
			},
			dirName:   strings.Repeat("a", 65),
			wantN:     1,
			wantField: "name",
		},
		{
			name: "invalid name format",
			spec: Spec{
				Name:        "Bad-Name",
				Description: "A test.",
			},
			dirName:   "Bad-Name",
			wantN:     1,
			wantField: "name",
		},
		{
			name: "description too long",
			spec: Spec{
				Name:        "test-skill",
				Description: strings.Repeat("x", 1025),
			},
			dirName:   "test-skill",
			wantN:     1,
			wantField: "description",
		},
		{
			name: "compatibility too long",
			spec: Spec{
				Name:          "test-skill",
				Description:   "A test.",
				Compatibility: strings.Repeat("y", 501),
			},
			dirName:   "test-skill",
			wantN:     1,
			wantField: "compatibility",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateSpec(tt.spec, tt.dirName)
			assert.Len(t, warnings, tt.wantN)
			for i, w := range warnings {
				t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
			}
			if tt.wantField != "" && len(warnings) > 0 {
				found := false
				for _, w := range warnings {
					if w.Field == tt.wantField {
						found = true
						break
					}
				}
				assert.True(t, found, "no warning with Field=%q found", tt.wantField)
			}
		})
	}
}

func TestGenerateSKILLMD_FieldOrderConsistent(t *testing.T) {
	t.Parallel()
	spec := Spec{
		Name:          "order-test",
		Description:   "Testing field order.",
		License:       "MIT",
		Compatibility: "Go 1.21+",
		AllowedTools:  "Bash(git:*)",
		Body:          "Body.",
	}

	data, err := GenerateSKILLMD(spec)
	require.NoError(t, err)

	content := string(data)

	// name must come before description
	nameIdx := strings.Index(content, "name:")
	descIdx := strings.Index(content, "description:")
	assert.Less(t, nameIdx, descIdx, "name should appear before description in frontmatter")

	// description must come before optional fields
	licenseIdx := strings.Index(content, "license:")
	assert.Less(t, descIdx, licenseIdx, "description should appear before license in frontmatter")
}
