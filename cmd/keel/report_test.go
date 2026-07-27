package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/report"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestReportCmd_JSONOutput(t *testing.T) {
	tempDir := t.TempDir()

	// Create a minimal manifest
	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
		Name:        "test-project",
		LayerStack: []manifest.LayerRecord{
			{Path: "/layer-a", Name: "layer-a", Type: "keel", Format: "keel", Hash: "abc123"},
		},
	}

	manifestPath := filepath.Join(tempDir, ".claude", ".keel.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	data, _ := json.MarshalIndent(mf, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	// Run report command
	cmd := newReportCmd()
	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetArgs([]string{tempDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("report command failed: %v", err)
	}

	// Verify JSON output
	outputStr := strings.TrimSpace(output.String())
	var r report.Report
	if err := json.Unmarshal([]byte(outputStr), &r); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, outputStr)
	}

	if r.SchemaVersion == "" {
		t.Error("expected schema_version in output")
	}
	if r.Compilation.Agent != "claude" {
		t.Errorf("expected agent claude, got %s", r.Compilation.Agent)
	}
	if len(r.LayerBOM) != 1 {
		t.Errorf("expected 1 layer, got %d", len(r.LayerBOM))
	}
}

func TestReportCmd_YAMLOutput(t *testing.T) {
	tempDir := t.TempDir()

	mf := &manifest.CompiledManifest{
		Agent:       "cursor",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "user",
	}

	manifestPath := filepath.Join(tempDir, ".cursor", ".keel.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	data, _ := json.MarshalIndent(mf, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	cmd := newReportCmd()
	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--format", "yaml", tempDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("report command failed: %v", err)
	}

	// Verify YAML output can be parsed
	outputStr := strings.TrimSpace(output.String())
	var r report.Report
	if err := yaml.Unmarshal([]byte(outputStr), &r); err != nil {
		t.Fatalf("failed to parse YAML output: %v\nOutput: %s", err, outputStr)
	}

	if r.Compilation.Agent != "cursor" {
		t.Errorf("expected agent cursor, got %s", r.Compilation.Agent)
	}
}

func TestReportCmd_TextOutput(t *testing.T) {
	tempDir := t.TempDir()

	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
		Name:        "test-project",
	}

	manifestPath := filepath.Join(tempDir, ".claude", ".keel.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	data, _ := json.MarshalIndent(mf, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	cmd := newReportCmd()
	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--format", "text", tempDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("report command failed: %v", err)
	}

	str := output.String()
	if !strings.Contains(str, "Keel Compliance Report") {
		t.Error("expected report title in text output")
	}
	if !strings.Contains(str, "claude") {
		t.Error("expected agent name in text output")
	}
}

func TestReportCmd_OutputToFile(t *testing.T) {
	tempDir := t.TempDir()

	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
	}

	manifestPath := filepath.Join(tempDir, ".claude", ".keel.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	data, _ := json.MarshalIndent(mf, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	outputFile := filepath.Join(tempDir, "report.json")

	cmd := newReportCmd()
	cmd.SetArgs([]string{"--output", outputFile, tempDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("report command failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatal("output file was not created")
	}

	// Verify content
	fileData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var r report.Report
	if err := json.Unmarshal(fileData, &r); err != nil {
		t.Fatalf("failed to parse output file: %v", err)
	}

	if r.Compilation.Agent != "claude" {
		t.Errorf("expected agent claude, got %s", r.Compilation.Agent)
	}
}

func TestReportCmd_MissingManifest(t *testing.T) {
	tempDir := t.TempDir()

	cmd := newReportCmd()
	cmd.SetArgs([]string{tempDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}

	// Error message should indicate no compiled config found
	errMsg := err.Error()
	if !strings.Contains(errMsg, "no compiled config found") &&
		!strings.Contains(errMsg, "not found") &&
		!strings.Contains(errMsg, "no such file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReportCmd_WithGovernance(t *testing.T) {
	tempDir := t.TempDir()

	mf := &manifest.CompiledManifest{
		Agent:       "claude",
		KeelVersion: "1.0.0",
		CompiledAt:  "2026-05-18T00:00:00Z",
		Scope:       "project",
		Governance: &manifest.GovernanceRecord{
			PermissionsPolicy: &manifest.PermissionsPolicyRecord{
				Merge: "additive",
				SetBy: "base-layer",
			},
			MCPPolicy: &schema.MCPPolicy{
				EnableAll: new(bool),
			},
		},
	}
	*mf.Governance.MCPPolicy.EnableAll = true

	manifestPath := filepath.Join(tempDir, ".claude", ".keel.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	data, _ := json.MarshalIndent(mf, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	cmd := newReportCmd()
	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetArgs([]string{tempDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("report command failed: %v", err)
	}

	outputStr := strings.TrimSpace(output.String())
	var r report.Report
	if err := json.Unmarshal([]byte(outputStr), &r); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, outputStr)
	}

	if r.Governance == nil {
		t.Fatal("expected governance in output")
	}
	if r.Governance.PermissionsPolicy == nil {
		t.Fatal("expected permissions policy")
	}
	if r.Governance.PermissionsPolicy.Merge != "additive" {
		t.Errorf("expected additive, got %s", r.Governance.PermissionsPolicy.Merge)
	}
}
