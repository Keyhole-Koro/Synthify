package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	localproviderv1 "github.com/synthify/backend/internal/gen/synthify/localprovider/v1"
	localproviderv1connect "github.com/synthify/backend/internal/gen/synthify/localprovider/v1/localproviderv1connect"
)

const localProviderTestToken = "0123456789abcdef0123456789abcdef"

type fakeLocalProvider struct {
	localproviderv1connect.UnimplementedLocalProviderServiceHandler

	token              string
	mu                 sync.Mutex
	allAuthenticated   bool
	generateTextCalls  int
	generateText       func(context.Context, *connect.Request[localproviderv1.GenerateTextRequest]) (*connect.Response[localproviderv1.GenerateTextResponse], error)
	generateStructured func(context.Context, *connect.Request[localproviderv1.GenerateStructuredRequest]) (*connect.Response[localproviderv1.GenerateStructuredResponse], error)
	cancelGeneration   func(context.Context, *connect.Request[localproviderv1.CancelGenerationRequest]) (*connect.Response[localproviderv1.CancelGenerationResponse], error)
}

func (f *fakeLocalProvider) authenticated(header string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if header != "Bearer "+f.token {
		f.allAuthenticated = false
	}
}

func (f *fakeLocalProvider) isAuthenticated() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.allAuthenticated
}

func (f *fakeLocalProvider) Check(_ context.Context, req *connect.Request[localproviderv1.CheckRequest]) (*connect.Response[localproviderv1.CheckResponse], error) {
	f.authenticated(req.Header().Get("Authorization"))
	return connect.NewResponse(&localproviderv1.CheckResponse{
		Status: localproviderv1.CheckResponse_STATUS_READY,
	}), nil
}

func (f *fakeLocalProvider) GetCapabilities(_ context.Context, req *connect.Request[localproviderv1.GetCapabilitiesRequest]) (*connect.Response[localproviderv1.GetCapabilitiesResponse], error) {
	f.authenticated(req.Header().Get("Authorization"))
	return connect.NewResponse(&localproviderv1.GetCapabilitiesResponse{
		ServerVersion:  "0.1.0",
		DefaultModelId: "antigravity:test-model",
		Models: []*localproviderv1.ModelCapability{{
			Id:                 "antigravity:test-model",
			SupportsStructured: true,
		}},
	}), nil
}

func (f *fakeLocalProvider) GenerateText(ctx context.Context, req *connect.Request[localproviderv1.GenerateTextRequest]) (*connect.Response[localproviderv1.GenerateTextResponse], error) {
	f.authenticated(req.Header().Get("Authorization"))
	f.mu.Lock()
	f.generateTextCalls++
	f.mu.Unlock()
	if f.generateText != nil {
		return f.generateText(ctx, req)
	}
	return connect.NewResponse(&localproviderv1.GenerateTextResponse{
		Text: "generated text",
		Usage: &localproviderv1.Usage{
			Model:        "antigravity:test-model",
			InputTokens:  10,
			OutputTokens: 4,
		},
	}), nil
}

func (f *fakeLocalProvider) GenerateStructured(ctx context.Context, req *connect.Request[localproviderv1.GenerateStructuredRequest]) (*connect.Response[localproviderv1.GenerateStructuredResponse], error) {
	f.authenticated(req.Header().Get("Authorization"))
	if f.generateStructured != nil {
		return f.generateStructured(ctx, req)
	}
	return connect.NewResponse(&localproviderv1.GenerateStructuredResponse{
		JsonPayload: []byte(`{"answer":"ok"}`),
		Usage: &localproviderv1.Usage{
			Model:        "antigravity:test-model",
			InputTokens:  12,
			OutputTokens: 3,
		},
	}), nil
}

func (f *fakeLocalProvider) CancelGeneration(ctx context.Context, req *connect.Request[localproviderv1.CancelGenerationRequest]) (*connect.Response[localproviderv1.CancelGenerationResponse], error) {
	f.authenticated(req.Header().Get("Authorization"))
	if f.cancelGeneration != nil {
		return f.cancelGeneration(ctx, req)
	}
	return connect.NewResponse(&localproviderv1.CancelGenerationResponse{Found: true}), nil
}

