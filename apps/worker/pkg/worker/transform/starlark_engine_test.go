package transform

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runStar(t *testing.T, code, input string) ExecResult {
	t.Helper()
	return NewStarlarkEngine(time.Second).Execute(ExecRequest{Code: code, Input: input})
}

func TestStarlarkEngine_Language(t *testing.T) {
	assert.Equal(t, LangStarlark, NewStarlarkEngine(time.Second).Language())
}

func TestStarlarkEngine_PureTransform_OK(t *testing.T) {
	res := runStar(t, "def transform(input):\n    return input.upper()", "hello")
	require.Equal(t, StatusOK, res.Status, res.Stderr)
	assert.Equal(t, "HELLO", res.Output)
}

func TestStarlarkEngine_JSONRoundTrip_OK(t *testing.T) {
	code := `def transform(input):
    rows = json.decode(input)
    return json.encode([r["n"] * 2 for r in rows])`
	res := runStar(t, code, `[{"n": 1}, {"n": 2}]`)
	require.Equal(t, StatusOK, res.Status, res.Stderr)
	assert.Equal(t, "[2,4]", res.Output)
}

func TestStarlarkEngine_MissingTransformFunc_NonzeroExit(t *testing.T) {
	res := runStar(t, "x = 1", "in")
	assert.Equal(t, StatusNonzeroExit, res.Status)
	assert.NotEmpty(t, res.Stderr, "must explain that transform() is required")
}

func TestStarlarkEngine_RuntimeError_NonzeroExit(t *testing.T) {
	res := runStar(t, "def transform(input):\n    return 1 // 0", "in")
	assert.Equal(t, StatusNonzeroExit, res.Status)
	assert.NotEmpty(t, res.Stderr)
}

func TestStarlarkEngine_NonStringReturn_NonzeroExit(t *testing.T) {
	res := runStar(t, "def transform(input):\n    return 42", "in")
	assert.Equal(t, StatusNonzeroExit, res.Status)
	assert.Contains(t, strings.ToLower(res.Stderr), "string")
}

func TestStarlarkEngine_InfiniteLoop_Timeout(t *testing.T) {
	code := `def transform(input):
    x = 0
    for i in range(10000000000):
        x += i
    return str(x)`
	res := NewStarlarkEngine(50 * time.Millisecond).Execute(ExecRequest{Code: code, Input: "in"})
	assert.Equal(t, StatusTimeout, res.Status, "long-running code must hit the timeout")
}

func TestStarlarkEngine_NoFilesystemBuiltin_NonzeroExit(t *testing.T) {
	// open is not provided to the sandbox; calling it is a runtime error,
	// not a successful escape.
	res := runStar(t, "def transform(input):\n    return open('/etc/passwd')", "in")
	assert.Equal(t, StatusNonzeroExit, res.Status)
}

func TestStarlarkEngine_Deterministic_SameInputSameOutput(t *testing.T) {
	code := "def transform(input):\n    return input + '!'"
	a := runStar(t, code, "x")
	b := runStar(t, code, "x")
	assert.Equal(t, a.Output, b.Output)
	assert.Equal(t, StatusOK, a.Status)
}
