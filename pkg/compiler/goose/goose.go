// Package goose provides the Goose adapter for the Keel compiler.
// Goose is Block's AI coding agent that uses YAML configuration.
// It registers a compiler, native reader, and writer for the "goose" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package goose

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "goose", Files: []string{".goosehints"}, Dirs: []string{".goose", ".config/goose"}})
	compiler.RegisterCompiler("goose", Compile)
	compiler.RegisterProfileFunc("goose", Profile)
}

// Compile produces Goose output:
//
//	.goosehints             project instruction file
//	AGENTS.md               shared instruction file
//	.goose/config.yaml      settings with embedded MCP extensions
//	.goose/commands/        overlaid command files
//	.goose/agents/          overlaid agent files
//	.goose/skills/          overlaid skill files
//
// User-scope output uses ~/.config/goose/config.yaml instead.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Goose.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Goose.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".config/goose"
	p.InstructionOutput = ".config/goose/AGENTS.md"
	p.SharedInstructionOutput = "" // User scope doesn't use shared file
	p.InstructionShimTemplate = ""
	return p
}

// ProjectProfile returns the project-level AgentProfile for Goose.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false       // Goose doesn't support hooks
	caps.Permissions = false // Goose doesn't have granular permissions like Claude/Cursor
	caps.Plugins = false     // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false       // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "goose",
		OutputRoot:              ".goose",
		InstructionOutput:       ".goosehints",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // Goose reads .goosehints directly
		SettingsFile:            "config.yaml",
		SettingsLockKey:         "config.yaml",
		SettingsCandidates:      []string{"config.yaml", "settings.goose.yaml"},
		MCPOutput:               "", // MCP embedded in settings as extensions
		HooksInSettings:         false,
		Categories:              compiler.BaseCategories(),
		InstructionCandidates: []string{
			".goosehints",
			"AGENTS.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(),
		MCPNativeLockCategory: "",
		ContentOutputs:        nil,
		ConfigOutputs:         nil,
		Capabilities:          caps,
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// MCP: embedded in settings as extensions (Goose format)
			if len(result.Content.MCP) > 0 {
				if translated := TranslateMCPToGoose(result.Content.MCP); len(translated) > 0 {
					// Merge with existing extensions
					if existing, ok := settings["extensions"].([]any); ok {
						settings["extensions"] = append(existing, translated...)
					} else {
						settings["extensions"] = translated
					}
				}
			}
		},
	}
}
