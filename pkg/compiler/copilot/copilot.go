// Package copilot provides the GitHub Copilot adapter for the Keel compiler.
// Copilot uses a single instruction file at .github/copilot-instructions.md.
// It supports instructions only -- no settings, hooks, MCP, permissions,
// or categories.
package copilot

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func init() {
	config.RegisterAgent(config.AgentMeta{Format: "copilot", Files: []string{"copilot-instructions.md"}, Dirs: []string{".github"}})
	compiler.RegisterCompiler("copilot", Compile)
	compiler.RegisterSimpleAdapter("copilot", ProjectProfile)
}

// Compile produces Copilot-native configuration output.
func Compile(layers []config.Layer, outputDir string, opts compiler.CompileOptions) (*compiler.CompileResult, error) {
	return compiler.CompileWithProfile(ProjectProfile(), layers, outputDir, opts)
}

// ProjectProfile returns the project-level AgentProfile for GitHub Copilot.
func ProjectProfile() compiler.AgentProfile {
	return compiler.AgentProfile{
		Name:              "copilot",
		OutputRoot:        ".copilot",
		InstructionOutput: ".github/copilot-instructions.md",
		SharedInstructionOutput: "AGENTS.md",
		SharedMCPOutput:         ".mcp.json",
		InstructionShimTemplate: "", // Instruction-only agent — no shim needed
		SettingsFile:            "",
		MCPOutput:               "",
		HooksInSettings:         false,
		Categories:        nil,
		InstructionCandidates: []string{
			"copilot-instructions.md",
		},
		HookKeyTransform:  compiler.IdentityTransform(),
		ToolNameTransform: compiler.IdentityTransform(),
		Capabilities: compiler.Capabilities{
			// Copilot supports only instructions.
		},
	}
}
