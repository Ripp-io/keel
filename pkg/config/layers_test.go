package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Register test agents since adapter packages aren't imported by config tests.
	// In production, adapters register themselves via init() when imported.
	RegisterAgent(AgentMeta{Format: "claude", Files: []string{"CLAUDE.md"}, Dirs: []string{".claude"}})
	RegisterAgent(AgentMeta{Format: "cursor", Files: []string{".cursorrules"}, Dirs: []string{".cursor"}})
	RegisterAgent(AgentMeta{Format: "agentsmd", Files: []string{"AGENTS.md"}, Dirs: []string{".agents"}})
	RegisterAgent(AgentMeta{Format: "copilot", Files: []string{"copilot-instructions.md"}, Dirs: []string{".github"}})
	RegisterAgent(AgentMeta{Format: "cline", Files: []string{".clinerules"}, Dirs: []string{".cline"}})
	RegisterAgent(AgentMeta{Format: "aider", Files: []string{"CONVENTIONS.md"}, Dirs: []string{".aider"}})

	os.Exit(m.Run())
}

// stringsToLayerEntries converts a slice of paths to LayerEntry objects for testing.
func stringsToLayerEntries(paths []string) []LayerEntry {
	entries := make([]LayerEntry, len(paths))
	for i, p := range paths {
		entries[i] = LayerEntry{Source: p}
	}
	return entries
}

func TestAgentMarkersUnique(t *testing.T) {
	require.NoError(t, ValidateAgentMarkers())
}

func TestResolveLayers_SinglePath(t *testing.T) {
	dir := t.TempDir()
	layerDir := filepath.Join(dir, "base")
	require.NoError(t, os.MkdirAll(layerDir, 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{layerDir}), "", "high", false, 0)
	require.NoError(t, err)

	require.Len(t, layers, 1)
	assert.Equal(t, layerDir, layers[0].Path)
}

func TestResolveLayers_MultiplePaths(t *testing.T) {
	dir := t.TempDir()

	dirs := []string{
		filepath.Join(dir, "base"),
		filepath.Join(dir, "org"),
		filepath.Join(dir, "project"),
	}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	layers, err := ResolveLayers(stringsToLayerEntries(dirs), "", "high", false, 0)
	require.NoError(t, err)
	require.Len(t, layers, 3)

	for i, d := range dirs {
		assert.Equalf(t, d, layers[i].Path, "layer[%d]", i)
	}
}

func TestResolveLayers_UserPriorityHigh(t *testing.T) {
	dir := t.TempDir()

	baseDir := filepath.Join(dir, "base")
	userDir := filepath.Join(dir, "user")
	for _, d := range []string{baseDir, userDir} {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	layers, err := ResolveLayers(stringsToLayerEntries([]string{baseDir}), userDir, "high", false, 0)
	require.NoError(t, err)
	require.Len(t, layers, 2)

	// User should be last (highest priority)
	assert.Equal(t, baseDir, layers[0].Path)
	assert.Equal(t, userDir, layers[1].Path)
	assert.Equal(t, "user", layers[1].Type)
}

func TestResolveLayers_UserPriorityLow(t *testing.T) {
	dir := t.TempDir()

	baseDir := filepath.Join(dir, "base")
	userDir := filepath.Join(dir, "user")
	for _, d := range []string{baseDir, userDir} {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	layers, err := ResolveLayers(stringsToLayerEntries([]string{baseDir}), userDir, "low", false, 0)
	require.NoError(t, err)
	require.Len(t, layers, 2)

	// User should be first (lowest priority)
	assert.Equal(t, userDir, layers[0].Path)
	assert.Equal(t, "user", layers[0].Type)
	assert.Equal(t, baseDir, layers[1].Path)
}

func TestResolveLayers_NoUser(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	require.NoError(t, os.MkdirAll(baseDir, 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{baseDir}), "", "high", false, 0)
	require.NoError(t, err)
	assert.Len(t, layers, 1)
}

func TestResolveLayers_UserDirMissing(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	require.NoError(t, os.MkdirAll(baseDir, 0755))

	userDir := filepath.Join(dir, "nonexistent_user")

	layers, err := ResolveLayers(stringsToLayerEntries([]string{baseDir}), userDir, "high", false, 0)
	require.NoError(t, err)
	assert.Len(t, layers, 1, "missing user dir should be ignored")
}

func TestResolveLayers_EmptyConfigLayers(t *testing.T) {
	layers, err := ResolveLayers(stringsToLayerEntries(nil), "", "high", false, 0)
	require.NoError(t, err)
	assert.Empty(t, layers)
}

func TestResolveLayers_NonexistentPath(t *testing.T) {
	_, err := ResolveLayers(stringsToLayerEntries([]string{"/nonexistent/path"}), "", "high", false, 0)
	assert.Error(t, err)
}

func TestResolveLayers_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not_a_dir")
	require.NoError(t, os.WriteFile(filePath, []byte("file"), 0644))

	_, err := ResolveLayers(stringsToLayerEntries([]string{filePath}), "", "high", false, 0)
	assert.Error(t, err)
}

func TestResolveLayers_SkipsEmptySegments(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "base")
	require.NoError(t, os.MkdirAll(baseDir, 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{"", baseDir, ""}), "", "high", false, 0)
	require.NoError(t, err)
	assert.Len(t, layers, 1, "empty segments should be skipped")
}

