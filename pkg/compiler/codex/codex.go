// Package codex provides the Codex CLI adapter for the Keel compiler.
// Codex CLI is OpenAI's CLI coding agent. It uses TOML config and AGENTS.md
// for instructions, with MCP support embedded in config.toml.
package codex

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "codex", Files: []string{"AGENTS.md"}, Dirs: []string{".codex"}})
	compiler.RegisterCompiler("codex", Compile)
	compiler.RegisterProfileFunc("codex", Profile)
}

// Compile produces Codex CLI output:
//
//	AGENTS.md          concatenated instructions (Codex native format)
//	.codex/config.toml settings with embedded MCP and permissions
//	.codex/commands/   overlaid command files
//	.codex/agents/     overlaid agent files
//	.codex/skills/     overlaid skill files
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Codex CLI.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Codex CLI.
// Codex reads config from ~/.codex/config.toml and instructions from
// ~/.codex/AGENTS.md.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.OutputRoot = ".config/codex"
	p.InstructionOutput = ".config/codex/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Codex CLI.
// Codex reads config from .codex/config.toml and instructions from AGENTS.md
// in the project root and along the directory path.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Hooks = false       // Codex doesn't support hooks
	caps.Plugins = false     // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false       // Rules are Claude/Cursor-only
	caps.Permissions = true  // Codex has approval policies and sandbox config

	return compiler.AgentProfile{
		Name:                    "codex",
		OutputRoot:              ".codex",
		InstructionOutput:       "AGENTS.md",
		SharedInstructionOutput: "", // Codex IS the shared format (native AGENTS.md)
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — AGENTS.md is the native format
		SettingsFile:            "config.toml",
		SettingsLockKey:         "config.toml",
		SettingsCandidates:      []string{"config.toml", "config.json", "settings.codex.json"},
		MCPOutput:               "", // MCP embedded in settings (config.toml format)
		HooksInSettings:         false,
		Categories:              compiler.BaseCategories(),
		InstructionCandidates: []string{
			"AGENTS.md",
			"AGENTS.override.md",
			"agents.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(),
		MCPNativeLockCategory: "",
		ContentOutputs:        nil,
		ConfigOutputs:         nil,
		Capabilities:          caps,
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// MCP: embedded in settings (Codex config.toml format)
			// Codex uses a "mcp" key with "servers" sub-key
			if len(result.Content.MCP) > 0 {
				if translated := TranslateMCPToCodex(result.Content.MCP); len(translated) > 0 {
					// Deep-merge with any existing MCP in settings from AgentSpecific
					if existing, ok := settings["mcp"].(map[string]any); ok {
						settings["mcp"] = merge.DeepMergeMapsWithPolicies(translated, existing, nil)
					} else {
						settings["mcp"] = translated
					}
				}
			}

			// Permissions: embedded in settings (Codex approval_policy format)
			// Codex doesn't use the same granular permissions as Claude/Cursor,
			// but we can map to approval_policy settings
			if len(result.Content.Permissions) > 0 {
				if translated := TranslatePermissionsToCodex(result.Content.Permissions); len(translated) > 0 {
					// Merge with existing approval_policy
					if existing, ok := settings["approval_policy"].(map[string]any); ok {
						for k, v := range translated {
							existing[k] = v
						}
					} else {
						settings["approval_policy"] = translated
					}
				}
			}

			// Model: Codex uses model_providers configuration
			// Extract portable model setting if present
			if model, ok := result.Content.Settings["model"].(string); ok && model != "" {
				settings["model"] = model
			}
		},
	}
}
