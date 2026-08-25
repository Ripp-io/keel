package signing

import (
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKeyPair(t *testing.T) {
	t.Parallel()

	pub, priv, err := GenerateKeyPair()
	require.NoError(t, err)
	require.NotNil(t, pub)
	require.NotNil(t, priv)

	// Verify key sizes match Ed25519 spec.
	assert.Equal(t, ed25519.PublicKeySize, len(pub), "public key should be 32 bytes")
	assert.Equal(t, ed25519.PrivateKeySize, len(priv), "private key should be 64 bytes")
}

func TestSignAndVerify(t *testing.T) {
	t.Parallel()

	pub, priv, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("test message for signing")

	// Sign the data.
	signature := ed25519.Sign(priv, testData)
	require.NotNil(t, signature)
	require.NotEmpty(t, signature)

	// Verify with correct data — should succeed.
	assert.True(t, ed25519.Verify(pub, testData, signature), "verification should succeed with correct data")

	// Verify with altered data — should fail.
	alteredData := []byte("altered message")
	assert.False(t, ed25519.Verify(pub, alteredData, signature), "verification should fail with altered data")

	// Verify with altered signature — should fail.
	alteredSig := make([]byte, len(signature))
	copy(alteredSig, signature)
	alteredSig[0] ^= 0xFF // Flip bits in first byte.
	assert.False(t, ed25519.Verify(pub, testData, alteredSig), "verification should fail with altered signature")
}

func TestKeyID_Deterministic(t *testing.T) {
	t.Parallel()

	pub, _, err := GenerateKeyPair()
	require.NoError(t, err)

	// Generate ID multiple times — should always be the same.
	id1 := KeyID(pub)
	id2 := KeyID(pub)
	id3 := KeyID(pub)

	assert.Equal(t, id1, id2, "KeyID should be deterministic")
	assert.Equal(t, id2, id3, "KeyID should be deterministic")
}

func TestKeyID_Format(t *testing.T) {
	t.Parallel()

	pub, _, err := GenerateKeyPair()
	require.NoError(t, err)

	id := KeyID(pub)

	// Check prefix.
	assert.True(t, strings.HasPrefix(id, "keel:sha256:"), "KeyID should start with 'keel:sha256:'")

	// Check overall format: "keel:sha256:" + 64 hex chars = 76 total.
	assert.Equal(t, 76, len(id), "KeyID should be 76 characters total (keel:sha256: + 64 hex)")

	// Extract hex portion and verify it's valid hex.
	hexPart := strings.TrimPrefix(id, "keel:sha256:")
	assert.Len(t, hexPart, 64, "hex portion should be 64 characters")
	assert.Regexp(t, "^[a-f0-9]{64}$", hexPart, "hex portion should be lowercase hex")
}

func TestKeyID_Uniqueness(t *testing.T) {
	t.Parallel()

	// Generate two different keypairs.
	pub1, _, err := GenerateKeyPair()
	require.NoError(t, err)

	pub2, _, err := GenerateKeyPair()
	require.NoError(t, err)

	id1 := KeyID(pub1)
	id2 := KeyID(pub2)

	// Different keys should produce different IDs.
	assert.NotEqual(t, id1, id2, "different keys should produce different KeyIDs")
}

func TestMarshalRoundtrip_PrivateKey(t *testing.T) {
	t.Parallel()

	_, priv, err := GenerateKeyPair()
	require.NoError(t, err)

	// Marshal to PEM.
	pemData, err := MarshalPrivateKey(priv)
	require.NoError(t, err)
	require.NotEmpty(t, pemData)

	// Verify PEM format.
	assert.Contains(t, string(pemData), "-----BEGIN PRIVATE KEY-----")
	assert.Contains(t, string(pemData), "-----END PRIVATE KEY-----")

	// Unmarshal back.
	recovered, err := UnmarshalPrivateKey(pemData)
	require.NoError(t, err)
	require.NotNil(t, recovered)

	// Verify bytes match.
	assert.Equal(t, []byte(priv), []byte(recovered), "roundtrip should preserve private key bytes")
}

func TestMarshalRoundtrip_PublicKey(t *testing.T) {
	t.Parallel()

	pub, _, err := GenerateKeyPair()
	require.NoError(t, err)

	// Marshal to PEM.
	pemData, err := MarshalPublicKey(pub)
	require.NoError(t, err)
	require.NotEmpty(t, pemData)

	// Verify PEM format.
	assert.Contains(t, string(pemData), "-----BEGIN PUBLIC KEY-----")
	assert.Contains(t, string(pemData), "-----END PUBLIC KEY-----")

	// Unmarshal back.
	recovered, err := UnmarshalPublicKey(pemData)
	require.NoError(t, err)
	require.NotNil(t, recovered)

	// Verify bytes match.
	assert.Equal(t, []byte(pub), []byte(recovered), "roundtrip should preserve public key bytes")
}

func TestUnmarshalPrivateKey_InvalidPEM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pemData   []byte
		wantError string
	}{
		{
			name:      "garbage data",
			pemData:   []byte("not a PEM file"),
			wantError: "failed to decode PEM block",
		},
		{
			name: "wrong PEM type",
			pemData: []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
-----END PUBLIC KEY-----`),
			wantError: "unexpected PEM block type",
		},
		{
			name: "invalid PKCS8 data",
			pemData: []byte(`-----BEGIN PRIVATE KEY-----
aW52YWxpZCBkYXRh
-----END PRIVATE KEY-----`),
			wantError: "parsing PKCS8 private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := UnmarshalPrivateKey(tt.pemData)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestUnmarshalPublicKey_InvalidPEM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pemData   []byte
		wantError string
	}{
		{
			name:      "garbage data",
			pemData:   []byte("not a PEM file"),
			wantError: "failed to decode PEM block",
		},
		{
			name: "wrong PEM type",
			pemData: []byte(`-----BEGIN PRIVATE KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
-----END PRIVATE KEY-----`),
			wantError: "unexpected PEM block type",
		},
		{
			name: "invalid PKIX data",
			pemData: []byte(`-----BEGIN PUBLIC KEY-----
aW52YWxpZCBkYXRh
-----END PUBLIC KEY-----`),
			wantError: "parsing PKIX public key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := UnmarshalPublicKey(tt.pemData)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestSignVerify_CrossKeyValidation(t *testing.T) {
	t.Parallel()

	// Generate two different keypairs.
	pub1, priv1, err := GenerateKeyPair()
	require.NoError(t, err)

	pub2, _, err := GenerateKeyPair()
	require.NoError(t, err)

	testData := []byte("test message")

	// Sign with key 1.
	sig := ed25519.Sign(priv1, testData)

	// Verify with correct public key — should succeed.
	assert.True(t, ed25519.Verify(pub1, testData, sig))

	// Verify with wrong public key — should fail.
	assert.False(t, ed25519.Verify(pub2, testData, sig), "signature should not verify with wrong public key")
}
