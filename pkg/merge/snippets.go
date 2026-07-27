package merge

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// IncludeMarkerRe matches <!-- keel:include name [key="value" ...] --> markers.
// The name may contain letters, digits, hyphens, and underscores.
// Optional key="value" parameters follow the name.
var IncludeMarkerRe = regexp.MustCompile(`^(\s*)<!--\s*keel:include\s+([a-zA-Z0-9_-]+)((?:\s+[a-zA-Z0-9_-]+="[^"]*")*)\s*-->(.*)$`)

// ParamRe extracts key="value" pairs from the parameter string.
var ParamRe = regexp.MustCompile(`([a-zA-Z0-9_-]+)="([^"]*)"`)

// placeholderRe matches {{key}} placeholders in snippet content.
var placeholderRe = regexp.MustCompile(`\{\{([a-zA-Z0-9_-]+)\}\}`)

// ResolveSnippets replaces <!-- keel:include name --> markers with snippet content.
// Markers inside fenced code blocks (``` ... ```) are treated as literal text.
// Returns an error if a referenced snippet is not found in the registry.
// If the marker line has leading whitespace, that whitespace is prepended to
// each line of the snippet content.
func ResolveSnippets(content []byte, snippets map[string][]byte) ([]byte, error) {
	if len(snippets) == 0 {
		// Fast path: scan for any marker to determine if there's work to do.
		// If no markers exist, return content unchanged.
		if !bytes.Contains(content, []byte("keel:include")) {
			return content, nil
		}
	}

	lines := bytes.Split(content, []byte("\n"))
	var out bytes.Buffer
	inCodeBlock := false

	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)

		// Track fenced code blocks — toggle on ``` at start of trimmed line
		if bytes.HasPrefix(trimmed, []byte("```")) {
			inCodeBlock = !inCodeBlock
		}

		// Skip marker detection inside code blocks
		if inCodeBlock {
			out.Write(line)
			if i < len(lines)-1 {
				out.WriteByte('\n')
			}
			continue
		}

		// Check for include marker
		matches := IncludeMarkerRe.FindSubmatch(line)
		if matches == nil {
			out.Write(line)
			if i < len(lines)-1 {
				out.WriteByte('\n')
			}
			continue
		}

		indent := string(matches[1])
		name := string(matches[2])
		paramStr := string(matches[3])

		snippetContent, ok := snippets[name]
		if !ok {
			return nil, fmt.Errorf("snippet %q not found", name)
		}

		// Trim trailing newline from snippet content to avoid double-spacing
		snippetContent = bytes.TrimRight(snippetContent, "\n")

		// Apply parameterized substitution: replace {{key}} with values
		if paramStr != "" {
			params := parseParams(paramStr)
			snippetContent = applyParams(snippetContent, params)
		}

		if len(snippetContent) == 0 {
			// Empty snippet: marker line becomes an empty line
			if i < len(lines)-1 {
				out.WriteByte('\n')
			}
			continue
		}

		// Apply indentation to each line of the snippet content
		snippetLines := strings.Split(string(snippetContent), "\n")
		for j, sl := range snippetLines {
			if j == 0 {
				out.WriteString(indent + sl)
			} else {
				out.WriteByte('\n')
				out.WriteString(indent + sl)
			}
		}
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}

	return out.Bytes(), nil
}

// parseParams extracts key="value" pairs from a parameter string.
func parseParams(paramStr string) map[string]string {
	params := make(map[string]string)
	for _, match := range ParamRe.FindAllStringSubmatch(paramStr, -1) {
		params[match[1]] = match[2]
	}
	return params
}

// applyParams replaces {{key}} placeholders in content with parameter values.
// Unreferenced placeholders are left as-is (they may be intentional template syntax).
func applyParams(content []byte, params map[string]string) []byte {
	return placeholderRe.ReplaceAllFunc(content, func(match []byte) []byte {
		key := string(placeholderRe.FindSubmatch(match)[1])
		if val, ok := params[key]; ok {
			return []byte(val)
		}
		return match // leave unmatched placeholders as-is
	})
}

// FindUnresolvedPlaceholders returns the names of all {{key}} placeholders in
// content whose key is NOT present in the params map. This is used by
// validation to detect snippet parameters that are never supplied.
func FindUnresolvedPlaceholders(content []byte, params map[string]string) []string {
	seen := make(map[string]bool)
	var unresolved []string
	for _, match := range placeholderRe.FindAllSubmatch(content, -1) {
		key := string(match[1])
		if _, ok := params[key]; !ok && !seen[key] {
			seen[key] = true
			unresolved = append(unresolved, key)
		}
	}
	return unresolved
}
