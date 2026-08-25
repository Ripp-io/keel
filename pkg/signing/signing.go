// Package signing provides Ed25519 signing and verification operations for Keel.
package signing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

// GenerateKeyPair generates a new Ed25519 keypair.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generating Ed25519 keypair: %w", err)
	}
	return pub, priv, nil
}

// KeyID computes a fingerprint: "keel:sha256:<64-hex-chars>" from the public key bytes.
func KeyID(pub ed25519.PublicKey) string {
	hash := sha256.Sum256(pub)
	return "keel:sha256:" + hex.EncodeToString(hash[:])
}

// MarshalPrivateKey encodes an Ed25519 private key as PEM (PKCS8).
func MarshalPrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshaling private key to PKCS8: %w", err)
	}

	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8,
	}
	return pem.EncodeToMemory(block), nil
}

// MarshalPublicKey encodes an Ed25519 public key as PEM (PKIX).
func MarshalPublicKey(key ed25519.PublicKey) ([]byte, error) {
	pkix, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshaling public key to PKIX: %w", err)
	}

	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkix,
	}
	return pem.EncodeToMemory(block), nil
}

// UnmarshalPrivateKey decodes a PEM-encoded Ed25519 private key (PKCS8).
func UnmarshalPrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return nil, fmt.Errorf("unexpected data after PEM block")
	}

	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("unexpected PEM block type: %s (expected PRIVATE KEY)", block.Type)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PKCS8 private key: %w", err)
	}

	ed25519Key, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519 (got %T)", key)
	}

	return ed25519Key, nil
}

// UnmarshalPublicKey decodes a PEM-encoded Ed25519 public key (PKIX).
func UnmarshalPublicKey(data []byte) (ed25519.PublicKey, error) {
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return nil, fmt.Errorf("unexpected data after PEM block")
	}

	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("unexpected PEM block type: %s (expected PUBLIC KEY)", block.Type)
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PKIX public key: %w", err)
	}

	ed25519Key, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519 (got %T)", key)
	}

	return ed25519Key, nil
}
