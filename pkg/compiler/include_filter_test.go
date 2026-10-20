package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// --- Include filter tests ---

func TestReadCategoryDir_IncludeFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create directory structure with multiple subdirs
	skillsDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "skill-a"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "skill-b"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "skill-c"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill-a", "skill-a.md"), []byte("skill a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill-b", "skill-b.md"), []byte("skill b"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill-c", "skill-c.md"), []byte("skill c"), 0644))

	ir := &LayerIR{}
	include := []string{"skill-a", "skill-c"}

	err := ReadCategoryDir(skillsDir, "skills", ir, include)
	require.NoError(t, err)

	files := ir.Categories["skills"]
	require.NotNil(t, files)

	// Should have skill-a and skill-c directories and their files, but NOT skill-b
	var fileNames []string
	for _, f := range files {
		fileNames = append(fileNames, f.RelPath)
	}

	assert.Contains(t, fileNames, "skill-a")
	assert.Contains(t, fileNames, "skill-a/skill-a.md")
	assert.Contains(t, fileNames, "skill-c")
	assert.Contains(t, fileNames, "skill-c/skill-c.md")
	assert.NotContains(t, fileNames, "skill-b")
	assert.NotContains(t, fileNames, "skill-b/skill-b.md")
}

func TestReadCategoryDir_MultipleCallsAppend(t *testing.T) {
	t.Parallel()

	// Simulate three separate source directories (local + two remotes)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()

	// Source 1: local skills
	require.NoError(t, os.MkdirAll(filepath.Join(dir1, "my-skill"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "my-skill", "SKILL.md"), []byte("local"), 0644))

	// Source 2: remote productivity skills
	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "grilling"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "grilling", "grilling.md"), []byte("grill"), 0644))

	// Source 3: remote engineering skills
	require.NoError(t, os.MkdirAll(filepath.Join(dir3, "research"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir3, "research", "research.md"), []byte("research"), 0644))

	ir := &LayerIR{}

	// Call ReadCategoryDir three times for the same category (simulates multi-source)
	require.NoError(t, ReadCategoryDir(dir1, "skills", ir, nil))
	require.NoError(t, ReadCategoryDir(dir2, "skills", ir, nil))
	require.NoError(t, ReadCategoryDir(dir3, "skills", ir, nil))

	files := ir.Categories["skills"]
	require.NotNil(t, files)

	var names []string
	for _, f := range files {
		names = append(names, f.RelPath)
	}

	// All three sources should be present
	assert.Contains(t, names, "my-skill")
	assert.Contains(t, names, "my-skill/SKILL.md")
	assert.Contains(t, names, "grilling")
	assert.Contains(t, names, "grilling/grilling.md")
	assert.Contains(t, names, "research")
	assert.Contains(t, names, "research/research.md")
}

func TestReadCategoryDir_EmptyInclude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	skillsDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "skill-a"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "skill-b"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "skill-c"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill-a", "skill-a.md"), []byte("skill a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill-b", "skill-b.md"), []byte("skill b"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill-c", "skill-c.md"), []byte("skill c"), 0644))

	ir := &LayerIR{}

	// Test with nil include
	err := ReadCategoryDir(skillsDir, "skills", ir, nil)
	require.NoError(t, err)

	files := ir.Categories["skills"]
	require.NotNil(t, files)

	// Should have all three skills
	var fileNames []string
	for _, f := range files {
		fileNames = append(fileNames, f.RelPath)
	}

	assert.Contains(t, fileNames, "skill-a")
	assert.Contains(t, fileNames, "skill-a/skill-a.md")
	assert.Contains(t, fileNames, "skill-b")
	assert.Contains(t, fileNames, "skill-b/skill-b.md")
	assert.Contains(t, fileNames, "skill-c")
	assert.Contains(t, fileNames, "skill-c/skill-c.md")
}

func TestReadCategoryDir_IncludeOnNonDirIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a single file (not a directory) and reference it with include
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"),
		[]byte("schema: v1\nname: Test\nskills:\n  - source: skill.md\n    include: [grilling]\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skill.md"), []byte("skill content"), 0644))

	reader := &KeelDSLReader{}
	ir, err := reader.Read(dir)
	require.NoError(t, err)
	assert.NotNil(t, ir)
}

func TestFileRef_IncludeValidation(t *testing.T) {
	t.Parallel()

	// Valid include entries
	validYAML := `
schema: v1
name: Test
skills:
  - source: skills/
    include: [grilling, research]
`
	cfg, err := schema.ParseBytes([]byte(validYAML))
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Invalid include entry with path separator
	invalidYAML := `
schema: v1
name: Test
skills:
  - source: skills/
    include: [grilling/advanced, research]
`
	_, err = schema.ParseBytes([]byte(invalidYAML))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")

	// Invalid include entry with ..
	invalidYAML2 := `
schema: v1
name: Test
skills:
  - source: skills/
    include: [.., research]
`
	_, err = schema.ParseBytes([]byte(invalidYAML2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "..")

	// Invalid include entry with backslash
	invalidYAML3 := `
schema: v1
name: Test
skills:
  - source: skills/
    include: ["grilling\\advanced", research]
`
	_, err = schema.ParseBytes([]byte(invalidYAML3))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separators")
}

func TestCompile_IncludeFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create layer with skills directory and subdirs
	layerDir := filepath.Join(dir, "layer")
	skillsDir := filepath.Join(layerDir, "skills")
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "grilling"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "research"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "ignored"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "grilling", "grilling.md"), []byte("# Grilling"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "research", "research.md"), []byte("# Research"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "ignored", "ignored.md"), []byte("# Ignored"), 0644))

	keelYml := `schema: v1
name: Test Layer
skills:
  - source: skills/
    include: [grilling, research]
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644))

	layers := []config.Layer{
		{Path: layerDir, Type: "project", Format: config.FormatKeel},
	}

	outputDir := t.TempDir()
	_, err := Compile("claude", layers, outputDir, CompileOptions{})
	require.NoError(t, err)

	// Check that grilling and research appear in output
	grillingPath := filepath.Join(outputDir, ".claude", "skills", "grilling", "grilling.md")
	researchPath := filepath.Join(outputDir, ".claude", "skills", "research", "research.md")
	ignoredPath := filepath.Join(outputDir, ".claude", "skills", "ignored", "ignored.md")

	_, err = os.Stat(grillingPath)
	assert.NoError(t, err, "grilling.md should exist")

	_, err = os.Stat(researchPath)
	assert.NoError(t, err, "research.md should exist")

	_, err = os.Stat(ignoredPath)
	assert.True(t, os.IsNotExist(err), "ignored.md should NOT exist")
}
