package merge

import (
	"fmt"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
)

const maxStripKeelDepth = 128

// StripKeelPolicies recursively walks a JSON structure (map[string]any)
// and removes all "_keel" keys. Returns the cleaned structure, a map of
// policies found (keyed by parent path, e.g. "mcpServers.gitlab"), and an
// error if the input exceeds the maximum nesting depth. A depth error is
// security-critical — _keel keys must not leak into output.
func StripKeelPolicies(data map[string]any) (map[string]any, map[string]policy.KeelPolicy, error) {
	policies := make(map[string]policy.KeelPolicy)
	cleaned, err := stripKeelRecursive(data, "", policies, 0)
	if err != nil {
		return nil, nil, err
	}
	return cleaned, policies, nil
}

func stripKeelRecursive(data map[string]any, path string, policies map[string]policy.KeelPolicy, depth int) (map[string]any, error) {
	if depth > maxStripKeelDepth {
		return nil, fmt.Errorf("_keel policy stripping exceeded maximum nesting depth %d", maxStripKeelDepth)
	}

	result := make(map[string]any)
	for key, value := range data {
		if key == "_keel" {
			// Parse the _keel object into a KeelPolicy
			if keelMap, ok := value.(map[string]any); ok {
				kp := policy.KeelPolicy{}
				if f, ok := keelMap["force"].(bool); ok {
					kp.Force = &f
				}
				if m, ok := keelMap["merge"].(string); ok {
					kp.Merge = policy.MergeStrategy(m)
				}
				// Store policy with the parent path as key
				policies[path] = kp
			}
			continue // Don't include _keel in output
		}

		// Recurse into nested maps
		if nested, ok := value.(map[string]any); ok {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			stripped, err := stripKeelRecursive(nested, childPath, policies, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = stripped
		} else {
			result[key] = value
		}
	}
	return result, nil
}
