// Package openhands provides the OpenHands adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "openhands" agent
// format via init() functions that call the parent compiler package's
// registration functions.
//
// OpenHands is an open-source AI coding agent platform that uses:
// - AGENTS.md for instructions (shared format)
// - .openhands/ directory for agent-specific config
// - config.toml for settings
// - mcp.json for MCP server configuration
package openhands

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "openhands", Files: []string{"AGENTS.md"}, Dirs: []string{".openhands"}})
	compiler.RegisterCompiler("openhands", Compile)
	compiler.RegisterProfileFunc("openhands", Profile)
}

// Compile produces OpenHands output:
//
//	AGENTS.md                     concatenated instructions
//	.openhands/config.toml        settings
//	.openhands/mcp.json           MCP server configuration
//	.openhands/commands/          overlaid command files
//	.openhands/agents/            overlaid agent files
//	.openhands/skills/            overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for OpenHands.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for OpenHands.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".openhands"
	p.InstructionOutput = ".openhands/AGENTS.md"
	p.MCPOutput = ".openhands/mcp.json"
	return p
}

// ProjectProfile returns the project-level AgentProfile for OpenHands.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false       // OpenHands doesn't support hooks
	caps.Permissions = false // OpenHands doesn't support permissions in the same way
	caps.Plugins = false     // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false       // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "openhands",
		OutputRoot:              ".openhands",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // OpenHands uses AGENTS.md as native format
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "config.toml",
		SettingsLockKey:         "config.toml",
		SettingsCandidates:      []string{"config.toml", "config.json", "settings.openhands.json"},
		MCPOutput:               ".openhands/mcp.json",
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
			// MCP: separate file (mcp.json) in .openhands/
			// Permissions: not supported in OpenHands format
			// The writer handles MCP as a separate file, so we don't embed it here
		},
	}
}
