package signing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// TestKeyRevocation tests the key revocation mechanism.
func TestKeyRevocation(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate a key and trust it.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyName := "testkey"
	if err := ks.SaveKeyPair(keyName, pub, priv); err != nil {
		t.Fatal(err)
	}

	// Trust the key.
	pubPath := filepath.Join(ks.BaseDir, keyName+".pub")
	if err := ks.TrustKey(pubPath); err != nil {
		t.Fatal(err)
	}

	keyID := KeyID(pub)

	// Revoke the key.
	if err := ks.RevokeKey(keyName); err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}

	// Key should no longer be in trusted directory.
	trustedPath := filepath.Join(ks.BaseDir, "trusted", keyName+".pub")
	if _, err := os.Stat(trustedPath); !os.IsNotExist(err) {
		t.Error("Key should have been removed from trusted directory")
	}

	// Revocation record should exist.
	revoked, err := ks.LoadRevokedKey(keyID)
	if err != nil {
		t.Fatalf("LoadRevokedKey failed: %v", err)
	}
	if revoked == nil {
		t.Fatal("Expected revoked key record, got nil")
	}
	if revoked.KeyID != keyID {
		t.Errorf("Revoked key ID mismatch: got %s, want %s", revoked.KeyID, keyID)
	}
	if revoked.Name != keyName {
		t.Errorf("Revoked key name mismatch: got %s, want %s", revoked.Name, keyName)
	}
	if revoked.RevokedAt.IsZero() {
		t.Error("RevokedAt should not be zero")
	}
}

// TestRevokedKeySignatureRejection tests that signatures created after revocation are rejected.
func TestRevokedKeySignatureRejection(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate and trust a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyName := "testkey"
	if err := ks.SaveKeyPair(keyName, pub, priv); err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(ks.BaseDir, keyName+".pub")
	if err := ks.TrustKey(pubPath); err != nil {
		t.Fatal(err)
	}

	// Create a layer.
	layerDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sign the layer.
	sig1, err := SignLayer(layerDir, keyName, ks)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteLayerSignature(layerDir, sig1); err != nil {
		t.Fatal(err)
	}

	// Verify signature (should pass).
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Signature verification should succeed before revocation: %s", result.Error)
	}

	// Revoke the key.
	time.Sleep(10 * time.Millisecond) // Ensure revocation time is after signing time
	if err := ks.RevokeKey(keyName); err != nil {
		t.Fatal(err)
	}

	// Old signature (created before revocation) should still verify with a warning.
	result, err = VerifyLayerSignature(layerDir, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Pre-revocation signature should still verify: %s", result.Error)
	}
	if len(result.Warnings) == 0 {
		t.Error("Expected warning about revoked key, got none")
	}

	// Create a new signature with a timestamp after revocation (simulate post-revocation signature).
	sig2 := &LayerSignature{
		Algorithm:   sig1.Algorithm,
		KeyID:       sig1.KeyID,
		ContentHash: sig1.ContentHash,
		Signature:   sig1.Signature,
		SignedAt:    time.Now().UTC().Add(1 * time.Minute).Format(time.RFC3339), // Future timestamp
		Signer:      sig1.Signer,
	}
	if err := WriteLayerSignature(layerDir, sig2); err != nil {
		t.Fatal(err)
	}

	// This signature should be rejected.
	result, err = VerifyLayerSignature(layerDir, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("Post-revocation signature should be rejected")
	}
	if result.Error == "" {
		t.Error("Expected error message for post-revocation signature")
	}
}

