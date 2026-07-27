package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/merge"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

const DefaultAgentToolsThreshold = 10

// checkAgentTools discovers agent definitions and validates that allowed-tools
// lists do not exceed the configured threshold. All findings are warnings.
func checkAgentTools(result *Result, layerPath string, keelExists bool, cfg *schema.KeelConfig) {
	// Resolve threshold from config or use default.
	threshold := DefaultAgentToolsThreshold
	if cfg != nil && cfg.ValidateOpts != nil && cfg.ValidateOpts.AgentTools != nil {
		threshold = *cfg.ValidateOpts.AgentTools
	}

	// 0 = disabled.
	if threshold == 0 {
		return
	}

	var agentFiles []string // paths to agent markdown files

	// Discovery logic mirrors validateSkills pattern in validate.go.
	if keelExists && cfg != nil {
		// DSL layer: resolve agent FileRef entries to files.
		for _, ref := range cfg.Agents {
			target := ref.Source
			if target == "" {
				continue
			}
			fullPath := filepath.Join(layerPath, target)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue // missing ref already reported by checkFileRefs
			}

			if !info.IsDir() {
				// Single file reference.
				if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
					agentFiles = append(agentFiles, fullPath)
				}
				continue
			}

			// Directory reference — enumerate all .md files.
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
					agentFiles = append(agentFiles, filepath.Join(fullPath, e.Name()))
				}
			}
		}
	} else {
		// Native layer: check standard agent directory locations.
		candidates := []string{
			filepath.Join(layerPath, "agents"),
			filepath.Join(layerPath, ".claude", "agents"),
			filepath.Join(layerPath, ".cursor", "agents"),
		}
		for _, dir := range candidates {
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
					agentFiles = append(agentFiles, filepath.Join(dir, e.Name()))
				}
			}
		}
	}

	if len(agentFiles) == 0 {
		return // no agents found, silent
	}

	agentsChecked := 0
	allUnderThreshold := true

	// Check each agent file.
	for _, path := range agentFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Extract frontmatter to get allowed-tools.
		fm, err := merge.ExtractRawFrontmatter(data)
		if err != nil || fm == nil {
			continue // no frontmatter or parse error — not a valid agent
		}

		// Extract allowed-tools from frontmatter.
		allowedTools, ok := fm["allowed-tools"]
		if !ok {
			continue // no allowed-tools field
		}

		// allowed-tools is a YAML list, which unmarshals to []any.
		toolsList, ok := allowedTools.([]any)
		if !ok {
			continue // unexpected type
		}

		toolCount := len(toolsList)

		// Get agent name from frontmatter, or fall back to filename.
		agentName := ""
		if name, ok := fm["name"].(string); ok {
			agentName = name
		} else {
			agentName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}

		agentsChecked++

		// Warn if over threshold.
		if toolCount > threshold {
			result.addWarn(fmt.Sprintf("tool count: agent %q has %d tools (threshold: %d)", agentName, toolCount, threshold))
			allUnderThreshold = false
		}
	}

	// If agents were checked and all are under threshold, emit a pass.
	if agentsChecked > 0 && allUnderThreshold {
		result.addPass("tool count: agent tool lists within threshold")
	}
}
