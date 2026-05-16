package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyze(t *testing.T, code string) AnalysisResult {
	t.Helper()
	return NewStarlarkAnalyzer().Analyze(code)
}

// ── syntax_error: パース失敗は実行前に弾く ───────────────────────────────────

func TestStarlarkAnalyze_BrokenSyntax_SyntaxErr(t *testing.T) {
	r := analyze(t, "def f(:\n  return 1")
	assert.True(t, r.SyntaxErr, "unparseable code must be syntax_error")
	assert.False(t, r.Rejected)
	assert.NotEmpty(t, r.Reason, "syntax error must carry parser message for LLM fix")
}

// ── analyzer_rejected: load() は常に禁止 ─────────────────────────────────────

func TestStarlarkAnalyze_LoadStmt_Rejected(t *testing.T) {
	r := analyze(t, `load("other.star", "helper")
def transform(x):
    return helper(x)`)
	assert.True(t, r.Rejected, "load() must be rejected outright")
	assert.False(t, r.SyntaxErr)
	assert.NotEmpty(t, r.Reason)
}

// ── tier_3: 危険 builtin / 擬似 IO ──────────────────────────────────────────

func TestStarlarkAnalyze_DangerousBuiltins_Tier3(t *testing.T) {
	cases := map[string]string{
		"open":      `def t(x):\n    return open(x)`,
		"eval":      `def t(x):\n    return eval(x)`,
		"exec":      `def t(x):\n    return exec(x)`,
		"os dotted": `def t(x):\n    return os.environ`,
		"socket":    `def t(x):\n    return socket.socket()`,
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			// the \n in the map literal is literal backslash-n; normalize
			r := analyze(t, unescape(code))
			require.False(t, r.SyntaxErr, "should parse: %s", name)
			require.False(t, r.Rejected, "dangerous builtin is tier_3, not rejected")
			assert.Equal(t, Tier3, r.FloorTier, "%s must floor at tier_3", name)
		})
	}
}

// ── tier_2: JSON 構造変換 ───────────────────────────────────────────────────

func TestStarlarkAnalyze_JSONDecode_Tier2(t *testing.T) {
	r := analyze(t, "def t(x):\n    return json.decode(x)")
	require.False(t, r.SyntaxErr)
	require.False(t, r.Rejected)
	assert.Equal(t, Tier2, r.FloorTier)
}

func TestStarlarkAnalyze_JSONEncode_Tier2(t *testing.T) {
	r := analyze(t, "def t(x):\n    return json.encode(x)")
	assert.Equal(t, Tier2, r.FloorTier)
}

// ── tier_1: 純粋計算・文字列変換のみ ─────────────────────────────────────────

func TestStarlarkAnalyze_PureComputation_Tier1(t *testing.T) {
	r := analyze(t, `def transform(rows):
    out = []
    for r in rows:
        out.append(r.strip().upper())
    return out`)
	require.False(t, r.SyntaxErr)
	require.False(t, r.Rejected)
	assert.Equal(t, Tier1, r.FloorTier, "no dangerous signal == tier_1")
}

// ── 危険シグナルが混在したら最も高い floor を採る ────────────────────────────

func TestStarlarkAnalyze_TakesHighestFloor(t *testing.T) {
	r := analyze(t, `def t(x):
    y = json.decode(x)
    return open(y)`)
	assert.Equal(t, Tier3, r.FloorTier, "json(tier_2) + open(tier_3) -> tier_3")
}

// unescape turns the literal \n in the table above into real newlines so the
// Starlark parser sees valid source.
func unescape(s string) string {
	out := make([]rune, 0, len(s))
	prev := rune(0)
	for _, c := range s {
		if prev == '\\' && c == 'n' {
			out[len(out)-1] = '\n'
			prev = 0
			continue
		}
		out = append(out, c)
		prev = c
	}
	return string(out)
}
