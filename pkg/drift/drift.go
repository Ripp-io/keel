// Package drift provides drift detection for compiled Keel output.
// It compares disk state against compiled manifests to detect manual edits.
package drift

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
)

// Result holds the results of comparing disk state against a compiled manifest.
type Result struct {
	Agent      string
	Modified   []Record                    // files whose hash changed
	Missing    []Record                    // files in manifest but not on disk
	Semantic   *manifest.EffectiveStateDiff // nil when --semantic not used or no effective_state in manifest
	Signatures *signing.SignatureDrift     `json:"signatures,omitempty"` // nil when --verify not used
}

// Record represents a single file that has drifted from its compiled state.
type Record struct {
	Path     string // output-relative path
	Expected string // hash from manifest (empty for overlay-only files)
	Actual   string // hash from disk (empty for missing files)
}

// HasDrift returns true if any files have been modified or are missing.
func (r *Result) HasDrift() bool {
	return len(r.Modified) > 0 || len(r.Missing) > 0
}

// Detect compares the current disk state against a compiled manifest.
// It checks all files tracked in file_ops by comparing content hashes.
func Detect(outputDir string, m *manifest.CompiledManifest) (*Result, error) {
	result := &Result{Agent: m.Agent}

	// Check file_ops entries by hash.
	for _, f := range m.FileOps {
		// Skip directory entries (backwards compat for old manifests).
		if strings.HasSuffix(f.Output, "/") {
			continue
		}

		path := filepath.Join(outputDir, f.Output)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				result.Missing = append(result.Missing, Record{
					Path:     f.Output,
					Expected: f.Hash,
				})
				continue
			}
			return nil, err
		}

		// Skip files without hashes (shouldn't exist after this change, but
		// backwards compat for old manifests).
		if f.Hash == "" {
			continue
		}

		diskHash := manifest.HashContent(data)
		if diskHash != f.Hash {
			result.Modified = append(result.Modified, Record{
				Path:     f.Output,
				Expected: f.Hash,
				Actual:   diskHash,
			})
		}
	}

	return result, nil
}

// StateReader reads the current output state and returns an EffectiveState.
// Used for dependency injection in DetectSemantic.
type StateReader func(outputDir string) (*manifest.EffectiveState, error)

// DetectSemantic reads the current output through a StateReader,
// and diffs it against the manifest's captured state.
// Returns nil if no effective_state in manifest.
func DetectSemantic(outputDir string, m *manifest.CompiledManifest, readState StateReader) (*manifest.EffectiveStateDiff, error) {
	if m.EffectiveState == nil {
		return nil, nil
	}

	// Read current disk state
	current, err := readState(outputDir)
	if err != nil {
		return nil, fmt.Errorf("reading current state: %w", err)
	}

	// Diff against compiled state
	return manifest.DiffEffectiveState(m.EffectiveState, current), nil
}