func TestDetectLayerFormat(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func(dir string) error
		wantFormat LayerFormat
	}{
		{
			name: "keel.yml with adapter",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("adapter: claude"), 0644)
			},
			wantFormat: FormatKeel,
		},
		{
			name:       "no keel.yml defaults to FormatKeel",
			setupFn:    func(dir string) error { return nil },
			wantFormat: FormatKeel,
		},
		{
			name: "keel.yml is a directory",
			setupFn: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, "keel.yml"), 0755)
			},
			wantFormat: FormatKeel,
		},
		{
			name: "schema v1-claude",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1-claude\n"), 0644)
			},
			wantFormat: LayerFormat("claude"),
		},
		{
			name: "schema v1-cursor",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1-cursor\n"), 0644)
			},
			wantFormat: LayerFormat("cursor"),
		},
		{
			name: "schema v1 plain",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644)
			},
			wantFormat: FormatKeel,
		},
		{
			name: "no schema field defaults to FormatKeel",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("name: test\n"), 0644)
			},
			wantFormat: FormatKeel,
		},
		{
			name: "unreadable YAML falls back to FormatKeel",
			setupFn: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "keel.yml"), []byte(":\n  :\n    :\nbad yaml {{{"), 0644)
			},
			wantFormat: FormatKeel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, tt.setupFn(dir))

			format, _ := DetectLayerFormat(dir)
			assert.Equal(t, tt.wantFormat, format)
		})
	}
}

func TestResolveLayers_FormatDetection(t *testing.T) {
	dir := t.TempDir()

	keelLayer := filepath.Join(dir, "keel_layer")
	require.NoError(t, os.MkdirAll(keelLayer, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keelLayer, "keel.yml"), []byte("adapter: claude"), 0644))

	nativeLayer := filepath.Join(dir, "native_layer")
	require.NoError(t, os.MkdirAll(nativeLayer, 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{keelLayer, nativeLayer}), "", "high", false, 0)
	require.NoError(t, err)

	assert.Equal(t, FormatKeel, layers[0].Format)
	assert.Equal(t, FormatKeel, layers[1].Format)
}

func TestResolveLayers_ClassifyLayerType(t *testing.T) {
	dir := t.TempDir()

	baseDir := filepath.Join(dir, "base")
	projectDir := filepath.Join(dir, "project")
	otherDir := filepath.Join(dir, "custom")
	for _, d := range []string{baseDir, projectDir, otherDir} {
		require.NoError(t, os.MkdirAll(d, 0755))
	}

	layers, err := ResolveLayers(stringsToLayerEntries([]string{baseDir, projectDir, otherDir}), "", "high", false, 0)
	require.NoError(t, err)

	assert.Equal(t, "base", layers[0].Type)
	assert.Equal(t, "project", layers[1].Type)
	// "custom" doesn't match base or project, defaults to "project"
	assert.Equal(t, "project", layers[2].Type)
}

func TestResolveLayers_GlobExpansion(t *testing.T) {
	dir := t.TempDir()
	layersDir := filepath.Join(dir, "layers")
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "01-company"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "02-org"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "03-team"), 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{layersDir + "/*"}), "", "high", false, 0)
	require.NoError(t, err)
	require.Len(t, layers, 3)

	// Should be sorted alphabetically
	assert.Equal(t, "01-company", filepath.Base(layers[0].Path))
	assert.Equal(t, "02-org", filepath.Base(layers[1].Path))
	assert.Equal(t, "03-team", filepath.Base(layers[2].Path))
}

func TestResolveLayers_GlobSkipsFiles(t *testing.T) {
	dir := t.TempDir()
	layersDir := filepath.Join(dir, "layers")
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "01-company"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(layersDir, "README.md"), []byte("hello"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "02-org"), 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{layersDir + "/*"}), "", "high", false, 0)
	require.NoError(t, err)
	assert.Len(t, layers, 2, "files should be skipped")
}

func TestResolveLayers_GlobEmptyDir(t *testing.T) {
	dir := t.TempDir()
	layersDir := filepath.Join(dir, "layers")
	require.NoError(t, os.MkdirAll(layersDir, 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{layersDir + "/*"}), "", "high", false, 0)
	require.NoError(t, err)
	assert.Empty(t, layers)
}

