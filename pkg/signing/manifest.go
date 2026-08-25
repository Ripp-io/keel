package signing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// SignManifest signs a compiled manifest with a private key from the keystore.
// Returns the signature structure (caller should attach to cm.Signature and rewrite the manifest).
func SignManifest(cm *manifest.CompiledManifest, keyName string, ks *KeyStore) (*manifest.ManifestSignature, error) {
	// Save current signature and clear it for canonical JSON generation.
	savedSig := cm.Signature
	cm.Signature = nil
	defer func() {
		// Restore original signature so caller can reuse the struct.
		cm.Signature = savedSig
	}()

	// Marshal to canonical JSON.
	jsonBytes, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest for signing: %w", err)
	}

	// Hash the canonical JSON.
	contentHash := manifest.HashContent(jsonBytes)

	// Load the private key and derive public key.
	priv, err := ks.LoadPrivateKey(keyName)
	if err != nil {
		return nil, fmt.Errorf("loading private key %q: %w", keyName, err)
	}
	pub := priv.Public().(ed25519.PublicKey)

	// Sign the content hash (as bytes).
	sig := ed25519.Sign(priv, []byte(contentHash))

	// Build the signature structure.
	return &manifest.ManifestSignature{
		Algorithm:   "ed25519",
		KeyID:       KeyID(pub),
		Signature:   base64.StdEncoding.EncodeToString(sig),
		SignedAt:    time.Now().UTC().Format(time.RFC3339),
		ContentHash: contentHash,
	}, nil
}

// VerifyManifest verifies a signed manifest against the keystore.
// Returns a VerifyResult with detailed outcome.
// Pass nil for opts to use default verification settings (no age limit).
func VerifyManifest(cm *manifest.CompiledManifest, ks *KeyStore, opts *VerifyOptions) (*VerifyResult, error) {
	if opts == nil {
		opts = &VerifyOptions{}
	}
	// Check if manifest is signed.
	if cm.Signature == nil {
		return &VerifyResult{
			Valid: false,
			Error: "manifest is not signed",
		}, nil
	}

	// Save signature and clear it for canonical JSON generation.
	savedSig := cm.Signature
	cm.Signature = nil
	defer func() {
		cm.Signature = savedSig
	}()

	// Marshal to canonical JSON.
	jsonBytes, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest for verification: %w", err)
	}

	// Hash the canonical JSON.
	contentHash := manifest.HashContent(jsonBytes)

	// Delegate to the shared verification core.
	return verifySignatureCore(
		contentHash,
		savedSig.KeyID,
		savedSig.Signature,
		savedSig.SignedAt,
		savedSig.ContentHash,
		ks,
		opts,
	)
}
