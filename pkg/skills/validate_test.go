package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDir(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		dirName   string // name of the subdirectory
		skillMD   string // content of SKILL.md (empty string = don't create file)
		lowercase bool   // if true, create as skill.md instead of SKILL.md
		wantN     int    // expected number of warnings
		wantField string // if set, at least one warning must have this Field value
	}{
		// Happy path
		{
			name:    "valid minimal",
			dirName: "code-review",
			skillMD: "---\nname: code-review\ndescription: Reviews code.\n---\nInstructions here.",
			wantN:   0,
		},
		{
			name:    "valid all fields",
			dirName: "pdf-processing",
			skillMD: "---\nname: pdf-processing\ndescription: Process PDFs.\nlicense: MIT\ncompatibility: Requires Python 3.11\nallowed-tools: Bash(git:*)\nmetadata:\n  author: test\n---\nBody.",
			wantN:   0,
		},

		// Missing SKILL.md
		{
			name:      "no skill file",
			dirName:   "empty-skill",
			skillMD:   "",
			wantN:     1,
			wantField: "frontmatter",
		},

		// Name checks
		{
			name:      "missing name",
			dirName:   "test-skill",
			skillMD:   "---\ndescription: Test.\n---\n",
			wantN:     1,
			wantField: "name",
		},
		{
			name:      "name too long",
			dirName:   "a-very-long-name",
			skillMD:   "---\nname: " + strings.Repeat("a", 65) + "\ndescription: Test.\n---\n",
			wantN:     2, // too long + name != dir
			wantField: "name",
		},
		{
			name:      "name uppercase",
			dirName:   "my-skill",
			skillMD:   "---\nname: My-Skill\ndescription: Test.\n---\n",
			wantN:     2, // format + name != dir
			wantField: "name",
		},
		{
			name:      "name leading hyphen",
			dirName:   "bad",
			skillMD:   "---\nname: -bad\ndescription: Test.\n---\n",
			wantN:     2, // format + name != dir
			wantField: "name",
		},
		{
			name:      "name trailing hyphen",
			dirName:   "bad",
			skillMD:   "---\nname: bad-\ndescription: Test.\n---\n",
			wantN:     2, // format + name != dir
			wantField: "name",
		},
		{
			name:      "name consecutive hyphens",
			dirName:   "bad-name",
			skillMD:   "---\nname: bad--name\ndescription: Test.\n---\n",
			wantN:     2, // format + name != dir
			wantField: "name",
		},
		{
			name:      "name mismatch dir",
			dirName:   "actual-dir",
			skillMD:   "---\nname: different-name\ndescription: Test.\n---\n",
			wantN:     1,
			wantField: "name",
		},

		// Description checks
		{
			name:      "missing description",
			dirName:   "test-skill",
			skillMD:   "---\nname: test-skill\n---\n",
			wantN:     1,
			wantField: "description",
		},
		{
			name:      "description too long",
			dirName:   "test-skill",
			skillMD:   "---\nname: test-skill\ndescription: " + strings.Repeat("x", 1025) + "\n---\n",
			wantN:     1,
			wantField: "description",
		},

		// Compatibility
		{
			name:      "compatibility too long",
			dirName:   "test-skill",
			skillMD:   "---\nname: test-skill\ndescription: Test.\ncompatibility: " + strings.Repeat("y", 501) + "\n---\n",
			wantN:     1,
			wantField: "compatibility",
		},

		// Unexpected fields
		{
			name:      "unexpected field",
			dirName:   "test-skill",
			skillMD:   "---\nname: test-skill\ndescription: Test.\nfoo: bar\n---\n",
			wantN:     1,
			wantField: "frontmatter",
		},

		// Keel fields tolerated
		{
			name:    "keel force tolerated",
			dirName: "test-skill",
			skillMD: "---\nforce: true\nname: test-skill\ndescription: Test.\n---\n",
			wantN:   0,
		},
		{
			name:    "keel scope tolerated",
			dirName: "test-skill",
			skillMD: "---\nscope: project\nname: test-skill\ndescription: Test.\n---\n",
			wantN:   0,
		},
		{
			name:    "keel merge tolerated",
			dirName: "test-skill",
			skillMD: "---\nmerge: replace\nname: test-skill\ndescription: Test.\n---\n",
			wantN:   0,
		},

		// Metadata type
		{
			name:      "metadata wrong type",
			dirName:   "test-skill",
			skillMD:   "---\nname: test-skill\ndescription: Test.\nmetadata: not-a-map\n---\n",
			wantN:     1,
			wantField: "metadata",
		},

		// Multiple violations
		{
			name:    "multiple violations missing name and description",
			dirName: "test-skill",
			skillMD: "---\nfoo: bar\n---\n",
			wantN:   3, // missing name + missing description + unexpected field
		},

		// lowercase skill.md
		{
			name:      "lowercase skillmd",
			dirName:   "test-skill",
			skillMD:   "---\nname: test-skill\ndescription: Valid skill.\n---\nBody.",
			lowercase: true,
			wantN:     0,
		},

		// No frontmatter
		{
			name:      "no frontmatter",
			dirName:   "test-skill",
			skillMD:   "Just plain markdown.",
			wantN:     1,
			wantField: "frontmatter",
		},

		// Empty frontmatter (parser returns nil map for empty YAML block)
		{
			name:      "empty frontmatter",
			dirName:   "test-skill",
			skillMD:   "---\n---\nBody.",
			wantN:     1,
			wantField: "frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, tt.dirName)
			require.NoError(t, os.MkdirAll(dir, 0o755))

			if tt.skillMD != "" {
				filename := "SKILL.md"
				if tt.lowercase {
					filename = "skill.md"
				}
				require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(tt.skillMD), 0o644))
			}

			warnings := ValidateDir(dir)
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
				if !found {
					for i, w := range warnings {
						t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
					}
				}
			}
		})
	}
}

