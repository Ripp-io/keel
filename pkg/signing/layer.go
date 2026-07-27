package signing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/manifest"
)

// VerifyOptions configures signature verification behavior.
type VerifyOptions struct {
	MaxSignatureAge time.Duration // Maximum age for signatures; 0 means no limit
}

// LayerSignature is the structure written to keel.sig alongside a layer's keel.yml.
type LayerSignature struct {
	Algorithm   string `json:"algorithm"`        // "ed25519"
	KeyID       string `json:"key_id"`           // keel:sha256:<hex>
	ContentHash string `json:"content_hash"`     // sha256:<hex> from HashLayerDir
	Signature   string `json:"signature"`        // base64-encoded raw Ed25519 signature
	SignedAt    string `json:"signed_at"`        // RFC3339 UTC
	Signer      string `json:"signer,omitempty"` // optional human-readable name
}

// VerifyResult reports the outcome of a signature verification.
type VerifyResult struct {
	Valid         bool
	KeyID         string
	KeyName       string        // human-readable name of the matching key
	ContentHash   string        // The verified content hash - can be used for re-verification
	SignedAt      string
	SignatureAge  time.Duration // How old the signature is
	Error         string        // non-empty if verification failed
	Warnings      []string      // non-fatal issues (e.g., signature predates revocation)
}

// SignLayer signs a layer directory with a private key from the keystore.
// Returns the signature structure (caller should write with WriteLayerSignature).
func SignLayer(layerPath, keyName string, ks *KeyStore) (*LayerSignature, error) {
	// Hash the layer directory content.
	contentHash, err := manifest.HashLayerDir(layerPath)
	if err != nil {
		return nil, fmt.Errorf("hashing layer directory: %w", err)
	}

	// Load the private key.
	priv, err := ks.LoadPrivateKey(keyName)
	if err != nil {
		return nil, fmt.Errorf("loading private key %q: %w", keyName, err)
	}

	// Derive the public key for key ID.
	pub := priv.Public().(ed25519.PublicKey)
	keyID := KeyID(pub)

	// Sign the content hash bytes.
	sig := Sign(priv, []byte(contentHash))

	// Build the signature structure.
	layerSig := &LayerSignature{
		Algorithm:   "ed25519",
		KeyID:       keyID,
		ContentHash: contentHash,
		Signature:   base64.StdEncoding.EncodeToString(sig),
		SignedAt:    time.Now().UTC().Format(time.RFC3339),
		Signer:      keyName,
	}

	return layerSig, nil
}

// WriteLayerSignature writes a layer signature to keel.sig in the layer directory.
func WriteLayerSignature(layerPath string, sig *LayerSignature) error {
	data, err := json.MarshalIndent(sig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling signature: %w", err)
	}

	// Append trailing newline for POSIX compliance.
	data = append(data, '\n')

	sigPath := filepath.Join(layerPath, "keel.sig")
	if err := os.WriteFile(sigPath, data, 0644); err != nil {
		return fmt.Errorf("writing signature file: %w", err)
	}

	return nil
}

// ReadLayerSignature reads a keel.sig file from a layer directory.
// Returns nil, nil if the file doesn't exist (unsigned layer).
func ReadLayerSignature(layerPath string) (*LayerSignature, error) {
	sigPath := filepath.Join(layerPath, "keel.sig")

	data, err := os.ReadFile(sigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not signed — not an error.
		}
		return nil, fmt.Errorf("reading signature file: %w", err)
	}

	var sig LayerSignature
	if err := json.Unmarshal(data, &sig); err != nil {
		return nil, fmt.Errorf("parsing signature file: %w", err)
	}

	return &sig, nil
}

