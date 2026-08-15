package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RegisterFlags tests ---

func TestRegisterFlags_AllFieldsRegistered(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &CompileConfig{}
	RegisterFlags(cmd, cfg)

	expectedFlags := []string{
		"output", "layer",
		"distributable", "user-config", "user-priority",
	}

	for _, name := range expectedFlags {
		assert.NotNilf(t, cmd.Flags().Lookup(name), "expected flag %q to be registered", name)
	}
}

func TestRegisterFlags_FlagTypes(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &CompileConfig{}
	RegisterFlags(cmd, cfg)

	stringFlags := []string{"name", "user-config", "user-priority"}
	for _, name := range stringFlags {
		f := cmd.Flags().Lookup(name)
		require.NotNilf(t, f, "flag %q not registered", name)
		assert.Equalf(t, "string", f.Value.Type(), "flag %q type", name)
	}

	boolFlags := []string{"distributable", "offline", "locked", "allow-install"}
	for _, name := range boolFlags {
		f := cmd.Flags().Lookup(name)
		require.NotNilf(t, f, "flag %q not registered", name)
		assert.Equalf(t, "bool", f.Value.Type(), "flag %q type", name)
	}

	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f, "flag \"output\" not registered")
	assert.Equal(t, "stringToString", f.Value.Type())

	f = cmd.Flags().Lookup("layer")
	require.NotNil(t, f, "flag \"layer\" not registered")
	assert.Equal(t, "stringArray", f.Value.Type())
}

func TestRegisterFlags_Defaults(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &CompileConfig{}
	RegisterFlags(cmd, cfg)

	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f, "flag 'output' not registered")
	assert.Equal(t, "[]", f.DefValue)
}

func TestRegisterFlags_UsageStrings(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &CompileConfig{}
	RegisterFlags(cmd, cfg)

	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f, "flag 'output' not registered")
	assert.NotEmpty(t, f.Usage, "flag 'output' should have a non-empty usage string")
}

func TestRegisterFlags_BindsToStruct(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cfg := &CompileConfig{}
	RegisterFlags(cmd, cfg)

	require.NoError(t, cmd.Flags().Set("output", "claude=/keel"))
	assert.Equal(t, "/keel", cfg.Output["claude"])
}

// --- LoadConfigFile tests ---

func TestLoadConfigFile_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "keel-config.yml")
	content := `output:
  cursor: /tmp/output
layers:
  - /a
  - /b
  - /c
distributable: true
name: my-dist
user-config: /custom/user
user-priority: low
`
	require.NoError(t, os.WriteFile(cfgFile, []byte(content), 0644))

	cfg, err := LoadConfigFile(cfgFile)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/output", cfg.Output["cursor"])
	require.Len(t, cfg.LayerEntries, 3)
	assert.Equal(t, "/a", cfg.LayerEntries[0].Source)
	assert.Equal(t, "/b", cfg.LayerEntries[1].Source)
	assert.Equal(t, "/c", cfg.LayerEntries[2].Source)
	assert.True(t, cfg.Distributable)
	assert.Equal(t, "my-dist", cfg.Name)
	assert.Equal(t, "/custom/user", cfg.UserConfig)
	assert.Equal(t, "low", cfg.UserPriority)
}

func TestLoadConfigFile_MissingFile(t *testing.T) {
	_, err := LoadConfigFile("/nonexistent/config.yml")
	assert.Error(t, err)
}

func TestLoadConfigFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "bad.yml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(":\n  :\nbad yaml {{{"), 0644))

	_, err := LoadConfigFile(cfgFile)
	assert.Error(t, err)
}

func TestLoadConfigFile_UnknownKeysWarning(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "extra.yml")
	content := `layers:
  - /a
  - /b
unknown-field: something
another-bad-key: 42
`
	require.NoError(t, os.WriteFile(cfgFile, []byte(content), 0644))

	// Capture slog output to verify warnings.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldLogger)

	cfg, err := LoadConfigFile(cfgFile)
	require.NoError(t, err, "LoadConfigFile() should not fail on unknown keys")

	require.Len(t, cfg.LayerEntries, 2)
	assert.Equal(t, "/a", cfg.LayerEntries[0].Source)
	assert.Equal(t, "/b", cfg.LayerEntries[1].Source)
	assert.NotEmpty(t, buf.String(), "expected warnings for unknown keys")
}

func TestLoadConfigFile_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "partial.yml")
	content := `output:
  claude: /keel
`
	require.NoError(t, os.WriteFile(cfgFile, []byte(content), 0644))

	cfg, err := LoadConfigFile(cfgFile)
	require.NoError(t, err)

	assert.Equal(t, "/keel", cfg.Output["claude"])
	assert.Empty(t, cfg.Layers, "unset Layers should be empty")
}

func TestLoadConfigFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "empty.yml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(""), 0644))

	cfg, err := LoadConfigFile(cfgFile)
	require.NoError(t, err)

	assert.Empty(t, cfg.Output, "Output should be empty/nil for empty file")
}

// --- ApplyDefaults tests ---

func TestApplyDefaults_NoEnvVars(t *testing.T) {
	t.Setenv("CONFIG_LAYERS", "")
	t.Setenv("USER_CONFIG", "")
	t.Setenv("USER_PRIORITY", "")

	cfg := &CompileConfig{}
	cfg.ApplyDefaults()

	assert.Empty(t, cfg.Output)
	assert.Equal(t, "/mnt/user-config", cfg.UserConfig)
	assert.Equal(t, "high", cfg.UserPriority)
}

func TestApplyDefaults_EnvVarFallback(t *testing.T) {
	tests := []struct {
		name         string
		envLayers    string
		envConfig    string
		envPriority  string
		wantLayers   []string
		wantConfig   string
		wantPriority string
	}{
		{
			name:         "CONFIG_LAYERS sets layers",
			envLayers:    "/env/layers/*",
			wantLayers:   []string{"/env/layers/*"},
			wantConfig:   "/mnt/user-config",
			wantPriority: "high",
		},
		{
			name:         "USER_CONFIG overrides default",
			envConfig:    "/env/user",
			wantConfig:   "/env/user",
			wantPriority: "high",
		},
		{
			name:         "USER_PRIORITY overrides default",
			envPriority:  "low",
			wantConfig:   "/mnt/user-config",
			wantPriority: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFIG_LAYERS", tt.envLayers)
			t.Setenv("USER_CONFIG", tt.envConfig)
			t.Setenv("USER_PRIORITY", tt.envPriority)

			cfg := &CompileConfig{}
			cfg.ApplyDefaults()

			if tt.wantLayers != nil {
				assert.Equal(t, tt.wantLayers, cfg.Layers)
			}
			assert.Equal(t, tt.wantConfig, cfg.UserConfig)
			assert.Equal(t, tt.wantPriority, cfg.UserPriority)
		})
	}
}

func TestApplyDefaults_ExplicitValuesNotOverridden(t *testing.T) {
	t.Setenv("CONFIG_LAYERS", "/env/layers")
	t.Setenv("USER_CONFIG", "/env/user")
	t.Setenv("USER_PRIORITY", "low")

	cfg := &CompileConfig{
		Layers:       []string{"/explicit/layers"},
		UserConfig:   "/explicit/user",
		UserPriority: "high",
	}
	cfg.ApplyDefaults()

	assert.Equal(t, "/explicit/layers", cfg.Layers[0])
	assert.Equal(t, "/explicit/user", cfg.UserConfig)
	assert.Equal(t, "high", cfg.UserPriority)
}

// --- DetectOutputTargets tests ---

func TestDetectOutputTargets(t *testing.T) {
	tests := []struct {
		name     string
		setupFn  func(dir string) error
		wantLen  int
		wantKeys map[string]bool
	}{
		{
			name: "detects claude directory",
			setupFn: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, ".claude"), 0755)
			},
			wantLen:  1,
			wantKeys: map[string]bool{"claude": true},
		},
		{
			name: "detects multiple targets",
			setupFn: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, ".cursor"), 0755)
			},
			wantLen:  2,
			wantKeys: map[string]bool{"claude": true, "cursor": true},
		},
		{
			name:    "no targets in empty dir",
			setupFn: func(dir string) error { return nil },
			wantLen: 0,
		},
		{
			name: "ignores .claude file (not directory)",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, ".claude"), []byte("not a dir"), 0644)
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, tt.setupFn(dir))

			got := DetectOutputTargets(dir)
			assert.Len(t, got, tt.wantLen)
			for key := range tt.wantKeys {
				assert.Equal(t, dir, got[key], "target %q", key)
			}
		})
	}
}

