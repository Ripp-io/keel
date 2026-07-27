package compiler

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// LayerCache caches LayerIR reads to avoid re-reading shared layers from disk.
// Only DSL layers (FormatKeel with keel.yml) produce agent-independent IRs and
// can be safely cached. Native-format and foreign-format layers are read
// per-agent since they may produce different IRs.
//
// Not thread-safe — used within a single CompileMulti call.
type LayerCache struct {
	entries map[string]*LayerIR // keyed by layer path
}

// NewLayerCache creates a new empty LayerCache.
func NewLayerCache() *LayerCache {
	return &LayerCache{entries: make(map[string]*LayerIR)}
}

// isDSLLayer checks if a layer has FormatKeel and contains a keel.yml file.
// Only DSL layers produce agent-independent IRs suitable for caching.
func isDSLLayer(layer config.Layer) bool {
	if layer.Format != config.FormatKeel {
		return false
	}
	keelYml := filepath.Join(layer.Path, "keel.yml")
	_, err := os.Stat(keelYml)
	return err == nil
}

// ReadOrCache reads a layer's IR, using the cache for DSL layers.
// Non-DSL layers (convention, native-format, foreign-format) are always
// read fresh since they may produce agent-specific IRs.
//
// The layerLabel return value is the layer name for accumulator tracking.
func (c *LayerCache) ReadOrCache(layer config.Layer, profile AgentProfile, myFormat config.LayerFormat, offline bool) (*LayerIR, string, error) {
	layerLabel := filepath.Base(layer.Path)

	// Check cache for DSL layers
	if isDSLLayer(layer) {
		if ir, ok := c.entries[layer.Path]; ok {
			cloned, err := ir.Clone()
			if err != nil {
				return nil, "", fmt.Errorf("cloning cached layer %q: %w", layer.Path, err)
			}
			return cloned, layerLabel, nil
		}
	}

	// Read the layer using the normal reader selection logic
	reader, err := selectReader(layer, profile, myFormat, offline)
	if err != nil {
		return nil, layerLabel, err
	}
	ir, err := reader.Read(layer.Path)
	if err != nil {
		return nil, layerLabel, err
	}

	// Cache DSL layers for reuse across agents
	if isDSLLayer(layer) {
		c.entries[layer.Path] = ir
	}

	return ir, layerLabel, nil
}

// ReadOrCacheDSL reads a DSL layer's IR without requiring an agent profile.
// DSL layers (FormatKeel with keel.yml) produce agent-independent IRs, so
// no profile or format parameter is needed. The cache key is based on the
// layer path alone.
//
// This is used by the shared compilation phase (WriteSharedOutput), which
// reads layers without a target agent context.
func (c *LayerCache) ReadOrCacheDSL(layer config.Layer, offline bool) (*LayerIR, string, error) {
	layerLabel := filepath.Base(layer.Path)

	// Check cache for DSL layers
	if isDSLLayer(layer) {
		if ir, ok := c.entries[layer.Path]; ok {
			cloned, err := ir.Clone()
			if err != nil {
				return nil, "", fmt.Errorf("cloning cached layer %q: %w", layer.Path, err)
			}
			return cloned, layerLabel, nil
		}
	}

	// DSL layers always use the KeelDSLReader
	reader := &KeelDSLReader{Offline: offline}
	ir, err := reader.Read(layer.Path)
	if err != nil {
		return nil, layerLabel, err
	}

	// Cache DSL layers for reuse across agents
	if isDSLLayer(layer) {
		c.entries[layer.Path] = ir
	}

	return ir, layerLabel, nil
}
