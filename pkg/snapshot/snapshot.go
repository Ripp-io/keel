package snapshot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

// Dir is the directory within .keel/ where snapshots are stored.
const Dir = "snapshots"

// Snapshot represents a saved compilation state.
type Snapshot struct {
	ID        string    // timestamp-based ID (e.g., "20260518T143022Z")
	Agent     string    // agent name from manifest
	CreatedAt time.Time // when the snapshot was taken
	Dir       string    // absolute path to the snapshot directory
}

// snapshotMetadata is what gets written to snapshot.json.
type snapshotMetadata struct {
	ID          string `json:"id"`
	Agent       string `json:"agent"`
	CreatedAt   string `json:"created_at"`
	KeelVersion string `json:"keel_version"`
}

// Create captures the current compiled state for an agent.
// It copies the manifest and all files tracked in it to .keel/snapshots/<id>/.
// The id is a UTC timestamp: 20060102T150405Z.
func Create(outputDir, agentConfigDir string) (*Snapshot, error) {
	// Load the manifest
	mf, err := manifest.LoadKeelState(agentConfigDir)
	if err != nil {
		return nil, fmt.Errorf("loading manifest: %w", err)
	}
	if mf == nil {
		return nil, fmt.Errorf("no .keel.json found in %s", agentConfigDir)
	}

	// Generate snapshot ID with collision handling.
	// Uses incrementing suffix for collisions (rapid successive snapshots).
	baseID := time.Now().UTC().Format("20060102T150405Z")
	id := baseID
	snapshotRoot := filepath.Join(outputDir, ".keel", Dir, id)

	for suffix := 1; ; suffix++ {
		if _, statErr := os.Stat(snapshotRoot); os.IsNotExist(statErr) {
			break
		}
		id = fmt.Sprintf("%s-%d", baseID, suffix)
		snapshotRoot = filepath.Join(outputDir, ".keel", Dir, id)
	}

	// Create snapshot directory
	if err := os.MkdirAll(snapshotRoot, 0755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	// Copy all files from FileOps
	for _, f := range mf.FileOps {
		src := filepath.Join(outputDir, f.Output)
		dst := filepath.Join(snapshotRoot, f.Output)

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", f.Output, err)
		}

		// Copy file
		if err := pathutil.CopyFile(src, dst); err != nil {
			// Skip missing files (they may have been manually deleted)
			if os.IsNotExist(err) {
				slog.Debug("skipping missing file during snapshot", "path", f.Output)
				continue
			}
			return nil, fmt.Errorf("copying %s: %w", f.Output, err)
		}
	}

	// Copy all overlay files
	for overlayPath := range mf.Overlays {
		src := filepath.Join(outputDir, overlayPath)
		dst := filepath.Join(snapshotRoot, overlayPath)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return nil, fmt.Errorf("creating directory for overlay %s: %w", overlayPath, err)
		}

		if err := pathutil.CopyFile(src, dst); err != nil {
			if os.IsNotExist(err) {
				slog.Debug("skipping missing overlay file during snapshot", "path", overlayPath)
				continue
			}
			return nil, fmt.Errorf("copying overlay %s: %w", overlayPath, err)
		}
	}

	// Copy .keel.json
	manifestSrc := filepath.Join(agentConfigDir, ".keel.json")
	manifestDst := filepath.Join(snapshotRoot, filepath.Base(agentConfigDir), ".keel.json")
	if err := os.MkdirAll(filepath.Dir(manifestDst), 0755); err != nil {
		return nil, fmt.Errorf("creating manifest directory: %w", err)
	}
	if err := pathutil.CopyFile(manifestSrc, manifestDst); err != nil {
		return nil, fmt.Errorf("copying manifest: %w", err)
	}

	// Get Keel version
	version := os.Getenv("KEEL_BUILD")
	if version == "" {
		version = "dev"
	}

	// Write snapshot metadata
	meta := snapshotMetadata{
		ID:          id,
		Agent:       mf.Agent,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		KeelVersion: version,
	}
	metaData, err := json.MarshalIndent(&meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}
	metaData = append(metaData, '\n')

	metaPath := filepath.Join(snapshotRoot, "snapshot.json")
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return nil, fmt.Errorf("writing metadata: %w", err)
	}

	snap := &Snapshot{
		ID:        id,
		Agent:     mf.Agent,
		CreatedAt: time.Now().UTC(),
		Dir:       snapshotRoot,
	}

	slog.Debug("snapshot created", "id", id, "agent", mf.Agent, "dir", snapshotRoot)
	return snap, nil
}

