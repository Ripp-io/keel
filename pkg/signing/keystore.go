package signing

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KeyStore manages Ed25519 key storage on disk.
type KeyStore struct {
	BaseDir string // e.g. ~/.keel/keys — injectable for testing
}

// KeyInfo describes a stored key.
type KeyInfo struct {
	Name       string
	KeyID      string // keel:sha256:<hex>
	HasPrivate bool
	Trusted    bool   // true if in trusted/ subdirectory
	Path       string // path to .pub file
}

// RevokedKey records a revoked key with revocation timestamp.
type RevokedKey struct {
	KeyID      string    `json:"key_id"`
	PublicKey  []byte    `json:"public_key"` // PEM-encoded public key
	RevokedAt  time.Time `json:"revoked_at"`
	Name       string    `json:"name,omitempty"`
}

// DefaultKeyStore returns a KeyStore using ~/.keel/keys/.
func DefaultKeyStore() *KeyStore {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home cannot be determined.
		homeDir = "."
	}
	return &KeyStore{
		BaseDir: filepath.Join(homeDir, ".keel", "keys"),
	}
}

// SaveKeyPair writes a keypair to disk.
// Writes <name>.key (mode 0600) and <name>.pub (mode 0644) to ks.BaseDir.
// Creates BaseDir with mode 0700 if needed.
func (ks *KeyStore) SaveKeyPair(name string, pub ed25519.PublicKey, priv ed25519.PrivateKey) error {
	// Create base directory if it doesn't exist.
	if err := os.MkdirAll(ks.BaseDir, 0700); err != nil {
		return fmt.Errorf("creating keystore directory: %w", err)
	}

	// Marshal keys to PEM.
	pubPEM, err := MarshalPublicKey(pub)
	if err != nil {
		return fmt.Errorf("marshaling public key: %w", err)
	}

	privPEM, err := MarshalPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}

	// Write private key first (more restrictive permissions).
	privPath := filepath.Join(ks.BaseDir, name+".key")
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return fmt.Errorf("writing private key: %w", err)
	}

	// Write public key.
	pubPath := filepath.Join(ks.BaseDir, name+".pub")
	if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
		// Clean up private key if public key write fails.
		_ = os.Remove(privPath)
		return fmt.Errorf("writing public key: %w", err)
	}

	return nil
}

// LoadPrivateKey reads a private key from BaseDir/<name>.key.
func (ks *KeyStore) LoadPrivateKey(name string) (ed25519.PrivateKey, error) {
	path := filepath.Join(ks.BaseDir, name+".key")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}

	key, err := UnmarshalPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling private key: %w", err)
	}

	return key, nil
}

// LoadPublicKey reads a public key from BaseDir/<name>.pub, falling back to BaseDir/trusted/<name>.pub.
// Only falls back to trusted if local key doesn't exist; a corrupted local key is a hard error.
func (ks *KeyStore) LoadPublicKey(name string) (ed25519.PublicKey, error) {
	// Try local key first.
	path := filepath.Join(ks.BaseDir, name+".pub")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading local public key: %w", err)
		}
		// Local key doesn't exist — fall back to trusted.
	} else {
		key, err := UnmarshalPublicKey(data)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling local public key %s: %w", path, err)
		}
		return key, nil
	}

	// Fall back to trusted key.
	trustedPath := filepath.Join(ks.BaseDir, "trusted", name+".pub")
	data, err = os.ReadFile(trustedPath)
	if err != nil {
		return nil, fmt.Errorf("reading public key (tried local and trusted): %w", err)
	}

	key, err := UnmarshalPublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling trusted public key %s: %w", trustedPath, err)
	}

	return key, nil
}

// LoadPublicKeyByID scans all .pub files in BaseDir and BaseDir/trusted/, returns (key, name, error).
func (ks *KeyStore) LoadPublicKeyByID(keyID string) (ed25519.PublicKey, string, error) {
	// Scan local keys.
	if key, name, err := ks.scanForKeyID(ks.BaseDir, keyID); err == nil {
		return key, name, nil
	}

	// Scan trusted keys.
	trustedDir := filepath.Join(ks.BaseDir, "trusted")
	if key, name, err := ks.scanForKeyID(trustedDir, keyID); err == nil {
		return key, name, nil
	}

	return nil, "", fmt.Errorf("key ID not found: %s", keyID)
}

// scanForKeyID scans a directory for .pub files and returns the first match for keyID.
func (ks *KeyStore) scanForKeyID(dir, keyID string) (ed25519.PublicKey, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		key, err := UnmarshalPublicKey(data)
		if err != nil {
			continue
		}

		if KeyID(key) == keyID {
			name := strings.TrimSuffix(entry.Name(), ".pub")
			return key, name, nil
		}
	}

	return nil, "", fmt.Errorf("key ID not found in %s", dir)
}

// ListKeys scans BaseDir/*.pub and BaseDir/trusted/*.pub.
func (ks *KeyStore) ListKeys() ([]KeyInfo, error) {
	var keys []KeyInfo

	// Scan local keys.
	localKeys, err := ks.scanKeys(ks.BaseDir, false)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("scanning local keys: %w", err)
	}
	keys = append(keys, localKeys...)

	// Scan trusted keys.
	trustedDir := filepath.Join(ks.BaseDir, "trusted")
	trustedKeys, err := ks.scanKeys(trustedDir, true)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("scanning trusted keys: %w", err)
	}
	keys = append(keys, trustedKeys...)

	return keys, nil
}

