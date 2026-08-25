package compiler

import (
	"sort"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// BuildEffectiveState builds an EffectiveState from an AccumulatedResult.
// Called at compile time — this is the authoritative record.
// Returns nil if the result contains no effective configuration.
func BuildEffectiveState(result *AccumulatedResult) *manifest.EffectiveState {
	if result == nil {
		return nil
	}
	return buildEffectiveStateCommon(
		result.Content.Instructions, result.Content.Permissions, result.Content.MCP,
		result.Content.Hooks, result.Content.Settings, result.Content.AgentSpecific,
		result.Content.Categories,
	)
}

// BuildEffectiveStateFromIR builds an EffectiveState from a NativeReader's LayerIR.
// Called at drift time to get the current disk state.
// Returns nil if the IR contains no effective configuration.
func BuildEffectiveStateFromIR(ir *LayerIR) *manifest.EffectiveState {
	if ir == nil {
		return nil
	}
	// Concatenate InstructionBlock content into a single string.
	var instrContent strings.Builder
	for _, block := range ir.Instructions {
		instrContent.WriteString(block.Content)
	}
	return buildEffectiveStateCommon(
		instrContent.String(), ir.Permissions, ir.MCP,
		ir.Hooks, ir.Settings, ir.AgentSpecific,
		ir.Categories,
	)
}

// buildEffectiveStateCommon is the shared implementation for BuildEffectiveState
// and BuildEffectiveStateFromIR. It builds an EffectiveState from the common
// fields that both AccumulatedResult and LayerIR share.
func buildEffectiveStateCommon(
	instructions string,
	permissions map[string]any,
	mcp map[string]any,
	hooks map[string]any,
	settings map[string]any,
	agentSpecific map[string]map[string]any,
	categories map[string][]CategoryFile,
) *manifest.EffectiveState {
	state := &manifest.EffectiveState{}
	hasContent := false

	if perms := extractPermissions(permissions); perms != nil {
		state.Permissions = perms
		hasContent = true
	}
	if servers := extractMCPServers(mcp); len(servers) > 0 {
		state.MCPServers = servers
		hasContent = true
	}
	if h := extractHooks(hooks); len(h) > 0 {
		state.Hooks = h
		hasContent = true
	}
	if instr := extractInstructions(instructions); instr != nil {
		state.Instructions = instr
		hasContent = true
	}

	// Merge settings and agent-specific into a flat map.
	// AgentSpecific is safe to flatten without namespacing because
	// BuildEffectiveState is called after AgentBuild, which filters
	// AgentSpecific to only the target agent's keys.
	merged := make(map[string]any)
	for k, v := range settings {
		merged[k] = v
	}
	for _, agentMap := range agentSpecific {
		for k, v := range agentMap {
			merged[k] = v
		}
	}
	if len(merged) > 0 {
		state.Settings = merged
		hasContent = true
	}

	if cats := extractCategories(categories); len(cats) > 0 {
		state.Categories = cats
		hasContent = true
	}

	if !hasContent {
		return nil
	}
	return state
}

// extractPermissions extracts and sorts allow/deny permission lists.
// Returns nil if no permissions are set.
func extractPermissions(perms map[string]any) *manifest.EffectivePermissions {
	if len(perms) == 0 {
		return nil
	}

	ep := &manifest.EffectivePermissions{}
	hasContent := false

	// Extract allow list
	if allowVal, ok := perms["allow"]; ok {
		if allowList, ok := allowVal.([]any); ok {
			for _, item := range allowList {
				if s, ok := item.(string); ok {
					ep.Allow = append(ep.Allow, s)
				}
			}
			sort.Strings(ep.Allow)
			hasContent = true
		}
	}

	// Extract deny list
	if denyVal, ok := perms["deny"]; ok {
		if denyList, ok := denyVal.([]any); ok {
			for _, item := range denyList {
				if s, ok := item.(string); ok {
					ep.Deny = append(ep.Deny, s)
				}
			}
			sort.Strings(ep.Deny)
			hasContent = true
		}
	}

	if !hasContent {
		return nil
	}

	return ep
}

// extractMCPServers extracts server names and types from the mcpServers map.
// Returns a sorted list by server name.
func extractMCPServers(mcp map[string]any) []manifest.EffectiveMCPServer {
	if mcp == nil {
		return nil
	}

	serversVal, ok := mcp["mcpServers"]
	if !ok {
		return nil
	}

	serversMap, ok := serversVal.(map[string]any)
	if !ok {
		return nil
	}

	var servers []manifest.EffectiveMCPServer
	for name, configVal := range serversMap {
		server := manifest.EffectiveMCPServer{
			Name: name,
			Type: "unknown",
		}

		// Extract type from server config
		if configMap, ok := configVal.(map[string]any); ok {
			// Try stdio command first
			if _, hasCommand := configMap["command"]; hasCommand {
				server.Type = "stdio"
			} else if _, hasUrl := configMap["url"]; hasUrl {
				// Check for SSE vs streamable-http
				if transport, ok := configMap["transport"].(string); ok {
					server.Type = transport
				} else {
					server.Type = "sse" // default for url-based
				}
			}
		}

		servers = append(servers, server)
	}

	// Sort by name for deterministic output
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	return servers
}

// extractHooks counts handlers per event and returns a sorted list by event name.
// Includes events even if they have zero handlers for presence tracking.
func extractHooks(hooks map[string]any) []manifest.EffectiveHook {
	if len(hooks) == 0 {
		return nil
	}

	var hookList []manifest.EffectiveHook
	for event, handlersVal := range hooks {
		count := 0

		// Hooks are stored as arrays of handler objects
		if handlersList, ok := handlersVal.([]any); ok {
			count = len(handlersList)
		}

		hookList = append(hookList, manifest.EffectiveHook{
			Event:    event,
			Handlers: count,
		})
	}

	// Sort by event name for deterministic output
	sort.Slice(hookList, func(i, j int) bool {
		return hookList[i].Event < hookList[j].Event
	})

	return hookList
}

// extractInstructions computes byte count and token estimate from instruction content.
// Returns nil if content is empty.
func extractInstructions(content string) *manifest.EffectiveInstructions {
	if content == "" {
		return nil
	}

	bytes := len(content)
	return &manifest.EffectiveInstructions{
		Bytes:         bytes,
		TokenEstimate: estimateTokens(bytes),
	}
}

// extractCategories counts non-directory files per category and returns a sorted list.
func extractCategories(cats map[string][]CategoryFile) []manifest.EffectiveCategory {
	if len(cats) == 0 {
		return nil
	}

	var catList []manifest.EffectiveCategory
	for name, files := range cats {
		count := 0
		for _, f := range files {
			if !f.IsDir {
				count++
			}
		}

		if count > 0 {
			catList = append(catList, manifest.EffectiveCategory{
				Name:      name,
				FileCount: count,
			})
		}
	}

	// Sort by name for deterministic output
	sort.Slice(catList, func(i, j int) bool {
		return catList[i].Name < catList[j].Name
	})

	return catList
}
