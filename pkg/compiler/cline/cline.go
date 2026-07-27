// Package cline provides the Cline adapter for the Keel compiler.
// Cline uses a single instruction file at .clinerules in the project root.
// It supports instructions only -- no settings, hooks, MCP, permissions,
// or categories.
package cline

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "cline", Files: []string{".clinerules"}, Dirs: []string{".cline"}})
	compiler.RegisterCompiler("cline", Compile)
	compiler.RegisterSimpleAdapter("cline", ProjectProfile)
}

// Compile produces Cline-native configuration output.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(ProjectProfile(), layers, outputDir, opts)
}

// ProjectProfile returns the project-level AgentProfile for Cline.
func ProjectProfile() compiler.AgentProfile {
	return compiler.AgentProfile{
		Name:              "cline",
		OutputRoot:        ".cline",
		InstructionOutput: ".clinerules",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // Instruction-only agent — no shim needed
		SettingsFile:            "",
		MCPOutput:               "",
		HooksInSettings:         false,
		Categories:        nil,
		InstructionCandidates: []string{
			".clinerules",
		},
		HookKeyTransform:  compiler.IdentityTransform(),
		ToolNameTransform: compiler.IdentityTransform(),
		Capabilities: compiler.Capabilities{
			// Cline supports only instructions.
		},
	}
}