// scanKeys scans a directory for .pub files and returns KeyInfo entries.
func (ks *KeyStore) scanKeys(dir string, trusted bool) ([]KeyInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var keys []KeyInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".pub")
		pubPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(pubPath)
		if err != nil {
			continue
		}

		pub, err := UnmarshalPublicKey(data)
		if err != nil {
			continue
		}

		// Check if private key exists (only for local keys, not trusted).
		hasPrivate := false
		if !trusted {
			privPath := filepath.Join(dir, name+".key")
			if _, err := os.Stat(privPath); err == nil {
				hasPrivate = true
			}
		}

		keys = append(keys, KeyInfo{
			Name:       name,
			KeyID:      KeyID(pub),
			HasPrivate: hasPrivate,
			Trusted:    trusted,
			Path:       pubPath,
		})
	}

	return keys, nil
}

// TrustKey reads a PEM file, validates it's Ed25519, and copies to BaseDir/trusted/<basename>.
func (ks *KeyStore) TrustKey(pubKeyPath string) error {
	// Read and validate the public key.
	data, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("reading public key file: %w", err)
	}

	pub, err := UnmarshalPublicKey(data)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Create trusted directory if it doesn't exist.
	trustedDir := filepath.Join(ks.BaseDir, "trusted")
	if err := os.MkdirAll(trustedDir, 0755); err != nil {
		return fmt.Errorf("creating trusted directory: %w", err)
	}

	// Write to trusted directory with the same basename.
	basename := filepath.Base(pubKeyPath)
	if !strings.HasSuffix(basename, ".pub") {
		basename = basename + ".pub"
	}
	destPath := filepath.Join(trustedDir, basename)

	// Re-marshal to ensure consistent formatting.
	pubPEM, err := MarshalPublicKey(pub)
	if err != nil {
		return fmt.Errorf("marshaling public key: %w", err)
	}

	if err := os.WriteFile(destPath, pubPEM, 0644); err != nil {
		return fmt.Errorf("writing trusted key: %w", err)
	}

	return nil
}

// RevokeKey revokes a key from BaseDir/trusted/ by moving it to BaseDir/revoked/.
// Records the revocation timestamp. Looks up by name first, then by KeyID scan.
func (ks *KeyStore) RevokeKey(nameOrID string) error {
	trustedDir := filepath.Join(ks.BaseDir, "trusted")
	revokedDir := filepath.Join(ks.BaseDir, "revoked")

	// Create revoked directory if it doesn't exist.
	if err := os.MkdirAll(revokedDir, 0755); err != nil {
		return fmt.Errorf("creating revoked directory: %w", err)
	}

	var keyPath string
	var keyName string
	var pubKey ed25519.PublicKey

	// Try by name first.
	testPath := filepath.Join(trustedDir, nameOrID+".pub")
	if _, err := os.Stat(testPath); err == nil {
		keyPath = testPath
		keyName = nameOrID
	} else {
		// Try by key ID scan.
		_, name, err := ks.scanForKeyID(trustedDir, nameOrID)
		if err != nil {
			return fmt.Errorf("key not found in trusted keyring: %s", nameOrID)
		}
		keyPath = filepath.Join(trustedDir, name+".pub")
		keyName = name
	}

	// Read the public key before moving.
	pubKeyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}

	pubKey, err = UnmarshalPublicKey(pubKeyData)
	if err != nil {
		return fmt.Errorf("unmarshaling public key: %w", err)
	}

	keyID := KeyID(pubKey)

	// Create revocation record.
	revoked := &RevokedKey{
		KeyID:      keyID,
		PublicKey:  pubKeyData,
		RevokedAt:  time.Now().UTC(),
		Name:       keyName,
	}

	// Write revocation record.
	revocationPath := filepath.Join(revokedDir, keyID+".json")
	revocationData, err := json.MarshalIndent(revoked, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling revocation record: %w", err)
	}

	if err := os.WriteFile(revocationPath, append(revocationData, '\n'), 0644); err != nil {
		return fmt.Errorf("writing revocation record: %w", err)
	}

	// Remove from trusted directory.
	if err := os.Remove(keyPath); err != nil {
		// Clean up revocation record if removal fails.
		_ = os.Remove(revocationPath)
		return fmt.Errorf("removing trusted key: %w", err)
	}

	return nil
}

// LoadRevokedKey loads a revocation record by key ID.
// Returns nil, nil if the key is not revoked.
func (ks *KeyStore) LoadRevokedKey(keyID string) (*RevokedKey, error) {
	revokedDir := filepath.Join(ks.BaseDir, "revoked")
	revocationPath := filepath.Join(revokedDir, keyID+".json")

	data, err := os.ReadFile(revocationPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not revoked.
		}
		return nil, fmt.Errorf("reading revocation record: %w", err)
	}

	var revoked RevokedKey
	if err := json.Unmarshal(data, &revoked); err != nil {
		return nil, fmt.Errorf("parsing revocation record: %w", err)
	}

	return &revoked, nil
}
