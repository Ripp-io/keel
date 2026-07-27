package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestLayerCache_DSLLayerCaching(t *testing.T) {
	// Create a temporary DSL layer with keel.yml
	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "test-layer")
	if err := os.MkdirAll(layerPath, 0755); err != nil {
		t.Fatalf("creating layer dir: %v", err)
	}

	keelYml := filepath.Join(layerPath, "keel.yml")
	content := `schema: v1
name: test-layer
description: Test layer for cache
`
	if err := os.WriteFile(keelYml, []byte(content), 0644); err != nil {
		t.Fatalf("writing keel.yml: %v", err)
	}

	layer := config.Layer{
		Path:   layerPath,
		Format: config.FormatKeel,
	}

	cache := NewLayerCache()
	profile := testProfile()

	// First read should populate the cache
	ir1, header1, err := cache.ReadOrCache(layer, profile, config.FormatKeel, false)
	if err != nil {
		t.Fatalf("first ReadOrCache failed: %v", err)
	}
	if header1 != "test-layer" {
		t.Errorf("expected header 'test-layer', got %q", header1)
	}
	if ir1.Name != "test-layer" {
		t.Errorf("expected name 'test-layer', got %q", ir1.Name)
	}

	// Second read should return a cloned IR from cache (same content, different pointer)
	ir2, _, err := cache.ReadOrCache(layer, profile, config.FormatKeel, false)
	if err != nil {
		t.Fatalf("second ReadOrCache failed: %v", err)
	}

	// Verify content equality (cache hit returns a clone for mutation safety)
	if ir2.Name != ir1.Name {
		t.Errorf("cached IR name mismatch: got %q, want %q", ir2.Name, ir1.Name)
	}
	if ir2.Description != ir1.Description {
		t.Errorf("cached IR description mismatch: got %q, want %q", ir2.Description, ir1.Description)
	}

	// Verify it's a different pointer (clone, not shared reference)
	if ir1 == ir2 {
		t.Errorf("expected cloned IR (different pointer) from cache, got same pointer")
	}

	// Verify cache entry exists
	if len(cache.entries) != 1 {
		t.Errorf("expected 1 cache entry, got %d", len(cache.entries))
	}
}

func TestLayerCache_ConventionLayerNotCached(t *testing.T) {
	// Create a temporary convention layer (FormatKeel WITHOUT keel.yml)
	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "convention-layer")
	claudeDir := filepath.Join(layerPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("creating layer dir: %v", err)
	}

	layer := config.Layer{
		Path:   layerPath,
		Format: config.FormatKeel,
	}

	cache := NewLayerCache()
	profile := testProfile()

	// Read the convention layer
	ir1, _, err := cache.ReadOrCache(layer, profile, config.FormatKeel, false)
	if err != nil {
		t.Fatalf("ReadOrCache failed: %v", err)
	}

	// Read again
	ir2, _, err := cache.ReadOrCache(layer, profile, config.FormatKeel, false)
	if err != nil {
		t.Fatalf("second ReadOrCache failed: %v", err)
	}

	// Convention layers should NOT be cached (different pointers)
	// Note: This assertion would fail if the readers happen to return
	// identical IRs by accident. The key point is that no cache entry exists.
	if len(cache.entries) != 0 {
		t.Errorf("expected 0 cache entries for convention layer, got %d", len(cache.entries))
	}

	// IRs might be different pointers (this is implementation-dependent,
	// but we verify the cache is empty above which is the key test)
	_ = ir1
	_ = ir2
}

func TestLayerCache_NativeLayerNotCached(t *testing.T) {
	// Create a temporary native Claude layer
	tmpDir := t.TempDir()
	layerPath := filepath.Join(tmpDir, "native-layer")
	claudeDir := filepath.Join(layerPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("creating layer dir: %v", err)
	}

	layer := config.Layer{
		Path:   layerPath,
		Format: config.LayerFormat("claude"), // native format
	}

	cache := NewLayerCache()
	profile := testProfile()

	// Read the native layer
	_, _, err := cache.ReadOrCache(layer, profile, "claude", false)
	if err != nil {
		t.Fatalf("ReadOrCache failed: %v", err)
	}

	// Verify no cache entry (native layers are not cached)
	if len(cache.entries) != 0 {
		t.Errorf("expected 0 cache entries for native layer, got %d", len(cache.entries))
	}
}

func TestIsDSLLayer(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		setup    func() config.Layer
		expected bool
	}{
		{
			name: "DSL layer with keel.yml",
			setup: func() config.Layer {
				layerPath := filepath.Join(tmpDir, "dsl-layer")
				os.MkdirAll(layerPath, 0755)
				os.WriteFile(filepath.Join(layerPath, "keel.yml"), []byte("schema: v1\n"), 0644)
				return config.Layer{Path: layerPath, Format: config.FormatKeel}
			},
			expected: true,
		},
		{
			name: "Convention layer without keel.yml",
			setup: func() config.Layer {
				layerPath := filepath.Join(tmpDir, "conv-layer")
				os.MkdirAll(layerPath, 0755)
				return config.Layer{Path: layerPath, Format: config.FormatKeel}
			},
			expected: false,
		},
		{
			name: "Native layer",
			setup: func() config.Layer {
				layerPath := filepath.Join(tmpDir, "native-layer")
				os.MkdirAll(layerPath, 0755)
				os.WriteFile(filepath.Join(layerPath, "keel.yml"), []byte("schema: v1\n"), 0644)
				return config.Layer{Path: layerPath, Format: "claude"}
			},
			expected: false, // Format is not FormatKeel
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layer := tt.setup()
			result := isDSLLayer(layer)
			if result != tt.expected {
				t.Errorf("isDSLLayer() = %v, want %v", result, tt.expected)
			}
		})
	}
}
