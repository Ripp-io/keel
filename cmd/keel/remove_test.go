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

func TestRemove_RemovesLayer(t *testing.T) {
	dir := t.TempDir()
	keelDir := filepath.Join(dir, ".keel")
	require.NoError(t, os.MkdirAll(keelDir, 0o755))

	configPath := filepath.Join(keelDir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./layer1
  - ./layer2
  - ./layer3
`), 0o644))

	var stdout bytes.Buffer
	err := runRemove(&stdout, dir, "./layer2", true)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Layers []string `yaml:"layers"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, []string{"./layer1", "./layer3"}, cfg.Layers)
	assert.Contains(t, stdout.String(), "Removed layer")
}

func TestRemove_LayerNotFound(t *testing.T) {
	dir := t.TempDir()
	keelDir := filepath.Join(dir, ".keel")
	require.NoError(t, os.MkdirAll(keelDir, 0o755))

	configPath := filepath.Join(keelDir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./layer1
`), 0o644))

	var stdout bytes.Buffer
	err := runRemove(&stdout, dir, "./nonexistent", true)
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "not found")
	assert.Contains(t, output, "skipping")

	// Config unchanged
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var cfg struct {
		Layers []string `yaml:"layers"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, []string{"./layer1"}, cfg.Layers)
}

func TestRemove_NoConfigFile(t *testing.T) {
	dir := t.TempDir()

	var stdout bytes.Buffer
	err := runRemove(&stdout, dir, "./layer1", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.yml")
}

func TestRemove_PreservesOtherConfig(t *testing.T) {
	dir := t.TempDir()
	keelDir := filepath.Join(dir, ".keel")
	require.NoError(t, os.MkdirAll(keelDir, 0o755))

	configPath := filepath.Join(keelDir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
  cursor: .
layers:
  - ./layer1
  - ./layer2
`), 0o644))

	var stdout bytes.Buffer
	err := runRemove(&stdout, dir, "./layer1", true)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Output map[string]string `yaml:"output"`
		Layers []string          `yaml:"layers"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, map[string]string{"claude": ".", "cursor": "."}, cfg.Output)
	assert.Equal(t, []string{"./layer2"}, cfg.Layers)
}

func TestRemove_LastLayer(t *testing.T) {
	dir := t.TempDir()
	keelDir := filepath.Join(dir, ".keel")
	require.NoError(t, os.MkdirAll(keelDir, 0o755))

	configPath := filepath.Join(keelDir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./only-layer
`), 0o644))

	var stdout bytes.Buffer
	err := runRemove(&stdout, dir, "./only-layer", true)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg struct {
		Layers []string `yaml:"layers"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Empty(t, cfg.Layers)
}

func TestRemove_NoCompileFlag(t *testing.T) {
	dir := t.TempDir()
	keelDir := filepath.Join(dir, ".keel")
	require.NoError(t, os.MkdirAll(keelDir, 0o755))

	configPath := filepath.Join(keelDir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./layer1
`), 0o644))

	var stdout bytes.Buffer
	err := runRemove(&stdout, dir, "./layer1", true) // noCompile=true
	require.NoError(t, err)
	assert.NotContains(t, stdout.String(), "Compiling")
}

func TestRemove_CobraCommand(t *testing.T) {
	cmd := newRemoveCmd()

	assert.Equal(t, "remove <layer>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	flag := cmd.Flags().Lookup("no-compile")
	require.NotNil(t, flag)
	assert.Equal(t, "bool", flag.Value.Type())
}
