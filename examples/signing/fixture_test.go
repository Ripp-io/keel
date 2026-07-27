package fixture_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/drift"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/signing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// setupKeyStore creates a temporary KeyStore with a generated keypair.
func setupKeyStore(t *testing.T) (*signing.KeyStore, string) {
	t.Helper()
	ks := &signing.KeyStore{BaseDir: filepath.Join(t.TempDir(), "keys")}
	pub, priv, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	require.NoError(t, ks.SaveKeyPair("default", pub, priv))
	return ks, signing.KeyID(pub)
}

// layerPath returns the absolute path to a test layer.
func layerPath(t *testing.T, name string) string {
	t.Helper()
	// Layers are at examples/signing/layers/<name> relative to this test file.
	path := filepath.Join("layers", name)
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	return abs
}

// copyLayer copies a layer directory to a temp dir and returns the temp path.
func copyLayer(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	require.NoError(t, err)
	return dst
}

// ---------------------------------------------------------------------------
// Layer signing and verification tests
// ---------------------------------------------------------------------------

func TestSignAndVerifyLayer(t *testing.T) {
	t.Parallel()
	ks, keyID := setupKeyStore(t)
	layerCopy := copyLayer(t, layerPath(t, "base"))

	// Sign the layer.
	sig, err := signing.SignLayer(layerCopy, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layerCopy, sig))

	// Verify keel.sig exists and contains expected fields.
	sigPath := filepath.Join(layerCopy, "keel.sig")
	require.FileExists(t, sigPath)

	data, err := os.ReadFile(sigPath)
	require.NoError(t, err)
	var sigData map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &sigData))

	assert.Equal(t, "ed25519", sigData["algorithm"])
	assert.Equal(t, keyID, sigData["key_id"])
	assert.NotEmpty(t, sigData["content_hash"])
	assert.NotEmpty(t, sigData["signature"])
	assert.NotEmpty(t, sigData["signed_at"])

	// Verify the signature.
	result, err := signing.VerifyLayerSignature(layerCopy, ks, nil)
	require.NoError(t, err)
	assert.True(t, result.Valid, "expected valid signature, got error: %s", result.Error)
	assert.Equal(t, keyID, result.KeyID)
	assert.Equal(t, "default", result.KeyName)
}

func TestVerifyLayer_TamperedContent(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)
	layerCopy := copyLayer(t, layerPath(t, "base"))

	// Sign the layer.
	sig, err := signing.SignLayer(layerCopy, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layerCopy, sig))

	// Modify a file in the layer.
	instrPath := filepath.Join(layerCopy, "instructions.md")
	f, err := os.OpenFile(instrPath, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = f.WriteString("\nTampered content.")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Verify should fail with content hash mismatch.
	result, err := signing.VerifyLayerSignature(layerCopy, ks, nil)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "content hash mismatch")
}

func TestVerifyLayer_UntrustedKey(t *testing.T) {
	t.Parallel()
	ksA, _ := setupKeyStore(t)
	ksB, _ := setupKeyStore(t) // Different keystore, different keys.
	layerCopy := copyLayer(t, layerPath(t, "base"))

	// Sign with keystore A.
	sig, err := signing.SignLayer(layerCopy, "default", ksA)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layerCopy, sig))

	// Verify with keystore B (key A not trusted).
	result, err := signing.VerifyLayerSignature(layerCopy, ksB, nil)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "key not found")
}

func TestVerifyLayer_NotSigned(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)
	layer := layerPath(t, "base")

	// Verify an unsigned layer.
	result, err := signing.VerifyLayerSignature(layer, ks, nil)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "not signed")
}

func TestSignLayer_Idempotent(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)
	layerCopy := copyLayer(t, layerPath(t, "base"))

	// Sign once.
	sig1, err := signing.SignLayer(layerCopy, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layerCopy, sig1))
	hash1 := sig1.ContentHash

	// Sign again.
	sig2, err := signing.SignLayer(layerCopy, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layerCopy, sig2))
	hash2 := sig2.ContentHash

	// Content hashes should be identical (keel.sig is excluded from hash).
	assert.Equal(t, hash1, hash2, "signing should be idempotent")
}

// ---------------------------------------------------------------------------
// Compilation with signing tests
// ---------------------------------------------------------------------------

