package globaldeps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest tracks globally installed deps and their consumers.
type Manifest struct {
	Deps map[string]Dep `json:"deps"`
}

// Dep is a single globally installed dependency.
type Dep struct {
	InstalledAt string   `json:"installed_at"`
	Consumers   []string `json:"consumers"`        // absolute paths of projects using this dep
	Method      string   `json:"method,omitempty"` // "install" or "download"
	Source      string   `json:"source,omitempty"` // install command or download URL
}

// ManifestPath returns the default path to the global manifest file.
func ManifestPath() string {
	return filepath.Join(BinDir(), ".manifest.json")
}

// LoadManifest reads the manifest from the given path.
// Returns an empty manifest if the file doesn't exist.
// Creates the parent directory if needed.
func LoadManifest(path string) (*Manifest, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating manifest directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Deps: make(map[string]Dep)}, nil
		}
		return nil, fmt.Errorf("reading global manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing global manifest: %w", err)
	}
	if m.Deps == nil {
		m.Deps = make(map[string]Dep)
	}
	return &m, nil
}

// SaveManifest writes the manifest atomically to the given path.
// Uses file locking to prevent concurrent write corruption.
func SaveManifest(m *Manifest, path string) error {
	lockFile, err := lockManifest(path)
	if err != nil {
		return err
	}
	defer unlockManifest(lockFile)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling global manifest: %w", err)
	}
	data = append(data, '\n')

	// Atomic write: temp file in the same directory, then rename.
	tmp, err := os.CreateTemp(dir, ".manifest-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp manifest file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp manifest: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp manifest to %s: %w", path, err)
	}
	return nil
}

// RegisterConsumer adds a consumer path to a dep entry.
// Creates the dep entry if it doesn't exist.
// Idempotent -- adding the same consumer twice is a no-op.
func (m *Manifest) RegisterConsumer(depName, consumerPath string, method, source string) {
	dep, exists := m.Deps[depName]
	if !exists {
		dep = Dep{
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
			Method:      method,
			Source:      source,
		}
	}

	// Check for duplicate consumer.
	for _, c := range dep.Consumers {
		if c == consumerPath {
			m.Deps[depName] = dep
			return
		}
	}

	dep.Consumers = append(dep.Consumers, consumerPath)
	m.Deps[depName] = dep
}

// DeregisterConsumer removes a consumer path from a dep entry.
// Returns true if the dep has zero consumers remaining (orphaned).
// Returns false if the consumer was not found or dep still has consumers.
func (m *Manifest) DeregisterConsumer(depName, consumerPath string) bool {
	dep, exists := m.Deps[depName]
	if !exists {
		return false
	}

	idx := -1
	for i, c := range dep.Consumers {
		if c == consumerPath {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}

	// Remove the consumer while preserving order.
	dep.Consumers = append(dep.Consumers[:idx], dep.Consumers[idx+1:]...)
	m.Deps[depName] = dep
	return len(dep.Consumers) == 0
}

// OrphanedDeps returns the names of deps with zero consumers.
func (m *Manifest) OrphanedDeps() []string {
	var orphans []string
	for name, dep := range m.Deps {
		if len(dep.Consumers) == 0 {
			orphans = append(orphans, name)
		}
	}
	return orphans
}

// CleanDeadConsumers removes consumer paths that no longer exist on disk.
// Returns the list of dep names that became orphaned as a result.
func (m *Manifest) CleanDeadConsumers() []string {
	var newlyOrphaned []string
	for name, dep := range m.Deps {
		hadConsumers := len(dep.Consumers) > 0
		var alive []string
		for _, c := range dep.Consumers {
			if _, err := os.Stat(c); err == nil {
				alive = append(alive, c)
			}
		}
		dep.Consumers = alive
		m.Deps[name] = dep
		if hadConsumers && len(alive) == 0 {
			newlyOrphaned = append(newlyOrphaned, name)
		}
	}
	return newlyOrphaned
}