func newLocalProviderTestClient(t *testing.T, fake *fakeLocalProvider) (*LocalProviderClient, func()) {
	t.Helper()
	fake.token = localProviderTestToken
	fake.allAuthenticated = true
	_, handler := localproviderv1connect.NewLocalProviderServiceHandler(fake)
	server := httptest.NewServer(handler)

	tokenFile := t.TempDir() + "/provider-token"
	require.NoError(t, os.WriteFile(tokenFile, []byte(localProviderTestToken+"\n"), 0o600))
	cfg := config.LLM{
		Provider:                    config.LLMProviderAntigravity,
		RuntimeEnv:                  "test",
		DeploymentMode:              "self-hosted",
		LocalProviderEndpoint:       server.URL,
		LocalProviderTokenFile:      tokenFile,
		LocalProviderRequestTimeout: 2 * time.Second,
		LocalProviderCancelTimeout:  time.Second,
		LocalProviderHealthTimeout:  time.Second,
	}
	client, err := NewLocalProviderClient(context.Background(), cfg, server.Client())
	require.NoError(t, err)
	return client, server.Close
}

func TestLocalProviderClientGenerateText(t *testing.T) {
	var captured *localproviderv1.GenerateTextRequest
	fake := &fakeLocalProvider{
		generateText: func(_ context.Context, req *connect.Request[localproviderv1.GenerateTextRequest]) (*connect.Response[localproviderv1.GenerateTextResponse], error) {
			captured = req.Msg
			return connect.NewResponse(&localproviderv1.GenerateTextResponse{
				Text: "hello",
				Usage: &localproviderv1.Usage{
					Model:        "antigravity:test-model",
					InputTokens:  8,
					OutputTokens: 2,
				},
			}), nil
		},
	}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer closeServer()
	client.newGenerationID = func() string { return "generation-1" }

	text, usage, err := client.GenerateText(context.Background(), TextRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
	})
	require.NoError(t, err)
	require.Equal(t, "hello", text)
	require.Equal(t, Usage{Model: "antigravity:test-model", InputTokens: 8, OutputTokens: 2}, usage)
	require.Equal(t, "generation-1", captured.GetGenerationId())
	require.Equal(t, "antigravity:test-model", captured.GetModelId())
	require.Equal(t, "system", captured.GetSystemPrompt())
	require.Equal(t, "user", captured.GetUserPrompt())
	require.True(t, fake.isAuthenticated())
}

func TestLocalProviderClientGenerateStructuredValidatesResponse(t *testing.T) {
	type output struct {
		Answer string `json:"answer"`
	}

	var sentSchema map[string]any
	fake := &fakeLocalProvider{
		generateStructured: func(_ context.Context, req *connect.Request[localproviderv1.GenerateStructuredRequest]) (*connect.Response[localproviderv1.GenerateStructuredResponse], error) {
			require.NoError(t, json.Unmarshal(req.Msg.GetJsonSchema(), &sentSchema))
			return connect.NewResponse(&localproviderv1.GenerateStructuredResponse{
				JsonPayload: []byte(`{"wrong":"field"}`),
				Usage: &localproviderv1.Usage{
					Model:        "antigravity:test-model",
					InputTokens:  7,
					OutputTokens: 1,
				},
			}), nil
		},
	}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer closeServer()

	_, usage, err := client.GenerateStructured(context.Background(), StructuredRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		Schema:       output{},
	})
	require.ErrorContains(t, err, "JSON schema validation")
	require.Equal(t, "antigravity:test-model", usage.Model)
	require.Equal(t, "object", sentSchema["type"])
	require.True(t, fake.isAuthenticated())
}

