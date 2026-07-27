package compiler

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// TestLayerIR_Clone_CoversAllFields uses reflection to verify that Clone()
// deep-copies all slice, map, and pointer fields in LayerIR. This prevents
// future additions from being missed.
func TestLayerIR_Clone_CoversAllFields(t *testing.T) {
	// Create a fully-populated LayerIR with non-nil values for all fields
	original := &LayerIR{
		Name:        "test",
		Description: "test layer",
		Instructions: []InstructionBlock{
			{Filename: "test.md", Content: "content", LayerLabel: "test"},
		},
		Permissions: map[string]any{"allow": []string{"Read"}},
		MCP:         map[string]any{"mcpServers": map[string]any{"test": map[string]any{}}},
		Settings:    map[string]any{"model": "claude"},
		Hooks:       map[string]any{"SubagentStart": []any{map[string]any{}}},
		ContentBlocks: map[string][]InstructionBlock{
			"loop": {{Filename: "loop.md", Content: "loop"}},
		},
		ConfigBlocks: map[string]map[string]any{
			"environment": {"PATH": "/usr/bin"},
		},
		Categories: map[string][]CategoryFile{
			"commands": {{RelPath: "deploy.md", Content: []byte("content")}},
		},
		ForceLocks:    []ForceLockEntry{{Category: "command", Key: "deploy.md"}},
		AgentSpecific: map[string]map[string]any{"claude": {"key": "value"}},
		SourceAgent:   "claude",
		Variables:     map[string]string{"var": "value"},
		Snippets:      map[string][]byte{"snippet": []byte("content")},
		Deps:          map[string]schema.DepSpec{"tool": {Install: "go install"}},
		Bin:           []BinFile{{RelPath: "tool", Content: []byte("binary")}},
		Attribution:   &CanonicalAttribution{CommitEnabled: true, CommitTemplate: "template"},
		Sandbox: &schema.SandboxConfig{
			Enabled:                  new(bool),
			AutoAllowBashIfSandboxed: new(bool),
			ExcludedCommands:         []string{"rm"},
			AdditionalReadwritePaths: []string{"/tmp"},
			AdditionalReadonlyPaths:  []string{"/etc"},
			IgnoreViolations:         map[string][]string{"cmd": {"violation"}},
			Filesystem: &schema.SandboxFSConfig{
				AllowWrite: []string{"/tmp"},
				DenyWrite:  []string{"/root"},
				AllowRead:  []string{"/etc"},
				DenyRead:   []string{"/secrets"},
			},
			Network: &schema.SandboxNetConfig{
				AllowedDomains:   []string{"example.com"},
				DeniedDomains:    []string{"evil.com"},
				AllowUnixSockets: []string{"/var/run/docker.sock"},
				AllowMachLookup:  []string{"com.apple.system"},
			},
			Ripgrep: &schema.SandboxRipgrepConfig{
				Command: "rg",
				Args:    []string{"--hidden"},
			},
		},
		FilePolicies:      map[string]schema.FilePolicy{"file.md": {Force: true}},
		MCPPolicy:         &schema.MCPPolicy{Allowed: []string{"server"}, Denied: []string{"bad"}},
		PermissionsPolicy: &schema.PermissionsPolicy{Merge: "additive"},
		Repos:             map[string]schema.Repo{"repo": {URL: "https://example.com"}},
		LayerPath:         "/path/to/layer",
		DSLConfig:         &schema.KeelConfig{Name: "config"},
	}
	*original.Sandbox.Enabled = true
	*original.Sandbox.AutoAllowBashIfSandboxed = true

	// Clone the IR
	cloned, err := original.Clone()
	require.NoError(t, err)

	// Verify deep copy by checking that mutable fields point to different memory
	verifyDeepCopy(t, "LayerIR", original, cloned)
}

// verifyDeepCopy recursively checks that all slices, maps, and pointers in original
// and cloned point to different memory addresses (proving deep copy occurred).
func verifyDeepCopy(t *testing.T, path string, original, cloned any) {
	if original == nil || cloned == nil {
		return
	}

	origVal := reflect.ValueOf(original)
	cloneVal := reflect.ValueOf(cloned)

	// Dereference pointers
	if origVal.Kind() == reflect.Ptr {
		if origVal.IsNil() && cloneVal.IsNil() {
			return
		}
		if origVal.IsNil() || cloneVal.IsNil() {
			t.Errorf("%s: one is nil, other is not", path)
			return
		}

		// For pointer fields, verify they point to different memory
		// Exception: DSLConfig is intentionally shared (read-only)
		if origVal.Pointer() == cloneVal.Pointer() && origVal.Pointer() != 0 && path != "LayerIR.DSLConfig" {
			t.Errorf("%s: pointer not deep-copied (same address)", path)
		}

		origVal = origVal.Elem()
		cloneVal = cloneVal.Elem()
	}

	switch origVal.Kind() {
	case reflect.Struct:
		// Recurse into struct fields
		for i := 0; i < origVal.NumField(); i++ {
			field := origVal.Type().Field(i)
			fieldPath := path + "." + field.Name
			origField := origVal.Field(i)
			cloneField := cloneVal.Field(i)

			if !origField.CanInterface() {
				continue // Skip unexported fields
			}

			verifyDeepCopy(t, fieldPath, origField.Interface(), cloneField.Interface())
		}

	case reflect.Slice:
		if origVal.IsNil() && cloneVal.IsNil() {
			return
		}
		if origVal.IsNil() || cloneVal.IsNil() {
			t.Errorf("%s: one slice is nil, other is not", path)
			return
		}
		// Verify slices point to different backing arrays
		if origVal.Len() > 0 && origVal.Pointer() == cloneVal.Pointer() {
			t.Errorf("%s: slice not deep-copied (same backing array)", path)
		}
		// Recurse into slice elements
		for i := 0; i < origVal.Len() && i < cloneVal.Len(); i++ {
			elemPath := path + "[" + string(rune('0'+i)) + "]"
			verifyDeepCopy(t, elemPath, origVal.Index(i).Interface(), cloneVal.Index(i).Interface())
		}

	case reflect.Map:
		if origVal.IsNil() && cloneVal.IsNil() {
			return
		}
		if origVal.IsNil() || cloneVal.IsNil() {
			t.Errorf("%s: one map is nil, other is not", path)
			return
		}
		// Verify maps point to different memory
		if origVal.Pointer() == cloneVal.Pointer() {
			t.Errorf("%s: map not deep-copied (same pointer)", path)
		}
		// Recurse into map values
		for _, key := range origVal.MapKeys() {
			keyPath := path + "[" + key.String() + "]"
			origMapVal := origVal.MapIndex(key)
			cloneMapVal := cloneVal.MapIndex(key)
			if origMapVal.IsValid() && cloneMapVal.IsValid() {
				verifyDeepCopy(t, keyPath, origMapVal.Interface(), cloneMapVal.Interface())
			}
		}
	default:
		// Scalar types (string, bool, int, etc.) are copied by value, no verification needed
	}
}

func boolPtr(b bool) *bool {
	return &b
}
