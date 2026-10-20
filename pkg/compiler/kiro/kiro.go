// Package kiro provides the Kiro (AWS Q Developer CLI successor) adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "kiro" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package kiro

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "kiro", Files: []string{"AGENTS.md"}, Dirs: []string{".kiro"}})
	compiler.RegisterCompiler("kiro", Compile)
	compiler.RegisterProfileFunc("kiro", Profile)
}

// Compile produces Kiro output:
//
//	AGENTS.md                  concatenated instructions
//	.kiro/settings/cli.json    settings
//	.kiro/settings/mcp.json    MCP server config
//	.kiro/agents/              overlaid agent files
//	.kiro/commands/            overlaid command files
//	.kiro/skills/              overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Kiro.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Kiro.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".kiro"
	p.InstructionOutput = ".kiro/AGENTS.md"
	p.MCPOutput = ".kiro/settings/mcp.json"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Kiro.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false       // Kiro doesn't support hooks
	caps.Plugins = false     // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false       // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "kiro",
		OutputRoot:              ".kiro",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // Kiro uses AGENTS.md natively
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "settings/cli.json",
		SettingsLockKey:         "cli.json",
		SettingsCandidates:      []string{"settings/cli.json", "cli.json", "settings.kiro.json"},
		MCPOutput:               ".kiro/settings/mcp.json",
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
			// Permissions: embedded in settings (Kiro format)
			if len(result.Content.Permissions) > 0 {
				if translated := TranslatePermissionsToKiro(result.Content.Permissions); len(translated) > 0 {
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
