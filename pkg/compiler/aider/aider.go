// Package aider provides the Aider adapter for the Keel compiler.
// Aider uses a single instruction file at CONVENTIONS.md in the project root.
// It supports instructions only -- no settings, hooks, MCP, permissions,
// or categories.
package aider

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "aider", Files: []string{"CONVENTIONS.md"}, Dirs: []string{".aider"}})
	compiler.RegisterCompiler("aider", Compile)
	compiler.RegisterSimpleAdapter("aider", ProjectProfile)
}

// Compile produces Aider-native configuration output.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(ProjectProfile(), layers, outputDir, opts)
}

// ProjectProfile returns the project-level AgentProfile for Aider.
func ProjectProfile() compiler.AgentProfile {
	return compiler.AgentProfile{
		Name:              "aider",
		OutputRoot:        ".aider",
		InstructionOutput: "CONVENTIONS.md",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // Instruction-only agent — no shim needed
		SettingsFile:            "",
		MCPOutput:               "",
		HooksInSettings:         false,
		Categories:        nil,
		InstructionCandidates: []string{
			"CONVENTIONS.md",
			"conventions.md",
		},
		HookKeyTransform:  compiler.IdentityTransform(),
		ToolNameTransform: compiler.IdentityTransform(),
		Capabilities: compiler.Capabilities{
			// Aider supports only instructions.
		},
	}
}
