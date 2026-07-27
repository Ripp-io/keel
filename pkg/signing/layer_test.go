package signing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestLayer creates a temporary layer directory with keel.yml and instructions.md.
func createTestLayer(t *testing.T) string {
	t.Helper()

	layerDir := t.TempDir()

	// Create keel.yml.
	keelYml := `schema: v1
name: test-layer
description: Test layer for signing
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte(keelYml), 0644))

	// Create instructions.md.
	instructions := `# Test Instructions

This is a test layer.
`
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(instructions), 0644))

	return layerDir
}

// createTestKeyStore creates a temporary keystore with a test keypair.
func createTestKeyStore(t *testing.T, keyName string) (*KeyStore, string) {
	t.Helper()

	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate a keypair.
	pub, priv, err := GenerateKeyPair()
	require.NoError(t, err)

	// Save to keystore.
	require.NoError(t, ks.SaveKeyPair(keyName, pub, priv))

	keyID := KeyID(pub)
	return ks, keyID
}

func TestSignLayer(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, keyID := createTestKeyStore(t, "test-key")

	sig, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)
	require.NotNil(t, sig)

	// Verify signature fields.
	assert.Equal(t, "ed25519", sig.Algorithm)
	assert.Equal(t, keyID, sig.KeyID)
	assert.NotEmpty(t, sig.ContentHash)
	assert.NotEmpty(t, sig.Signature)
	assert.NotEmpty(t, sig.SignedAt)
	assert.Equal(t, "test-key", sig.Signer)

	// Verify content hash format.
	assert.Contains(t, sig.ContentHash, "sha256:")

	// Verify the keel.sig file is NOT created yet (SignLayer doesn't write).
	sigPath := filepath.Join(layerDir, "keel.sig")
	_, err = os.Stat(sigPath)
	assert.True(t, os.IsNotExist(err), "SignLayer should not write keel.sig")
}

func TestWriteAndReadLayerSignature(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, keyID := createTestKeyStore(t, "test-key")

	// Sign the layer.
	sig, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)

	// Write the signature.
	err = WriteLayerSignature(layerDir, sig)
	require.NoError(t, err)

	// Verify keel.sig was created.
	sigPath := filepath.Join(layerDir, "keel.sig")
	_, err = os.Stat(sigPath)
	require.NoError(t, err)

	// Read the signature back.
	readSig, err := ReadLayerSignature(layerDir)
	require.NoError(t, err)
	require.NotNil(t, readSig)

	// Verify roundtrip.
	assert.Equal(t, sig.Algorithm, readSig.Algorithm)
	assert.Equal(t, sig.KeyID, readSig.KeyID)
	assert.Equal(t, sig.ContentHash, readSig.ContentHash)
	assert.Equal(t, sig.Signature, readSig.Signature)
	assert.Equal(t, sig.SignedAt, readSig.SignedAt)
	assert.Equal(t, sig.Signer, readSig.Signer)

	// Verify the file is valid JSON with trailing newline.
	data, err := os.ReadFile(sigPath)
	require.NoError(t, err)
	assert.Equal(t, byte('\n'), data[len(data)-1], "keel.sig should have trailing newline")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, keyID, parsed["key_id"])
}

func TestReadLayerSignature_NotSigned(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)

	// Read signature from unsigned layer.
	sig, err := ReadLayerSignature(layerDir)
	require.NoError(t, err)
	assert.Nil(t, sig, "unsigned layer should return nil, nil")
}

func TestVerifyLayerSignature_Valid(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, keyID := createTestKeyStore(t, "test-key")

	// Sign and write.
	sig, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)
	err = WriteLayerSignature(layerDir, sig)
	require.NoError(t, err)

	// Verify.
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Valid)
	assert.Equal(t, keyID, result.KeyID)
	assert.Equal(t, "test-key", result.KeyName)
	assert.Equal(t, sig.ContentHash, result.ContentHash)
	assert.Equal(t, sig.SignedAt, result.SignedAt)
	assert.Empty(t, result.Error)
}

func TestVerifyLayerSignature_TamperedContent(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, _ := createTestKeyStore(t, "test-key")

	// Sign and write.
	sig, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)
	err = WriteLayerSignature(layerDir, sig)
	require.NoError(t, err)

	// Tamper with the layer content by modifying a file.
	tampered := "# TAMPERED\n\nThis content was modified.\n"
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "instructions.md"), []byte(tampered), 0644))

	// Verify should fail with content hash mismatch.
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "content hash mismatch")
	assert.NotEqual(t, sig.ContentHash, result.ContentHash, "hash should differ after tampering")
}

func TestVerifyLayerSignature_UntrustedKey(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks1, _ := createTestKeyStore(t, "signing-key")

	// Sign with ks1.
	sig, err := SignLayer(layerDir, "signing-key", ks1)
	require.NoError(t, err)
	err = WriteLayerSignature(layerDir, sig)
	require.NoError(t, err)

	// Create a different keystore with a different key.
	ks2, _ := createTestKeyStore(t, "different-key")

	// Verify with ks2 should fail (key not found).
	result, err := VerifyLayerSignature(layerDir, ks2, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "signing key not found in keystore")
}

func TestVerifyLayerSignature_NotSigned(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, _ := createTestKeyStore(t, "test-key")

	// Verify unsigned layer.
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "layer is not signed")
}

func TestSignLayer_IdempotentHash(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, _ := createTestKeyStore(t, "test-key")

	// Sign the layer.
	sig1, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)
	hash1 := sig1.ContentHash

	// Write the signature.
	err = WriteLayerSignature(layerDir, sig1)
	require.NoError(t, err)

	// Sign again — hash should be identical (keel.sig is excluded from hash).
	sig2, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)
	hash2 := sig2.ContentHash

	assert.Equal(t, hash1, hash2, "signing should be idempotent: keel.sig should not affect hash")
}

func TestVerifyLayerSignature_TamperedSignature(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, _ := createTestKeyStore(t, "test-key")

	// Sign and write.
	sig, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)
	err = WriteLayerSignature(layerDir, sig)
	require.NoError(t, err)

	// Tamper with the signature file by changing the signature field.
	sigPath := filepath.Join(layerDir, "keel.sig")
	data, err := os.ReadFile(sigPath)
	require.NoError(t, err)

	var parsed LayerSignature
	require.NoError(t, json.Unmarshal(data, &parsed))

	// Corrupt the signature.
	parsed.Signature = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="

	corrupted, err := json.MarshalIndent(parsed, "", "  ")
	require.NoError(t, err)
	corrupted = append(corrupted, '\n')
	require.NoError(t, os.WriteFile(sigPath, corrupted, 0644))

	// Verify should fail.
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "signature verification failed")
}

func TestSignLayer_MissingKey(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Try to sign with a non-existent key.
	_, err := SignLayer(layerDir, "nonexistent", ks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading private key")
}

func TestVerifyLayerSignature_InvalidSignatureEncoding(t *testing.T) {
	t.Parallel()

	layerDir := createTestLayer(t)
	ks, _ := createTestKeyStore(t, "test-key")

	// First sign properly to get the correct content hash.
	validSig, err := SignLayer(layerDir, "test-key", ks)
	require.NoError(t, err)

	// Create a signature with the correct hash but invalid base64 encoding.
	sig := &LayerSignature{
		Algorithm:   validSig.Algorithm,
		KeyID:       validSig.KeyID,
		ContentHash: validSig.ContentHash, // Use the actual content hash
		Signature:   "not-valid-base64!@#$%",
		SignedAt:    validSig.SignedAt,
		Signer:      validSig.Signer,
	}

	err = WriteLayerSignature(layerDir, sig)
	require.NoError(t, err)

	// Verify should fail with encoding error.
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "invalid signature encoding")
}
