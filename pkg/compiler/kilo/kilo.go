// Package kilo provides the Kilo Code adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "kilo" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package kilo

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "kilo", Files: []string{"AGENTS.md"}, Dirs: []string{".kilo"}})
	compiler.RegisterCompiler("kilo", Compile)
	compiler.RegisterProfileFunc("kilo", Profile)
}

// Compile produces Kilo Code output:
//
//	AGENTS.md            concatenated instructions
//	.kilo/kilo.jsonc     settings with embedded MCP and permissions
//	.kilo/commands/      overlaid command files
//	.kilo/agents/        overlaid agent files
//	.kilo/skills/        overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Kilo.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Kilo.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".config/kilo"
	p.InstructionOutput = ".config/kilo/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Kilo.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false      // Kilo doesn't document hooks support
	caps.Plugins = false    // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false      // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "kilo",
		OutputRoot:              ".kilo",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // Kilo uses AGENTS.md as the native format
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "kilo.jsonc",
		SettingsLockKey:         "kilo.jsonc",
		SettingsCandidates:      []string{"kilo.jsonc", "settings.kilo.json"},
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
			// MCP: embedded in settings (Kilo format)
			if len(result.Content.MCP) > 0 {
				if translated := TranslateMCPToKilo(result.Content.MCP); len(translated) > 0 {
					// Deep-merge with any existing MCP in settings from AgentSpecific
					if existing, ok := settings["mcp"].(map[string]any); ok {
						settings["mcp"] = merge.DeepMergeMapsWithPolicies(translated, existing, nil)
					} else {
						settings["mcp"] = translated
					}
				}
			}

			// Permissions: embedded in settings (Kilo format uses "permission" key, not "permissions")
			if len(result.Content.Permissions) > 0 {
				if translated := TranslatePermissionsToKilo(result.Content.Permissions); len(translated) > 0 {
					// Deep-merge with any existing permissions in settings
					if existing, ok := settings["permission"].(map[string]any); ok {
						settings["permission"] = merge.DeepMergeMapsWithPolicies(translated, existing, nil)
					} else {
						settings["permission"] = translated
					}
				}
			}
		},
	}
}
