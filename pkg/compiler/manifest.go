package compiler

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// FindKeelState auto-discovers .keel.json in a directory.
// It checks:
//  1. <dir>/.keel.json directly
//  2. Known agent subdirs from config.AgentList (e.g. .claude, .cursor)
//
// Returns the loaded manifest(s) and their paths.
// If exactly one is found, returns it. If multiple are found, returns all.
// Returns nil, nil if none found.
func FindKeelState(dir string) ([]*manifest.CompiledManifest, []string, error) {
	// Collect all agent directories from registered agents
	var agentDirs []string
	for _, agent := range config.AgentList() {
		agentDirs = append(agentDirs, agent.Dirs...)
	}

	return manifest.FindKeelState(dir, agentDirs)
}

// writeKeelState merges compilation results and layer hashes into .keel.json
// inside the agent's config directory.
func writeKeelState(result *CompileResult, layers []config.Layer, outputDir string, opts CompileOptions) error {
	version := keelBuildVersion()

	scope := string(opts.Scope)
	if scope == "" {
		scope = string(ScopeProject)
	}

	cm := manifest.CompiledManifest{
		Agent:       result.Output.Agent,
		Scope:       scope,
		CompiledAt:  time.Now().UTC().Format(time.RFC3339),
		KeelVersion: version,
		Name:        result.Tracking.Name,
		Description: result.Tracking.Description,
		Repos:       result.Tracking.Repos,
	}

	// config.yml / --name override (takes precedence over layer-derived name)
	if opts.Name != "" {
		cm.Name = opts.Name
	}

	// Find first init.d/ directory across layers
	for _, layer := range layers {
		initDir := filepath.Join(layer.Path, "init.d")
		if info, err := os.Stat(initDir); err == nil && info.IsDir() {
			cm.InitDir = initDir
			break
		}
	}

	// Layer stack with hashes (merged from old lockfile concept)
	for _, layer := range layers {
		lr := manifest.LayerRecord{
			Path:    layer.Path,
			Source:  layer.Source,
			Type:    layer.Type,
			Format:  string(layer.Format),
			Version: layer.Version,
		}
		if name, ok := result.Tracking.LayerNames[layer.Path]; ok {
			lr.Name = name
		}
		// Compute content hash for each layer
		hash, err := manifest.HashLayerDir(layer.Path)
		if err != nil {
			slog.Warn("failed to hash layer for state file", "layer", layer.Path, "error", err)
		} else {
			lr.Hash = hash
		}
		cm.LayerStack = append(cm.LayerStack, lr)
	}

	// File operation records
	for _, f := range result.Output.Files {
		fr := manifest.FileRecord{
			Output:   f.Output,
			Strategy: f.Strategy,
			Hash:     f.Hash,
		}
		for _, s := range f.Sources {
			fr.Sources = append(fr.Sources, manifest.FileSource{
				Layer: s.Layer,
				File:  s.File,
			})
		}
		cm.FileOps = append(cm.FileOps, fr)
	}

	// Force-locks
	cm.ForceLocked = result.Governance.ForceLocked

	// Installed deps — external binary dependencies resolved during compilation.
	cm.InstalledDeps = result.Dependencies.InstalledDeps

	// Overlay file paths — individual files Keel wrote into overlay directories.
	// Used by cleanup to avoid walking overlay dirs and claiming unrelated files.
	cm.Overlays = result.Output.Overlays

	// Managed MCP server names for merge-into files.
	cm.ManagedMCPServers = result.Dependencies.ManagedMCPServers

	// Governance decisions for compliance reporting.
	cm.Governance = result.Governance.Record

	// Effective state for compliance reporting and drift detection.
	cm.EffectiveState = result.State.EffectiveState

	statePath := filepath.Join(outputDir, result.Output.OutputRoot, ".keel.json")
	return manifest.WriteKeelState(statePath, &cm)
}
