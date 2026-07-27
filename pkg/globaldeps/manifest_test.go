package globaldeps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func manifestTestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".manifest.json")
}

func TestLoadManifest_NonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", ".manifest.json")

	m, err := LoadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Empty(t, m.Deps)

	// The parent directory should have been created.
	_, err = os.Stat(filepath.Dir(path))
	assert.NoError(t, err, "expected directory to be created")
}

func TestSaveAndReload(t *testing.T) {
	path := manifestTestPath(t)

	m := &Manifest{Deps: map[string]Dep{
		"ripgrep": {
			InstalledAt: "2026-05-12T10:00:00Z",
			Consumers:   []string{"/home/user/project-a"},
			Method:      "download",
			Source:      "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-x86_64-unknown-linux-musl.tar.gz",
		},
	}}

	require.NoError(t, SaveManifest(m, path))

	loaded, err := LoadManifest(path)
	require.NoError(t, err)

	dep, ok := loaded.Deps["ripgrep"]
	require.True(t, ok, "expected ripgrep dep to exist")
	assert.Equal(t, "2026-05-12T10:00:00Z", dep.InstalledAt)
	assert.Equal(t, []string{"/home/user/project-a"}, dep.Consumers)
	assert.Equal(t, "download", dep.Method)
	assert.Equal(t, m.Deps["ripgrep"].Source, dep.Source)
}

func TestRegisterConsumer_CreatesNewDep(t *testing.T) {
	m := &Manifest{Deps: make(map[string]Dep)}

	m.RegisterConsumer("fd", "/projects/alpha", "install", "cargo install fd-find")

	dep, ok := m.Deps["fd"]
	require.True(t, ok, "expected fd dep to exist after register")
	assert.Equal(t, []string{"/projects/alpha"}, dep.Consumers)
	assert.Equal(t, "install", dep.Method)
	assert.Equal(t, "cargo install fd-find", dep.Source)
	assert.NotEmpty(t, dep.InstalledAt)
}

func TestRegisterConsumer_Idempotent(t *testing.T) {
	m := &Manifest{Deps: make(map[string]Dep)}

	m.RegisterConsumer("fd", "/projects/alpha", "install", "cargo install fd-find")
	m.RegisterConsumer("fd", "/projects/alpha", "install", "cargo install fd-find")
	m.RegisterConsumer("fd", "/projects/alpha", "install", "cargo install fd-find")

	dep := m.Deps["fd"]
	assert.Len(t, dep.Consumers, 1, "expected 1 consumer after idempotent adds")
}

func TestRegisterConsumer_MultipleConsumers(t *testing.T) {
	m := &Manifest{Deps: make(map[string]Dep)}

	m.RegisterConsumer("ripgrep", "/projects/alpha", "download", "https://example.com/rg")
	m.RegisterConsumer("ripgrep", "/projects/beta", "download", "https://example.com/rg")
	m.RegisterConsumer("ripgrep", "/projects/gamma", "download", "https://example.com/rg")

	dep := m.Deps["ripgrep"]
	assert.Len(t, dep.Consumers, 3)
}

func TestDeregisterConsumer_RemovesConsumer(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{"/projects/alpha", "/projects/beta", "/projects/gamma"},
		},
	}}

	orphaned := m.DeregisterConsumer("rg", "/projects/beta")
	assert.False(t, orphaned, "expected not orphaned, others remain")

	dep := m.Deps["rg"]
	assert.Len(t, dep.Consumers, 2)
	assert.NotContains(t, dep.Consumers, "/projects/beta")
}

func TestDeregisterConsumer_LastConsumer(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{"/projects/alpha"},
		},
	}}

	orphaned := m.DeregisterConsumer("rg", "/projects/alpha")
	assert.True(t, orphaned, "expected orphaned after removing last consumer")

	dep := m.Deps["rg"]
	assert.Empty(t, dep.Consumers)
}

