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
	sig := Sign(priv, []byte(contentHash))

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

	// Parse the signature timestamp.
	signedAt, err := time.Parse(time.RFC3339, savedSig.SignedAt)
	if err != nil {
		return &VerifyResult{
			Valid:       false,
			KeyID:       savedSig.KeyID,
			ContentHash: contentHash,
			SignedAt:    savedSig.SignedAt,
			Error:       fmt.Sprintf("invalid signature timestamp: %v", err),
		}, nil
	}

	signatureAge := time.Since(signedAt)

	// Check signature age if MaxSignatureAge is set.
	if opts.MaxSignatureAge > 0 && signatureAge > opts.MaxSignatureAge {
		return &VerifyResult{
			Valid:        false,
			KeyID:        savedSig.KeyID,
			ContentHash:  contentHash,
			SignedAt:     savedSig.SignedAt,
			SignatureAge: signatureAge,
			Error:        fmt.Sprintf("signature expired: signed at %s, maximum age is %s", savedSig.SignedAt, opts.MaxSignatureAge),
		}, nil
	}

	// Verify content hash matches.
	if contentHash != savedSig.ContentHash {
		return &VerifyResult{
			Valid:        false,
			KeyID:        savedSig.KeyID,
			ContentHash:  contentHash,
			SignedAt:     savedSig.SignedAt,
			SignatureAge: signatureAge,
			Error:        "content hash mismatch: manifest has been modified since signing",
		}, nil
	}

	// Decode the signature bytes.
	sigBytes, err := base64.StdEncoding.DecodeString(savedSig.Signature)
	if err != nil {
		return &VerifyResult{
			Valid:       false,
			KeyID:       savedSig.KeyID,
			ContentHash: savedSig.ContentHash,
			SignedAt:    savedSig.SignedAt,
			Error:       fmt.Sprintf("invalid signature encoding: %v", err),
		}, nil
	}

	// Check if the key is revoked.
	revokedKey, err := ks.LoadRevokedKey(savedSig.KeyID)
	if err != nil {
		return nil, fmt.Errorf("checking key revocation: %w", err)
	}

	var pubKey ed25519.PublicKey
	var keyName string
	var warnings []string

	if revokedKey != nil {
		// Key is revoked — check if signature predates revocation.
		if signedAt.After(revokedKey.RevokedAt) {
			// Signature created after revocation — reject.
			return &VerifyResult{
				Valid:        false,
				KeyID:        savedSig.KeyID,
				ContentHash:  savedSig.ContentHash,
				SignedAt:     savedSig.SignedAt,
				SignatureAge: signatureAge,
				Error:        fmt.Sprintf("signature created after key revocation (signed: %s, revoked: %s)", savedSig.SignedAt, revokedKey.RevokedAt.Format(time.RFC3339)),
			}, nil
		}

		// Signature predates revocation — allow but warn.
		warnings = append(warnings, fmt.Sprintf("signing key was revoked on %s, but signature predates revocation", revokedKey.RevokedAt.Format(time.RFC3339)))

		// Use the revoked key's public key for verification.
		pubKey, err = UnmarshalPublicKey(revokedKey.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling revoked public key: %w", err)
		}
		keyName = revokedKey.Name
	} else {
		// Load the public key by ID from active keys.
		var err error
		pubKey, keyName, err = ks.LoadPublicKeyByID(savedSig.KeyID)
		if err != nil {
			return &VerifyResult{
				Valid:        false,
				KeyID:        savedSig.KeyID,
				ContentHash:  savedSig.ContentHash,
				SignedAt:     savedSig.SignedAt,
				SignatureAge: signatureAge,
				Error:        fmt.Sprintf("signing key not found in keystore: %s", savedSig.KeyID),
			}, nil
		}
	}

	// Verify the signature.
	if !Verify(pubKey, []byte(contentHash), sigBytes) {
		return &VerifyResult{
			Valid:        false,
			KeyID:        savedSig.KeyID,
			KeyName:      keyName,
			ContentHash:  savedSig.ContentHash,
			SignedAt:     savedSig.SignedAt,
			SignatureAge: signatureAge,
			Error:        "signature verification failed",
		}, nil
	}

	// Success.
	return &VerifyResult{
		Valid:        true,
		KeyID:        savedSig.KeyID,
		KeyName:      keyName,
		ContentHash:  savedSig.ContentHash,
		SignedAt:     savedSig.SignedAt,
		SignatureAge: signatureAge,
		Warnings:     warnings,
	}, nil
}
