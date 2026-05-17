package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
)

func newCreateTransformDeps() (transform.Analyzer, transform.Engine) {
	return transform.NewStarlarkAnalyzer(), transform.NewStarlarkEngine(time.Second)
}

func TestRunCreateTransform_PureStarlark_OK(t *testing.T) {
	an, eng := newCreateTransformDeps()
	res := runCreateTransform(context.Background(), an, eng, CreateTransformArgs{
		Name:        "shout",
		Description: "uppercases input",
		Language:    "starlark",
		Code:        "def transform(input):\n    return input.upper()",
		InputSample: "hi",
	})

	require.Equal(t, string(transform.StatusOK), res.Status)
	assert.Equal(t, "HI", res.Output)
	assert.Equal(t, "shout", res.ToolName)
	assert.Equal(t, "tier_1", res.FloorTier, "pure computation floors at tier_1")
}

func TestRunCreateTransform_JSONTransform_Tier2(t *testing.T) {
	an, eng := newCreateTransformDeps()
	res := runCreateTransform(context.Background(), an, eng, CreateTransformArgs{
		Name:        "double",
		Language:    "starlark",
		Code:        `def transform(input):` + "\n" + `    return json.encode([x*2 for x in json.decode(input)])`,
		InputSample: "[1,2,3]",
	})

	require.Equal(t, string(transform.StatusOK), res.Status)
	assert.Equal(t, "[2,4,6]", res.Output)
	assert.Equal(t, "tier_2", res.FloorTier, "json.* floors at tier_2")
}

func TestRunCreateTransform_SyntaxError_NoExecution(t *testing.T) {
	an, eng := newCreateTransformDeps()
	res := runCreateTransform(context.Background(), an, eng, CreateTransformArgs{
		Name:     "broken",
		Language: "starlark",
		Code:     "def transform(:\n  bad",
	})

	assert.Equal(t, string(transform.StatusSyntaxError), res.Status)
	assert.Empty(t, res.Output, "must not execute unparseable code")
	assert.NotEmpty(t, res.Error, "syntax error must carry the parser message for the LLM fix")
}

func TestRunCreateTransform_LoadRejected(t *testing.T) {
	an, eng := newCreateTransformDeps()
	res := runCreateTransform(context.Background(), an, eng, CreateTransformArgs{
		Name:     "loader",
		Language: "starlark",
		Code:     `load("x.star", "y")` + "\n" + `def transform(input):` + "\n" + `    return y(input)`,
	})

	assert.Equal(t, string(transform.StatusAnalyzerRejected), res.Status)
	assert.Empty(t, res.Output)
	assert.NotEmpty(t, res.Error)
}

func TestRunCreateTransform_RuntimeError_Reported(t *testing.T) {
	an, eng := newCreateTransformDeps()
	res := runCreateTransform(context.Background(), an, eng, CreateTransformArgs{
		Name:     "boom",
		Language: "starlark",
		Code:     "def transform(input):\n    return 1 // 0",
	})

	assert.Equal(t, string(transform.StatusNonzeroExit), res.Status)
	assert.NotEmpty(t, res.Error)
}

func TestRunCreateTransform_UnsupportedLanguage_Rejected(t *testing.T) {
	an, eng := newCreateTransformDeps()
	res := runCreateTransform(context.Background(), an, eng, CreateTransformArgs{
		Name:     "py",
		Language: "python", // stage 1 only wires Starlark
		Code:     "def transform(input):\n    return input",
	})

	assert.Equal(t, string(transform.StatusNonzeroExit), res.Status)
	assert.Contains(t, res.Error, "python")
}
