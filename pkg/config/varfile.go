package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadVarFile parses a KEY=VALUE file into a map.
// Blank lines and lines starting with # are skipped.
// The value is everything after the first = sign, preserving any additional
// = characters in the value. Leading/trailing whitespace on keys is trimmed.
func LoadVarFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening var file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip blank lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			return nil, fmt.Errorf("%s:%d: invalid line (no '=' found): %s", path, lineNum, trimmed)
		}

		key := strings.TrimSpace(trimmed[:idx])
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNum)
		}

		value := trimmed[idx+1:]
		vars[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading var file %q: %w", path, err)
	}

	return vars, nil
}
