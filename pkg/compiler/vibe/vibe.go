// Package vibe provides the Mistral Vibe adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "vibe" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package vibe

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "vibe", Files: []string{"AGENTS.md"}, Dirs: []string{".vibe"}})
	compiler.RegisterCompiler("vibe", Compile)
	compiler.RegisterProfileFunc("vibe", Profile)
}

// Compile produces Mistral Vibe output:
//
//	AGENTS.md               concatenated instructions
//	.vibe/config.toml       settings with embedded MCP
//	.vibe/commands/         overlaid command files
//	.vibe/agents/           overlaid agent files
//	.vibe/skills/           overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Vibe.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Vibe.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".vibe"
	p.InstructionOutput = ".vibe/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Vibe.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false       // Vibe doesn't support hooks
	caps.Permissions = false // Vibe doesn't have permission system
	caps.Plugins = false     // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false       // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "vibe",
		OutputRoot:              ".vibe",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // Vibe IS the shared format
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "config.toml",
		SettingsLockKey:         "config.toml",
		SettingsCandidates:      []string{"config.toml", "config.json", "settings.vibe.json"},
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
		SettingsAssembler:     assembleVibeSettings,
	}
}

// assembleVibeSettings applies Vibe-specific settings transformations.
func assembleVibeSettings(settings map[string]any, result *compiler.AccumulatedResult) {
	// MCP: embedded in settings (Vibe format uses [[mcp_servers]] array)
	if len(result.Content.MCP) > 0 {
		if translated := TranslateMCPToVibe(result.Content.MCP); len(translated) > 0 {
			// Vibe uses mcp_servers array, not mcpServers object
			if existing, ok := settings["mcp_servers"].([]any); ok {
				settings["mcp_servers"] = append(existing, translated...)
			} else {
				settings["mcp_servers"] = translated
			}
		}
	}

	// Note: Vibe doesn't have a permission system like Claude/OpenCode,
	// so permissions are silently ignored (not an error, just unsupported).
}
