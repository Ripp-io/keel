package remote

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// validateIP tests
// ---------------------------------------------------------------------------

func TestValidateIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		addr    string
		wantErr bool
		errMsg  string
	}{
		// Unspecified
		{name: "unspecified IPv4", addr: "0.0.0.0", wantErr: true, errMsg: "unspecified"},
		{name: "unspecified IPv6", addr: "::", wantErr: true, errMsg: "unspecified"},

		// Loopback
		{name: "loopback IPv4", addr: "127.0.0.1", wantErr: true, errMsg: "loopback"},
		{name: "loopback IPv6", addr: "::1", wantErr: true, errMsg: "loopback"},

		// Private
		{name: "private 10.x", addr: "10.0.0.1", wantErr: true, errMsg: "private"},
		{name: "private 172.16.x", addr: "172.16.0.1", wantErr: true, errMsg: "private"},
		{name: "private 192.168.x", addr: "192.168.1.1", wantErr: true, errMsg: "private"},

		// Link-local
		{name: "link-local IPv4", addr: "169.254.169.254", wantErr: true, errMsg: "link-local"},
		{name: "link-local IPv6", addr: "fe80::1", wantErr: true, errMsg: "link-local"},

		// Public (allowed)
		{name: "public 8.8.8.8", addr: "8.8.8.8", wantErr: false},
		{name: "public 1.1.1.1", addr: "1.1.1.1", wantErr: false},
		{name: "public 203.0.113.1", addr: "203.0.113.1", wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.addr)
			require.NotNil(t, ip, "net.ParseIP(%q) returned nil", tt.addr)

			err := validateIP(ip)
			if tt.wantErr {
				require.Error(t, err, "validateIP(%s)", tt.addr)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err, "validateIP(%s)", tt.addr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ssrfSafeDialContext tests -- DNS rebinding protection
// ---------------------------------------------------------------------------

func TestSSRFSafeDialContext_RejectsLoopbackResolution(t *testing.T) {
	skipSSRFValidation.Store(false)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })

	ctx := context.Background()
	_, err := ssrfSafeDialContext(ctx, "tcp", "localhost:80")
	require.Error(t, err, "connecting to localhost should be rejected")
	assert.Contains(t, err.Error(), "DNS rebinding protection")
	assert.Contains(t, err.Error(), "loopback")
}

func TestSSRFSafeDialContext_SkipsWhenDisabled(t *testing.T) {
	skipSSRFValidation.Store(true)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })

	// This should not error from the SSRF check (though the connection
	// itself may fail because nothing listens on this port).
	ctx := context.Background()
	_, err := ssrfSafeDialContext(ctx, "tcp", "localhost:1")
	if err != nil {
		// Connection refused is OK -- the important thing is it was NOT
		// rejected by SSRF validation.
		assert.NotContains(t, err.Error(), "DNS rebinding",
			"SSRF check should be skipped in test mode")
	}
}

func TestSSRFSafeDialContext_InvalidAddress(t *testing.T) {
	skipSSRFValidation.Store(false)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })

	ctx := context.Background()
	_, err := ssrfSafeDialContext(ctx, "tcp", "no-port")
	require.Error(t, err, "address without port should fail")
}

// ---------------------------------------------------------------------------
// HardenedClient integration test
// ---------------------------------------------------------------------------

func TestHardenedClient(t *testing.T) {
	t.Parallel()
	client := HardenedClient()

	assert.Equal(t, 120*time.Second, client.Timeout, "timeout")
	assert.NotNil(t, client.Transport, "should have custom Transport for DNS rebinding protection")
	assert.NotNil(t, client.CheckRedirect, "should have CheckRedirect function")
}

// ---------------------------------------------------------------------------
// ValidateURL tests
// ---------------------------------------------------------------------------

func TestValidateURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "rejects private IP", url: "https://10.0.0.1/path", wantErr: true},
		{name: "accepts public IP", url: "https://8.8.8.8/path", wantErr: false},
		{name: "accepts DNS name", url: "https://example.com/path", wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
