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
	sig := ed25519.Sign(priv, []byte(contentHash))

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

	// Delegate to the shared verification core.
	return verifySignatureCore(
		currentHash,
		sig.KeyID,
		sig.Signature,
		sig.SignedAt,
		sig.ContentHash,
		ks,
		opts,
	)
}
