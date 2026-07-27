package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// loadCompiledConfig auto-discovers and loads .keel.json from dir.
// It checks dir itself, then scans known agent subdirs.
// Returns exactly one manifest or an error.
func loadCompiledConfig(dir string) (*manifest.CompiledManifest, error) {
	manifests, _, err := compiler.FindKeelState(dir)
	if err != nil {
		return nil, fmt.Errorf("%w (run 'keel compile' first)", err)
	}
	if len(manifests) == 0 {
		return nil, fmt.Errorf("no compiled config found in %q (run 'keel compile' first)", dir)
	}
	if len(manifests) == 1 {
		return manifests[0], nil
	}
	// Multiple found — use first, warn the user
	slog.Warn("multiple compiled configs found, using first; use --agent to specify",
		"count", len(manifests))
	return manifests[0], nil
}

// validateArchiveFormat returns an error if format is not a supported archive type.
func validateArchiveFormat(format string) error {
	switch strings.ToLower(format) {
	case "tar.gz", "tgz", "zip":
		return nil
	default:
		return fmt.Errorf("invalid archive format %q: must be \"tar.gz\", \"tgz\", or \"zip\"", format)
	}
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, string(data))
	return nil
}