// VerifyLayerSignature verifies a signed layer against the keystore.
// Returns a VerifyResult with detailed outcome.
// Pass nil for opts to use default verification settings (no age limit).
func VerifyLayerSignature(layerPath string, ks *KeyStore, opts *VerifyOptions) (*VerifyResult, error) {
	if opts == nil {
		opts = &VerifyOptions{}
	}
	// Read the signature file.
	sig, err := ReadLayerSignature(layerPath)
	if err != nil {
		return nil, fmt.Errorf("reading signature: %w", err)
	}
	if sig == nil {
		return &VerifyResult{
			Valid: false,
			Error: "layer is not signed",
		}, nil
	}

	// Hash the current layer content.
	currentHash, err := manifest.HashLayerDir(layerPath)
	if err != nil {
		return nil, fmt.Errorf("hashing layer directory: %w", err)
	}

	// Parse the signature timestamp.
	signedAt, err := time.Parse(time.RFC3339, sig.SignedAt)
	if err != nil {
		return &VerifyResult{
			Valid:       false,
			KeyID:       sig.KeyID,
			ContentHash: currentHash,
			SignedAt:    sig.SignedAt,
			Error:       fmt.Sprintf("invalid signature timestamp: %v", err),
		}, nil
	}

	signatureAge := time.Since(signedAt)

	// Check signature age if MaxSignatureAge is set.
	if opts.MaxSignatureAge > 0 && signatureAge > opts.MaxSignatureAge {
		return &VerifyResult{
			Valid:        false,
			KeyID:        sig.KeyID,
			ContentHash:  currentHash,
			SignedAt:     sig.SignedAt,
			SignatureAge: signatureAge,
			Error:        fmt.Sprintf("signature expired: signed at %s, maximum age is %s", sig.SignedAt, opts.MaxSignatureAge),
		}, nil
	}

	// Verify content hash matches.
	if currentHash != sig.ContentHash {
		return &VerifyResult{
			Valid:        false,
			KeyID:        sig.KeyID,
			ContentHash:  currentHash,
			SignedAt:     sig.SignedAt,
			SignatureAge: signatureAge,
			Error:        "content hash mismatch: layer has been modified since signing",
		}, nil
	}

	// Decode the signature bytes.
	sigBytes, err := base64.StdEncoding.DecodeString(sig.Signature)
	if err != nil {
		return &VerifyResult{
			Valid:       false,
			KeyID:       sig.KeyID,
			ContentHash: sig.ContentHash,
			SignedAt:    sig.SignedAt,
			Error:       fmt.Sprintf("invalid signature encoding: %v", err),
		}, nil
	}

	// Check if the key is revoked.
	revokedKey, err := ks.LoadRevokedKey(sig.KeyID)
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
				KeyID:        sig.KeyID,
				ContentHash:  sig.ContentHash,
				SignedAt:     sig.SignedAt,
				SignatureAge: signatureAge,
				Error:        fmt.Sprintf("signature created after key revocation (signed: %s, revoked: %s)", sig.SignedAt, revokedKey.RevokedAt.Format(time.RFC3339)),
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
		pubKey, keyName, err = ks.LoadPublicKeyByID(sig.KeyID)
		if err != nil {
			return &VerifyResult{
				Valid:        false,
				KeyID:        sig.KeyID,
				ContentHash:  sig.ContentHash,
				SignedAt:     sig.SignedAt,
				SignatureAge: signatureAge,
				Error:        fmt.Sprintf("signing key not found in keystore: %s", sig.KeyID),
			}, nil
		}
	}

	// Verify the signature.
	if !Verify(pubKey, []byte(sig.ContentHash), sigBytes) {
		return &VerifyResult{
			Valid:        false,
			KeyID:        sig.KeyID,
			KeyName:      keyName,
			ContentHash:  sig.ContentHash,
			SignedAt:     sig.SignedAt,
			SignatureAge: signatureAge,
			Error:        "signature verification failed",
		}, nil
	}

	// Success.
	return &VerifyResult{
		Valid:        true,
		KeyID:        sig.KeyID,
		KeyName:      keyName,
		ContentHash:  sig.ContentHash,
		SignedAt:     sig.SignedAt,
		SignatureAge: signatureAge,
		Warnings:     warnings,
	}, nil
}

// VerifyContentHash re-verifies that a layer directory matches an expected hash.
// This is used to detect TOCTOU attacks where the layer is modified between
// signature verification and compilation.
func VerifyContentHash(layerPath string, expectedHash string) error {
	// Re-hash the layer directory.
	currentHash, err := manifest.HashLayerDir(layerPath)
	if err != nil {
		return fmt.Errorf("hashing layer directory: %w", err)
	}

	// Compare hashes.
	if currentHash != expectedHash {
		return fmt.Errorf("layer %q was modified after signature verification - content hash mismatch (expected %s, got %s)", layerPath, expectedHash, currentHash)
	}

	return nil
}
