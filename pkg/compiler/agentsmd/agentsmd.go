// Package agentsmd provides the AGENTS.md adapter for the Keel compiler.
// AGENTS.md is the Linux Foundation/AAIF-backed universal config standard
// for AI coding agents. It supports instructions only — no settings, hooks,
// MCP, permissions, or categories.
package agentsmd

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "agentsmd", Files: []string{"AGENTS.md"}, Dirs: []string{".agents"}})
	compiler.RegisterCompiler("agentsmd", Compile)
	compiler.RegisterSimpleAdapter("agentsmd", ProjectProfile)
}

// Compile produces AGENTS.md-native configuration output.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(ProjectProfile(), layers, outputDir, opts)
}

// ProjectProfile returns the project-level AgentProfile for AGENTS.md.
// AGENTS.md has no native dot-directory — .agents is used solely for
// .keel.json manifest storage.
func ProjectProfile() compiler.AgentProfile {
	return compiler.AgentProfile{
		Name:              "agentsmd",
		OutputRoot:        ".agents",
		InstructionOutput: "AGENTS.md",
		SharedInstructionOutput: "", // AgentsMD IS the shared format
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // No shim — output IS AGENTS.md
		SettingsFile:            "",
		MCPOutput:               "",
		HooksInSettings:         false,
		Categories:        nil,
		InstructionCandidates: []string{
			"AGENTS.md",
			"agents.md",
		},
		HookKeyTransform:  compiler.IdentityTransform(),
		ToolNameTransform: compiler.IdentityTransform(),
		Capabilities: compiler.Capabilities{
			// AGENTS.md supports only instructions — suppress warnings
			// about unsupported features since they're expected.
			InstructionOnly: true,
		},
	}
}
