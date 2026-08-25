package signing

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"
)

// verifySignatureCore contains the shared verification logic used by both
// VerifyLayerSignature and VerifyManifest. It handles timestamp parsing,
// age checking, revocation lookup, signature decoding, and cryptographic
// verification.
func verifySignatureCore(
	contentHash string,
	sigKeyID string,
	sigSignature string,
	sigSignedAt string,
	sigContentHash string,
	ks *KeyStore,
	opts *VerifyOptions,
) (*VerifyResult, error) {
	// Parse the signature timestamp.
	signedAt, err := time.Parse(time.RFC3339, sigSignedAt)
	if err != nil {
		return &VerifyResult{
			Valid:       false,
			KeyID:       sigKeyID,
			ContentHash: contentHash,
			SignedAt:    sigSignedAt,
			Error:       fmt.Sprintf("invalid signature timestamp: %v", err),
		}, nil
	}

	signatureAge := time.Since(signedAt)

	// Check signature age if MaxSignatureAge is set.
	if opts.MaxSignatureAge > 0 && signatureAge > opts.MaxSignatureAge {
		return &VerifyResult{
			Valid:        false,
			KeyID:        sigKeyID,
			ContentHash:  contentHash,
			SignedAt:     sigSignedAt,
			SignatureAge: signatureAge,
			Error:        fmt.Sprintf("signature expired: signed at %s, maximum age is %s", sigSignedAt, opts.MaxSignatureAge),
		}, nil
	}

	// Verify content hash matches.
	if contentHash != sigContentHash {
		return &VerifyResult{
			Valid:        false,
			KeyID:        sigKeyID,
			ContentHash:  contentHash,
			SignedAt:     sigSignedAt,
			SignatureAge: signatureAge,
			Error:        "content hash mismatch: content has been modified since signing",
		}, nil
	}

	// Decode the signature bytes.
	sigBytes, err := base64.StdEncoding.DecodeString(sigSignature)
	if err != nil {
		return &VerifyResult{
			Valid:       false,
			KeyID:       sigKeyID,
			ContentHash: sigContentHash,
			SignedAt:    sigSignedAt,
			Error:       fmt.Sprintf("invalid signature encoding: %v", err),
		}, nil
	}

	// Check if the key is revoked.
	revokedKey, err := ks.LoadRevokedKey(sigKeyID)
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
				KeyID:        sigKeyID,
				ContentHash:  sigContentHash,
				SignedAt:     sigSignedAt,
				SignatureAge: signatureAge,
				Error:        fmt.Sprintf("signature created after key revocation (signed: %s, revoked: %s)", sigSignedAt, revokedKey.RevokedAt.Format(time.RFC3339)),
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
		pubKey, keyName, err = ks.LoadPublicKeyByID(sigKeyID)
		if err != nil {
			return &VerifyResult{
				Valid:        false,
				KeyID:        sigKeyID,
				ContentHash:  sigContentHash,
				SignedAt:     sigSignedAt,
				SignatureAge: signatureAge,
				Error:        fmt.Sprintf("signing key not found in keystore: %s", sigKeyID),
			}, nil
		}
	}

	// Verify the signature.
	if !ed25519.Verify(pubKey, []byte(sigContentHash), sigBytes) {
		return &VerifyResult{
			Valid:        false,
			KeyID:        sigKeyID,
			KeyName:      keyName,
			ContentHash:  sigContentHash,
			SignedAt:     sigSignedAt,
			SignatureAge: signatureAge,
			Error:        "signature verification failed",
		}, nil
	}

	// Success.
	return &VerifyResult{
		Valid:        true,
		KeyID:        sigKeyID,
		KeyName:      keyName,
		ContentHash:  sigContentHash,
		SignedAt:     sigSignedAt,
		SignatureAge: signatureAge,
		Warnings:     warnings,
	}, nil
}
