package merge

import (
	"log/slog"
	"strings"
)

// SubstituteVariables replaces ${keel:name} patterns in content with values
// from vars. The key in vars corresponds to the name after the "keel:" prefix.
// For example, ${keel:keel.agent} looks up "keel.agent" in vars.
// Unknown variables (not found in vars) are left as-is in the output.
func SubstituteVariables(content []byte, vars map[string]string) []byte {
	if len(vars) == 0 {
		return content
	}
	return []byte(substituteVariablesString(string(content), vars))
}

// substituteVariablesString replaces ${keel:name} patterns in a string with
// values from vars. It is the string-native variant of SubstituteVariables,
// avoiding unnecessary []byte↔string conversions when the caller already
// works with strings.
func substituteVariablesString(content string, vars map[string]string) string {
	if len(vars) == 0 {
		return content
	}

	const prefix = "${keel:"
	const suffix = "}"

	var result strings.Builder
	result.Grow(len(content))

	i := 0
	for i < len(content) {
		idx := strings.Index(content[i:], prefix)
		if idx < 0 {
			result.WriteString(content[i:])
			break
		}

		// Write everything before the marker
		result.WriteString(content[i : i+idx])

		// Find the closing brace
		start := i + idx + len(prefix)
		end := strings.Index(content[start:], suffix)
		if end < 0 {
			// No closing brace — write the prefix literally and continue
			result.WriteString(prefix)
			i = start
			continue
		}

		name := content[start : start+end]
		if val, ok := vars[name]; ok {
			result.WriteString(val)
		} else {
			// Unknown variable — leave as-is
			result.WriteString(prefix)
			result.WriteString(name)
			result.WriteString(suffix)
		}
		i = start + end + len(suffix)
	}

	return result.String()
}

// SubstituteVariablesInMap recursively walks a JSON-like map and applies
// ${keel:name} substitution to all string values. Non-string values and
// map keys are left as-is. Returns a new map (does not mutate input).
func SubstituteVariablesInMap(data map[string]any, vars map[string]string) map[string]any {
	if len(vars) == 0 || len(data) == 0 {
		return data
	}
	return substituteMapAtDepth(data, vars, 0)
}

// maxSubstitutionDepth prevents pathological input from causing stack overflow.
const maxSubstitutionDepth = 128

// substituteMapAtDepth is the recursive implementation of SubstituteVariablesInMap.
func substituteMapAtDepth(data map[string]any, vars map[string]string, depth int) map[string]any {
	if depth >= maxSubstitutionDepth {
		slog.Warn("variable substitution exceeded maximum nesting depth, stopping recursion", "maxDepth", maxSubstitutionDepth)
		return data
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = substituteValueAtDepth(v, vars, depth)
	}
	return out
}

// substituteValueAtDepth applies variable substitution to a single value,
// recursing into maps and slices.
func substituteValueAtDepth(v any, vars map[string]string, depth int) any {
	switch val := v.(type) {
	case string:
		return substituteVariablesString(val, vars)
	case map[string]any:
		return substituteMapAtDepth(val, vars, depth+1)
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = substituteValueAtDepth(elem, vars, depth+1)
		}
		return out
	default:
		// bool, float64, nil, etc. — pass through unchanged
		return v
	}
}