func TestValidateDir_SkillDirInWarning(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "my-skill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	// No SKILL.md
	warnings := ValidateDir(dir)
	require.Len(t, warnings, 1)
	assert.Equal(t, "my-skill", warnings[0].SkillDir)
}

func TestValidateAll(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	// Valid skill directory
	validDir := filepath.Join(base, "valid-skill")
	require.NoError(t, os.MkdirAll(validDir, 0o755))
	validContent := "---\nname: valid-skill\ndescription: A valid skill.\n---\nBody."
	require.NoError(t, os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte(validContent), 0o644))

	// Invalid skill directory (missing SKILL.md)
	invalidDir := filepath.Join(base, "invalid-skill")
	require.NoError(t, os.MkdirAll(invalidDir, 0o755))

	// Create a regular file (should be skipped, not a directory)
	require.NoError(t, os.WriteFile(filepath.Join(base, "not-a-dir.txt"), []byte("ignore"), 0o644))

	warnings := ValidateAll(base)

	// Should have warnings only from invalid-skill
	require.Len(t, warnings, 1)
	for i, w := range warnings {
		t.Logf("  warning[%d]: dir=%q field=%q msg=%q", i, w.SkillDir, w.Field, w.Message)
	}
	assert.Equal(t, "invalid-skill", warnings[0].SkillDir)
}

func TestValidateAll_EmptyDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	warnings := ValidateAll(base)
	assert.Empty(t, warnings)
}

func TestValidateAll_NonexistentDir(t *testing.T) {
	t.Parallel()
	warnings := ValidateAll("/nonexistent/path")
	assert.Empty(t, warnings)
}

func TestValidateDir_NameWithUnicodeLowercase(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "revisión")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := "---\nname: revisión\ndescription: Unicode skill.\n---\nBody."
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))

	warnings := ValidateDir(dir)
	assert.Empty(t, warnings, "expected no warnings for valid unicode name")
	for i, w := range warnings {
		t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
	}
}

func TestValidateDir_MultipleUnexpectedFields(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "test-skill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := "---\nname: test-skill\ndescription: Test.\nfoo: bar\nbaz: qux\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))

	warnings := ValidateDir(dir)
	require.Len(t, warnings, 1, "expected single warning listing all unexpected fields")
	assert.Equal(t, "frontmatter", warnings[0].Field)
	assert.Contains(t, warnings[0].Message, "foo")
	assert.Contains(t, warnings[0].Message, "baz")
}

func TestValidateDir_NameExactly64Chars(t *testing.T) {
	t.Parallel()
	name := strings.Repeat("a", 64)
	base := t.TempDir()
	dir := filepath.Join(base, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := fmt.Sprintf("---\nname: %s\ndescription: Boundary test.\n---\nBody.\n", name)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))

	warnings := ValidateDir(dir)
	assert.Empty(t, warnings, "expected no warnings for exactly 64-char name")
	for i, w := range warnings {
		t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
	}
}

func TestValidateDir_DescriptionExactly1024Chars(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "test-skill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	desc := strings.Repeat("x", 1024)
	content := fmt.Sprintf("---\nname: test-skill\ndescription: %s\n---\nBody.\n", desc)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))

	warnings := ValidateDir(dir)
	assert.Empty(t, warnings, "expected no warnings for exactly 1024-char description")
	for i, w := range warnings {
		t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
	}
}

func TestValidateDir_CompatibilityExactly500Chars(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "test-skill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	compat := strings.Repeat("y", 500)
	content := fmt.Sprintf("---\nname: test-skill\ndescription: Test.\ncompatibility: %s\n---\nBody.\n", compat)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))

	warnings := ValidateDir(dir)
	assert.Empty(t, warnings, "expected no warnings for exactly 500-char compatibility")
	for i, w := range warnings {
		t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
	}
}

func TestValidateDir_NameIsInteger(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "test-skill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	// YAML parses `name: 123` as an integer, not a string
	content := "---\nname: 123\ndescription: Test.\n---\nBody.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))

	warnings := ValidateDir(dir)
	require.NotEmpty(t, warnings, "expected warnings for integer name")
	found := false
	for _, w := range warnings {
		if w.Field == "name" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a warning with Field=\"name\" for integer value")
	if !found {
		for i, w := range warnings {
			t.Logf("  warning[%d]: field=%q message=%q", i, w.Field, w.Message)
		}
	}
}
