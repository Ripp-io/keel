package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAdd_CreatesConfigWhenMissing(t *testing.T) {
	dir := t.TempDir()

	// Create a .claude directory so output detection works
	err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	require.NoError(t, err)

	// Create a minimal layer for compilation
	layerDir := filepath.Join(dir, "test-layer")
	createMinimalLayer(t, layerDir, "test-layer")

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, layerDir, true, nil, "") // noCompile=true to skip compilation for this test
	require.NoError(t, err)

	// Verify .keel/config.yml was created
	configPath := filepath.Join(dir, ".keel", "config.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Parse and verify content
	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"claude": dir}, cfg.Output)
	assert.Equal(t, []string{layerDir}, cfg.Layers)

	// Verify output message
	output := stdout.String()
	assert.Contains(t, output, "Created .keel/config.yml")
	assert.Contains(t, output, "Added layer")
}

func TestAdd_NoAgentDirCreatesConfigWithoutOutput(t *testing.T) {
	dir := t.TempDir()

	var stdout bytes.Buffer
	err := runAdd(&stdout, dir, "./some-layer", false, nil, "")
	require.NoError(t, err)

	// Verify config was created with layers but no output
	configPath := filepath.Join(dir, ".keel", "config.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Empty(t, cfg.Output)
	assert.Equal(t, []string{"./some-layer"}, cfg.Layers)

	// Verify output hints at setting output targets
	output := stdout.String()
	assert.Contains(t, output, "no output targets detected")
	assert.NotContains(t, output, "Compiling")
}

func TestAdd_AppendsToExistingConfig(t *testing.T) {
	dir := t.TempDir()

	// Create existing config
	keelDir := filepath.Join(dir, ".keel")
	err := os.MkdirAll(keelDir, 0o755)
	require.NoError(t, err)

	configPath := filepath.Join(keelDir, "config.yml")
	initial := `output:
  claude: .
layers:
  - ./layer1
`
	err = os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, "./layer2", true, nil, "") // noCompile=true
	require.NoError(t, err)

	// Verify layer was added
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Layers []string `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"./layer1", "./layer2"}, cfg.Layers)

	// Verify output message
	output := stdout.String()
	assert.Contains(t, output, "Added layer")
	assert.Contains(t, output, "./layer2")
}

func TestAdd_DuplicateSkipped(t *testing.T) {
	dir := t.TempDir()

	// Create existing config
	keelDir := filepath.Join(dir, ".keel")
	err := os.MkdirAll(keelDir, 0o755)
	require.NoError(t, err)

	configPath := filepath.Join(keelDir, "config.yml")
	initial := `output:
  claude: .
layers:
  - ./layer1
`
	err = os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, "./layer1", true, nil, "") // Add same layer again
	require.NoError(t, err)

	// Verify no error and only one entry
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Layers []string `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"./layer1"}, cfg.Layers)

	// Verify output message says it was skipped
	output := stdout.String()
	assert.Contains(t, output, "already in config")
	assert.Contains(t, output, "skipping")
}

func TestAdd_PreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()

	// Create existing config with multiple outputs
	keelDir := filepath.Join(dir, ".keel")
	err := os.MkdirAll(keelDir, 0o755)
	require.NoError(t, err)

	configPath := filepath.Join(keelDir, "config.yml")
	initial := `output:
  claude: .
  cursor: .
layers:
  - ./layer1
`
	err = os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, "./layer2", true, nil, "")
	require.NoError(t, err)

	// Verify both outputs and both layers are present
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"claude": ".", "cursor": "."}, cfg.Output)
	assert.Equal(t, []string{"./layer1", "./layer2"}, cfg.Layers)
}

func TestAdd_NoCompileFlag(t *testing.T) {
	dir := t.TempDir()

	// Create .claude directory
	err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	require.NoError(t, err)

	// Create a minimal layer
	layerDir := filepath.Join(dir, "test-layer")
	createMinimalLayer(t, layerDir, "test-layer")

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, layerDir, true, nil, "") // noCompile=true
	require.NoError(t, err)

	// Verify config was created
	configPath := filepath.Join(dir, ".keel", "config.yml")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify no compilation artifacts (CLAUDE.md should not exist)
	claudePath := filepath.Join(dir, "CLAUDE.md")
	_, err = os.Stat(claudePath)
	assert.True(t, os.IsNotExist(err), "CLAUDE.md should not exist when --no-compile is used")

	// Verify output doesn't mention compilation
	output := stdout.String()
	assert.NotContains(t, output, "Compiling")
}

