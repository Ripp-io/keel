// Package opencode provides the OpenCode V2 adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "opencode" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package opencode

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "opencode", Files: []string{"AGENTS.md"}, Dirs: []string{".opencode"}})
	compiler.RegisterCompiler("opencode", Compile)
	compiler.RegisterProfileFunc("opencode", Profile)
}

// Compile produces OpenCode output:
//
//	AGENTS.md               concatenated instructions
//	.opencode/opencode.json settings with embedded MCP and permissions
//	.opencode/commands/     overlaid command files
//	.opencode/agents/       overlaid agent files
//	.opencode/skills/       overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for OpenCode.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for OpenCode.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".config/opencode"
	p.InstructionOutput = ".config/opencode/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for OpenCode.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false      // OpenCode V2 doesn't support hooks
	caps.Plugins = false    // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false      // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "opencode",
		OutputRoot:              ".opencode",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // OpenCode IS the shared format
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "opencode.json",
		SettingsLockKey:         "opencode.json",
		SettingsCandidates:      []string{"opencode.json", "settings.opencode.json"},
		MCPOutput:               "", // MCP embedded in settings
		HooksInSettings:         false,
		Categories:              compiler.BaseCategories(),
		InstructionCandidates: []string{
			"AGENTS.md",
			"agents.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(),
		MCPNativeLockCategory: "",
		ContentOutputs:        nil,
		ConfigOutputs:         nil,
		Capabilities:          caps,
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// MCP: embedded in settings (OpenCode V2 format)
			if len(result.Content.MCP) > 0 {
				if translated := TranslateMCPToOpenCode(result.Content.MCP); len(translated) > 0 {
					// Deep-merge with any existing MCP in settings from AgentSpecific
					if existing, ok := settings["mcp"].(map[string]any); ok {
						settings["mcp"] = merge.DeepMergeMapsWithPolicies(translated, existing, nil)
					} else {
						settings["mcp"] = translated
					}
				}
			}

			// Permissions: embedded in settings (OpenCode V2 format)
			if len(result.Content.Permissions) > 0 {
				if translated := TranslatePermissionsToOpenCode(result.Content.Permissions); len(translated) > 0 {
					// Deep-merge with any existing permissions in settings
					if existing, ok := settings["permissions"].([]any); ok {
						settings["permissions"] = append(existing, translated...)
					} else {
						settings["permissions"] = translated
					}
				}
			}
		},
	}
}