// TestSignatureFreshness tests the signature age verification.
func TestSignatureFreshness(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyName := "testkey"
	if err := ks.SaveKeyPair(keyName, pub, priv); err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(ks.BaseDir, keyName+".pub")
	if err := ks.TrustKey(pubPath); err != nil {
		t.Fatal(err)
	}

	// Create a layer.
	layerDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(layerDir, "keel.yml"), []byte("schema: v1\nname: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sign the layer.
	sig, err := SignLayer(layerDir, keyName, ks)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteLayerSignature(layerDir, sig); err != nil {
		t.Fatal(err)
	}

	// Verify with no age limit (should pass).
	result, err := VerifyLayerSignature(layerDir, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Signature verification should succeed with no age limit: %s", result.Error)
	}
	if result.SignatureAge == 0 {
		t.Error("SignatureAge should be set")
	}

	// Verify with generous age limit (should pass).
	opts := &VerifyOptions{MaxSignatureAge: 24 * time.Hour}
	result, err = VerifyLayerSignature(layerDir, ks, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Signature verification should succeed within age limit: %s", result.Error)
	}

	// Create an old signature (1 hour ago).
	oldSig := &LayerSignature{
		Algorithm:   sig.Algorithm,
		KeyID:       sig.KeyID,
		ContentHash: sig.ContentHash,
		Signature:   sig.Signature,
		SignedAt:    time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		Signer:      sig.Signer,
	}
	if err := WriteLayerSignature(layerDir, oldSig); err != nil {
		t.Fatal(err)
	}

	// Verify with 30-minute age limit (should fail).
	opts = &VerifyOptions{MaxSignatureAge: 30 * time.Minute}
	result, err = VerifyLayerSignature(layerDir, ks, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("Signature verification should fail when exceeding age limit")
	}
	if result.Error == "" {
		t.Error("Expected error message for expired signature")
	}
	if result.SignatureAge == 0 {
		t.Error("SignatureAge should be set even when verification fails")
	}

	// Verify with 2-hour age limit (should pass).
	opts = &VerifyOptions{MaxSignatureAge: 2 * time.Hour}
	result, err = VerifyLayerSignature(layerDir, ks, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Signature verification should succeed within age limit: %s", result.Error)
	}
}

// TestManifestRevocationCheck tests revocation checking for manifest signatures.
func TestManifestRevocationCheck(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate and trust a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyName := "testkey"
	if err := ks.SaveKeyPair(keyName, pub, priv); err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(ks.BaseDir, keyName+".pub")
	if err := ks.TrustKey(pubPath); err != nil {
		t.Fatal(err)
	}

	// Create a minimal manifest.
	cm := &manifest.CompiledManifest{
		Schema: "v1",
		Name:   "test-manifest",
	}

	// Sign the manifest.
	sig, err := SignManifest(cm, keyName, ks)
	if err != nil {
		t.Fatal(err)
	}
	cm.Signature = sig

	// Verify (should pass).
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Manifest verification should succeed before revocation: %s", result.Error)
	}

	// Revoke the key.
	time.Sleep(10 * time.Millisecond)
	if err := ks.RevokeKey(keyName); err != nil {
		t.Fatal(err)
	}

	// Pre-revocation signature should still verify with warning.
	result, err = VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Pre-revocation manifest signature should still verify: %s", result.Error)
	}
	if len(result.Warnings) == 0 {
		t.Error("Expected warning about revoked key")
	}
}

// TestManifestSignatureFreshness tests signature age checking for manifests.
func TestManifestSignatureFreshness(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyName := "testkey"
	if err := ks.SaveKeyPair(keyName, pub, priv); err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(ks.BaseDir, keyName+".pub")
	if err := ks.TrustKey(pubPath); err != nil {
		t.Fatal(err)
	}

	// Create a minimal manifest.
	cm := &manifest.CompiledManifest{
		Schema: "v1",
		Name:   "test-manifest",
	}

	// Sign the manifest.
	sig, err := SignManifest(cm, keyName, ks)
	if err != nil {
		t.Fatal(err)
	}
	cm.Signature = sig

	// Verify with no age limit (should pass).
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Errorf("Manifest verification should succeed with no age limit: %s", result.Error)
	}

	// Create an old signature.
	sig.SignedAt = time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	cm.Signature = sig

	// Verify with 1-hour age limit (should fail).
	opts := &VerifyOptions{MaxSignatureAge: 1 * time.Hour}
	result, err = VerifyManifest(cm, ks, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("Manifest verification should fail when exceeding age limit")
	}
	if result.Error == "" {
		t.Error("Expected error message for expired signature")
	}
}

// TestRevokeKeyByKeyID tests revoking a key by its key ID.
func TestRevokeKeyByKeyID(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Generate and trust a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyName := "testkey"
	if err := ks.SaveKeyPair(keyName, pub, priv); err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(ks.BaseDir, keyName+".pub")
	if err := ks.TrustKey(pubPath); err != nil {
		t.Fatal(err)
	}

	keyID := KeyID(pub)

	// Revoke by key ID.
	if err := ks.RevokeKey(keyID); err != nil {
		t.Fatalf("RevokeKey by ID failed: %v", err)
	}

	// Check revocation record exists.
	revoked, err := ks.LoadRevokedKey(keyID)
	if err != nil {
		t.Fatal(err)
	}
	if revoked == nil {
		t.Fatal("Expected revoked key record")
	}
	if revoked.KeyID != keyID {
		t.Errorf("Revoked key ID mismatch: got %s, want %s", revoked.KeyID, keyID)
	}
}

// TestLoadRevokedKey_NotRevoked tests that LoadRevokedKey returns nil for non-revoked keys.
func TestLoadRevokedKey_NotRevoked(t *testing.T) {
	ks := &KeyStore{BaseDir: t.TempDir()}

	// Load a non-existent revocation record.
	revoked, err := ks.LoadRevokedKey("keel:sha256:nonexistent")
	if err != nil {
		t.Fatalf("LoadRevokedKey should not error for non-revoked key: %v", err)
	}
	if revoked != nil {
		t.Error("Expected nil for non-revoked key, got a record")
	}
}