// --- ApplyDefaults auto-detection tests ---

func TestNormalizeLayers_LocalLayerOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".keel", "local"), 0755))
	t.Chdir(dir)

	entries := (&CompileConfig{}).NormalizeLayers()

	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Join(".keel", "local"), entries[0].Source)
}

func TestNormalizeLayers_LocalLayerAppendedToCLI(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".keel", "local"), 0755))
	t.Chdir(dir)

	entries := (&CompileConfig{Layers: []string{"/some/layers"}}).NormalizeLayers()

	require.Len(t, entries, 2)
	assert.Equal(t, "/some/layers", entries[0].Source)
	assert.Equal(t, filepath.Join(".keel", "local"), entries[1].Source)
}

func TestNormalizeLayers_LocalLayerAppendedToConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".keel", "local"), 0755))
	t.Chdir(dir)

	entries := (&CompileConfig{
		LayerEntries: []LayerEntry{{Source: "/config/layer"}},
	}).NormalizeLayers()

	require.Len(t, entries, 2)
	assert.Equal(t, "/config/layer", entries[0].Source)
	assert.Equal(t, filepath.Join(".keel", "local"), entries[1].Source)
}

func TestNormalizeLayers_UserScopeSkipsLocal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".keel", "local"), 0755))
	t.Chdir(dir)

	entries := (&CompileConfig{
		Layers: []string{"/some/layer"},
		Scope:  "user",
	}).NormalizeLayers()

	require.Len(t, entries, 1)
	assert.Equal(t, "/some/layer", entries[0].Source)
}

func TestNormalizeLayers_UserScopeIncludeLocalOverride(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".keel", "local"), 0755))
	t.Chdir(dir)

	entries := (&CompileConfig{
		Layers:       []string{"/some/layer"},
		Scope:        "user",
		IncludeLocal: true,
	}).NormalizeLayers()

	require.Len(t, entries, 2)
	assert.Equal(t, "/some/layer", entries[0].Source)
	assert.Equal(t, filepath.Join(".keel", "local"), entries[1].Source)
}

func TestNormalizeLayers_ProjectScopeIncludesLocal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".keel", "local"), 0755))
	t.Chdir(dir)

	// Default scope (empty string = project) still includes .keel/local.
	entries := (&CompileConfig{
		Layers: []string{"/some/layer"},
	}).NormalizeLayers()

	require.Len(t, entries, 2)
	assert.Equal(t, "/some/layer", entries[0].Source)
	assert.Equal(t, filepath.Join(".keel", "local"), entries[1].Source)
}

func TestApplyDefaults_NoLocalLayer(t *testing.T) {
	t.Setenv("CONFIG_LAYERS", "")
	t.Setenv("USER_CONFIG", "")
	t.Setenv("USER_PRIORITY", "")

	dir := t.TempDir()
	t.Chdir(dir)

	cfg := &CompileConfig{}
	cfg.ApplyDefaults()

	assert.Empty(t, cfg.Layers)
}

func TestApplyDefaults_AutoDetectOutput(t *testing.T) {
	t.Setenv("CONFIG_LAYERS", "")
	t.Setenv("USER_CONFIG", "")
	t.Setenv("USER_PRIORITY", "")

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))

	t.Chdir(dir)

	cfg := &CompileConfig{}
	cfg.ApplyDefaults()

	require.Len(t, cfg.Output, 1)
	assert.Equal(t, ".", cfg.Output["claude"])
}

func TestApplyDefaults_ExplicitOutputNotOverridden(t *testing.T) {
	t.Setenv("CONFIG_LAYERS", "")
	t.Setenv("USER_CONFIG", "")
	t.Setenv("USER_PRIORITY", "")

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))

	t.Chdir(dir)

	cfg := &CompileConfig{
		Output: map[string]string{"cursor": "/custom/path"},
	}
	cfg.ApplyDefaults()

	require.Len(t, cfg.Output, 1)
	assert.Equal(t, "/custom/path", cfg.Output["cursor"])
}
