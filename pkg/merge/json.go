// Package merge provides strategies for combining config files across layers.
package merge

import (
	"fmt"
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
)

// DeepMergeMapsWithPolicies merges src into dst, applying per-key merge strategies
// from the policies map. Policy keys are dot-separated paths (e.g., "mcpServers.gitlab").
// When a key's policy specifies policy.MergeStrategyAdditive and both values are arrays,
// the arrays are unioned (src items added to dst, duplicates removed).
// When policy.MergeStrategyReplace is specified, src replaces dst (same as default).
// Keys without a policy use the default deep-merge behavior.
func DeepMergeMapsWithPolicies(dst, src map[string]any, policies map[string]policy.KeelPolicy) map[string]any {
	return deepMergeMapsWithPolicies(dst, src, policies, "", 0, 128)
}

func deepMergeMapsWithPolicies(dst, src map[string]any, policies map[string]policy.KeelPolicy, path string, depth, maxDepth int) map[string]any {
	if depth > maxDepth {
		slog.Warn("deep merge exceeded maximum nesting depth, truncating source values", "maxDepth", maxDepth, "path", path)
		return dst
	}

	// Check for a policy on the current map itself (the _keel sibling pattern).
	// When _keel sits inside a map, StripKeelPolicies stores the policy under
	// the map's own path — e.g., _keel inside "permissions" produces a policy
	// keyed "permissions". This policy applies to all array-typed children of
	// that map: if the strategy is additive, child arrays are unioned.
	var parentPolicy *policy.KeelPolicy
	if policies != nil {
		if p, ok := policies[path]; ok {
			parentPolicy = &p
		}
	}

	result := make(map[string]any)

	// Copy all dst entries
	for k, v := range dst {
		result[k] = v
	}

	// Merge src entries
	for k, srcVal := range src {
		currentPath := k
		if path != "" {
			currentPath = path + "." + k
		}

		dstVal, exists := result[k]
		if !exists {
			result[k] = srcVal
			continue
		}

		// Determine the effective merge strategy for this key.
		// A direct policy on the key's path takes precedence, then the parent
		// policy applies to array-typed children of the current map.
		effectiveStrategy := policy.MergeStrategy("")
		if policies != nil {
			if p, ok := policies[currentPath]; ok {
				effectiveStrategy = p.Merge
			}
		}
		if effectiveStrategy == "" && parentPolicy != nil {
			effectiveStrategy = parentPolicy.Merge
		}

		switch effectiveStrategy {
		case policy.MergeStrategyAdditive:
			srcArr, srcIsArr := srcVal.([]any)
			dstArr, dstIsArr := dstVal.([]any)
			if srcIsArr && dstIsArr {
				result[k] = unionArrays(dstArr, srcArr)
				continue
			}
			// Additive on maps: recurse (the policy is stored at this path,
			// so the recursive call will pick it up as parentPolicy).
			srcMap, srcIsMap := srcVal.(map[string]any)
			dstMap, dstIsMap := dstVal.(map[string]any)
			if srcIsMap && dstIsMap {
				result[k] = deepMergeMapsWithPolicies(dstMap, srcMap, policies, currentPath, depth+1, maxDepth)
				continue
			}
			// Non-array, non-map additive: LWW
			result[k] = srcVal
			continue
		case policy.MergeStrategyReplace:
			result[k] = srcVal
			continue
		}

		// Default (deep-merge or no policy): recurse for maps, LWW for others
		srcMap, srcIsMap := srcVal.(map[string]any)
		dstMap, dstIsMap := dstVal.(map[string]any)
		if srcIsMap && dstIsMap {
			result[k] = deepMergeMapsWithPolicies(dstMap, srcMap, policies, currentPath, depth+1, maxDepth)
		} else {
			result[k] = srcVal
		}
	}

	return result
}

// unionArrays merges two arrays, deduplicating entries.
// Deduplication uses fmt.Sprintf("%v") for comparison, which works correctly
// for strings, numbers, and booleans. For complex types (nested maps, slices),
// this may produce false matches if two different values have the same %v
// representation. This is acceptable for the current use case (permission
// strings, server names) but should be revisited if unionArrays is used
// for object arrays.
//
// Order is preserved: dst items first, then new src items in their original order.
func unionArrays(dst, src []any) []any {
	// Warn if any element is a complex type (map or slice) where fmt.Sprintf("%v")
	// deduplication may produce false matches. This makes the documented limitation
	// observable at runtime without breaking existing callers.
	for _, item := range dst {
		switch item.(type) {
		case map[string]any, []any:
			slog.Warn("unionArrays: complex type in array may cause incorrect deduplication",
				"type", fmt.Sprintf("%T", item))
			goto dedup // warn once, not per element
		}
	}
	for _, item := range src {
		switch item.(type) {
		case map[string]any, []any:
			slog.Warn("unionArrays: complex type in array may cause incorrect deduplication",
				"type", fmt.Sprintf("%T", item))
			goto dedup
		}
	}
dedup:
	seen := make(map[string]struct{}, len(dst))
	result := make([]any, 0, len(dst)+len(src))

	for _, item := range dst {
		key := fmt.Sprintf("%v", item)
		seen[key] = struct{}{}
		result = append(result, item)
	}

	for _, item := range src {
		key := fmt.Sprintf("%v", item)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