func TestResolveLayers_GlobMissingDir(t *testing.T) {
	layers, err := ResolveLayers(stringsToLayerEntries([]string{"/nonexistent/path/*"}), "", "high", false, 0)
	require.NoError(t, err, "should not error on missing glob dir")
	assert.Empty(t, layers)
}

func TestResolveLayers_GlobMixedWithExplicit(t *testing.T) {
	dir := t.TempDir()

	baseDir := filepath.Join(dir, "base")
	require.NoError(t, os.MkdirAll(baseDir, 0755))

	layersDir := filepath.Join(dir, "layers")
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "01-company"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "02-org"), 0755))

	layers, err := ResolveLayers(stringsToLayerEntries([]string{baseDir, layersDir + "/*"}), "", "high", false, 0)
	require.NoError(t, err)
	require.Len(t, layers, 3)

	assert.Equal(t, "base", filepath.Base(layers[0].Path))
	assert.Equal(t, "01-company", filepath.Base(layers[1].Path))
}

func TestResolveLayers_GlobWithUserPriority(t *testing.T) {
	dir := t.TempDir()
	layersDir := filepath.Join(dir, "layers")
	require.NoError(t, os.MkdirAll(filepath.Join(layersDir, "01-company"), 0755))

	userDir := filepath.Join(dir, "user")
	require.NoError(t, os.MkdirAll(userDir, 0755))

	// High priority -- user last
	layers, err := ResolveLayers(stringsToLayerEntries([]string{layersDir + "/*"}), userDir, "high", false, 0)
	require.NoError(t, err)
	require.Len(t, layers, 2)
	assert.Equal(t, "user", layers[1].Type)

	// Low priority -- user first
	layers, err = ResolveLayers(stringsToLayerEntries([]string{layersDir + "/*"}), userDir, "low", false, 0)
	require.NoError(t, err)
	assert.Equal(t, "user", layers[0].Type)
}

// --- Agent registry query tests ---

func TestAgentList_ReturnsCopy(t *testing.T) {
	first := AgentList()
	require.NotEmpty(t, first, "expected at least one registered agent")

	// Mutate the first slice.
	first[0].Format = "mutated"

	// A second call should return the original, unmodified data.
	second := AgentList()
	assert.NotEqual(t, LayerFormat("mutated"), second[0].Format, "AgentList should return a copy, not a reference")
}

func TestAgentList_ContainsRegisteredAgents(t *testing.T) {
	agents := AgentList()

	formats := make(map[LayerFormat]AgentMeta)
	for _, a := range agents {
		formats[a.Format] = a
	}

	claude, ok := formats["claude"]
	require.True(t, ok, "expected claude agent to be registered")
	assert.NotEmpty(t, claude.Files, "claude agent should have marker files")
	assert.NotEmpty(t, claude.Dirs, "claude agent should have marker dirs")

	cursor, ok := formats["cursor"]
	require.True(t, ok, "expected cursor agent to be registered")
	assert.NotEmpty(t, cursor.Files, "cursor agent should have marker files")
	assert.NotEmpty(t, cursor.Dirs, "cursor agent should have marker dirs")
}

func TestAgentNames_ReturnsAgentNames(t *testing.T) {
	names := AgentNames()
	assert.Contains(t, names, "claude")
	assert.Contains(t, names, "cursor")
}

func TestIsAgentFormat_Positive(t *testing.T) {
	assert.True(t, IsAgentFormat("claude"))
	assert.True(t, IsAgentFormat("cursor"))
}

func TestIsAgentFormat_Negative(t *testing.T) {
	assert.False(t, IsAgentFormat("unknown"))
	assert.False(t, IsAgentFormat("keel"))
	assert.False(t, IsAgentFormat(""))
}

func TestLookupAgent_Found(t *testing.T) {
	agent := LookupAgent("claude")
	require.NotNil(t, agent, "expected LookupAgent to find claude")
	assert.Equal(t, LayerFormat("claude"), agent.Format)
	assert.Contains(t, agent.Dirs, ".claude")
}

func TestLookupAgent_NotFound(t *testing.T) {
	agent := LookupAgent("nonexistent")
	assert.Nil(t, agent)
}

// --- keel/ directory fallback tests ---

func TestApplyKeelDirFallback_RootIsKeelDSLLayer(t *testing.T) {
	dir := t.TempDir()
	// Root has keel.yml → recognized layer
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644))

	result := applyKeelDirFallback(dir, "https://example.com/repo.git#main")
	assert.Equal(t, dir, result, "should return root unchanged when root is a Keel DSL layer")
}

func TestApplyKeelDirFallback_RootIsClaudeLayer(t *testing.T) {
	dir := t.TempDir()
	// Root has CLAUDE.md and .claude/ → recognized Claude native layer
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude config"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))

	result := applyKeelDirFallback(dir, "https://example.com/repo.git#main")
	assert.Equal(t, dir, result, "should return root unchanged when root is a Claude native layer")
}

