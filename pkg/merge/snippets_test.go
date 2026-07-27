package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSnippets_SimpleInclude(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets([]byte("Before\n<!-- keel:include greeting -->\nAfter"), map[string][]byte{"greeting": []byte("Hello, world!")})
	require.NoError(t, err)
	assert.Equal(t, "Before\nHello, world!\nAfter", string(got))
}

func TestResolveSnippets_MissingSnippet(t *testing.T) {
	t.Parallel()
	_, err := ResolveSnippets([]byte("<!-- keel:include nonexistent -->"), map[string][]byte{})
	require.Error(t, err)
	assert.Equal(t, `snippet "nonexistent" not found`, err.Error())
}

func TestResolveSnippets_MultipleIncludes(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets(
		[]byte("Start\n<!-- keel:include alpha -->\nMiddle\n<!-- keel:include beta -->\nEnd"),
		map[string][]byte{"alpha": []byte("Alpha content"), "beta": []byte("Beta content")},
	)
	require.NoError(t, err)
	assert.Equal(t, "Start\nAlpha content\nMiddle\nBeta content\nEnd", string(got))
}

func TestResolveSnippets_InsideCodeBlock(t *testing.T) {
	t.Parallel()
	content := []byte("Before\n```\n<!-- keel:include greeting -->\n```\nAfter")
	got, err := ResolveSnippets(content, map[string][]byte{"greeting": []byte("Hello, world!")})
	require.NoError(t, err)
	assert.Equal(t, string(content), string(got), "marker inside code block should be literal")
}

func TestResolveSnippets_MultiLineSnippet(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets(
		[]byte("Before\n<!-- keel:include steps -->\nAfter"),
		map[string][]byte{"steps": []byte("Step 1\nStep 2\nStep 3")},
	)
	require.NoError(t, err)
	assert.Equal(t, "Before\nStep 1\nStep 2\nStep 3\nAfter", string(got))
}

func TestResolveSnippets_NoMarkers(t *testing.T) {
	t.Parallel()
	content := []byte("Just plain text\nNo markers here")
	got, err := ResolveSnippets(content, map[string][]byte{"unused": []byte("Unused")})
	require.NoError(t, err)
	assert.Equal(t, string(content), string(got))
}

func TestResolveSnippets_EmptySnippets(t *testing.T) {
	t.Parallel()
	content := []byte("No markers, no snippets")
	t.Run("nil map", func(t *testing.T) {
		got, err := ResolveSnippets(content, nil)
		require.NoError(t, err)
		assert.Equal(t, string(content), string(got))
	})
	t.Run("empty map", func(t *testing.T) {
		got, err := ResolveSnippets(content, map[string][]byte{})
		require.NoError(t, err)
		assert.Equal(t, string(content), string(got))
	})
}

func TestResolveSnippets_NameWithHyphensUnderscores(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets([]byte("<!-- keel:include my-snippet_v2 -->"), map[string][]byte{"my-snippet_v2": []byte("Hyphen and underscore work")})
	require.NoError(t, err)
	assert.Equal(t, "Hyphen and underscore work", string(got))
}

func TestResolveSnippets_IndentationPreserved(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets([]byte("- Item:\n    <!-- keel:include detail -->"), map[string][]byte{"detail": []byte("Line 1\nLine 2\nLine 3")})
	require.NoError(t, err)
	assert.Equal(t, "- Item:\n    Line 1\n    Line 2\n    Line 3", string(got))
}

func TestResolveSnippets_EmptySnippetContent(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets([]byte("Before\n<!-- keel:include empty -->\nAfter"), map[string][]byte{"empty": []byte("")})
	require.NoError(t, err)
	assert.Equal(t, "Before\n\nAfter", string(got))
}

func TestResolveSnippets_TrailingNewlineHandling(t *testing.T) {
	t.Parallel()
	got, err := ResolveSnippets([]byte("Before\n<!-- keel:include trailing -->\nAfter"), map[string][]byte{"trailing": []byte("Content with trailing newline\n")})
	require.NoError(t, err)
	assert.Equal(t, "Before\nContent with trailing newline\nAfter", string(got))
}

func TestResolveSnippets_Parameterized(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  string
		snippets map[string][]byte
		want     string
	}{
		{"single parameter", `<!-- keel:include tip domain="security" -->`, map[string][]byte{"tip": []byte("Always follow {{domain}} best practices.")}, "Always follow security best practices."},
		{"multiple parameters", `<!-- keel:include rule domain="auth" action="validate tokens" -->`, map[string][]byte{"rule": []byte("In the {{domain}} domain, always {{action}}.")}, "In the auth domain, always validate tokens."},
		{"unmatched placeholder left as-is", `<!-- keel:include tip domain="security" -->`, map[string][]byte{"tip": []byte("Follow {{domain}} rules and {{other}} guidelines.")}, "Follow security rules and {{other}} guidelines."},
		{"no params leaves placeholders alone", "<!-- keel:include tip -->", map[string][]byte{"tip": []byte("Content with {{placeholder}} left alone.")}, "Content with {{placeholder}} left alone."},
		{"multi-line snippet", `<!-- keel:include checklist phase="review" -->`, map[string][]byte{"checklist": []byte("## {{phase}} Checklist\n1. Check for bugs\n2. Run {{phase}} tests")}, "## review Checklist\n1. Check for bugs\n2. Run review tests"},
		{"with indentation", `  <!-- keel:include item name="deploy" -->`, map[string][]byte{"item": []byte("- {{name}}: execute")}, "  - deploy: execute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveSnippets([]byte(tt.content), tt.snippets)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestFindUnresolvedPlaceholders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		params  map[string]string
		want    []string
	}{
		{"all matched", "Hello {{name}}, welcome to {{domain}}.", map[string]string{"name": "Alice", "domain": "security"}, nil},
		{"all unmatched", "Follow {{domain}} rules and {{action}} guidelines.", map[string]string{}, []string{"domain", "action"}},
		{"mixed", "In {{domain}}, always {{action}} and check {{extra}}.", map[string]string{"domain": "auth"}, []string{"action", "extra"}},
		{"nil params", "Check {{item}} now.", nil, []string{"item"}},
		{"no placeholders", "No placeholders here at all.", map[string]string{"unused": "value"}, nil},
		{"duplicates deduped", "{{typo}} and then {{typo}} again.", map[string]string{}, []string{"typo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FindUnresolvedPlaceholders([]byte(tt.content), tt.params)
			if tt.want == nil {
				assert.Empty(t, got)
			} else {
				assert.Len(t, got, len(tt.want))
				for _, w := range tt.want {
					assert.Contains(t, got, w)
				}
			}
		})
	}
}
