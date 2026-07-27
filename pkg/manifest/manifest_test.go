package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// TestGovernanceRecord_JSONRoundtrip verifies that GovernanceRecord can be
// marshaled to JSON and unmarshaled back without loss.
func TestGovernanceRecord_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	gov := GovernanceRecord{
		PermissionsPolicy: &PermissionsPolicyRecord{
			Merge: "additive",
			SetBy: "layer-base",
		},
		MCPPolicy: &schema.MCPPolicy{
			EnableAll: new(bool),
		},
		HookLockStrategy: "settings-locked",
		OverrideConflicts: []OverrideConflict{
			{
				Category:    "settings",
				Key:         "settings",
				AttemptedBy: "layer-custom",
				LockedBy:    "layer-base",
			},
		},
	}
	*gov.MCPPolicy.EnableAll = true

	// Marshal to JSON
	data, err := json.Marshal(gov)
	require.NoError(t, err)

	// Unmarshal back
	var decoded GovernanceRecord
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, "additive", decoded.PermissionsPolicy.Merge)
	assert.Equal(t, "layer-base", decoded.PermissionsPolicy.SetBy)
	assert.NotNil(t, decoded.MCPPolicy.EnableAll)
	assert.True(t, *decoded.MCPPolicy.EnableAll)
	assert.Equal(t, "settings-locked", decoded.HookLockStrategy)
	assert.Len(t, decoded.OverrideConflicts, 1)
	assert.Equal(t, "settings", decoded.OverrideConflicts[0].Category)
}

// TestGovernanceRecord_Omitempty verifies that nil/empty fields are omitted
// from JSON output.
func TestGovernanceRecord_Omitempty(t *testing.T) {
	t.Parallel()

	// Empty governance record
	gov := GovernanceRecord{}

	data, err := json.Marshal(gov)
	require.NoError(t, err)

	// Should be empty object
	var decoded map[string]any
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded, "expected empty JSON object for zero-value GovernanceRecord")

	// Partial record
	gov2 := GovernanceRecord{
		HookLockStrategy: "settings-locked",
	}

	data2, err := json.Marshal(gov2)
	require.NoError(t, err)

	var decoded2 map[string]any
	err = json.Unmarshal(data2, &decoded2)
	require.NoError(t, err)

	assert.Len(t, decoded2, 1, "expected only hook_lock_strategy field")
	assert.Equal(t, "settings-locked", decoded2["hook_lock_strategy"])
}

// TestHashLayerDir_ExcludesKeelSig verifies that keel.sig is excluded from
// the directory hash, ensuring that signing a layer doesn't change its hash.
func TestHashLayerDir_ExcludesKeelSig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create some files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.yml"), []byte("name: test\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("# Test\n"), 0644))

	// Hash the directory.
	hash1, err := HashLayerDir(dir)
	require.NoError(t, err)

	// Add a keel.sig file.
	sigContent := `{
  "algorithm": "ed25519",
  "key_id": "keel:sha256:abc123",
  "content_hash": "sha256:def456",
  "signature": "fake-signature",
  "signed_at": "2024-01-01T00:00:00Z"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keel.sig"), []byte(sigContent), 0644))

	// Hash again — should be identical.
	hash2, err := HashLayerDir(dir)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "keel.sig should be excluded from directory hash")
}
