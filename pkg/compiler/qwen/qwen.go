// Package qwen provides the Qwen Code adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "qwen" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package qwen

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "qwen", Files: []string{"QWEN.md"}, Dirs: []string{".qwen"}})
	compiler.RegisterCompiler("qwen", Compile)
	compiler.RegisterProfileFunc("qwen", Profile)
}

// Compile produces Qwen Code output:
//
//	QWEN.md                concatenated instructions
//	.qwen/settings.json    settings with embedded MCP
//	.qwen/commands/        overlaid command files
//	.qwen/agents/          overlaid agent files
//	.qwen/skills/          overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Qwen Code.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Qwen Code.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".config/qwen"
	p.InstructionOutput = ".config/qwen/QWEN.md"
	p.SharedInstructionOutput = ".config/qwen/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Qwen Code.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false       // Qwen Code doesn't support hooks (Gemini CLI fork)
	caps.Plugins = false     // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false       // Rules are Claude/Cursor-only
	caps.Permissions = false // Qwen Code doesn't have documented permissions support

	return compiler.AgentProfile{
		Name:                    "qwen",
		OutputRoot:              ".qwen",
		InstructionOutput:       "QWEN.md",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "@AGENTS.md\n",
		SettingsFile:            "settings.json",
		SettingsLockKey:         "settings.json",
		SettingsCandidates:      []string{"settings.json", "settings.qwen.json"},
		MCPOutput:               "",     // MCP embedded in settings
		HooksInSettings:         false,  // Qwen doesn't support hooks
		Categories:              compiler.BaseCategories(),
		InstructionCandidates: []string{
			"QWEN.md",
			"qwen.md",
			"AGENTS.md",
			"agents.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(),
		MCPNativeLockCategory: "",
		ContentOutputs:        nil, // Qwen doesn't support loop or worktree-include
		ConfigOutputs:         nil,
		Capabilities:          caps,
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// MCP: embedded in settings using standard mcpServers format
			if len(result.Content.MCP) > 0 {
				// Deep-merge with any existing MCP in settings from AgentSpecific
				if existing, ok := settings["mcpServers"].(map[string]any); ok {
					if incoming, ok := result.Content.MCP["mcpServers"].(map[string]any); ok {
						settings["mcpServers"] = merge.DeepMergeMapsWithPolicies(incoming, existing, nil)
					}
				} else {
					if incoming, ok := result.Content.MCP["mcpServers"].(map[string]any); ok {
						settings["mcpServers"] = incoming
					}
				}
			}

			// Note: Qwen Code doesn't support permissions in the documented format,
			// so we don't include them in settings output
		},
	}
}