func TestDeregisterConsumer_UnknownDep(t *testing.T) {
	m := &Manifest{Deps: make(map[string]Dep)}

	orphaned := m.DeregisterConsumer("nonexistent", "/projects/alpha")
	assert.False(t, orphaned, "expected false for unknown dep")
}

func TestDeregisterConsumer_UnknownConsumer(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{"/projects/alpha"},
		},
	}}

	orphaned := m.DeregisterConsumer("rg", "/projects/unknown")
	assert.False(t, orphaned, "expected false for unknown consumer")

	dep := m.Deps["rg"]
	assert.Len(t, dep.Consumers, 1, "expected 1 consumer unchanged")
}

func TestOrphanedDeps(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{"/projects/alpha"},
		},
		"fd": {
			Consumers: []string{},
		},
		"bat": {
			Consumers: nil,
		},
		"jq": {
			Consumers: []string{"/projects/beta", "/projects/gamma"},
		},
	}}

	orphans := m.OrphanedDeps()
	sort.Strings(orphans)

	require.Len(t, orphans, 2)
	assert.Equal(t, []string{"bat", "fd"}, orphans)
}

func TestCleanDeadConsumers_RemovesNonExistent(t *testing.T) {
	// Create a real directory to act as a surviving consumer.
	aliveDir := t.TempDir()

	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{
				aliveDir,
				"/definitely/does/not/exist/abc123",
				"/also/does/not/exist/xyz789",
			},
		},
	}}

	orphaned := m.CleanDeadConsumers()
	assert.Empty(t, orphaned, "expected no newly orphaned (alive dir still exists)")

	dep := m.Deps["rg"]
	assert.Equal(t, []string{aliveDir}, dep.Consumers)
}

func TestCleanDeadConsumers_ReturnsNewlyOrphaned(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{"/does/not/exist/a", "/does/not/exist/b"},
		},
		"fd": {
			Consumers: []string{"/does/not/exist/c"},
		},
	}}

	orphaned := m.CleanDeadConsumers()
	sort.Strings(orphaned)

	require.Len(t, orphaned, 2)
	assert.Equal(t, []string{"fd", "rg"}, orphaned)
}

func TestCleanDeadConsumers_SkipsAlreadyOrphaned(t *testing.T) {
	// A dep that already had zero consumers should NOT appear in the result.
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{},
		},
	}}

	orphaned := m.CleanDeadConsumers()
	assert.Empty(t, orphaned, "expected 0 newly orphaned (was already orphaned)")
}

func TestConcurrentAccess(t *testing.T) {
	path := manifestTestPath(t)

	// Seed with an empty manifest.
	m := &Manifest{Deps: make(map[string]Dep)}
	require.NoError(t, SaveManifest(m, path))

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			loaded, err := LoadManifest(path)
			if err != nil {
				errors <- err
				return
			}
			consumer := filepath.Join("/projects", "project-"+string(rune('A'+id)))
			loaded.RegisterConsumer("rg", consumer, "download", "https://example.com/rg")
			if err := SaveManifest(loaded, path); err != nil {
				errors <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		assert.NoError(t, err, "goroutine error")
	}

	// The manifest should be valid JSON and loadable.
	final, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Contains(t, final.Deps, "rg", "expected rg dep to exist in final manifest")

	// Due to races, not all goroutines may have their consumer recorded,
	// but the file must not be corrupt.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var check Manifest
	require.NoError(t, json.Unmarshal(data, &check), "manifest is corrupt JSON after concurrent access: %s", string(data))
}

func TestSaveManifest_AtomicWrite(t *testing.T) {
	// Verify that a partial write doesn't leave a corrupt file.
	path := manifestTestPath(t)

	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			InstalledAt: "2026-05-12T10:00:00Z",
			Consumers:   []string{"/projects/alpha"},
			Method:      "download",
			Source:      "https://example.com/rg",
		},
	}}

	require.NoError(t, SaveManifest(m, path))

	// No temp files should remain in the directory.
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Name() != ".manifest.json" && e.Name() != ".manifest.json.lock" {
			assert.Failf(t, "unexpected file left behind", "file: %s", e.Name())
		}
	}
}

