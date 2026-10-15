package localprovider_test

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/require"
	localproviderv1 "github.com/synthify/backend/internal/gen/synthify/localprovider/v1"
)

func TestGenerateStructuredRequestValidation(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	validRequest := func() *localproviderv1.GenerateStructuredRequest {
		return &localproviderv1.GenerateStructuredRequest{
			GenerationId: "01K3ABCDEF0123456789ABCDEF",
			ModelId:      "antigravity:claude-sonnet",
			SystemPrompt: "Return structured content.",
			JsonSchema:   []byte("{}"),
		}
	}

	tests := []struct {
		name   string
		mutate func(*localproviderv1.GenerateStructuredRequest)
		valid  bool
	}{
		{name: "valid", mutate: func(*localproviderv1.GenerateStructuredRequest) {}, valid: true},
		{name: "missing generation id", mutate: func(request *localproviderv1.GenerateStructuredRequest) {
			request.GenerationId = ""
		}},
		{name: "model id with whitespace", mutate: func(request *localproviderv1.GenerateStructuredRequest) {
			request.ModelId = "antigravity: claude"
		}},
		{name: "oversized prompt", mutate: func(request *localproviderv1.GenerateStructuredRequest) {
			request.SystemPrompt = strings.Repeat("x", 1_048_577)
		}},
		{name: "missing schema", mutate: func(request *localproviderv1.GenerateStructuredRequest) {
			request.JsonSchema = nil
		}},
		{name: "too many source files", mutate: func(request *localproviderv1.GenerateStructuredRequest) {
			request.SourceFiles = make([]*localproviderv1.SourceFile, 33)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := validRequest()
			test.mutate(request)
			err := validator.Validate(request)
			if test.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestLocalProviderErrorDetailValidation(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	require.NoError(t, err)

	require.NoError(t, validator.Validate(&localproviderv1.LocalProviderErrorDetail{
		Reason:       localproviderv1.LocalProviderErrorDetail_REASON_RATE_LIMITED,
		RetryAfterMs: 60_000,
	}))
	require.Error(t, validator.Validate(&localproviderv1.LocalProviderErrorDetail{
		Reason:       localproviderv1.LocalProviderErrorDetail_REASON_RATE_LIMITED,
		RetryAfterMs: 60_001,
	}))
	require.Error(t, validator.Validate(&localproviderv1.LocalProviderErrorDetail{}))
}