func TestLocalProviderClientGenerateStructured(t *testing.T) {
	type output struct {
		Answer string `json:"answer"`
	}

	fake := &fakeLocalProvider{}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer closeServer()

	payload, usage, err := client.GenerateStructured(context.Background(), StructuredRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		Schema:       output{},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"answer":"ok"}`, string(payload))
	require.Equal(t, Usage{Model: "antigravity:test-model", InputTokens: 12, OutputTokens: 3}, usage)
	require.True(t, fake.isAuthenticated())
}

func TestLocalProviderClientCancellationCallsExplicitRPC(t *testing.T) {
	started := make(chan string, 1)
	cancelled := make(chan string, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	fake := &fakeLocalProvider{
		generateText: func(_ context.Context, req *connect.Request[localproviderv1.GenerateTextRequest]) (*connect.Response[localproviderv1.GenerateTextResponse], error) {
			started <- req.Msg.GetGenerationId()
			<-release
			return nil, connect.NewError(connect.CodeCanceled, errors.New("cancelled"))
		},
		cancelGeneration: func(_ context.Context, req *connect.Request[localproviderv1.CancelGenerationRequest]) (*connect.Response[localproviderv1.CancelGenerationResponse], error) {
			cancelled <- req.Msg.GetGenerationId()
			releaseOnce.Do(func() { close(release) })
			return connect.NewResponse(&localproviderv1.CancelGenerationResponse{Found: true}), nil
		},
	}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer func() {
		releaseOnce.Do(func() { close(release) })
		closeServer()
	}()
	client.newGenerationID = func() string { return "generation-cancel" }

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, err := client.GenerateText(ctx, TextRequest{UserPrompt: "user"})
		result <- err
	}()
	require.Equal(t, "generation-cancel", <-started)
	cancel()
	require.ErrorIs(t, <-result, context.Canceled)
	require.Equal(t, "generation-cancel", <-cancelled)
	require.True(t, fake.isAuthenticated())
}

func TestLocalProviderClientTimeoutCallsExplicitRPC(t *testing.T) {
	started := make(chan string, 1)
	cancelled := make(chan string, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	fake := &fakeLocalProvider{
		generateText: func(_ context.Context, req *connect.Request[localproviderv1.GenerateTextRequest]) (*connect.Response[localproviderv1.GenerateTextResponse], error) {
			started <- req.Msg.GetGenerationId()
			<-release
			return nil, connect.NewError(connect.CodeCanceled, errors.New("cancelled"))
		},
		cancelGeneration: func(_ context.Context, req *connect.Request[localproviderv1.CancelGenerationRequest]) (*connect.Response[localproviderv1.CancelGenerationResponse], error) {
			cancelled <- req.Msg.GetGenerationId()
			releaseOnce.Do(func() { close(release) })
			return connect.NewResponse(&localproviderv1.CancelGenerationResponse{Found: true}), nil
		},
	}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer func() {
		releaseOnce.Do(func() { close(release) })
		closeServer()
	}()
	client.newGenerationID = func() string { return "generation-timeout" }
	client.requestTimeout = 25 * time.Millisecond

	result := make(chan error, 1)
	go func() {
		_, _, err := client.GenerateText(context.Background(), TextRequest{UserPrompt: "user"})
		result <- err
	}()
	require.Equal(t, "generation-timeout", <-started)
	require.ErrorIs(t, <-result, context.DeadlineExceeded)
	require.Equal(t, "generation-timeout", <-cancelled)
	require.True(t, fake.isAuthenticated())
}

func TestLocalProviderClientDoesNotRetryGeneration(t *testing.T) {
	fake := &fakeLocalProvider{
		generateText: func(_ context.Context, _ *connect.Request[localproviderv1.GenerateTextRequest]) (*connect.Response[localproviderv1.GenerateTextResponse], error) {
			providerErr := connect.NewError(connect.CodeResourceExhausted, errors.New("provider detail must not escape"))
			detail, err := connect.NewErrorDetail(&localproviderv1.LocalProviderErrorDetail{
				Reason:       localproviderv1.LocalProviderErrorDetail_REASON_RATE_LIMITED,
				TurnStarted:  false,
				RetryAfterMs: 500,
			})
			require.NoError(t, err)
			providerErr.AddDetail(detail)
			return nil, providerErr
		},
	}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer closeServer()

	_, _, err := client.GenerateText(context.Background(), TextRequest{UserPrompt: "user"})
	var providerErr *LocalProviderError
	require.ErrorAs(t, err, &providerErr)
	require.True(t, providerErr.RetryablePreTurnRateLimit())
	require.Equal(t, 500*time.Millisecond, providerErr.RetryAfter)
	require.NotContains(t, err.Error(), "provider detail must not escape")
	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Equal(t, 1, fake.generateTextCalls)
}

func TestLocalProviderClientRejectsSourceFilesBeforeRPC(t *testing.T) {
	fake := &fakeLocalProvider{}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer closeServer()

	_, _, err := client.GenerateText(context.Background(), TextRequest{
		SourceFiles: []domain.SourceFile{{Filename: "source.pdf"}},
	})
	require.ErrorIs(t, err, ErrLocalProviderSourceFilesUnsupported)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Zero(t, fake.generateTextCalls)
}

func TestLocalProviderClientValidatesTextBeforeRPC(t *testing.T) {
	fake := &fakeLocalProvider{}
	client, closeServer := newLocalProviderTestClient(t, fake)
	defer closeServer()

	_, _, err := client.GenerateText(context.Background(), TextRequest{
		SystemPrompt: strings.Repeat("x", 1_048_577),
	})
	require.ErrorContains(t, err, "invalid local provider text request")
	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Zero(t, fake.generateTextCalls)
}

func TestReadLocalProviderTokenRequiresOwnerOnlyFile(t *testing.T) {
	path := t.TempDir() + "/provider-token"
	require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("a", 32)), 0o644))
	_, err := readLocalProviderToken(path)
	require.ErrorContains(t, err, "owner-only")
	require.NotContains(t, err.Error(), path)
	require.NotContains(t, err.Error(), strings.Repeat("a", 32))
	require.NoError(t, os.Chmod(path, 0o600))
	token, err := readLocalProviderToken(path)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 32), token)
}

func TestValidateLocalProviderCapabilitiesFailsClosed(t *testing.T) {
	valid := func() *localproviderv1.GetCapabilitiesResponse {
		return &localproviderv1.GetCapabilitiesResponse{
			ServerVersion:  "0.1.0",
			DefaultModelId: "antigravity:test-model",
			Models: []*localproviderv1.ModelCapability{{
				Id:                 "antigravity:test-model",
				SupportsStructured: true,
			}},
		}
	}

	tests := []struct {
		name   string
		mutate func(*localproviderv1.GetCapabilitiesResponse)
	}{
		{name: "default missing", mutate: func(response *localproviderv1.GetCapabilitiesResponse) {
			response.DefaultModelId = "antigravity:other"
		}},
		{name: "default lacks structured support", mutate: func(response *localproviderv1.GetCapabilitiesResponse) {
			response.Models[0].SupportsStructured = false
		}},
		{name: "duplicate model", mutate: func(response *localproviderv1.GetCapabilitiesResponse) {
			response.Models = append(response.Models, response.Models[0])
		}},
		{name: "wrong provider prefix", mutate: func(response *localproviderv1.GetCapabilitiesResponse) {
			response.DefaultModelId = "codex:test-model"
			response.Models[0].Id = "codex:test-model"
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := valid()
			test.mutate(response)
			_, err := validateLocalProviderCapabilities(config.LLMProviderAntigravity, response)
			require.Error(t, err)
		})
	}
}

func TestInitProcessClientGeminiKeepsExistingClient(t *testing.T) {
	gemini := &GeminiClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := InitProcessClient(context.Background(), config.LLM{Provider: config.LLMProviderGemini}, gemini, nil, logger)
	require.NoError(t, err)
	require.Same(t, gemini, client)
}

func TestInitProcessClientLocalFailsClosedInProduction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := InitProcessClient(context.Background(), config.LLM{
		Provider:                    config.LLMProviderAntigravity,
		RuntimeEnv:                  "production",
		DeploymentMode:              "self-hosted",
		LocalProviderEndpoint:       "http://127.0.0.1:8787",
		LocalProviderTokenFile:      "/not-read",
		LocalProviderRequestTimeout: time.Minute,
		LocalProviderCancelTimeout:  time.Second,
		LocalProviderHealthTimeout:  time.Second,
	}, nil, nil, logger)
	require.ErrorContains(t, err, "disabled")
	require.Nil(t, client)
}
