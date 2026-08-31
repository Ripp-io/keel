// Package claude provides the Claude Code adapter for the Keel compiler.
// It registers a compiler, native reader, and writer for the "claude" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package claude

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "claude", Files: []string{"CLAUDE.md"}, Dirs: []string{".claude"}})
	compiler.RegisterCompiler("claude", Compile)
	compiler.RegisterProfileFunc("claude", Profile)
}

// Compile produces Claude output:
//
//	CLAUDE.md               concatenated instructions
//	.claude/settings.json   deep-merged settings
//	.claude/commands/       overlaid command files
//	.claude/agents/         overlaid agent files
//	.claude/skills/         overlaid skill files
//	.mcp.json               MCP server config
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Claude Code.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Claude Code.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	p.MCPOutput = ".claude.json"
	p.MCPMergeInto = true
	p.InstructionOutput = ".claude/CLAUDE.md"
	p.SharedInstructionOutput = ".claude/AGENTS.md"
	// worktree-include is project-only — remove it from ContentOutputs
	p.ContentOutputs = compiler.FilterOut(p.ContentOutputs, func(b compiler.ContentBlockDef) string { return b.Name }, "worktree-include")
	return p
}

// ProjectProfile returns the project-level AgentProfile for Claude Code.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	caps.Plugins = true
	caps.AgentMemory = true
	caps.Rules = true

	return compiler.AgentProfile{
		Name:                    "claude",
		OutputRoot:              ".claude",
		InstructionOutput:       "CLAUDE.md",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "@AGENTS.md\n",
		SettingsFile:            "settings.json",
		SettingsLockKey:         "settings.json",
		SettingsCandidates:      []string{"settings.json", "settings.claude.json"},
		MCPOutput:               ".mcp.json",
		HooksInSettings:         true,
		Categories: append(compiler.BaseCategories(),
			compiler.CategoryDef{DirName: "plugins", LockCategory: "plugin"},
			compiler.CategoryDef{DirName: "agent-memory", LockCategory: "agent-memory"},
			compiler.CategoryDef{DirName: "rules", LockCategory: "rule"},
		),
		InstructionCandidates: []string{
			"instructions.md",
			"INSTRUCTIONS.md",
			"CLAUDE.md",
			"claude.md",
			"AGENTS.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(), // Claude and Cursor share tool names
		MCPNativeLockCategory: "",                           // Claude has no file-level MCP lock in native layers
		ContentOutputs: []compiler.ContentBlockDef{
			{Name: "loop", JoinSeparator: "\n\n", OutputPath: ".claude/loop.md"},
			{Name: "worktree-include", JoinSeparator: "\n", OutputPath: ".worktreeinclude"},
		},
		ConfigOutputs: []compiler.ConfigBlockDef{},
		Capabilities:  caps,
		SettingsAssembler: func(settings map[string]any, result *compiler.AccumulatedResult) {
			// Permissions: nested format.
			// Deep-merge with any permissions already in settings from AgentSpecific
			// (set by AssembleAgentOutput step 3). Agent-specific permissions (e.g.,
			// from settings.claude.json) are higher priority and win on conflict.
			if len(result.Content.Permissions) > 0 {
				if existing, ok := settings["permissions"].(map[string]any); ok {
					settings["permissions"] = merge.DeepMergeMapsWithPolicies(
						result.Content.Permissions, existing, nil)
				} else {
					settings["permissions"] = result.Content.Permissions
				}
			}

			// Hooks: embedded in settings (Claude stores hooks in settings.json)
			compiler.AssembleHooks(settings, result.Content.Hooks)

			// Sandbox: embedded in settings
			compiler.AssembleSandboxEmbed(settings, result.Content.Sandbox)

			// Attribution: template strings (Claude-specific format)
			if result.Content.Attribution != nil {
				settings["attribution"] = map[string]any{
					"commit": result.Content.Attribution.CommitTemplate,
					"pr":     result.Content.Attribution.PRTemplate,
				}
			}

			// MCP governance policy keys
			compiler.AssembleMCPPolicy(settings, result.Governance.MCPPolicy)
		},
	}
}
