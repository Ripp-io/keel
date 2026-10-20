// Package amp provides the Amp adapter for the Keel compiler.
// Amp is Sourcegraph's AI coding agent. It registers a compiler, native reader,
// and writer for the "amp" agent format via init() functions that call the
// parent compiler package's registration functions.
package amp

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "amp", Files: []string{"AGENTS.md"}, Dirs: []string{".amp"}})
	compiler.RegisterCompiler("amp", Compile)
	compiler.RegisterProfileFunc("amp", Profile)
}

// Compile produces Amp output:
//
//	AGENTS.md            concatenated instructions
//	.amp/settings.json   settings with embedded MCP and permissions
//	.amp/commands/       overlaid command files
//	.amp/agents/         overlaid agent files
//	.amp/skills/         overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Amp.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Amp.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".config/amp"
	p.InstructionOutput = ".config/amp/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Amp.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false      // Amp doesn't support hooks
	caps.Plugins = false    // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false      // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "amp",
		OutputRoot:              ".amp",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // Amp IS the shared format
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "settings.json",
		SettingsLockKey:         "settings.json",
		SettingsCandidates:      []string{"settings.json", "settings.amp.json"},
		MCPOutput:               "", // MCP embedded in settings
		HooksInSettings:         false,
		Categories:              compiler.BaseCategories(),
		InstructionCandidates: []string{
			"AGENTS.md",
			"AGENT.md",
			"CLAUDE.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(),
		MCPNativeLockCategory: "",
		ContentOutputs:        nil,
		ConfigOutputs:         nil,
		Capabilities:          caps,
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// MCP: embedded in settings under amp.mcpServers
			if len(result.Content.MCP) > 0 {
				if translated := TranslateMCPToAmp(result.Content.MCP); len(translated) > 0 {
					// Deep-merge with any existing MCP in settings from AgentSpecific
					if existing, ok := settings["amp.mcpServers"].(map[string]any); ok {
						settings["amp.mcpServers"] = merge.DeepMergeMapsWithPolicies(translated, existing, nil)
					} else {
						settings["amp.mcpServers"] = translated
					}
				}
			}

			// Permissions: embedded in settings under amp.commands.allowlist
			if len(result.Content.Permissions) > 0 {
				if translated := TranslatePermissionsToAmp(result.Content.Permissions); len(translated) > 0 {
					// Deep-merge with any existing allowlist in settings
					if existing, ok := settings["amp.commands.allowlist"].([]any); ok {
						settings["amp.commands.allowlist"] = append(existing, translated...)
					} else {
						settings["amp.commands.allowlist"] = translated
					}
				}
			}
		},
	}
}
