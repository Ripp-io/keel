package remote

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/testutil"
)

// disableSSRFForTest disables SSRF host validation for the duration of the
// test, allowing httptest.NewServer (which binds to 127.0.0.1) to be used.
// The cleanup function restores the original value.
func disableSSRFForTest(t *testing.T) {
	t.Helper()
	skipSSRFValidation.Store(true)
	t.Cleanup(func() { skipSSRFValidation.Store(false) })
}

// ---------------------------------------------------------------------------
// Resolve() dispatcher tests
// ---------------------------------------------------------------------------

func TestResolve_SourceLocal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		segment string
	}{
		{name: "absolute path", segment: "/some/local/path"},
		{name: "relative path", segment: "relative/path"},
		{name: "dot path", segment: "."},
		{name: "parent dir", segment: ".."},
		{name: "single word", segment: "base"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			localPath, originalURL, err := Resolve(tt.segment, t.TempDir(), false)
			require.NoError(t, err)
			assert.Equal(t, tt.segment, localPath)
			assert.Empty(t, originalURL)
		})
	}
}

func TestResolve_ParseError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		segment string
		errMsg  string
	}{
		{
			name:    "http URL rejected",
			segment: "http://example.com/layers.tar.gz",
			errMsg:  "https",
		},
		{
			name:    "ext:: protocol rejected",
			segment: "ext::sh -c 'evil'",
			errMsg:  "ext::",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Resolve(tt.segment, t.TempDir(), false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestResolve_SourceGit_Dispatches(t *testing.T) {
	t.Parallel()
	// A valid git URL that will fail during FetchGit because the repo
	// doesn't exist. This verifies the SourceGit case dispatches correctly.
	cacheBase := t.TempDir()
	_, _, err := Resolve("https://127.0.0.1:1/org/repo.git", cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloning")
}

func TestResolve_SourceArchive_Dispatches(t *testing.T) {
	// Verify the SourceArchive case in Resolve dispatches to FetchHTTP.
	// A loopback HTTPS URL passes Parse (HTTPS scheme) but FetchHTTP
	// rejects it via SSRF protection.
	cacheBase := t.TempDir()
	_, _, err := Resolve("https://127.0.0.1:1/layers.tar.gz", cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private/loopback")
}

func TestResolve_SourceArchive_Success(t *testing.T) {
	disableSSRFForTest(t)
	archivePath := testutil.CreateTestTarGz(t, t.TempDir(), map[string]string{
		"keel.yml": "via-resolve",
	})

	ts := httptest.NewServer(serveArchive(t, archivePath, "", ""))
	defer ts.Close()

	cacheBase := t.TempDir()

	// Call FetchHTTP directly to simulate what Resolve does (since Resolve
	// requires https:// but httptest serves http://).
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/layers.tar.gz",
		Original: ts.URL + "/layers.tar.gz",
	}

	path, err := FetchHTTP(src, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(path, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "via-resolve", string(data))
}

func TestResolve_SourceGit_Success(t *testing.T) {
	t.Parallel()
	bareDir := setupBareRepo(t, map[string]string{
		"keel.yml": "via-git-resolve",
	})

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceGit,
		RawURL:   bareDir,
		Original: bareDir,
	}

	path, err := FetchGit(src, cacheBase, false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(path, "keel.yml"))
	require.NoError(t, err)
	assert.Equal(t, "via-git-resolve", string(data))
}

// ---------------------------------------------------------------------------
// FetchHTTP additional edge cases
// ---------------------------------------------------------------------------

func TestFetchHTTP_ServerError_500(t *testing.T) {
	disableSSRFForTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cacheBase := t.TempDir()
	src := ParsedSource{
		Type:     SourceArchive,
		RawURL:   ts.URL + "/error.tar.gz",
		Original: ts.URL + "/error.tar.gz",
	}

	_, err := FetchHTTP(src, cacheBase, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}