func TestAdd_AutoCompiles(t *testing.T) {
	dir := t.TempDir()

	// Create .claude directory
	claudeDir := filepath.Join(dir, ".claude")
	err := os.MkdirAll(claudeDir, 0o755)
	require.NoError(t, err)

	// Create a minimal compilable layer
	layerDir := filepath.Join(dir, "test-layer")
	createMinimalLayer(t, layerDir, "test-layer")

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, layerDir, false, nil, "") // noCompile=false (auto-compile)
	require.NoError(t, err)

	// Verify config was created
	configPath := filepath.Join(dir, ".keel", "config.yml")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify compilation happened (CLAUDE.md should exist)
	claudePath := filepath.Join(dir, "CLAUDE.md")
	_, err = os.Stat(claudePath)
	require.NoError(t, err, "CLAUDE.md should exist after auto-compile")

	// Verify output mentions compilation
	output := stdout.String()
	assert.Contains(t, output, "Compiling")
}

func TestAdd_CreatesKeelDir(t *testing.T) {
	dir := t.TempDir()

	// Create .claude directory
	err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	require.NoError(t, err)

	// Create a minimal layer
	layerDir := filepath.Join(dir, "test-layer")
	createMinimalLayer(t, layerDir, "test-layer")

	// Verify .keel doesn't exist yet
	keelDir := filepath.Join(dir, ".keel")
	_, err = os.Stat(keelDir)
	require.True(t, os.IsNotExist(err))

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, layerDir, true, nil, "")
	require.NoError(t, err)

	// Verify .keel was created
	_, err = os.Stat(keelDir)
	require.NoError(t, err)
}

func TestAdd_OutputFlagNewConfig(t *testing.T) {
	dir := t.TempDir()

	var stdout bytes.Buffer
	err := runAdd(&stdout, dir, "./some-layer", true, map[string]string{"claude": "."}, "")
	require.NoError(t, err)

	configPath := filepath.Join(dir, ".keel", "config.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"claude": "."}, cfg.Output)
	assert.Equal(t, []string{"./some-layer"}, cfg.Layers)
}

func TestAdd_OutputFlagExistingConfig(t *testing.T) {
	dir := t.TempDir()

	// Create existing config with claude output
	keelDir := filepath.Join(dir, ".keel")
	err := os.MkdirAll(keelDir, 0o755)
	require.NoError(t, err)

	configPath := filepath.Join(keelDir, "config.yml")
	initial := `output:
  claude: .
layers:
  - ./layer1
`
	err = os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, "./layer2", true, map[string]string{"cursor": "."}, "")
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"claude": ".", "cursor": "."}, cfg.Output)
	assert.Equal(t, []string{"./layer1", "./layer2"}, cfg.Layers)
}

func TestAdd_UserScopeWritesToDir(t *testing.T) {
	// Use a temp dir to simulate the home directory.
	// The cobra command resolves os.UserHomeDir() → dir;
	// runAdd takes dir directly, so we test the core logic here.
	dir := t.TempDir()

	// Create .claude directory so output detection works
	err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	require.NoError(t, err)

	var stdout bytes.Buffer
	err = runAdd(&stdout, dir, "./my-layer", true, nil, "user")
	require.NoError(t, err)

	// Verify config was written inside the "home" directory
	configPath := filepath.Join(dir, ".keel", "config.yml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"claude": dir}, cfg.Output)
	assert.Equal(t, []string{"./my-layer"}, cfg.Layers)

	output := stdout.String()
	assert.Contains(t, output, "Created")
	assert.Contains(t, output, "config.yml")
}

func TestAdd_CobraCommand(t *testing.T) {
	cmd := newAddCmd()

	assert.Equal(t, "add <layer>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify --no-compile flag exists
	flag := cmd.Flags().Lookup("no-compile")
	require.NotNil(t, flag)
	assert.Equal(t, "bool", flag.Value.Type())

	// Verify --scope flag exists
	scopeFlag := cmd.Flags().Lookup("scope")
	require.NotNil(t, scopeFlag)
	assert.Equal(t, "string", scopeFlag.Value.Type())
}

// createMinimalLayer creates a minimal valid layer for testing compilation.
func createMinimalLayer(t *testing.T, dir, name string) {
	t.Helper()

	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err)

	// Create keel.yml
	keelYML := `schema: v1
name: ` + name + `
instructions:
  - source: instructions.md
`
	err = os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(keelYML), 0o644)
	require.NoError(t, err)

	// Create instructions.md
	instructions := "# " + name + "\n\nTest instructions.\n"
	err = os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(instructions), 0o644)
	require.NoError(t, err)
}

func TestFormatOutputMap(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		output string
	}{
		{
			name:   "single output",
			input:  map[string]string{"claude": "."},
			output: "claude=.",
		},
		{
			name:   "empty map",
			input:  map[string]string{},
			output: "",
		},
		{
			name:   "multiple outputs",
			input:  map[string]string{"claude": ".", "cursor": "."},
			output: "claude=., cursor=.", // Note: map iteration order may vary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatOutputMap(tt.input)
			if len(tt.input) <= 1 {
				// For single or empty maps, order is deterministic
				assert.Equal(t, tt.output, result)
			} else {
				// For multiple outputs, just verify both are present
				for agent, path := range tt.input {
					expected := agent + "=" + path
					assert.Contains(t, result, expected)
				}
			}
		})
	}
}
