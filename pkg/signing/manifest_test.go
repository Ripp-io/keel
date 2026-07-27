package signing

import (
	"path/filepath"
	"testing"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// testManifest creates a minimal CompiledManifest for testing.
func testManifest() *manifest.CompiledManifest {
	return &manifest.CompiledManifest{
		Name:        "test-project",
		Agent:       "claude",
		Scope:       "project",
		CompiledAt:  "2026-01-01T00:00:00Z",
		KeelVersion: "test",
		LayerStack: []manifest.LayerRecord{
			{Path: "/layers/base", Type: "keel", Format: "keel", Hash: "sha256:abc123"},
		},
		FileOps: []manifest.FileRecord{
			{Output: "CLAUDE.md", Strategy: "concatenate", Hash: "sha256:def456"},
		},
	}
}

func TestSignManifest_Roundtrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	if err := ks.SaveKeyPair("test-key", pub, priv); err != nil {
		t.Fatalf("saving keypair: %v", err)
	}

	// Create and sign a manifest.
	cm := testManifest()
	sig, err := SignManifest(cm, "test-key", ks)
	if err != nil {
		t.Fatalf("signing manifest: %v", err)
	}

	if sig.Algorithm != "ed25519" {
		t.Errorf("algorithm = %q, want ed25519", sig.Algorithm)
	}
	if sig.KeyID == "" {
		t.Error("KeyID is empty")
	}
	if sig.Signature == "" {
		t.Error("Signature is empty")
	}
	if sig.ContentHash == "" {
		t.Error("ContentHash is empty")
	}
	if sig.SignedAt == "" {
		t.Error("SignedAt is empty")
	}

	// Attach signature and verify.
	cm.Signature = sig
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatalf("verifying manifest: %v", err)
	}

	if !result.Valid {
		t.Errorf("verification failed: %s", result.Error)
	}
	if result.KeyID != sig.KeyID {
		t.Errorf("KeyID = %q, want %q", result.KeyID, sig.KeyID)
	}
	if result.KeyName != "test-key" {
		t.Errorf("KeyName = %q, want test-key", result.KeyName)
	}
	if result.ContentHash != sig.ContentHash {
		t.Errorf("ContentHash = %q, want %q", result.ContentHash, sig.ContentHash)
	}
}

func TestVerifyManifest_TamperedField(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	if err := ks.SaveKeyPair("test-key", pub, priv); err != nil {
		t.Fatalf("saving keypair: %v", err)
	}

	// Create and sign a manifest.
	cm := testManifest()
	sig, err := SignManifest(cm, "test-key", ks)
	if err != nil {
		t.Fatalf("signing manifest: %v", err)
	}
	cm.Signature = sig

	// Tamper with the manifest.
	cm.Name = "tampered-project"

	// Verify should fail.
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatalf("verifying manifest: %v", err)
	}

	if result.Valid {
		t.Error("expected verification to fail for tampered manifest")
	}
	if result.Error != "content hash mismatch: manifest has been modified since signing" {
		t.Errorf("Error = %q, want content hash mismatch", result.Error)
	}
}

func TestVerifyManifest_Unsigned(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Create an unsigned manifest.
	cm := testManifest()

	// Verify should fail.
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatalf("verifying manifest: %v", err)
	}

	if result.Valid {
		t.Error("expected verification to fail for unsigned manifest")
	}
	if result.Error != "manifest is not signed" {
		t.Errorf("Error = %q, want manifest is not signed", result.Error)
	}
}

func TestVerifyManifest_UntrustedKey(t *testing.T) {
	t.Parallel()

	tmpDirA := t.TempDir()
	tmpDirB := t.TempDir()
	ksA := &KeyStore{BaseDir: tmpDirA}
	ksB := &KeyStore{BaseDir: tmpDirB}

	// Generate key A.
	pubA, privA, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair A: %v", err)
	}
	if err := ksA.SaveKeyPair("key-a", pubA, privA); err != nil {
		t.Fatalf("saving keypair A: %v", err)
	}

	// Generate key B (different keystore).
	pubB, privB, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair B: %v", err)
	}
	if err := ksB.SaveKeyPair("key-b", pubB, privB); err != nil {
		t.Fatalf("saving keypair B: %v", err)
	}

	// Sign with key A.
	cm := testManifest()
	sig, err := SignManifest(cm, "key-a", ksA)
	if err != nil {
		t.Fatalf("signing manifest: %v", err)
	}
	cm.Signature = sig

	// Verify with keystore B (doesn't have key A).
	result, err := VerifyManifest(cm, ksB, nil)
	if err != nil {
		t.Fatalf("verifying manifest: %v", err)
	}

	if result.Valid {
		t.Error("expected verification to fail with untrusted key")
	}
	if result.Error == "" || result.Error == "manifest is not signed" {
		t.Errorf("Error = %q, want key not found error", result.Error)
	}
}

