package compiler

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
)

// WriteSharedOutput runs a shared accumulation phase without agent-specific
// transforms to produce universal instructions and MCP config. It splits
// agent-conditional content and writes shared files (AGENTS.md, .mcp.json).
//
// The shared phase runs the normal accumulation pipeline (Read → Fold → Finalize)
// but WITHOUT setting keel.agent, so conditionals referencing keel.agent remain
// unevaluated. After accumulation, SplitAgentConditionals partitions content
// into universal (no agent reference) and per-agent addenda.
//
// Returns SharedResult with universal content, agent addenda, and file records.
func WriteSharedOutput(layers []config.Layer, outputDir string, sharedInstrPath string, agents map[string]string, opts CompileOptions) (*SharedResult, error) {
	slog.Debug("running shared compilation phase", "outputDir", outputDir, "agents", len(agents))

	// Compute absolute output dir for auto-vars
	absOutputDir := outputDir
	if !filepath.IsAbs(absOutputDir) {
		if abs, err := filepath.Abs(absOutputDir); err == nil {
			absOutputDir = abs
		}
	}

	// Build fold config with no categories (shared phase writes only instructions and MCP)
	foldConfig := FoldConfig{
		Categories: nil, // Categories are agent-specific
	}

	// Initialize accumulation state WITHOUT setting keel.agent.
	// Skip conditional evaluation so SplitAgentConditionals can partition
	// agent-specific content before it gets stripped.
	state := NewAccumulationState(foldConfig, opts, absOutputDir, len(layers))
	state.skipConditionals = true

	// Read → Fold each layer using DSL reader (shared phase uses format-agnostic reader)
	for _, layer := range layers {
		var ir *LayerIR
		var layerLabel string
		var err error

		// Use cache when available
		if opts.layerCache != nil {
			// Shared phase is agent-agnostic — use ReadOrCacheDSL which
			// doesn't require a profile. Cache key is layer path only.
			ir, layerLabel, err = opts.layerCache.ReadOrCacheDSL(layer, opts.Offline)
		} else {
			layerLabel = filepath.Base(layer.Path)
			reader := &KeelDSLReader{Offline: opts.Offline}
			ir, err = reader.Read(layer.Path)
		}

		if err != nil {
			return nil, fmt.Errorf("reading layer %q for shared phase: %w", layer.Path, err)
		}

		if err := state.Fold(ir, layerLabel); err != nil {
			return nil, fmt.Errorf("merging layer %q in shared phase: %w", layer.Path, err)
		}
	}

	// Run GOVERN → RESOLVE (no agent-specific transforms in shared phase).
	accumulated, err := runPostFoldPipeline(state, nil, outputDir, opts)
	if err != nil {
		return nil, err
	}

	// Split instructions into universal and agent-specific addenda.
	// At this point, conditionals are still intact (skipConditionals=true).
	// SplitAgentConditionals partitions by keel.agent references, then we
	// evaluate remaining non-agent conditionals on the universal part.
	universalInstructions := []byte(accumulated.Content.Instructions)
	var agentAddendaBytes map[string][]byte
	if len(universalInstructions) > 0 {
		universalInstructions, agentAddendaBytes = merge.SplitAgentConditionals(universalInstructions)

		// Evaluate non-agent conditionals (e.g., keel.scope) on the universal part.
		if len(accumulated.Tracking.Variables) > 0 {
			universalInstructions = merge.ProcessConditionals(universalInstructions, accumulated.Tracking.Variables)
		}
	}

	// Convert agent addenda from []byte to string
	agentAddenda := make(map[string]string)
	for agent, content := range agentAddendaBytes {
		agentAddenda[agent] = string(content)
	}

	// Split MCP into universal and agent-specific
	universalMCP := accumulated.Content.MCP
	var agentSpecificMCP map[string]map[string]any
	if len(universalMCP) > 0 {
		universalMCP, agentSpecificMCP = merge.SplitAgentConditionJSON(universalMCP)
	}

	// Merge agent-specific MCP back into agentAddenda as instructions
	// (MCP conditionals get written as part of the agent's .mcp.json, not instruction addenda)
	// So we keep them separate for now and let per-agent compilation handle them

	var fileRecords []manifest.FileRecord

	// Write AGENTS.md if there's universal content
	if len(universalInstructions) > 0 {
		agentsPath := filepath.Join(outputDir, sharedInstrPath)
		if err := os.MkdirAll(filepath.Dir(agentsPath), 0755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", sharedInstrPath, err)
		}
		if err := os.WriteFile(agentsPath, universalInstructions, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", sharedInstrPath, err)
		}
		fileRecords = append(fileRecords, manifest.FileRecord{
			Output:   sharedInstrPath,
			Strategy: "user-editable",
			Hash:     manifest.HashContent(universalInstructions),
		})
		slog.Debug("wrote shared instructions", "path", sharedInstrPath, "size", len(universalInstructions))
	}

	// Write .mcp.json if there's universal MCP config
	if len(universalMCP) > 0 {
		mcpPath := filepath.Join(outputDir, ".mcp.json")
		if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
			return nil, fmt.Errorf("creating directory for .mcp.json: %w", err)
		}
		hash, err := WriteJSONFileWithHash(mcpPath, universalMCP)
		if err != nil {
			return nil, fmt.Errorf("writing .mcp.json: %w", err)
		}
		fileRecords = append(fileRecords, manifest.FileRecord{
			Output:   ".mcp.json",
			Strategy: "deep-merge",
			Hash:     hash,
		})
		slog.Debug("wrote shared MCP config", "path", ".mcp.json", "servers", len(universalMCP))
	}

	// Build SharedResult
	result := &SharedResult{
		UniversalInstructions: string(universalInstructions),
		UniversalMCP:          universalMCP,
		AgentAddenda:          agentAddenda,
		FileRecords:           fileRecords,
	}

	// KNOWN LIMITATION: Agent-specific MCP servers (those inside keel-if blocks
	// referencing keel.agent) are detected during the shared phase but silently
	// discarded. Per-agent compilation handles its own MCP independently, so this
	// only affects the shared .mcp.json output. Agent-conditional MCP servers will
	// not appear in .mcp.json — they appear in each agent's own MCP output.
	if len(agentSpecificMCP) > 0 {
		slog.Warn("agent-specific MCP servers excluded from shared output", "count", len(agentSpecificMCP))
	}

	return result, nil
}
