package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// deepMergeJSON merges src JSON file into dst JSON file on disk.
// Used by unit tests to validate core merge mechanics independent of
// force-lock and _keel policy logic. Production code uses DeepMergeMaps.
func deepMergeJSON(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil // Source doesn't exist, nothing to merge
	}

	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading source JSON %q: %w", src, err)
	}

	var srcMap map[string]any
	if err := json.Unmarshal(srcData, &srcMap); err != nil {
		return fmt.Errorf("parsing source JSON %q: %w", src, err)
	}

	// If destination exists, merge into it; otherwise start fresh
	dstMap := make(map[string]any)
	if dstData, err := os.ReadFile(dst); err == nil {
		if err := json.Unmarshal(dstData, &dstMap); err != nil {
			return fmt.Errorf("parsing destination JSON %q: %w", dst, err)
		}
	}

	merged := DeepMergeMapsWithPolicies(dstMap, srcMap, nil)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", dst, err)
	}

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling merged JSON: %w", err)
	}

	if err := os.WriteFile(dst, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing merged JSON %q: %w", dst, err)
	}

	return nil
}
