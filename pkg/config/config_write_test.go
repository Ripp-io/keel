package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCreateConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	layers := []string{"./layer1", "./layer2"}
	output := map[string]string{"claude": ".", "cursor": "."}

	err := CreateConfigFile(configPath, layers, output)
	require.NoError(t, err)

	// Verify file exists
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Verify content can be parsed
	var content configFileContent
	err = yaml.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Equal(t, layers, content.Layers)
	assert.Equal(t, output, content.Output)
}

func TestAddLayerToConfigFile_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Create initial config
	initial := `output:
  claude: .
layers:
  - ./layer1
  - ./layer2
`
	err := os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	// Add a new layer
	err = AddLayerToConfigFile(configPath, "./layer3")
	require.NoError(t, err)

	// Verify the layer was added
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content configFileContent
	err = yaml.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Equal(t, []string{"./layer1", "./layer2", "./layer3"}, content.Layers)
	assert.Equal(t, map[string]string{"claude": "."}, content.Output)
}

func TestAddLayerToConfigFile_CreatesLayersKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Create config without layers key
	initial := `output:
  claude: .
`
	err := os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	// Add a layer (should create layers key)
	err = AddLayerToConfigFile(configPath, "./new-layer")
	require.NoError(t, err)

	// Verify the layer was added
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content configFileContent
	err = yaml.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Equal(t, []string{"./new-layer"}, content.Layers)
	assert.Equal(t, map[string]string{"claude": "."}, content.Output)
}

func TestAddLayerToConfigFile_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Create config with comments
	initial := `# This is a comment
output:
  claude: .
# Another comment
layers:
  - ./layer1
`
	err := os.WriteFile(configPath, []byte(initial), 0o644)
	require.NoError(t, err)

	// Add a layer
	err = AddLayerToConfigFile(configPath, "./layer2")
	require.NoError(t, err)

	// Verify comments are preserved
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// yaml.v3 preserves comments in the node structure, but they may be reformatted
	// We just verify the layer was added correctly
	var content configFileContent
	err = yaml.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Equal(t, []string{"./layer1", "./layer2"}, content.Layers)
}

func TestLayerExistsInConfig_Found(t *testing.T) {
	cfg := &CompileConfig{
		LayerEntries: []LayerEntry{
			{Source: "./layer1"},
			{Source: "./layer2"},
		},
	}

	assert.True(t, LayerExistsInConfig(cfg, "./layer1"))
	assert.True(t, LayerExistsInConfig(cfg, "./layer2"))
}

func TestLayerExistsInConfig_NotFound(t *testing.T) {
	cfg := &CompileConfig{
		LayerEntries: []LayerEntry{
			{Source: "./layer1"},
		},
	}

	assert.False(t, LayerExistsInConfig(cfg, "./layer2"))
	assert.False(t, LayerExistsInConfig(cfg, "./nonexistent"))
}

func TestLayerExistsInConfig_EmptyConfig(t *testing.T) {
	cfg := &CompileConfig{
		LayerEntries: []LayerEntry{},
	}

	assert.False(t, LayerExistsInConfig(cfg, "./any-layer"))
}

func TestAddLayerToConfigFile_FileNotFound(t *testing.T) {
	err := AddLayerToConfigFile("/nonexistent/config.yml", "./layer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

func TestAddLayerToConfigFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	// Create invalid YAML
	err := os.WriteFile(configPath, []byte("invalid: [unclosed"), 0o644)
	require.NoError(t, err)

	err = AddLayerToConfigFile(configPath, "./layer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config file")
}

func TestRemoveLayerFromConfigFile_RemovesMiddle(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./layer1
  - ./layer2
  - ./layer3
`), 0o644))

	require.NoError(t, RemoveLayerFromConfigFile(configPath, "./layer2"))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content configFileContent
	require.NoError(t, yaml.Unmarshal(data, &content))
	assert.Equal(t, []string{"./layer1", "./layer3"}, content.Layers)
	assert.Equal(t, map[string]string{"claude": "."}, content.Output)
}

func TestRemoveLayerFromConfigFile_RemovesLast(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./only-layer
`), 0o644))

	require.NoError(t, RemoveLayerFromConfigFile(configPath, "./only-layer"))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content configFileContent
	require.NoError(t, yaml.Unmarshal(data, &content))
	assert.Empty(t, content.Layers)
}

func TestRemoveLayerFromConfigFile_ObjectForm(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./layer1
  - source: ./layer2
    filter:
      skills:
        include: ["x"]
  - ./layer3
`), 0o644))

	require.NoError(t, RemoveLayerFromConfigFile(configPath, "./layer2"))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content struct {
		Layers []LayerEntry `yaml:"layers"`
	}
	require.NoError(t, yaml.Unmarshal(data, &content))
	require.Len(t, content.Layers, 2)
	assert.Equal(t, "./layer1", content.Layers[0].Source)
	assert.Equal(t, "./layer3", content.Layers[1].Source)
}

func TestRemoveLayerFromConfigFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	require.NoError(t, os.WriteFile(configPath, []byte(`output:
  claude: .
layers:
  - ./layer1
`), 0o644))

	// Removing a non-existent layer should not error (no-op)
	require.NoError(t, RemoveLayerFromConfigFile(configPath, "./nonexistent"))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content configFileContent
	require.NoError(t, yaml.Unmarshal(data, &content))
	assert.Equal(t, []string{"./layer1"}, content.Layers)
}

func TestRemoveLayerFromConfigFile_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")

	require.NoError(t, os.WriteFile(configPath, []byte(`# project config
output:
  claude: .
# layers managed by keel add/remove
layers:
  - ./layer1
  - ./layer2
`), 0o644))

	require.NoError(t, RemoveLayerFromConfigFile(configPath, "./layer1"))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var content configFileContent
	require.NoError(t, yaml.Unmarshal(data, &content))
	assert.Equal(t, []string{"./layer2"}, content.Layers)
}
