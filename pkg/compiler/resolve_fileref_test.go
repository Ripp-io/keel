package compiler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/remote"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/schema"
)

// disableSSRFForResolveTest disables SSRF host validation for the duration of
// the test, allowing httptest.NewServer (which binds to 127.0.0.1) to be used.
func disableSSRFForResolveTest(t *testing.T) {
	t.Helper()
	remote.DisableSSRFForTest()
	t.Cleanup(func() { remote.RestoreSSRFForTest() })
}

func TestResolveRemoteFileRefs_HTTPSSource(t *testing.T) {
	disableSSRFForResolveTest(t)

	content := "# Remote Instructions"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: ts.URL + "/instructions/security.md"},
		},
	}

	err := resolveRemoteFileRefs(cfg, cacheBase, false)
	require.NoError(t, err)

	// The source should now be a local cache path, not the URL.
	assert.False(t, strings.HasPrefix(cfg.Instructions[0].Source, "http"),
		"source should be replaced with local path, got %q", cfg.Instructions[0].Source)

	// The resolved file should be readable with correct content.
	data, err := os.ReadFile(cfg.Instructions[0].Source)
	require.NoError(t, err, "reading resolved file")
	assert.Equal(t, content, string(data))
}

func TestResolveRemoteFileRefs_LocalSource(t *testing.T) {
	t.Parallel()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: "instructions.md"},
		},
	}

	err := resolveRemoteFileRefs(cfg, t.TempDir(), false)
	require.NoError(t, err)

	// Local paths should remain unchanged.
	assert.Equal(t, "instructions.md", cfg.Instructions[0].Source)
}

func TestResolveRemoteFileRefs_EmptySource(t *testing.T) {
	t.Parallel()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: ""},
		},
	}

	err := resolveRemoteFileRefs(cfg, t.TempDir(), false)
	require.NoError(t, err)

	// Empty source should remain unchanged.
	assert.Equal(t, "", cfg.Instructions[0].Source)
}

func TestResolveRemoteFileRefs_DirectorySource(t *testing.T) {
	t.Parallel()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: "instructions/"},
		},
	}

	err := resolveRemoteFileRefs(cfg, t.TempDir(), false)
	require.NoError(t, err)

	// Source field entries pointing to directories should be skipped (directories not supported for remote).
	assert.Equal(t, "instructions/", cfg.Instructions[0].Source)
}

func TestResolveRemoteFileRefs_MultipleRefs(t *testing.T) {
	disableSSRFForResolveTest(t)

	contents := map[string]string{
		"/instructions/security.md": "# Security",
		"/mcp/sonarqube.json":       `{"name": "sonarqube"}`,
		"/rules/naming.md":          "# Naming Rules",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := contents[r.URL.Path]; ok {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(c))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: ts.URL + "/instructions/security.md"},
			{Source: "local-instructions.md"}, // local — should stay
		},
		MCP: &schema.MCPConfig{
			Servers: []schema.FileRef{
				{Source: ts.URL + "/mcp/sonarqube.json"},
			},
		},
		Rules: []schema.FileRef{
			{Source: ts.URL + "/rules/naming.md"},
		},
	}

	err := resolveRemoteFileRefs(cfg, cacheBase, false)
	require.NoError(t, err)

	// Remote instruction source should be resolved.
	assert.False(t, strings.HasPrefix(cfg.Instructions[0].Source, "http"),
		"instructions[0] source should be local, got %q", cfg.Instructions[0].Source)
	data, err := os.ReadFile(cfg.Instructions[0].Source)
	require.NoError(t, err, "reading instructions[0]")
	assert.Equal(t, "# Security", string(data))

	// Local instruction source should be unchanged.
	assert.Equal(t, "local-instructions.md", cfg.Instructions[1].Source)

	// Remote MCP source should be resolved.
	assert.False(t, strings.HasPrefix(cfg.MCP.Servers[0].Source, "http"),
		"mcp.servers[0] source should be local, got %q", cfg.MCP.Servers[0].Source)
	data, err = os.ReadFile(cfg.MCP.Servers[0].Source)
	require.NoError(t, err, "reading mcp.servers[0]")
	assert.Equal(t, `{"name": "sonarqube"}`, string(data))

	// Remote rules source should be resolved.
	assert.False(t, strings.HasPrefix(cfg.Rules[0].Source, "http"),
		"rules[0] source should be local, got %q", cfg.Rules[0].Source)
	data, err = os.ReadFile(cfg.Rules[0].Source)
	require.NoError(t, err, "reading rules[0]")
	assert.Equal(t, "# Naming Rules", string(data))
}

func TestResolveRemoteFileRefs_FetchError(t *testing.T) {
	disableSSRFForResolveTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: ts.URL + "/broken.md"},
		},
	}

	err := resolveRemoteFileRefs(cfg, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestResolveRemoteFileRefs_NilConfig(t *testing.T) {
	t.Parallel()
	// nil MCP and Permissions should not panic.
	cfg := &schema.KeelConfig{
		MCP:         nil,
		Permissions: nil,
	}

	err := resolveRemoteFileRefs(cfg, t.TempDir(), false)
	require.NoError(t, err)
}

func TestResolveRemoteFileRefs_ForceFieldPreserved(t *testing.T) {
	disableSSRFForResolveTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("locked content"))
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	cfg := &schema.KeelConfig{
		Instructions: []schema.FileRef{
			{Source: ts.URL + "/locked.md", Force: true},
		},
	}

	err := resolveRemoteFileRefs(cfg, cacheBase, false)
	require.NoError(t, err)

	// Force field should be preserved after resolution.
	assert.True(t, cfg.Instructions[0].Force, "Force field should be preserved after URL resolution")

	// Source should be resolved to a local path.
	assert.False(t, strings.HasPrefix(cfg.Instructions[0].Source, "http"),
		"source should be resolved, got %q", cfg.Instructions[0].Source)
}
