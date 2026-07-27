package cursor

import (
	"log/slog"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

func init() {
	compiler.RegisterWriter("cursor", func() compiler.Writer { return &Writer{} })
}

// Writer serializes an AccumulatedResult to Cursor's native format.
// Output paths are determined by the AgentProfile (scope-dependent).
type Writer struct{}

func (w *Writer) Write(result *compiler.AccumulatedResult, profile compiler.AgentProfile, outputDir string, opts compiler.CompileOptions) ([]manifest.FileRecord, error) {
	var records []manifest.FileRecord
	agentDir := filepath.Join(outputDir, profile.OutputRoot)

	// 1. Create output directories.
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, err
	}

	// 2. Instructions (Cursor wraps with MDC frontmatter)
	if rec, ok, err := compiler.WriteInstructions(result, profile, outputDir, WrapMDC); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3. Settings — result.Settings is already fully assembled
	if rec, ok, err := compiler.WriteSettings(result, profile, outputDir); err != nil {
		return nil, err
	} else if ok {
		records = append(records, rec)
	}

	// 3b. Sandbox (separate file, driven by profile.SandboxOutput)
	if profile.SandboxOutput != "" && result.Content.Sandbox != nil {
		cursorSandbox := buildCursorSandboxFromCanonical(result.Content.Sandbox)
		if len(cursorSandbox) > 0 {
			sandboxPath := filepath.Join(outputDir, profile.SandboxOutput)
			if err := os.MkdirAll(filepath.Dir(sandboxPath), 0755); err != nil {
				return nil, err
			}
			hash, err := compiler.WriteJSONFileWithHash(sandboxPath, cursorSandbox)
			if err != nil {
				return nil, err
			}
			records = append(records, manifest.FileRecord{
				Output:   profile.SandboxOutput,
				Strategy: "deep-merge",
				Hash:     hash,
			})
		}
	}

	// 4. Hooks (separate file — result.Hooks already has transformed keys)
	if !profile.HooksInSettings && len(result.Content.Hooks) > 0 {
		hooksPath := filepath.Join(agentDir, "hooks.json")
		hash, err := compiler.WriteJSONFileWithHash(hooksPath, result.Content.Hooks)
		if err != nil {
			return nil, err
		}
		records = append(records, manifest.FileRecord{
			Output:   filepath.Join(profile.OutputRoot, "hooks.json"),
			Strategy: "deep-merge",
			Hash:     hash,
		})
	}

	// 5. MCP
	if profile.MCPOutput != "" && len(result.Content.MCP) > 0 {
		mcpPath := filepath.Join(outputDir, profile.MCPOutput)
		if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
			return nil, err
		}
		hash, err := compiler.WriteJSONFileWithHash(mcpPath, result.Content.MCP)
		if err != nil {
			return nil, err
		}
		records = append(records, manifest.FileRecord{
			Output:   profile.MCPOutput,
			Strategy: "deep-merge",
			Hash:     hash,
		})
	}

	// 6-10. Common output (categories, bin, content blocks, config blocks, state)
	commonRecords, err := compiler.WriteCommonOutput(result, profile, outputDir)
	if err != nil {
		return nil, err
	}
	records = append(records, commonRecords...)

	// 6. Warn about MCP policy (Cursor doesn't support governance controls)
	if result.Governance.MCPPolicy != nil {
		if len(result.Governance.MCPPolicy.Allowed) > 0 || len(result.Governance.MCPPolicy.Denied) > 0 || result.Governance.MCPPolicy.EnableAll != nil {
			slog.Warn("mcp.policy is not supported by Cursor, governance controls will be ignored")
		}
	}

	return records, nil
}

// buildCursorSandboxFromCanonical builds the Cursor-format sandbox map from
// the canonical SandboxConfig. It maps Cursor-only fields directly and
// translates portable fields (filesystem, network) to Cursor equivalents.
func buildCursorSandboxFromCanonical(sb *schema.SandboxConfig) map[string]any {
	result := make(map[string]any)

	// Cursor-only fields
	if sb.Type != "" {
		result["type"] = sb.Type
	}

	// Collect readwrite/readonly paths from Cursor-specific fields first.
	var rwPaths []string
	var roPaths []string

	if len(sb.AdditionalReadwritePaths) > 0 {
		rwPaths = append(rwPaths, sb.AdditionalReadwritePaths...)
	}
	if len(sb.AdditionalReadonlyPaths) > 0 {
		roPaths = append(roPaths, sb.AdditionalReadonlyPaths...)
	}
	if sb.DisableTmpWrite != nil {
		result["disableTmpWrite"] = *sb.DisableTmpWrite
	}
	if sb.EnableSharedBuildCache != nil {
		result["enableSharedBuildCache"] = *sb.EnableSharedBuildCache
	}

	// Portable fields mapped to Cursor format
	if sb.Filesystem != nil {
		if len(sb.Filesystem.AllowWrite) > 0 {
			rwPaths = append(rwPaths, sb.Filesystem.AllowWrite...)
		}
		if len(sb.Filesystem.AllowRead) > 0 {
			roPaths = append(roPaths, sb.Filesystem.AllowRead...)
		}
	}

	if len(rwPaths) > 0 {
		result["additionalReadwritePaths"] = rwPaths
	}
	if len(roPaths) > 0 {
		result["additionalReadonlyPaths"] = roPaths
	}

	// Network → networkPolicy
	if sb.Network != nil {
		np := make(map[string]any)
		if sb.Network.DefaultPolicy != "" {
			np["default"] = sb.Network.DefaultPolicy
		}
		if len(sb.Network.AllowedDomains) > 0 {
			np["allow"] = sb.Network.AllowedDomains
		}
		if len(sb.Network.DeniedDomains) > 0 {
			np["deny"] = sb.Network.DeniedDomains
		}
		if len(np) > 0 {
			result["networkPolicy"] = np
		}
	}

	return result
}
