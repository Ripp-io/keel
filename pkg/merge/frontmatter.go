package merge

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
	"gopkg.in/yaml.v3"
)

// keelFields is the set of top-level YAML keys that are Keel policy fields.
// These are stripped from frontmatter during compilation; all other keys
// are tool-native and preserved.
var keelFields = map[string]bool{
	"force": true,
	"scope": true,
	"merge": true,
}

// hasKeelFields returns true if any Keel policy field is present in the policy.
func hasKeelFields(p *policy.KeelPolicy) bool {
	return p.Force != nil || p.Scope != "" || p.Merge != ""
}

// extractFrontmatterParts splits a file into its raw YAML frontmatter bytes
// and the body after the closing delimiter. Returns nil rawYAML if no
// frontmatter is found. The input is normalized to \n line endings internally.
func extractFrontmatterParts(data []byte) (rawYAML []byte, body []byte, normalized []byte, found bool) {
	if len(data) == 0 {
		return nil, nil, data, false
	}

	normalized = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))

	// Frontmatter must start with --- as the very first line
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return nil, nil, normalized, false
	}

	// Find the closing --- delimiter (search after the opening one)
	rest := normalized[4:] // skip opening "---\n"
	closingIdx := bytes.Index(rest, []byte("\n---\n"))
	if closingIdx == -1 {
		// Check if it ends with \n--- at EOF (no trailing newline after closer)
		if bytes.HasSuffix(rest, []byte("\n---")) {
			closingIdx = len(rest) - 4 // position of \n before ---
		} else {
			return nil, nil, normalized, false
		}
	}

	rawYAML = rest[:closingIdx]
	// Body starts after the closing "---\n" (5 bytes: \n---\n).
	// When frontmatter ends at EOF (no trailing newline), bodyStart may
	// exceed len(rest). The guard below clamps it, yielding an empty body.
	bodyStart := closingIdx + 5
	if bodyStart > len(rest) {
		bodyStart = len(rest)
	}
	body = rest[bodyStart:]

	return rawYAML, body, normalized, true
}

// ExtractRawFrontmatter parses YAML frontmatter into a generic map without
// stripping any fields. Returns a nil map if no frontmatter is found.
// This is useful for inspecting all frontmatter fields (e.g., Agent Skills
// validation) without Keel-specific field stripping.
func ExtractRawFrontmatter(data []byte) (map[string]any, error) {
	rawYAML, _, _, found := extractFrontmatterParts(data)
	if !found {
		return nil, nil
	}

	var rawMap map[string]any
	if err := yaml.Unmarshal(rawYAML, &rawMap); err != nil {
		return nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	return rawMap, nil
}

// ParseAndCleanFrontmatter extracts Keel policy from YAML frontmatter and
// returns cleaned content with Keel fields removed but tool fields preserved.
//
// Frontmatter is delimited by --- on its own line at the very start of the file:
//
//	---
//	force: true
//	name: code-reviewer
//	---
//	Content body here.
//
// Returns:
//   - *policy.KeelPolicy: the extracted Keel policy (nil if no Keel fields found)
//   - []byte: the content with Keel fields stripped from frontmatter
//   - error: if YAML parsing fails
//
// If no frontmatter is found, returns nil and the original data unchanged.
// If frontmatter is found but contains no Keel fields, returns nil and the
// original data unchanged.
// If frontmatter contains ONLY Keel fields, the entire frontmatter block is removed.
// If frontmatter contains both Keel and tool fields, only Keel fields are removed.
func ParseAndCleanFrontmatter(data []byte) (*policy.KeelPolicy, []byte, error) {
	rawYAML, body, _, found := extractFrontmatterParts(data)
	if !found {
		return nil, data, nil
	}

	// Parse the YAML into a generic map to inspect all fields
	var rawMap map[string]any
	if err := yaml.Unmarshal(rawYAML, &rawMap); err != nil {
		return nil, nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
	}

	// Handle empty frontmatter block (nil map from YAML)
	if rawMap == nil {
		return nil, data, nil
	}

	// Extract Keel fields into the KeelPolicy struct
	fm := &policy.KeelPolicy{}
	if err := yaml.Unmarshal(rawYAML, fm); err != nil {
		return nil, nil, fmt.Errorf("parsing Keel frontmatter fields: %w", err)
	}

	// Check if any Keel fields are actually present
	if !hasKeelFields(fm) {
		return nil, data, nil
	}

	// Count Keel vs non-Keel fields
	keelCount := 0
	nonKeelCount := 0
	for key := range rawMap {
		if keelFields[key] {
			keelCount++
		} else {
			nonKeelCount++
		}
	}

	// If all fields are Keel fields, strip the entire frontmatter block
	if nonKeelCount == 0 {
		// Use original line endings if Windows
		if bytes.Contains(data, []byte("\r\n")) {
			return fm, bytes.ReplaceAll(body, []byte("\n"), []byte("\r\n")), nil
		}
		return fm, body, nil
	}

	// Mixed: remove only Keel fields, keep tool fields
	for key := range keelFields {
		delete(rawMap, key)
	}

	// Re-marshal the remaining fields.
	// Note: YAML re-marshaling does not preserve original field ordering.
	// This is acceptable for the current use case (frontmatter key-value pairs)
	// but should be considered if ordering becomes significant.
	cleanedYAML, err := yaml.Marshal(rawMap)
	if err != nil {
		return nil, nil, fmt.Errorf("re-marshaling cleaned frontmatter: %w", err)
	}

	// Rebuild the content: frontmatter + body
	var buf bytes.Buffer
	buf.WriteString("---\n")
	// yaml.Marshal adds a trailing newline; trim it and add our own for consistency
	buf.Write(bytes.TrimRight(cleanedYAML, "\n"))
	buf.WriteString("\n---\n")
	buf.Write(body)

	result := buf.Bytes()

	// Restore original line endings if Windows.
	// Note: line ending restoration is lossy for files with mixed \n and \r\n endings.
	// All line endings are normalized to \r\n if the original contained any \r\n sequences.
	if bytes.Contains(data, []byte("\r\n")) {
		result = bytes.ReplaceAll(result, []byte("\n"), []byte("\r\n"))
	}

	return fm, result, nil
}

// StripFrontmatter removes Keel policy fields from YAML frontmatter, returning
// content with tool-native frontmatter preserved.
func StripFrontmatter(data []byte) []byte {
	_, body, err := ParseAndCleanFrontmatter(data)
	if err != nil {
		// On parse error, return original data unchanged
		return data
	}
	return body
}

// ReplaceFrontmatter replaces the YAML frontmatter in data with newFM,
// preserving the body after the frontmatter delimiter. If no frontmatter
// exists in data, the new frontmatter is prepended.
// Returns the modified content and whether the operation succeeded.
func ReplaceFrontmatter(data []byte, newFM map[string]any) ([]byte, bool) {
	_, body, _, found := extractFrontmatterParts(data)
	if !found {
		// No existing frontmatter — prepend new frontmatter.
		body = data
	}

	newYAML, err := yaml.Marshal(newFM)
	if err != nil {
		return data, false
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(bytes.TrimRight(newYAML, "\n"))
	buf.WriteString("\n---\n")
	buf.Write(body)

	result := buf.Bytes()

	// Restore original line endings if the input used Windows line endings.
	if bytes.Contains(data, []byte("\r\n")) {
		result = bytes.ReplaceAll(result, []byte("\n"), []byte("\r\n"))
	}

	return result, true
}

// IsMarkdownFile returns true if the file extension indicates a markdown-like
// file that may contain frontmatter (.md, .mdc).
func IsMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdc"
}