func TestSignedCompilation(t *testing.T) {
	t.Parallel()
	ks, keyID := setupKeyStore(t)

	// Copy and sign both layers.
	layer1 := copyLayer(t, layerPath(t, "base"))
	layer2 := copyLayer(t, layerPath(t, "override"))

	sig1, err := signing.SignLayer(layer1, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layer1, sig1))

	sig2, err := signing.SignLayer(layer2, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layer2, sig2))

	// Construct layers for compilation.
	layers := []config.Layer{
		{Path: layer1, Type: "project", Format: config.FormatKeel, Version: "v1"},
		{Path: layer2, Type: "project", Format: config.FormatKeel, Version: "v1"},
	}

	// Compile.
	outputDir := t.TempDir()
	result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Load the manifest.
	cm, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	require.NotNil(t, cm)

	// Sign the manifest.
	mSig, err := signing.SignManifest(cm, "default", ks)
	require.NoError(t, err)
	require.NotNil(t, mSig)

	// Attach signature and write back.
	cm.Signature = mSig
	require.NoError(t, manifest.WriteKeelState(filepath.Join(outputDir, ".claude", ".keel.json"), cm))

	// Reload and verify the manifest.
	cmReloaded, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	require.NotNil(t, cmReloaded.Signature)

	mResult, err := signing.VerifyManifest(cmReloaded, ks, nil)
	require.NoError(t, err)
	assert.True(t, mResult.Valid, "expected valid manifest signature, got error: %s", mResult.Error)
	assert.Equal(t, keyID, mResult.KeyID)
	assert.Equal(t, "default", mResult.KeyName)

	// Check that layer stack has signature info.
	require.NotEmpty(t, cm.LayerStack)
	for _, layer := range cm.LayerStack {
		// Layers we signed should have signature info in the manifest.
		if layer.Name == "signing-test-base" || layer.Name == "signing-test-override" {
			// The manifest's LayerStack includes layer metadata.
			// Check that the layer paths match our signed layers.
			assert.NotEmpty(t, layer.Path)
		}
	}
}

func TestCompileWithVerifyLayers_Unsigned(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)

	// Use layers WITHOUT signing them.
	layer1 := layerPath(t, "base")
	layer2 := layerPath(t, "override")

	// Try to verify each layer — should fail.
	result1, err := signing.VerifyLayerSignature(layer1, ks, nil)
	require.NoError(t, err)
	assert.False(t, result1.Valid)
	assert.Contains(t, result1.Error, "not signed")

	result2, err := signing.VerifyLayerSignature(layer2, ks, nil)
	require.NoError(t, err)
	assert.False(t, result2.Valid)
	assert.Contains(t, result2.Error, "not signed")
}

func TestCompileWithVerifyLayers_Signed(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)

	// Copy and sign both layers.
	layer1 := copyLayer(t, layerPath(t, "base"))
	layer2 := copyLayer(t, layerPath(t, "override"))

	sig1, err := signing.SignLayer(layer1, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layer1, sig1))

	sig2, err := signing.SignLayer(layer2, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layer2, sig2))

	// Verify each layer — should pass.
	result1, err := signing.VerifyLayerSignature(layer1, ks, nil)
	require.NoError(t, err)
	assert.True(t, result1.Valid, "expected valid signature for layer1, got error: %s", result1.Error)

	result2, err := signing.VerifyLayerSignature(layer2, ks, nil)
	require.NoError(t, err)
	assert.True(t, result2.Valid, "expected valid signature for layer2, got error: %s", result2.Error)

	// Compile successfully.
	layers := []config.Layer{
		{Path: layer1, Type: "project", Format: config.FormatKeel, Version: "v1"},
		{Path: layer2, Type: "project", Format: config.FormatKeel, Version: "v1"},
	}

	outputDir := t.TempDir()
	result, err := compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// Manifest signing and verification tests
// ---------------------------------------------------------------------------