// List returns all snapshots for a given output directory, sorted newest first.
func List(outputDir string) ([]Snapshot, error) {
	snapshotsDir := filepath.Join(outputDir, ".keel", Dir)

	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading snapshots directory: %w", err)
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := filepath.Join(snapshotsDir, entry.Name(), "snapshot.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			slog.Warn("skipping snapshot without metadata", "id", entry.Name(), "error", err)
			continue
		}

		var meta snapshotMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			slog.Warn("skipping snapshot with invalid metadata", "id", entry.Name(), "error", err)
			continue
		}

		createdAt, err := time.Parse(time.RFC3339, meta.CreatedAt)
		if err != nil {
			slog.Warn("skipping snapshot with invalid timestamp", "id", entry.Name(), "error", err)
			continue
		}

		snapshots = append(snapshots, Snapshot{
			ID:        meta.ID,
			Agent:     meta.Agent,
			CreatedAt: createdAt,
			Dir:       filepath.Join(snapshotsDir, entry.Name()),
		})
	}

	// Sort newest first
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// Restore copies a snapshot's files back to the output directory,
// overwriting the current compiled state.
func Restore(snapshotDir, outputDir string) error {
	// Read snapshot metadata
	metaPath := filepath.Join(snapshotDir, "snapshot.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("reading snapshot metadata: %w", err)
	}

	var meta snapshotMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing snapshot metadata: %w", err)
	}

	// Find the .keel.json in the snapshot
	var manifestPath string
	err = filepath.WalkDir(snapshotDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == ".keel.json" {
			manifestPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("searching for manifest: %w", err)
	}
	if manifestPath == "" {
		return fmt.Errorf("no .keel.json found in snapshot")
	}

	// Read the manifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	var mf manifest.CompiledManifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	// Restore all files from FileOps
	for _, f := range mf.FileOps {
		src := filepath.Join(snapshotDir, f.Output)
		dst := filepath.Join(outputDir, f.Output)

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", f.Output, err)
		}

		// Copy file
		if err := pathutil.CopyFile(src, dst); err != nil {
			// Skip missing files in snapshot (may have been skipped during create)
			if os.IsNotExist(err) {
				slog.Debug("skipping missing file during restore", "path", f.Output)
				continue
			}
			return fmt.Errorf("restoring %s: %w", f.Output, err)
		}

		slog.Debug("restored file", "path", f.Output)
	}

	// Restore overlay files
	for overlayPath := range mf.Overlays {
		src := filepath.Join(snapshotDir, overlayPath)
		dst := filepath.Join(outputDir, overlayPath)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("creating directory for overlay %s: %w", overlayPath, err)
		}

		if err := pathutil.CopyFile(src, dst); err != nil {
			if os.IsNotExist(err) {
				slog.Debug("skipping missing overlay file during restore", "path", overlayPath)
				continue
			}
			return fmt.Errorf("restoring overlay %s: %w", overlayPath, err)
		}

		slog.Debug("restored overlay file", "path", overlayPath)
	}

	// Restore .keel.json to agent config dir
	agentConfigDir := filepath.Dir(manifestPath)
	agentConfigBase := filepath.Base(agentConfigDir)
	manifestDst := filepath.Join(outputDir, agentConfigBase, ".keel.json")
	if err := os.MkdirAll(filepath.Dir(manifestDst), 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}
	if err := pathutil.CopyFile(manifestPath, manifestDst); err != nil {
		return fmt.Errorf("restoring manifest: %w", err)
	}

	slog.Info("snapshot restored", "id", meta.ID, "agent", meta.Agent)
	return nil
}

// Prune removes snapshots exceeding the retention limit (keep newest N).
// Returns the list of removed snapshot IDs.
func Prune(outputDir string, keep int) ([]string, error) {
	snapshots, err := List(outputDir)
	if err != nil {
		return nil, err
	}

	if len(snapshots) <= keep {
		return nil, nil
	}

	toRemove := snapshots[keep:]
	var removed []string

	for _, snap := range toRemove {
		if !pathutil.ContainsPath(outputDir, snap.Dir) {
			slog.Warn("skipping snapshot outside output directory", "id", snap.ID, "dir", snap.Dir)
			continue
		}

		if err := os.RemoveAll(snap.Dir); err != nil {
			slog.Warn("failed to remove snapshot", "id", snap.ID, "error", err)
			continue
		}

		removed = append(removed, snap.ID)
		slog.Debug("removed snapshot", "id", snap.ID)
	}

	return removed, nil
}
