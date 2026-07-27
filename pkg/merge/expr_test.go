package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_evalCondition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		expr   string
		vars   map[string]string
		want   bool
		errMsg string // non-empty means we expect an error containing this
	}{
		// --- equality / inequality ---
		{"equal match", "keel.agent == 'claude'", map[string]string{"keel.agent": "claude"}, true, ""},
		{"equal no match", "keel.agent == 'cursor'", map[string]string{"keel.agent": "claude"}, false, ""},
		{"not equal match", "keel.agent != 'cursor'", map[string]string{"keel.agent": "claude"}, true, ""},
		{"double quotes", `keel.agent == "claude"`, map[string]string{"keel.agent": "claude"}, true, ""},
		{"missing var", "keel.missing == 'claude'", map[string]string{"keel.agent": "claude"}, false, ""},
		{"operator in quoted value", "msg == 'a != b'", map[string]string{"msg": "a != b"}, true, ""},
		{"escaped single quote in value", `msg == 'it\'s fine'`, map[string]string{"msg": `it\'s fine`}, true, ""},
		{"escaped double quote in value", `msg == "say \"hello\""`, map[string]string{"msg": `say \"hello\"`}, true, ""},

		// --- logical operators ---
		{"or first match", "keel.agent == 'claude' || keel.agent == 'cursor'", map[string]string{"keel.agent": "claude"}, true, ""},
		{"or second match", "keel.agent == 'claude' || keel.agent == 'cursor'", map[string]string{"keel.agent": "cursor"}, true, ""},
		{"or no match", "keel.agent == 'claude' || keel.agent == 'cursor'", map[string]string{"keel.agent": "theia"}, false, ""},
		{"or double quotes", `keel.agent == "claude" || keel.agent == "cursor"`, map[string]string{"keel.agent": "claude"}, true, ""},
		{"and both match", "keel.agent == 'claude' && keel.scope == 'project'", map[string]string{"keel.agent": "claude", "keel.scope": "project"}, true, ""},
		{"and partial match", "keel.agent == 'claude' && keel.scope == 'project'", map[string]string{"keel.agent": "claude", "keel.scope": "user"}, false, ""},

		// --- in / not in ---
		{"in first match", `keel.agent in ["claude", "cursor"]`, map[string]string{"keel.agent": "claude"}, true, ""},
		{"in second match", `keel.agent in ["claude", "cursor"]`, map[string]string{"keel.agent": "cursor"}, true, ""},
		{"in no match", `keel.agent in ["claude", "cursor"]`, map[string]string{"keel.agent": "theia"}, false, ""},
		{"in single quotes", "keel.agent in ['claude', 'cursor']", map[string]string{"keel.agent": "claude"}, true, ""},
		{"in empty list", "keel.agent in []", map[string]string{"keel.agent": "claude"}, false, ""},
		{"not in match", `keel.agent not in ["gemini"]`, map[string]string{"keel.agent": "gemini"}, false, ""},
		{"not in no match", `keel.agent not in ["gemini"]`, map[string]string{"keel.agent": "claude"}, true, ""},
		{"in with logical and", "keel.agent in ['claude', 'cursor'] && keel.scope == 'project'", map[string]string{"keel.agent": "claude", "keel.scope": "project"}, true, ""},
		{"escaped quote in or clause", `msg == 'it\'s ok' || msg == 'fine'`, map[string]string{"msg": `it\'s ok`}, true, ""},
		{"escaped quote in in-list", `msg in ['it\'s', 'fine']`, map[string]string{"msg": `it\'s`}, true, ""},

		// --- error cases ---
		{"empty expression", "", map[string]string{"keel.agent": "claude"}, false, "empty"},
		{"no operator", "keel.agent", map[string]string{"keel.agent": "claude"}, false, "operator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := evalCondition(tt.expr, tt.vars)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}
