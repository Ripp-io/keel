package kiro

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

func TestKiroCompile(t *testing.T) {
	tmpDir := t.TempDir()
	layerDir := filepath.Join(tmpDir, "layer")
	outputDir := filepath.Join(tmpDir, "output")

	// Create a minimal layer
	if err := os.MkdirAll(layerDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write keel.yml
	keelYml := `schema: v1
name: test-layer
description: Test layer for Kiro

instructions:
  - source: test.md
`
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644); err != nil {
		t.Fatal(err)
	}

	// Write test.md
	testMd := `# Test Instructions

This is a test instruction file for Kiro.
`
	if err := os.WriteFile(filepath.Join(layerDir, "test.md"), []byte(testMd), 0644); err != nil {
		t.Fatal(err)
	}

	// Create layer directly
	layers := []config.Layer{{Path: layerDir, Format: config.FormatKeel, Type: "test"}}

	// Compile
	opts := compiler.CompileOptions{
		Scope: compiler.ScopeProject,
	}
	result, err := Compile(layers, outputDir, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify outputs
	if result == nil {
		t.Fatal("result is nil")
	}

	// Check AGENTS.md exists
	agentsMd := filepath.Join(outputDir, "AGENTS.md")
	if _, err := os.Stat(agentsMd); os.IsNotExist(err) {
		t.Errorf("AGENTS.md not created")
	}

	// Check .kiro directory exists
	kiroDir := filepath.Join(outputDir, ".kiro")
	if _, err := os.Stat(kiroDir); os.IsNotExist(err) {
		t.Errorf(".kiro directory not created")
	}
}

func TestKiroProfile(t *testing.T) {
	// Test project scope
	profile := Profile(compiler.ScopeProject)
	if profile.Name != "kiro" {
		t.Errorf("expected name 'kiro', got %q", profile.Name)
	}
	if profile.OutputRoot != ".kiro" {
		t.Errorf("expected OutputRoot '.kiro', got %q", profile.OutputRoot)
	}
	if profile.InstructionOutput != "AGENTS.md" {
		t.Errorf("expected InstructionOutput 'AGENTS.md', got %q", profile.InstructionOutput)
	}
	if profile.MCPOutput != ".kiro/settings/mcp.json" {
		t.Errorf("expected MCPOutput '.kiro/settings/mcp.json', got %q", profile.MCPOutput)
	}

	// Test user scope
	userProfile := Profile(compiler.ScopeUser)
	if userProfile.OutputRoot != ".kiro" {
		t.Errorf("expected user OutputRoot '.kiro', got %q", userProfile.OutputRoot)
	}
	if userProfile.InstructionOutput != ".kiro/AGENTS.md" {
		t.Errorf("expected user InstructionOutput '.kiro/AGENTS.md', got %q", userProfile.InstructionOutput)
	}
}

func TestTranslatePermissionsToKiro(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected int // number of permission entries
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: 0,
		},
		{
			name: "allow only",
			input: map[string]any{
				"allow": []any{"Bash(npm test)", "Read"},
			},
			expected: 2,
		},
		{
			name: "allow and deny",
			input: map[string]any{
				"allow": []any{"Bash(npm test)"},
				"deny":  []any{"Bash(rm *)"},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsToKiro(tt.input)
			if tt.expected == 0 && result != nil {
				t.Errorf("expected nil, got %v", result)
			}
			if tt.expected > 0 && len(result) != tt.expected {
				t.Errorf("expected %d entries, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestTranslatePermissionsFromKiro(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected map[string]int // number of entries per key
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "allow only",
			input: []any{
				map[string]any{"tool": "Bash", "resource": "npm test", "effect": "allow"},
				map[string]any{"tool": "Read", "resource": "*", "effect": "allow"},
			},
			expected: map[string]int{"allow": 2},
		},
		{
			name: "allow and deny",
			input: []any{
				map[string]any{"tool": "Bash", "resource": "npm test", "effect": "allow"},
				map[string]any{"tool": "Bash", "resource": "rm *", "effect": "deny"},
			},
			expected: map[string]int{"allow": 1, "deny": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslatePermissionsFromKiro(tt.input)
			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
			}
			if tt.expected != nil {
				for key, count := range tt.expected {
					if list, ok := result[key].([]any); !ok {
						t.Errorf("expected key %q to be []any", key)
					} else if len(list) != count {
						t.Errorf("expected %d entries for %q, got %d", count, key, len(list))
					}
				}
			}
		})
	}
}