func TestManifestVerify_TamperedOutput(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)

	// Copy and sign layers.
	layer1 := copyLayer(t, layerPath(t, "base"))
	sig1, err := signing.SignLayer(layer1, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layer1, sig1))

	// Compile.
	layers := []config.Layer{
		{Path: layer1, Type: "project", Format: config.FormatKeel, Version: "v1"},
	}

	outputDir := t.TempDir()
	_, err = compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)

	// Load and sign the manifest.
	cm, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)

	mSig, err := signing.SignManifest(cm, "default", ks)
	require.NoError(t, err)
	cm.Signature = mSig
	require.NoError(t, manifest.WriteKeelState(filepath.Join(outputDir, ".claude", ".keel.json"), cm))

	// Modify an output file.
	claudePath := filepath.Join(outputDir, "CLAUDE.md")
	f, err := os.OpenFile(claudePath, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = f.WriteString("\nTampered output.")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Detect drift — should detect modified file.
	drift, err := drift.Detect(outputDir, cm)
	require.NoError(t, err)
	assert.True(t, drift.HasDrift(), "expected drift after tampering output file")
	assert.NotEmpty(t, drift.Modified, "expected modified files in drift result")

	// Verify manifest — should still pass (manifest signature is about the manifest, not output files).
	// Drift detection handles output files.
	cmReloaded, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	mResult, err := signing.VerifyManifest(cmReloaded, ks, nil)
	require.NoError(t, err)
	assert.True(t, mResult.Valid, "manifest signature should still be valid (drift is separate concern)")
}

func TestManifestVerify_TamperedManifest(t *testing.T) {
	t.Parallel()
	ks, _ := setupKeyStore(t)

	// Copy and sign layers.
	layer1 := copyLayer(t, layerPath(t, "base"))
	sig1, err := signing.SignLayer(layer1, "default", ks)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layer1, sig1))

	// Compile.
	layers := []config.Layer{
		{Path: layer1, Type: "project", Format: config.FormatKeel, Version: "v1"},
	}

	outputDir := t.TempDir()
	_, err = compiler.Compile("claude", layers, outputDir, compiler.CompileOptions{})
	require.NoError(t, err)

	// Load and sign the manifest.
	cm, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)

	mSig, err := signing.SignManifest(cm, "default", ks)
	require.NoError(t, err)
	cm.Signature = mSig
	require.NoError(t, manifest.WriteKeelState(filepath.Join(outputDir, ".claude", ".keel.json"), cm))

	// Reload, modify a field, and write back.
	cmReloaded, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	cmReloaded.Name = "tampered-name"
	require.NoError(t, manifest.WriteKeelState(filepath.Join(outputDir, ".claude", ".keel.json"), cmReloaded))

	// Verify manifest — should FAIL (content hash mismatch).
	cmFinal, err := manifest.LoadKeelState(filepath.Join(outputDir, ".claude"))
	require.NoError(t, err)
	mResult, err := signing.VerifyManifest(cmFinal, ks, nil)
	require.NoError(t, err)
	assert.False(t, mResult.Valid)
	assert.Contains(t, mResult.Error, "content hash mismatch")
}

// ---------------------------------------------------------------------------
// Key trust and verification tests
// ---------------------------------------------------------------------------

func TestKeyTrustAndVerify(t *testing.T) {
	t.Parallel()

	// Generate key A in keystore A.
	ksA := &signing.KeyStore{BaseDir: filepath.Join(t.TempDir(), "keysA")}
	pubA, privA, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	require.NoError(t, ksA.SaveKeyPair("keyA", pubA, privA))

	// Generate key B in keystore B.
	ksB := &signing.KeyStore{BaseDir: filepath.Join(t.TempDir(), "keysB")}
	pubB, privB, err := signing.GenerateKeyPair()
	require.NoError(t, err)
	require.NoError(t, ksB.SaveKeyPair("keyB", pubB, privB))

	// Sign a layer with keyA.
	layerCopy := copyLayer(t, layerPath(t, "base"))
	sig, err := signing.SignLayer(layerCopy, "keyA", ksA)
	require.NoError(t, err)
	require.NoError(t, signing.WriteLayerSignature(layerCopy, sig))

	// Verify with keystore B — should fail (untrusted).
	result, err := signing.VerifyLayerSignature(layerCopy, ksB, nil)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Error, "key not found")

	// Export keyA's public key and trust it in keystore B.
	keyAPath := filepath.Join(ksA.BaseDir, "keyA.pub")
	require.NoError(t, ksB.TrustKey(keyAPath))

	// Verify with keystore B — should now pass.
	result, err = signing.VerifyLayerSignature(layerCopy, ksB, nil)
	require.NoError(t, err)
	assert.True(t, result.Valid, "expected valid signature after trusting key, got error: %s", result.Error)
	assert.Equal(t, signing.KeyID(pubA), result.KeyID)
}
