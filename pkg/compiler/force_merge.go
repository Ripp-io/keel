package compiler

import (
	"fmt"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/policy"
)

// registerMCPPolicies registers force-locks from _keel policies found in MCP
// JSON files. The policy path (e.g. "mcpServers.gitlab") is parsed to extract
// the server name, which becomes the force-lock key under category "mcp".
func registerMCPPolicies(policies map[string]policy.KeelPolicy, force *forceRegistry, layer string) {
	if policies == nil {
		return
	}
	for path, pol := range policies {
		if pol.Force != nil && *pol.Force {
			serverName := extractMCPServerName(path)
			if serverName != "" {
				force.lock("mcp", serverName, layer)
			}
		}
	}
}

// extractMCPServerName extracts the server name from a _keel policy path.
// For a path like "mcpServers.gitlab", returns "gitlab".
// For a path like "mcpServers.nested.deep", returns "nested.deep".
// Returns empty string if the path doesn't start with "mcpServers.".
func extractMCPServerName(path string) string {
	const prefix = "mcpServers."
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return ""
}

// ExtractTopLevelKeelPolicies extracts per-event-name policies from a top-level
// _keel object in hooks JSON. The hooks _keel format maps event names to policy
// objects: {"_keel": {"preToolUse": {"force": true}}}. This differs from the
// MCP format where _keel is nested inside each server entry.
func ExtractTopLevelKeelPolicies(srcMap map[string]any) map[string]policy.KeelPolicy {
	keelRaw, ok := srcMap["_keel"]
	if !ok {
		return nil
	}
	keelMap, ok := keelRaw.(map[string]any)
	if !ok {
		return nil
	}
	policies := make(map[string]policy.KeelPolicy)
	for eventName, policyRaw := range keelMap {
		policyMap, ok := policyRaw.(map[string]any)
		if !ok {
			continue
		}
		p := policy.KeelPolicy{}
		if f, ok := policyMap["force"].(bool); ok {
			p.Force = &f
		}
		if m, ok := policyMap["merge"].(string); ok {
			p.Merge = policy.MergeStrategy(m)
		}
		policies[eventName] = p
	}
	return policies
}

// cleanContainerForceLocked strips _keel policies and flat "force" fields from
// srcMap, removes entries under containerKey that are force-locked, and returns
// the cleaned map plus extracted policies. This is a pure function with no file I/O.
//
// This ensures that a lower-trust layer cannot override a force-locked entry
// from a higher-trust layer during deep-merge.
func cleanContainerForceLocked(
	srcMap map[string]any,
	containerKey, lockCategory string,
	force *forceRegistry,
) (map[string]any, map[string]policy.KeelPolicy, error) {
	// Extract and strip _keel policies from source
	cleanedSrc, policies, err := merge.StripKeelPolicies(srcMap)
	if err != nil {
		return nil, nil, fmt.Errorf("stripping _keel policies: %w", err)
	}

	// Extract flat "force" fields from server entries inside the container
	// (keel DSL mcp.json uses first-class "force" fields, not nested _keel).
	// These are stripped from the output since they are policy metadata, not
	// valid fields in the target tool's mcp.json format.
	if container, ok := cleanedSrc[containerKey].(map[string]any); ok {
		for name, entry := range container {
			if entryMap, ok := entry.(map[string]any); ok {
				if forceVal, ok := entryMap["force"].(bool); ok {
					policyKey := containerKey + "." + name
					if _, exists := policies[policyKey]; !exists {
						policies[policyKey] = policy.KeelPolicy{Force: &forceVal}
					}
					delete(entryMap, "force")
				}
			}
		}
	}

	// Remove force-locked entries from the container (e.g. mcpServers)
	if container, ok := cleanedSrc[containerKey].(map[string]any); ok {
		for name := range container {
			if locked, _ := force.isLocked(lockCategory, name); locked {
				delete(container, name)
			}
		}
		// If container is now empty, remove it to avoid empty objects in output
		if len(container) == 0 {
			delete(cleanedSrc, containerKey)
		}
	}

	return cleanedSrc, policies, nil
}
