package compiler

import (
	"encoding/json"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// AssembleAgentOutput prepares the AccumulatedResult for the target agent.
// It assembles the final settings map by merging accumulated settings,
// agent-specific settings, permissions, attribution, hooks, sandbox, and
// MCP policy in the format expected by the target agent.
//
// After this call, result.Settings contains the final map ready for
// serialization, and result.Hooks has transformed keys.
//
// This is the TRANSFORM stage — called after Finalize, before Write.
func AssembleAgentOutput(result *AccumulatedResult, p AgentProfile) {
	settings := make(map[string]any)

	// 1. Seed with agent defaults (e.g., Cursor's version: 1)
	for k, v := range p.SettingsDefaults {
		settings[k] = v
	}

	// 2. Merge accumulated settings
	for k, v := range result.Content.Settings {
		settings[k] = v
	}

	// 3. Merge agent-specific settings
	if agentData, ok := result.Content.AgentSpecific[p.Name]; ok {
		for k, v := range agentData {
			settings[k] = v
		}
	}

	// 4. Apply hook key transform (must happen before embedding)
	if p.HookKeyTransform != nil && len(result.Content.Hooks) > 0 {
		transformed := make(map[string]any, len(result.Content.Hooks))
		for k, v := range result.Content.Hooks {
			transformed[p.HookKeyTransform(k)] = v
		}
		result.Content.Hooks = transformed
	}

	// 5. Agent-specific assembly (permissions, attribution, sandbox, hooks, MCPPolicy)
	if p.SettingsAssembler != nil {
		p.SettingsAssembler(settings, result)
	}

	result.Content.Settings = settings
}

// AssembleHooks embeds hooks into settings if present.
// Used by agents that embed hooks in their settings file (Claude).
func AssembleHooks(settings map[string]any, hooks map[string]any) {
	if len(hooks) > 0 {
		settings["hooks"] = hooks
	}
}

// AssembleMCPPolicy embeds MCP governance policy keys into settings.
// Used by agents that embed MCP policy in their settings file (Claude).
func AssembleMCPPolicy(settings map[string]any, policy *schema.MCPPolicy) {
	if policy == nil {
		return
	}
	if len(policy.Allowed) > 0 {
		settings["allowedMcpServers"] = policy.Allowed
	}
	if len(policy.Denied) > 0 {
		settings["deniedMcpServers"] = policy.Denied
	}
	if policy.EnableAll != nil {
		settings["enableAllProjectMcpServers"] = *policy.EnableAll
	}
}

// AssembleSandboxEmbed marshals a SandboxConfig into a map and embeds it in settings.
// Used by agents that embed sandbox config in their settings file (Claude).
func AssembleSandboxEmbed(settings map[string]any, sandbox *schema.SandboxConfig) {
	if sandbox == nil {
		return
	}
	data, err := json.Marshal(sandbox)
	if err != nil {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil || len(m) == 0 {
		return
	}
	settings["sandbox"] = m
}