func TestSignManifest_PreservesOriginalSignature(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	if err := ks.SaveKeyPair("test-key", pub, priv); err != nil {
		t.Fatalf("saving keypair: %v", err)
	}

	// Create a manifest with an existing signature.
	cm := testManifest()
	originalSig := &manifest.ManifestSignature{
		Algorithm:   "ed25519",
		KeyID:       "original-key-id",
		Signature:   "original-signature",
		SignedAt:    "2026-01-01T00:00:00Z",
		ContentHash: "sha256:original",
	}
	cm.Signature = originalSig

	// Sign the manifest.
	newSig, err := SignManifest(cm, "test-key", ks)
	if err != nil {
		t.Fatalf("signing manifest: %v", err)
	}

	// Check that the original signature was restored.
	if cm.Signature != originalSig {
		t.Error("SignManifest did not restore original signature")
	}
	if cm.Signature.KeyID != "original-key-id" {
		t.Errorf("Signature.KeyID = %q, want original-key-id", cm.Signature.KeyID)
	}

	// The new signature should be different.
	if newSig.KeyID == originalSig.KeyID {
		t.Error("new signature has same KeyID as original")
	}
}

func TestSignManifest_InvalidKey(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Try to sign with a non-existent key.
	cm := testManifest()
	_, err := SignManifest(cm, "nonexistent", ks)
	if err == nil {
		t.Fatal("expected error when signing with non-existent key")
	}
}

func TestVerifyManifest_InvalidSignatureEncoding(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	if err := ks.SaveKeyPair("test-key", pub, priv); err != nil {
		t.Fatalf("saving keypair: %v", err)
	}

	// Create a manifest with an invalid base64 signature.
	cm := testManifest()
	cm.Signature = &manifest.ManifestSignature{
		Algorithm:   "ed25519",
		KeyID:       KeyID(pub),
		Signature:   "!!!invalid-base64!!!",
		SignedAt:    "2026-01-01T00:00:00Z",
		ContentHash: manifest.HashContent([]byte("{}")),
	}

	// Verify should fail.
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatalf("verifying manifest: %v", err)
	}

	if result.Valid {
		t.Error("expected verification to fail for invalid signature encoding")
	}
	if result.Error == "" {
		t.Error("expected error message for invalid signature encoding")
	}
}

func TestSignManifest_RealisticManifest(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ks := &KeyStore{BaseDir: tmpDir}

	// Generate a key.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	if err := ks.SaveKeyPair("production-key", pub, priv); err != nil {
		t.Fatalf("saving keypair: %v", err)
	}

	// Create a realistic manifest with multiple layers and files.
	cm := &manifest.CompiledManifest{
		Name:        "acme-corp-policy",
		Agent:       "claude",
		Scope:       "project",
		CompiledAt:  "2026-06-18T12:00:00Z",
		KeelVersion: "0.9.0",
		LayerStack: []manifest.LayerRecord{
			{Path: "/layers/company", Type: "keel", Format: "keel", Hash: "sha256:layer1"},
			{Path: "/layers/team", Type: "keel", Format: "keel", Hash: "sha256:layer2"},
			{Path: "/layers/project", Type: "keel", Format: "keel", Hash: "sha256:layer3"},
		},
		FileOps: []manifest.FileRecord{
			{Output: "CLAUDE.md", Strategy: "concatenate", Hash: "sha256:file1"},
			{Output: filepath.Join(".claude", "rules", "security.md"), Strategy: "overlay", Hash: "sha256:file2"},
			{Output: filepath.Join(".claude", "rules", "style.md"), Strategy: "overlay", Hash: "sha256:file3"},
		},
		ForceLocked: []manifest.ForceLockRecord{
			{Category: "permissions", Key: "deny", LockedBy: "/layers/company"},
		},
	}

	// Sign the manifest.
	sig, err := SignManifest(cm, "production-key", ks)
	if err != nil {
		t.Fatalf("signing manifest: %v", err)
	}
	cm.Signature = sig

	// Verify the manifest.
	result, err := VerifyManifest(cm, ks, nil)
	if err != nil {
		t.Fatalf("verifying manifest: %v", err)
	}

	if !result.Valid {
		t.Errorf("verification failed: %s", result.Error)
	}
	if result.KeyName != "production-key" {
		t.Errorf("KeyName = %q, want production-key", result.KeyName)
	}
}
