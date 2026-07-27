package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runValidateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newValidateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestValidate_ValidDSLLayer(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: Test Project\ninstructions:\n  - source: instructions.md\ncommands:\n  - source: commands/\nrepos:\n  myrepo:\n    url: https://example.com/repo.git\n    path: myrepo\n    branch: main\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Instructions"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "commands"), 0o755))

	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "keel.yml schema valid")
	assert.Contains(t, got, "instructions.md exists")
	assert.Contains(t, got, "commands/")
	assert.Contains(t, got, "1 repo(s) defined")
	assert.Contains(t, got, "0 errors")
	assert.Contains(t, got, "Project: Test Project")
}

func TestValidate_MissingReferencedFile(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: Broken\ninstructions:\n  - source: instructions.md\ncommands:\n  - source: commands/deploy.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# OK"), 0o644))

	got, err := runValidateCmd(t, dir)
	require.Error(t, err)
	assert.Contains(t, got, "commands/deploy.md referenced but not found")
	assert.Contains(t, got, "1 error(s)")
	assert.Contains(t, err.Error(), "validation error")
}

func TestValidate_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(":::invalid yaml ["), 0o644))
	got, err := runValidateCmd(t, dir)
	require.Error(t, err)
	assert.Contains(t, got, "keel.yml:")
}

func TestValidate_NativeClaudeLayer(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "native claude layer")
	assert.Contains(t, got, "Format:  claude")
	assert.Contains(t, got, "CLAUDE.md exists")
}

func TestValidate_EmptyDirectory(t *testing.T) {
	got, err := runValidateCmd(t, t.TempDir())
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "no keel.yml found and no native agent files detected")
	assert.Contains(t, got, "1 warning(s)")
}

func TestValidate_SchemaValidationError(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: Unsafe\nrepos:\n  hacked:\n    url: https://example.com/repo.git\n    path: ../../../etc/passwd\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	got, err := runValidateCmd(t, dir)
	require.Error(t, err)
	assert.Contains(t, got, "invalid path")
}

func TestValidate_NonexistentPath(t *testing.T) {
	got, err := runValidateCmd(t, "/nonexistent/path/to/layer")
	require.Error(t, err)
	assert.Contains(t, got, "path does not exist")
}

func TestValidate_PathIsFile(t *testing.T) {
	f, err := os.CreateTemp("", "keel-validate-test-*")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())
	got, execErr := runValidateCmd(t, f.Name())
	require.Error(t, execErr)
	assert.Contains(t, got, "not a directory")
}

func TestValidate_AgentFlagKeelDSL(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: KeelLayer\n"), 0o644))
	got, err := runValidateCmd(t, "--agent", "claude", dir)
	require.NoError(t, err)
	assert.Contains(t, got, "compatible with agent")
}

func TestValidate_AgentFlagMismatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))
	got, err := runValidateCmd(t, "--agent", "cursor", dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "targets a different agent")
}

func TestValidate_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: CWD\n"), 0o644))
	got, err := runValidateCmd(t)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "keel.yml schema valid")
}

func TestValidate_MultipleErrors(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: MultiError\ninstructions:\n  - source: missing1.md\n  - source: missing2.md\ncommands:\n  - source: missing3.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	got, err := runValidateCmd(t, dir)
	require.Error(t, err)
	assert.Contains(t, got, "missing1.md referenced but not found")
	assert.Contains(t, got, "missing2.md referenced but not found")
	assert.Contains(t, got, "missing3.md referenced but not found")
	assert.Contains(t, got, "3 error(s)")
}

func TestValidate_MissingSettingsFile(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: SettingsTest\nsettings:\n  claude: settings.claude.json\n  cursor: settings.cursor.json\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.claude.json"), []byte("{}"), 0o644))
	got, err := runValidateCmd(t, dir)
	require.Error(t, err)
	assert.Contains(t, got, "settings.claude.json exists")
	assert.Contains(t, got, "settings.cursor.json referenced but not found")
}

func TestValidate_NativeCursorLayer(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "native cursor layer")
	assert.Contains(t, got, "Format:  cursor")
}

func TestValidate_DSLLayerWithValidSkills(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: SkillTest\nskills:\n  - source: skills/\n"), 0o644))
	skillDir := filepath.Join(dir, "skills", "code-review")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: code-review\ndescription: Reviews code for quality.\n---\nReview instructions.\n"), 0o644))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "skills: Agent Skills format compliant")
}

func TestValidate_DSLLayerWithInvalidSkills(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: SkillTest\nskills:\n  - source: skills/\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "broken-skill"), 0o755))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "skill broken-skill")
	assert.Contains(t, got, "SKILL.md")
}

func TestValidate_SkipSkillsWithFlag(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: SkipTest\nskills:\n  - source: skills/\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "broken-skill"), 0o755))
	got, err := runValidateCmd(t, "--agentskills-check=false", dir)
	require.NoError(t, err, "output: %s", got)
	assert.NotContains(t, got, "skill broken-skill")
	assert.NotContains(t, got, "SKILL.md")
}

func TestValidate_SkipSkillsWithKeelYml(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: SkipTest\nskills:\n  - source: skills/\nvalidate:\n  agentskills: false\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "broken-skill"), 0o755))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.NotContains(t, got, "skill broken-skill")
}

func TestValidate_NativeLayerWithSkills(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))
	skillDir := filepath.Join(dir, "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\ndescription: A test skill.\n---\nBody.\n"), 0o644))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "skills: Agent Skills format compliant")
}

func TestValidate_StrictWithNoWarnings(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: CleanLayer\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Instructions"), 0o644))
	got, err := runValidateCmd(t, "--strict", dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "0 warnings")
	assert.Contains(t, got, "0 errors")
}

func TestValidate_StrictWithWarnings(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: WarningLayer\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Instructions"), 0o644))
	// Create empty directory to trigger a warning
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "commands"), 0o755))
	got, err := runValidateCmd(t, "--strict", dir)
	require.Error(t, err, "output: %s", got)
	assert.Contains(t, err.Error(), "warning(s) treated as errors")
	assert.Contains(t, got, "1 warning(s)")
}

func TestValidate_WithoutStrictWarningsAllowed(t *testing.T) {
	dir := t.TempDir()
	keelYAML := "schema: v1\nname: WarningLayer\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Instructions"), 0o644))
	// Create empty directory to trigger a warning
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "commands"), 0o755))
	got, err := runValidateCmd(t, dir)
	require.NoError(t, err, "output: %s", got)
	assert.Contains(t, got, "1 warning(s)")
	assert.Contains(t, got, "0 errors")
}
