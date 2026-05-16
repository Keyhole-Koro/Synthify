package starlarkrt

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

func TestAnalyzeComputesStarlarkFloorTier(t *testing.T) {
	engine := New()
	tests := []struct {
		name       string
		code       string
		wantStatus treev1.ExecuteStatus
		wantTier   treev1.RiskTier
		wantDiag   string
	}{
		{
			name: "pure string transform is tier1",
			code: `
def transform(input):
    return input.strip().upper()
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_OK,
			wantTier:   treev1.RiskTier_RISK_TIER_1,
		},
		{
			name: "json structural transform is tier2",
			code: `
def transform(input):
    data = json.decode(input)
    return json.encode({"name": data["name"]})
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_OK,
			wantTier:   treev1.RiskTier_RISK_TIER_2,
			wantDiag:   "json.decode",
		},
		{
			name: "disallowed filesystem-like call is tier3",
			code: `
def transform(input):
    return open("/etc/passwd")
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_OK,
			wantTier:   treev1.RiskTier_RISK_TIER_3,
			wantDiag:   "open",
		},
		{
			name: "load is rejected before execution",
			code: `
load("@repo//:file.star", "helper")
def transform(input):
    return input
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_ANALYZER_REJECTED,
			wantTier:   treev1.RiskTier_RISK_TIER_3,
			wantDiag:   "load statements are not allowed",
		},
		{
			name: "syntax error is reported separately",
			code: `
def transform(input):
    if
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_SYNTAX_ERROR,
			wantTier:   treev1.RiskTier_RISK_TIER_3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.Analyze(tc.code)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Equal(t, tc.wantTier, got.FloorTier)
			if tc.wantDiag != "" {
				require.NotEmpty(t, got.Diagnostics)
				assert.Contains(t, diagnosticsString(got.Diagnostics), tc.wantDiag)
			}
		})
	}
}

func TestExecuteRunsTransformAndReturnsOutput(t *testing.T) {
	engine := New()
	result := engine.Execute(context.Background(), ExecuteRequest{
		Code: `
def transform(input):
    return input + " Synthify"
`,
		Input: []byte("hello"),
	})

	require.Equal(t, treev1.ExecuteStatus_EXECUTE_STATUS_OK, result.Status)
	assert.Equal(t, []byte("hello Synthify"), result.Output)
	assert.Equal(t, treev1.RiskTier_RISK_TIER_1, result.FloorTier)
}

func TestExecuteSupportsJSONModuleAndOutputLimit(t *testing.T) {
	engine := New()
	result := engine.Execute(context.Background(), ExecuteRequest{
		Code: `
def transform(input):
    data = json.decode(input)
    return json.encode({"greeting": "hello " + data["name"]})
`,
		Input:          []byte(`{"name":"Ada"}`),
		MaxOutputBytes: 8,
	})

	require.Equal(t, treev1.ExecuteStatus_EXECUTE_STATUS_OUTPUT_TOO_LARGE, result.Status)
	assert.Equal(t, treev1.RiskTier_RISK_TIER_2, result.FloorTier)
	assert.Contains(t, result.Stderr, "exceeds limit")
}

func TestExecuteRejectsInvalidRuntimeShapes(t *testing.T) {
	engine := New()
	tests := []struct {
		name       string
		code       string
		wantStatus treev1.ExecuteStatus
		wantStderr string
	}{
		{
			name: "missing transform",
			code: `
def helper(input):
    return input
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT,
			wantStderr: "missing required transform",
		},
		{
			name: "non callable transform",
			code: `
transform = "not a function"
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT,
			wantStderr: "not callable",
		},
		{
			name: "unsupported return value",
			code: `
def transform(input):
    return {"value": input}
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_NONZERO_EXIT,
			wantStderr: "must return string or bytes",
		},
		{
			name: "empty output",
			code: `
def transform(input):
    return ""
`,
			wantStatus: treev1.ExecuteStatus_EXECUTE_STATUS_OUTPUT_MISSING,
			wantStderr: "empty output",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.Execute(context.Background(), ExecuteRequest{Code: tc.code, Input: []byte("x")})
			assert.Equal(t, tc.wantStatus, result.Status)
			assert.Contains(t, result.Stderr, tc.wantStderr)
		})
	}
}

func TestExecuteTimeoutCancelsRunawayTransform(t *testing.T) {
	engine := New()
	result := engine.Execute(context.Background(), ExecuteRequest{
		Code: `
def transform(input):
    i = 0
    while True:
        i = i + 1
    return str(i)
`,
		Input:   []byte("x"),
		Timeout: 10 * time.Millisecond,
	})

	require.Equal(t, treev1.ExecuteStatus_EXECUTE_STATUS_TIMEOUT, result.Status)
	assert.True(t, strings.Contains(result.Stderr, "timeout") || strings.Contains(result.Stderr, "too many steps"))
}