func TestApplyKeelDirFallback_RootNotLayerButKeelDirExists(t *testing.T) {
	dir := t.TempDir()
	// Root is NOT a layer (no keel.yml, no agent markers)
	// But keel/ exists with keel.yml
	keelDir := filepath.Join(dir, "keel")
	require.NoError(t, os.MkdirAll(keelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keelDir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644))

	result := applyKeelDirFallback(dir, "https://example.com/repo.git#main")
	assert.Equal(t, keelDir, result, "should return keel/ when root is not a layer but keel/ is")
}

func TestApplyKeelDirFallback_KeelDirDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	// Root is NOT a layer, keel/ does NOT exist

	result := applyKeelDirFallback(dir, "https://example.com/repo.git#main")
	assert.Equal(t, dir, result, "should return root unchanged when keel/ does not exist")
}

func TestApplyKeelDirFallback_KeelDirExistsButNotALayer(t *testing.T) {
	dir := t.TempDir()
	// Root is NOT a layer
	// keel/ exists but is NOT a layer (no keel.yml, no markers)
	keelDir := filepath.Join(dir, "keel")
	require.NoError(t, os.MkdirAll(keelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keelDir, "README.md"), []byte("# Not a layer"), 0644))

	result := applyKeelDirFallback(dir, "https://example.com/repo.git#main")
	assert.Equal(t, dir, result, "should return root unchanged when keel/ exists but is not a layer")
}

func TestApplyKeelDirFallback_SourceHasExplicitSubpath(t *testing.T) {
	dir := t.TempDir()
	// Root is NOT a layer, keel/ exists with keel.yml
	keelDir := filepath.Join(dir, "keel")
	require.NoError(t, os.MkdirAll(keelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keelDir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644))

	// Source has explicit subpath — should NOT apply fallback
	result := applyKeelDirFallback(dir, "https://example.com/repo.git#main:config")
	assert.Equal(t, dir, result, "should return root unchanged when source has explicit subpath")
}

func TestApplyKeelDirFallback_LocalSource(t *testing.T) {
	dir := t.TempDir()
	// Root is NOT a layer, keel/ exists
	keelDir := filepath.Join(dir, "keel")
	require.NoError(t, os.MkdirAll(keelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keelDir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644))

	// Local source (not a remote URL)
	// Note: In practice, applyKeelDirFallback is only called when originalURL != "",
	// which means remote sources. For local sources, Parse() classifies them as
	// SourceLocal, and they would fall through to the keel/ check. This test
	// verifies the fallback logic works correctly even with local paths.
	result := applyKeelDirFallback(dir, "/local/path")
	// Since remote.Parse() sees this as a local path (not a Git/archive URL),
	// it returns SourceLocal with no error, so the fallback proceeds.
	assert.Equal(t, keelDir, result, "local paths go through fallback logic if Parse succeeds")
}

func TestApplyKeelDirFallback_ParseError(t *testing.T) {
	dir := t.TempDir()
	keelDir := filepath.Join(dir, "keel")
	require.NoError(t, os.MkdirAll(keelDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keelDir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644))

	// Malformed source that Parse() would reject
	result := applyKeelDirFallback(dir, "ext::malicious")
	assert.Equal(t, dir, result, "should return root unchanged when parse fails")
}

func TestIsRecognizedLayer_KeelDSL(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("schema: v1\n"), 0644))

	format, _ := DetectLayerFormat(dir)
	assert.True(t, isRecognizedLayer(dir, format), "should recognize Keel DSL layer with keel.yml")
}

func TestIsRecognizedLayer_ClaudeNative(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))

	format, _ := DetectLayerFormat(dir)
	assert.True(t, isRecognizedLayer(dir, format), "should recognize Claude native layer")
}

func TestIsRecognizedLayer_CursorNative(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CURSOR.md"), []byte("# Cursor"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".cursor"), 0755))

	format, _ := DetectLayerFormat(dir)
	assert.True(t, isRecognizedLayer(dir, format), "should recognize Cursor native layer")
}

func TestIsRecognizedLayer_NoMarkers(t *testing.T) {
	dir := t.TempDir()
	// Empty directory — no markers

	format, _ := DetectLayerFormat(dir)
	assert.False(t, isRecognizedLayer(dir, format), "should NOT recognize directory without markers")
}

func TestIsRecognizedLayer_KeelDirButNoKeelYml(t *testing.T) {
	dir := t.TempDir()
	// Directory named "keel" but no keel.yml inside
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Not a layer"), 0644))

	format, _ := DetectLayerFormat(dir)
	assert.False(t, isRecognizedLayer(dir, format), "should NOT recognize directory without keel.yml")
}
