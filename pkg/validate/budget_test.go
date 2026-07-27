package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Token estimation unit test
// ---------------------------------------------------------------------------

func TestEstimateTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		data     []byte
		expected int
	}{
		{"empty", []byte(""), 0},
		{"1 byte", []byte("a"), 1},
		{"4 bytes", []byte("abcd"), 1},
		{"5 bytes", []byte("abcde"), 2},
		{"8 bytes", []byte("abcdefgh"), 2},
		{"9 bytes", []byte("abcdefghi"), 3},
		{"1000 bytes", []byte(strings.Repeat("x", 1000)), 250},
		{"1001 bytes", []byte(strings.Repeat("x", 1001)), 251},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.data)
			assert.Equal(t, tt.expected, result, "estimateTokens(%d bytes) should be %d", len(tt.data), tt.expected)
		})
	}
}

// ---------------------------------------------------------------------------
// Skill body token checks
// ---------------------------------------------------------------------------

func TestTokenBudget_SkillBodyOverThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SkillBudget\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "large-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Create a skill with body >5000 tokens (~20001+ bytes).
	largeBody := strings.Repeat("x", 20001)
	skillMD := "---\nname: large-skill\ndescription: A large skill.\n---\n" + largeBody
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: skill", "large-skill", "body", "tokens", "threshold"), "expected warning for large skill body")
}

func TestTokenBudget_SkillBodyUnderThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: SmallSkill\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "small-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Small body (~100 bytes = 25 tokens).
	smallBody := strings.Repeat("x", 100)
	skillMD := "---\nname: small-skill\ndescription: A small skill.\n---\n" + smallBody
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, true, false, "token budget: skill bodies within threshold"), "expected pass for small skill body")
}

// ---------------------------------------------------------------------------
// Skill description length checks
// ---------------------------------------------------------------------------

func TestTokenBudget_SkillDescriptionOverLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: LongDesc\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "long-desc")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Description >1024 chars.
	longDesc := strings.Repeat("x", 1025)
	skillMD := "---\nname: long-desc\ndescription: " + longDesc + "\n---\nBody content."
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: skill", "long-desc", "description", "1025 chars", "discovery tier limit", "1024"), "expected warning for long description")
}

func TestTokenBudget_SkillDescriptionUnderLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: ShortDesc\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "short-desc")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Description well under 1024 chars.
	skillMD := "---\nname: short-desc\ndescription: A brief description.\n---\nBody content."
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	// No description warning expected.
	for _, item := range result.Items {
		if item.Warn && strings.Contains(item.Message, "description") {
			assert.NotContains(t, item.Message, "discovery tier limit", "expected no description limit warning for short description")
		}
	}
}

// ---------------------------------------------------------------------------
// Instructions total token checks
// ---------------------------------------------------------------------------

func TestTokenBudget_InstructionsTotal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Set threshold to 1000 tokens (~4000 bytes).
	keelYAML := "schema: v1\nname: InstrTotal\ninstructions:\n  - source: instructions.md\nvalidate:\n  token-budget:\n    instructions: 1000\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Create instructions >1000 tokens (~4001+ bytes).
	largeInstructions := strings.Repeat("x", 4001)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(largeInstructions), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: instructions total", "tokens", "threshold: 1000"), "expected warning for instructions exceeding threshold")
}

func TestTokenBudget_InstructionsTotalNoThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No threshold set (default: 0 = disabled).
	keelYAML := "schema: v1\nname: InstrNoThresh\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	instructions := strings.Repeat("x", 5000)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(instructions), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	// Should report estimate as a pass, not a warning.
	assert.True(t, hasItem(result, true, false, "token budget: instructions total", "tokens"), "expected pass reporting instructions total")

	// Should NOT contain a threshold warning.
	for _, item := range result.Items {
		if strings.Contains(item.Message, "instructions total") {
			assert.NotContains(t, item.Message, "threshold:", "expected no threshold mention when threshold is disabled")
		}
	}
}

