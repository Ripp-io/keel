package compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// TestGovernanceTracking_OverrideConflicts verifies that override conflicts
// are recorded when a layer attempts to modify force-locked content.
func TestGovernanceTracking_OverrideConflicts(t *testing.T) {
	tempDir := t.TempDir()

	// Layer A: force-locks settings
	layerA := filepath.Join(tempDir, "layer-a")
	os.MkdirAll(layerA, 0755)
	os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
settings:
  model: claude-sonnet-4-20250514
files:
  settings.json:
    force: true
`), 0644)

	// Layer B: attempts to add settings
	layerB := filepath.Join(tempDir, "layer-b")
	os.MkdirAll(layerB, 0755)
	os.WriteFile(filepath.Join(layerB, "keel.yml"), []byte(`
schema: v1
name: layer-b
settings:
  model: claude-opus-4
`), 0644)

	outputDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outputDir, 0755)

	// Compile with both layers
	result, err := Compile("claude", []config.Layer{
		{Path: layerA, Format: config.FormatKeel},
		{Path: layerB, Format: config.FormatKeel},
	}, outputDir, CompileOptions{})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Verify governance was recorded
	if result.Governance.Record == nil {
		t.Fatal("expected Governance to be non-nil")
	}

	// Should have a conflict for settings
	if len(result.Governance.Record.OverrideConflicts) == 0 {
		t.Fatal("expected at least one override conflict")
	}

	found := false
	for _, c := range result.Governance.Record.OverrideConflicts {
		if c.Category == "settings" && c.AttemptedBy == "layer-b" && c.LockedBy == "layer-a (settings.json)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected override conflict for settings not found: %+v", result.Governance.Record.OverrideConflicts)
	}

	// Verify manifest contains governance
	manifestPath := filepath.Join(outputDir, ".claude", ".keel.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var mf manifest.CompiledManifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}

	if mf.Governance == nil {
		t.Fatal("manifest governance is nil")
	}

	if len(mf.Governance.OverrideConflicts) == 0 {
		t.Fatal("manifest has no override conflicts")
	}
}

// TestGovernanceTracking_PermissionsPolicy verifies that permissions policy
// is captured in the governance record.
func TestGovernanceTracking_PermissionsPolicy(t *testing.T) {
	tempDir := t.TempDir()

	layerA := filepath.Join(tempDir, "layer-a")
	os.MkdirAll(layerA, 0755)
	os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
permissions:
  policy:
    merge: additive
  rules:
    - source: permissions.json
`), 0644)
	os.WriteFile(filepath.Join(layerA, "permissions.json"), []byte(`{"allow": ["Read"]}`), 0644)

	outputDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outputDir, 0755)

	result, err := Compile("claude", []config.Layer{{Path: layerA, Format: config.FormatKeel}}, outputDir, CompileOptions{})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if result.Governance.Record == nil {
		t.Fatal("expected Governance to be non-nil")
	}

	if result.Governance.Record.PermissionsPolicy == nil {
		t.Fatal("expected PermissionsPolicy to be recorded")
	}

	if result.Governance.Record.PermissionsPolicy.Merge != "additive" {
		t.Errorf("expected merge=additive, got %s", result.Governance.Record.PermissionsPolicy.Merge)
	}

	if result.Governance.Record.PermissionsPolicy.SetBy != "layer-a" {
		t.Errorf("expected SetBy=layer-a, got %s", result.Governance.Record.PermissionsPolicy.SetBy)
	}

	// Verify in manifest
	manifestPath := filepath.Join(outputDir, ".claude", ".keel.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var mf manifest.CompiledManifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}

	if mf.Governance == nil || mf.Governance.PermissionsPolicy == nil {
		t.Fatal("manifest governance permissions policy is nil")
	}

	if mf.Governance.PermissionsPolicy.Merge != "additive" {
		t.Errorf("manifest: expected merge=additive, got %s", mf.Governance.PermissionsPolicy.Merge)
	}
}

// TestGovernanceTracking_NoGovernance verifies that when no governance
// decisions are made, the Governance field is nil.
func TestGovernanceTracking_NoGovernance(t *testing.T) {
	tempDir := t.TempDir()

	layerA := filepath.Join(tempDir, "layer-a")
	os.MkdirAll(layerA, 0755)
	os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
instructions:
  - source: instructions.md
`), 0644)
	os.WriteFile(filepath.Join(layerA, "instructions.md"), []byte("Test instructions"), 0644)

	outputDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outputDir, 0755)

	result, err := Compile("claude", []config.Layer{{Path: layerA, Format: config.FormatKeel}}, outputDir, CompileOptions{})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if result.Governance.Record != nil {
		t.Errorf("expected Governance to be nil when no governance decisions made, got: %+v", result.Governance.Record)
	}

	// Verify manifest omits governance field
	manifestPath := filepath.Join(outputDir, ".claude", ".keel.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(manifestData, &raw); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}

	if _, hasGov := raw["governance"]; hasGov {
		t.Error("expected governance field to be omitted from manifest when nil")
	}
}

// TestGovernanceTracking_MCPPolicy verifies that MCP policy is captured.
func TestGovernanceTracking_MCPPolicy(t *testing.T) {
	tempDir := t.TempDir()

	layerA := filepath.Join(tempDir, "layer-a")
	os.MkdirAll(layerA, 0755)
	os.WriteFile(filepath.Join(layerA, "keel.yml"), []byte(`
schema: v1
name: layer-a
mcp:
  policy:
    enableAll: true
  servers:
    - source: mcp.json
`), 0644)
	os.WriteFile(filepath.Join(layerA, "mcp.json"), []byte(`{"mcpServers": {"test": {"command": "echo"}}}`), 0644)

	outputDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outputDir, 0755)

	result, err := Compile("claude", []config.Layer{{Path: layerA, Format: config.FormatKeel}}, outputDir, CompileOptions{})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if result.Governance.Record == nil {
		t.Fatal("expected Governance to be non-nil")
	}

	if result.Governance.Record.MCPPolicy == nil {
		t.Fatal("expected MCPPolicy to be recorded")
	}

	if result.Governance.Record.MCPPolicy.EnableAll == nil || !*result.Governance.Record.MCPPolicy.EnableAll {
		t.Error("expected EnableAll to be true")
	}

	// Verify in manifest
	manifestPath := filepath.Join(outputDir, ".claude", ".keel.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var mf manifest.CompiledManifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}

	if mf.Governance == nil || mf.Governance.MCPPolicy == nil {
		t.Fatal("manifest governance MCP policy is nil")
	}

	if mf.Governance.MCPPolicy.EnableAll == nil || !*mf.Governance.MCPPolicy.EnableAll {
		t.Error("manifest: expected EnableAll to be true")
	}
}