func TestDeregisterConsumer_PreservesDepEntry(t *testing.T) {
	// After deregistering the last consumer, the dep entry itself should remain
	// in the manifest so the caller can decide whether to remove the binary.
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			InstalledAt: "2026-05-12T10:00:00Z",
			Consumers:   []string{"/projects/alpha"},
			Method:      "download",
			Source:      "https://example.com/rg",
		},
	}}

	m.DeregisterConsumer("rg", "/projects/alpha")

	dep, ok := m.Deps["rg"]
	require.True(t, ok, "dep entry should still exist after deregistering last consumer")
	assert.Equal(t, "2026-05-12T10:00:00Z", dep.InstalledAt, "dep metadata should be preserved")
	assert.Equal(t, "download", dep.Method, "dep method should be preserved")
}

func TestRegisterConsumer_ExistingDepPreservesMetadata(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			InstalledAt: "2026-01-01T00:00:00Z",
			Consumers:   []string{"/projects/alpha"},
			Method:      "download",
			Source:      "https://example.com/rg",
		},
	}}

	// Register a second consumer -- the original InstalledAt, Method, Source
	// should be preserved.
	m.RegisterConsumer("rg", "/projects/beta", "install", "different-source")

	dep := m.Deps["rg"]
	assert.Equal(t, "2026-01-01T00:00:00Z", dep.InstalledAt, "InstalledAt changed from original")
	assert.Equal(t, "download", dep.Method, "Method changed from original")
	assert.Equal(t, "https://example.com/rg", dep.Source, "Source changed from original")
	assert.Len(t, dep.Consumers, 2)
}

func TestDeregisterConsumer_OrderPreservation(t *testing.T) {
	m := &Manifest{Deps: map[string]Dep{
		"rg": {
			Consumers: []string{"/a", "/b", "/c", "/d"},
		},
	}}

	m.DeregisterConsumer("rg", "/b")

	dep := m.Deps["rg"]
	assert.Equal(t, []string{"/a", "/c", "/d"}, dep.Consumers)
}

func TestLoadManifest_CorruptJSON(t *testing.T) {
	path := manifestTestPath(t)
	require.NoError(t, os.WriteFile(path, []byte(`{corrupt json!!!}`), 0644))

	m, err := LoadManifest(path)
	require.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "parsing global manifest")
}

func TestLoadManifest_NonReadableDir(t *testing.T) {
	// Create a regular file, then try to load a manifest from a path
	// under that file. MkdirAll will fail because the parent is a file.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("I am a file"), 0644))

	path := filepath.Join(blocker, "subdir", "manifest.json")
	m, err := LoadManifest(path)
	require.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "creating manifest directory")
}

func TestLoadManifest_ReadError(t *testing.T) {
	// Create the manifest path as a directory so ReadFile returns an error
	// that is not NotExist (EISDIR on Unix).
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.MkdirAll(path, 0755))

	m, err := LoadManifest(path)
	require.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "reading global manifest")
}

func TestSaveManifest_RoundTrip(t *testing.T) {
	path := manifestTestPath(t)

	original := &Manifest{Deps: map[string]Dep{
		"fd": {
			InstalledAt: "2026-05-14T12:00:00Z",
			Method:      "install",
			Source:      "cargo install fd-find",
			Consumers:   []string{"/projects/alpha", "/projects/beta"},
		},
	}}

	require.NoError(t, SaveManifest(original, path))

	loaded, err := LoadManifest(path)
	require.NoError(t, err)

	assert.Equal(t, original.Deps["fd"].InstalledAt, loaded.Deps["fd"].InstalledAt)
	assert.Equal(t, original.Deps["fd"].Method, loaded.Deps["fd"].Method)
	assert.Equal(t, original.Deps["fd"].Source, loaded.Deps["fd"].Source)
	assert.Equal(t, original.Deps["fd"].Consumers, loaded.Deps["fd"].Consumers)
}