func TestTokenBudget_InstructionsTotalUnderThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Set threshold to 5000 tokens (~20000 bytes).
	keelYAML := "schema: v1\nname: InstrUnder\ninstructions:\n  - source: instructions.md\nvalidate:\n  token-budget:\n    instructions: 5000\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Small instructions (~1000 bytes = 250 tokens).
	instructions := strings.Repeat("x", 1000)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(instructions), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, true, false, "token budget: instructions total", "within threshold"), "expected pass for instructions under threshold")
}

// ---------------------------------------------------------------------------
// Layer content total token checks
// ---------------------------------------------------------------------------

func TestTokenBudget_LayerTotal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Set threshold to 2000 tokens (~8000 bytes).
	keelYAML := "schema: v1\nname: LayerTotal\ninstructions:\n  - source: instructions.md\ncommands:\n  - source: commands.md\nvalidate:\n  token-budget:\n    layer: 2000\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Create content totaling >2000 tokens.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(strings.Repeat("x", 5000)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "commands.md"), []byte(strings.Repeat("x", 4000)), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: layer content total", "tokens", "threshold: 2000"), "expected warning for layer content exceeding threshold")
}

func TestTokenBudget_LayerTotalNoThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No layer threshold set (default: 0 = disabled).
	keelYAML := "schema: v1\nname: LayerNoThresh\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(strings.Repeat("x", 10000)), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	// Should report estimate as a pass.
	assert.True(t, hasItem(result, true, false, "token budget: layer content total", "tokens"), "expected pass reporting layer total")

	// Should NOT warn about threshold.
	for _, item := range result.Items {
		if strings.Contains(item.Message, "layer content total") {
			assert.NotContains(t, item.Message, "threshold:", "expected no threshold mention when disabled")
		}
	}
}

// ---------------------------------------------------------------------------
// Disabled token budget check
// ---------------------------------------------------------------------------

func TestTokenBudget_Disabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: BudgetOff\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := "---\nname: test-skill\ndescription: Test.\n---\n" + strings.Repeat("x", 20001)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: false})

	// No token budget messages expected.
	for _, item := range result.Items {
		assert.NotContains(t, item.Message, "token budget:", "expected no token budget messages when TokenBudget=false")
	}
}

// ---------------------------------------------------------------------------
// Custom thresholds via keel.yml
// ---------------------------------------------------------------------------

func TestTokenBudget_CustomThresholds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Custom skill-body threshold: 100 tokens (~400 bytes).
	keelYAML := "schema: v1\nname: CustomThresh\nskills:\n  - source: skills/\nvalidate:\n  token-budget:\n    skill-body: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	skillDir := filepath.Join(dir, "skills", "custom-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Body ~500 bytes = 125 tokens (exceeds custom threshold of 100).
	skillMD := "---\nname: custom-skill\ndescription: Custom threshold test.\n---\n" + strings.Repeat("x", 500)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: skill", "custom-skill", "body", "tokens", "threshold: 100"), "expected warning with custom threshold of 100")
}

// ---------------------------------------------------------------------------
// Native layer skill token checks
// ---------------------------------------------------------------------------

func TestTokenBudget_NativeLayer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a native Claude layer (no keel.yml).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644))

	skillDir := filepath.Join(dir, "skills", "native-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Large skill body.
	skillMD := "---\nname: native-skill\ndescription: Native layer skill.\n---\n" + strings.Repeat("x", 20001)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: skill", "native-skill", "body", "tokens"), "expected warning for native layer skill body")
}

// ---------------------------------------------------------------------------
// Multiple skills
// ---------------------------------------------------------------------------

