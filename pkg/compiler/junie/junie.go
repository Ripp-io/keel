// Package junie provides the Junie adapter for the Keel compiler.
// Junie is JetBrains' AI coding agent. It uses AGENTS.md (preferred) or
// .junie/guidelines.md (legacy) for instructions. It supports instructions
// only -- no settings, hooks, MCP, permissions, or categories.
package junie

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "junie", Files: []string{".junie/guidelines.md", "AGENTS.md"}, Dirs: []string{".junie"}})
	compiler.RegisterCompiler("junie", Compile)
	compiler.RegisterSimpleAdapter("junie", ProjectProfile)
	compiler.RegisterProfileFunc("junie", func(compiler.Scope) compiler.AgentProfile { return ProjectProfile() })
}

// Compile produces Junie-native configuration output.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(ProjectProfile(), layers, outputDir, opts)
}

// ProjectProfile returns the project-level AgentProfile for Junie.
func ProjectProfile() compiler.AgentProfile {
	return compiler.AgentProfile{
		Name:                    "junie",
		OutputRoot:              ".junie",
		InstructionOutput:       ".junie/guidelines.md",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // Instruction-only agent — no shim needed
		SettingsFile:            "",
		MCPOutput:               "",
		HooksInSettings:         false,
		Categories:              nil,
		InstructionCandidates: []string{
			".junie/guidelines.md",
			"AGENTS.md",
		},
		HookKeyTransform:  compiler.IdentityTransform(),
		ToolNameTransform: compiler.IdentityTransform(),
		Capabilities: compiler.Capabilities{
			// Junie supports only instructions.
		},
	}
}
