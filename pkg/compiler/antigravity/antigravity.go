// Package antigravity provides the Antigravity CLI adapter for the Keel compiler.
// Antigravity is Google's CLI agent that replaced Gemini CLI in June 2026.
// It registers a compiler, native reader, and writer for the "antigravity" agent
// format via init() functions that call the parent compiler package's
// registration functions.
package antigravity

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "antigravity", Files: []string{"GEMINI.md"}, Dirs: []string{".gemini"}})
	compiler.RegisterCompiler("antigravity", Compile)
	compiler.RegisterProfileFunc("antigravity", Profile)
}

// Compile produces Antigravity CLI output:
//
//	GEMINI.md                              global instructions
//	.gemini/antigravity-cli/settings.json  settings
//	.agents/mcp_config.json                MCP server config (separate file)
//	.agents/skills/                        overlaid skill files
//	.agents/commands/                      overlaid command files
//	.agents/agents/                        overlaid agent files
//
// Antigravity uses .agents/ for skills, commands, and agents instead of placing
// them inside .gemini/ like Claude does with .claude/.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(Profile(opts.Scope), layers, outputDir, opts)
}

// Profile returns the scope-appropriate AgentProfile for Antigravity CLI.
func Profile(scope compiler.Scope) compiler.AgentProfile {
	if scope == compiler.ScopeUser {
		return UserProfile()
	}
	return ProjectProfile()
}

// UserProfile returns a user-level AgentProfile for Antigravity CLI.
// User-scope config resides in ~/.gemini/ with settings in a subdirectory.
func UserProfile() compiler.AgentProfile {
	p := ProjectProfile()
	// User scope: settings in subdirectory, MCP in config/
	p.InstructionOutput = ".gemini/GEMINI.md"
	p.MCPOutput = ".gemini/config/mcp_config.json"
	p.SharedInstructionOutput = ".gemini/AGENTS.md"
	return p
}

// ProjectProfile returns the project-level AgentProfile for Antigravity CLI.
func ProjectProfile() compiler.AgentProfile {
	caps := compiler.DefaultCapabilities()
	// Antigravity supports MCP, skills, commands, agents, bin, deps
	// Hooks support is unclear from documentation — capability disabled pending research
	caps.Hooks = false
	// Permissions support is unclear from documentation — capability disabled pending research
	caps.Permissions = false
	caps.Plugins = false    // Plugins are Claude-only
	caps.AgentMemory = false // Agent-memory is Claude-only
	caps.Rules = false      // Rules are Claude/Cursor-only

	return compiler.AgentProfile{
		Name:                    "antigravity",
		OutputRoot:              ".gemini",
		InstructionOutput:       "GEMINI.md",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "@AGENTS.md\n",
		SettingsFile:            "settings.json",
		SettingsLockKey:         "settings.json",
		SettingsCandidates:      []string{"settings.json", "settings.antigravity.json"},
		MCPOutput:               ".agents/mcp_config.json",
		HooksInSettings:         false, // Hooks support unclear
		Categories: []compiler.CategoryDef{
			// Antigravity uses .agents/ prefix for categories instead of .gemini/
			{DirName: "commands", LockCategory: "command"},
			{DirName: "agents", LockCategory: "agent"},
			{DirName: "skills", LockCategory: "skill"},
		},
		InstructionCandidates: []string{
			"GEMINI.md",
			"gemini.md",
			"AGENTS.md",
			"agents.md",
		},
		HookKeyTransform:      compiler.IdentityTransform(),
		ToolNameTransform:     compiler.IdentityTransform(),
		MCPNativeLockCategory: "",
		MCPMergeInto:          false,
		ContentOutputs:        nil, // No Claude-specific content blocks
		ConfigOutputs:         nil,
		Capabilities:          caps,
		SettingsAssembler:     nil, // No special settings assembly needed
	}
}