func TestTokenBudget_MultipleSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: MultiSkill\nskills:\n  - source: skills/\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Create two skills: one over threshold, one under.
	skill1Dir := filepath.Join(dir, "skills", "skill-one")
	require.NoError(t, os.MkdirAll(skill1Dir, 0o755))
	skill1MD := "---\nname: skill-one\ndescription: First skill.\n---\n" + strings.Repeat("x", 20001)
	require.NoError(t, os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1MD), 0o644))

	skill2Dir := filepath.Join(dir, "skills", "skill-two")
	require.NoError(t, os.MkdirAll(skill2Dir, 0o755))
	skill2MD := "---\nname: skill-two\ndescription: Second skill.\n---\n" + strings.Repeat("x", 100)
	require.NoError(t, os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(skill2MD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	// Should warn about skill-one only.
	assert.True(t, hasItem(result, false, true, "token budget: skill", "skill-one", "body", "tokens"), "expected warning for skill-one")

	// Should NOT have a pass for "within threshold" since one skill exceeded.
	for _, item := range result.Items {
		if item.OK && strings.Contains(item.Message, "skill bodies within threshold") {
			t.Error("expected no 'within threshold' pass when at least one skill exceeds threshold")
		}
	}
}

// ---------------------------------------------------------------------------
// No skills present
// ---------------------------------------------------------------------------

func TestTokenBudget_NoSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: NoSkills\ninstructions:\n  - source: instructions.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Instructions"), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	// No skill-related token budget messages expected.
	for _, item := range result.Items {
		if strings.Contains(item.Message, "token budget:") {
			assert.NotContains(t, item.Message, "skill", "expected no skill messages when no skills are present")
		}
	}
}

// ---------------------------------------------------------------------------
// Instructions with multiple files
// ---------------------------------------------------------------------------

func TestTokenBudget_InstructionsMultipleFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: MultiInstr\ninstructions:\n  - source: instructions1.md\n  - source: instructions2.md\nvalidate:\n  token-budget:\n    instructions: 500\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Two files totaling >500 tokens.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions1.md"), []byte(strings.Repeat("x", 1200)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions2.md"), []byte(strings.Repeat("x", 1000)), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: instructions total", "tokens", "threshold: 500"), "expected warning for combined instructions exceeding threshold")
}

// ---------------------------------------------------------------------------
// Layer content with all categories
// ---------------------------------------------------------------------------

func TestTokenBudget_LayerContentAllCategories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := `schema: v1
name: FullLayer
instructions:
  - source: instructions.md
commands:
  - source: commands.md
agents:
  - source: agents.md
skills:
  - source: skills/
rules:
  - source: rules.md
validate:
  token-budget:
    layer: 500
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	// Create content in each category (totaling ~500 bytes per file = 2500 bytes = 625 tokens).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(strings.Repeat("x", 500)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "commands.md"), []byte(strings.Repeat("x", 500)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agents.md"), []byte(strings.Repeat("x", 500)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules.md"), []byte(strings.Repeat("x", 500)), 0o644))

	skillDir := filepath.Join(dir, "skills", "test-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	skillMD := "---\nname: test-skill\ndescription: Test.\n---\n" + strings.Repeat("x", 500)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: layer content total", "tokens", "threshold: 500"), "expected warning for layer content exceeding threshold")
}

// ---------------------------------------------------------------------------
// Token budget with directory refs
// ---------------------------------------------------------------------------

func TestTokenBudget_InstructionsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	keelYAML := "schema: v1\nname: InstrDir\ninstructions:\n  - source: instructions/\nvalidate:\n  token-budget:\n    instructions: 300\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYAML), 0o644))

	instrDir := filepath.Join(dir, "instructions")
	require.NoError(t, os.MkdirAll(instrDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "intro.md"), []byte(strings.Repeat("x", 800)), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(instrDir, "details.md"), []byte(strings.Repeat("x", 500)), 0o644))

	result := Run(dir, Options{TokenBudget: true})

	assert.True(t, hasItem(result, false, true, "token budget: instructions total", "tokens", "threshold: 300"), "expected warning for instructions directory exceeding threshold")
}
