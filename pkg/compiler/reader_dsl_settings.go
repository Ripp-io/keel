package compiler

import (
	"encoding/json"
	"log/slog"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// readSettings populates ir.Settings and ir.AgentSpecific from keel.yml settings.
func (r *KeelDSLReader) readSettings(layerPath string, cfg *schema.KeelConfig, ir *LayerIR) {
	if cfg.Settings == nil {
		return
	}

	// Portable settings
	if cfg.Settings.Model != "" {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		ir.Settings["model"] = cfg.Settings.Model
	}

	// Portable concepts: set on IR directly, not in agent-specific.
	if cfg.Settings.Sandbox != nil {
		ir.Sandbox = cfg.Settings.Sandbox
	}
	if cfg.Settings.Attribution != nil {
		ir.Attribution = &CanonicalAttribution{
			CommitEnabled:  cfg.Settings.Attribution.Commit != "",
			CommitTemplate: cfg.Settings.Attribution.Commit,
			PREnabled:      cfg.Settings.Attribution.PR != "",
			PRTemplate:     cfg.Settings.Attribution.PR,
		}
	}

	// External agent-specific settings files.
	// Falls back to settings.json for Claude and cli-config.json for Cursor
	// when the agent-prefixed file doesn't exist (convention layers).
	// Hooks are extracted into ir.Hooks for proper merge/force-lock semantics.
	// Remaining keys stay agent-scoped to avoid cross-agent leakage.
	if cfg.Settings.Claude != "" {
		data := readSettingsWithFallbacks(layerPath, cfg.Settings.Claude, "settings.json")
		if data != nil {
			decomposeSettingsIntoIR(ir, "claude", data)
		}
	}
	if cfg.Settings.Cursor != "" {
		data := readSettingsWithFallbacks(layerPath, cfg.Settings.Cursor, "cli-config.json")
		if data != nil {
			decomposeSettingsIntoIR(ir, "cursor", data)
		}
	}
	if cfg.Settings.OpenCode != "" {
		data := readSettingsWithFallbacks(layerPath, cfg.Settings.OpenCode, "opencode.json")
		if data != nil {
			decomposeSettingsIntoIR(ir, "opencode", data)
		}
	}

	// Portable settings from the struct — all Settings struct fields are portable.
	// They go to ir.Settings (not ir.AgentSpecific) so writers can consume them.
	portableSettings := r.extractPortableSettings(cfg.Settings)
	if len(portableSettings) > 0 {
		if ir.Settings == nil {
			ir.Settings = make(map[string]any)
		}
		for k, v := range portableSettings {
			ir.Settings[k] = v
		}
	}
}

// decomposeSettingsIntoIR extracts hooks from a raw settings JSON map into
// ir.Hooks for proper merge semantics (force-locks, contribution tracking).
// All other keys stay in AgentSpecific[agent] to avoid cross-agent leakage.
func decomposeSettingsIntoIR(ir *LayerIR, agent string, data map[string]any) {
	// Extract hooks into ir.Hooks — they need the accumulator's contribution
	// system for force-locks and last-writer-wins per event.
	if hooks, ok := data["hooks"]; ok {
		if hookMap, ok := hooks.(map[string]any); ok {
			if ir.Hooks == nil {
				ir.Hooks = make(map[string]any)
			}
			for k, v := range hookMap {
				ir.Hooks[k] = v
			}
		}
		delete(data, "hooks")
	}

	// All remaining keys (permissions, model, agent-specific) → agent-scoped.
	if len(data) > 0 {
		if ir.AgentSpecific == nil {
			ir.AgentSpecific = make(map[string]map[string]any)
		}
		if ir.AgentSpecific[agent] == nil {
			ir.AgentSpecific[agent] = make(map[string]any)
		}
		for k, v := range data {
			ir.AgentSpecific[agent][k] = v
		}
	}

	// Extract portable concepts (attribution, sandbox) from agent-specific
	// settings loaded via convention fallback (settings.json / cli-config.json).
	ExtractPortableConcepts(ir, agent)
}

// extractPortableSettings converts the typed Settings struct into a map[string]any
// for storage in ir.Settings. ALL Settings struct fields are portable — they are
// available to every agent adapter. Writers ignore keys they don't recognize.
//
// This function relies on json.Marshal to convert the struct to a map. The json
// struct tags on Settings fields determine the output key names. If a json tag
// is missing or incorrect, the field will silently use Go's default naming
// (capitalized field name), which may not match what the agent expects.
//
// The contract: every field in schema.Settings MUST have a json tag that matches
// the agent's expected key name. The schema_test.go TestSettingsJSONTags test
// validates that all fields have json tags.
func (r *KeelDSLReader) extractPortableSettings(s *schema.Settings) map[string]any {
	data, err := json.Marshal(s)
	if err != nil {
		slog.Warn("failed to marshal settings for portable extraction", "error", err)
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		slog.Warn("failed to unmarshal settings map", "error", err)
		return nil
	}
	// Remove passthrough keys (file references to agent-specific settings).
	delete(result, "claude")
	delete(result, "cursor")
	// Remove portable concepts that are set directly on IR fields.
	delete(result, "sandbox")     // set on IR.Sandbox
	delete(result, "attribution") // set on IR.Attribution
	// Strip empty sub-maps that arise from non-nil pointers to zero-value structs.
	stripEmptyMaps(result)
	return result
}

// stripEmptyMaps recursively removes map entries whose value is an empty map.
// This handles the case where a non-nil pointer to a zero-value struct
// produces {"key":{}} after json.Marshal.
func stripEmptyMaps(m map[string]any) {
	stripEmptyMapsDepth(m, 0)
}

func stripEmptyMapsDepth(m map[string]any, depth int) {
	if depth > 10 {
		return
	}
	for k, v := range m {
		if subMap, ok := v.(map[string]any); ok {
			stripEmptyMapsDepth(subMap, depth+1)
			if len(subMap) == 0 {
				delete(m, k)
			}
		}
	}
}
