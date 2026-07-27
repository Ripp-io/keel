package compiler

import (
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// computeStats depends on BuildEffectiveState from effective.go
// for consistent metric extraction (single extraction path).

// CompileStats holds content metrics computed from the accumulated result.
type CompileStats struct {
	Sections    []SectionStat
	MCPServers  int
	HookEvents  int
	TotalBytes  int
	TotalTokens int
}

// SectionStat holds metrics for one content section.
type SectionStat struct {
	Name   string // "instructions", "skills", "agents", "commands", "rules", "loop", "worktreeinclude"
	Count  int    // number of items (0 for instructions/loop/worktreeinclude since they're single blobs)
	Bytes  int
	Tokens int
}

// computeStats calculates content metrics from the accumulated result.
// It derives most metrics from the effective state to ensure a single extraction path.
// Token estimates use the (bytes + 3) / 4 heuristic.
// Only sections with content (bytes > 0 or count > 0) are included.
func computeStats(result *AccumulatedResult, effectiveState *manifest.EffectiveState) *CompileStats {
	stats := &CompileStats{}

	if effectiveState == nil {
		return stats
	}

	// Instructions — from effective state
	if effectiveState.Instructions != nil {
		stats.Sections = append(stats.Sections, SectionStat{
			Name:   "instructions",
			Count:  0,
			Bytes:  effectiveState.Instructions.Bytes,
			Tokens: effectiveState.Instructions.TokenEstimate,
		})
	}

	// Categories — from effective state
	for _, cat := range effectiveState.Categories {
		// Compute bytes from the raw result (effective state doesn't store bytes)
		bytes := 0
		if files, ok := result.Content.Categories[cat.Name]; ok {
			for _, cf := range files {
				if !cf.IsDir {
					bytes += len(cf.Content)
				}
			}
		}

		stats.Sections = append(stats.Sections, SectionStat{
			Name:   cat.Name,
			Count:  cat.FileCount,
			Bytes:  bytes,
			Tokens: estimateTokens(bytes),
		})
	}

	// ContentBlocks — not in effective state, compute directly
	for name, content := range result.Content.ContentBlocks {
		if len(content) == 0 {
			continue
		}
		bytes := len(content)
		stats.Sections = append(stats.Sections, SectionStat{
			Name:   name,
			Count:  0,
			Bytes:  bytes,
			Tokens: estimateTokens(bytes),
		})
	}

	// MCP servers — from effective state
	stats.MCPServers = len(effectiveState.MCPServers)

	// Hook events — from effective state
	stats.HookEvents = len(effectiveState.Hooks)

	// Compute totals
	for _, section := range stats.Sections {
		stats.TotalBytes += section.Bytes
		stats.TotalTokens += section.Tokens
	}

	return stats
}

// estimateTokens uses the (bytes + 3) / 4 heuristic for token estimation.
func estimateTokens(bytes int) int {
	return (bytes + 3) / 4
}
