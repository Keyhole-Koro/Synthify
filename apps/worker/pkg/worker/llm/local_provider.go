package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"buf.build/go/protovalidate"
	connect "connectrpc.com/connect"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/oklog/ulid/v2"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	localproviderv1 "github.com/synthify/backend/internal/gen/synthify/localprovider/v1"
	localproviderv1connect "github.com/synthify/backend/internal/gen/synthify/localprovider/v1/localproviderv1connect"
)

const (
	localProviderMaxResponseBytes = 18 << 20
	localProviderMaxTokenBytes    = 4096
)

var ErrLocalProviderSourceFilesUnsupported = errors.New("local provider source files are not supported until a shared job directory is configured")

type LocalProviderModel struct {
	ID                 string
	SupportsStructured bool
}

type LocalProviderCapabilities struct {
	ServerVersion  string
	DefaultModelID string
	Models         []LocalProviderModel
}

type LocalProviderError struct {
	Code        connect.Code
	Reason      localproviderv1.LocalProviderErrorDetail_Reason
	TurnStarted bool
	RetryAfter  time.Duration
}

func (e *LocalProviderError) Error() string {
	if e.Reason != localproviderv1.LocalProviderErrorDetail_REASON_UNSPECIFIED {
		return fmt.Sprintf("local provider request failed: code=%s reason=%s", e.Code, e.Reason)
	}
	return fmt.Sprintf("local provider request failed: code=%s", e.Code)
}

func (e *LocalProviderError) RetryablePreTurnRateLimit() bool {
	return e.Code == connect.CodeResourceExhausted &&
		e.Reason == localproviderv1.LocalProviderErrorDetail_REASON_RATE_LIMITED &&
		!e.TurnStarted
}

type LocalProviderClient struct {
	rpc             localproviderv1connect.LocalProviderServiceClient
	provider        config.LLMProvider
	modelID         string
	capabilities    LocalProviderCapabilities
	requestTimeout  time.Duration
	cancelTimeout   time.Duration
	newGenerationID func() string
}

func NewLocalProviderClient(ctx context.Context, cfg config.LLM, httpClient connect.HTTPClient, opts ...connect.ClientOption) (*LocalProviderClient, error) {
	if err := cfg.ValidateProcessProvider(); err != nil {
		return nil, err
	}
	if !cfg.UsesLocalProvider() {
		return nil, fmt.Errorf("LLM_PROVIDER %q is not a local provider", cfg.Provider)
	}

	token, err := readLocalProviderToken(cfg.LocalProviderTokenFile)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: http.DefaultTransport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	clientOptions := []connect.ClientOption{
		connect.WithInterceptors(localProviderBearerToken(token)),
		connect.WithReadMaxBytes(localProviderMaxResponseBytes),
	}
	clientOptions = append(clientOptions, opts...)
	rpc := localproviderv1connect.NewLocalProviderServiceClient(
		httpClient,
		strings.TrimRight(cfg.LocalProviderEndpoint, "/"),
		clientOptions...,
	)

	client := &LocalProviderClient{
		rpc:             rpc,
		provider:        cfg.Provider,
		requestTimeout:  cfg.LocalProviderRequestTimeout,
		cancelTimeout:   cfg.LocalProviderCancelTimeout,
		newGenerationID: func() string { return ulid.Make().String() },
	}
	if err := client.initialize(ctx, cfg.LocalProviderHealthTimeout); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *LocalProviderClient) Capabilities() LocalProviderCapabilities {
	capabilities := c.capabilities
	capabilities.Models = append([]LocalProviderModel(nil), c.capabilities.Models...)
	return capabilities
}

func (c *LocalProviderClient) GenerateText(ctx context.Context, req TextRequest) (string, Usage, error) {
	if len(req.SourceFiles) > 0 {
		return "", Usage{}, ErrLocalProviderSourceFilesUnsupported
	}

	generationID := c.newGenerationID()
	message := &localproviderv1.GenerateTextRequest{
		GenerationId: generationID,
		ModelId:      c.modelID,
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
	}
	if err := protovalidate.Validate(message); err != nil {
		return "", Usage{}, fmt.Errorf("invalid local provider text request: %w", err)
	}
	callCtx, finish := c.watchGeneration(ctx, generationID)
	response, err := c.rpc.GenerateText(callCtx, connect.NewRequest(message))
	callErr := callCtx.Err()
	cancelSent := finish()
	if err != nil {
		if !cancelSent {
			c.cancelGeneration(generationID)
		}
		if callErr != nil {
			return "", Usage{}, callErr
		}
		return "", Usage{}, normalizeLocalProviderError(err)
	}
	if err := protovalidate.Validate(response.Msg); err != nil {
		return "", Usage{}, fmt.Errorf("local provider returned an invalid text response: %w", err)
	}
	return response.Msg.GetText(), localProviderUsage(response.Msg.GetUsage()), nil
}

func (c *LocalProviderClient) GenerateStructured(ctx context.Context, req StructuredRequest) (json.RawMessage, Usage, error) {
	if len(req.SourceFiles) > 0 {
		return nil, Usage{}, ErrLocalProviderSourceFilesUnsupported
	}

	schemaBytes, schema, err := localProviderJSONSchema(req.Schema)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("prepare local provider JSON schema: %w", err)
	}
	generationID := c.newGenerationID()
	message := &localproviderv1.GenerateStructuredRequest{
		GenerationId: generationID,
		ModelId:      c.modelID,
		SystemPrompt: req.SystemPrompt,
		UserPrompt:   req.UserPrompt,
		JsonSchema:   schemaBytes,
	}
	if err := protovalidate.Validate(message); err != nil {
		return nil, Usage{}, fmt.Errorf("invalid local provider structured request: %w", err)
	}

	callCtx, finish := c.watchGeneration(ctx, generationID)
	response, err := c.rpc.GenerateStructured(callCtx, connect.NewRequest(message))
	callErr := callCtx.Err()
	cancelSent := finish()
	if err != nil {
		if !cancelSent {
			c.cancelGeneration(generationID)
		}
		if callErr != nil {
			return nil, Usage{}, callErr
		}
		return nil, Usage{}, normalizeLocalProviderError(err)
	}
	if err := protovalidate.Validate(response.Msg); err != nil {
		return nil, Usage{}, fmt.Errorf("local provider returned an invalid structured response: %w", err)
	}

	payload := json.RawMessage(append([]byte(nil), response.Msg.GetJsonPayload()...))
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, localProviderUsage(response.Msg.GetUsage()), fmt.Errorf("local provider returned invalid JSON: %w", err)
	}
	if err := schema.Validate(value); err != nil {
		return nil, localProviderUsage(response.Msg.GetUsage()), fmt.Errorf("local provider response failed JSON schema validation: %w", err)
	}
	return payload, localProviderUsage(response.Msg.GetUsage()), nil
}

func (c *LocalProviderClient) initialize(ctx context.Context, timeout time.Duration) error {
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	check, err := c.rpc.Check(healthCtx, connect.NewRequest(&localproviderv1.CheckRequest{}))
	if err != nil {
		return fmt.Errorf("local provider readiness check failed: %w", normalizeLocalProviderError(err))
	}
	if err := protovalidate.Validate(check.Msg); err != nil {
		return fmt.Errorf("local provider readiness response is invalid: %w", err)
	}
	if check.Msg.GetStatus() != localproviderv1.CheckResponse_STATUS_READY {
		return fmt.Errorf("local provider is not ready")
	}

	capabilities, err := c.rpc.GetCapabilities(healthCtx, connect.NewRequest(&localproviderv1.GetCapabilitiesRequest{}))
	if err != nil {
		return fmt.Errorf("local provider capabilities failed: %w", normalizeLocalProviderError(err))
	}
	if err := protovalidate.Validate(capabilities.Msg); err != nil {
		return fmt.Errorf("local provider capabilities response is invalid: %w", err)
	}
	parsed, err := validateLocalProviderCapabilities(c.provider, capabilities.Msg)
	if err != nil {
		return err
	}
	c.modelID = parsed.DefaultModelID
	c.capabilities = parsed
	return nil
}

func (c *LocalProviderClient) watchGeneration(parent context.Context, generationID string) (context.Context, func() bool) {
	callCtx, cancel := context.WithTimeout(parent, c.requestTimeout)
	stop := make(chan struct{})
	result := make(chan bool, 1)
	go func() {
		select {
		case <-callCtx.Done():
			c.cancelGeneration(generationID)
			result <- true
		case <-stop:
			result <- false
		}
	}()
	return callCtx, func() bool {
		close(stop)
		cancelSent := <-result
		cancel()
		return cancelSent
	}
}

func (c *LocalProviderClient) cancelGeneration(generationID string) {
	ctx, cancel := context.WithTimeout(context.Background(), c.cancelTimeout)
	defer cancel()
	_, _ = c.rpc.CancelGeneration(ctx, connect.NewRequest(&localproviderv1.CancelGenerationRequest{
		GenerationId: generationID,
	}))
}

func localProviderBearerToken(token string) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
			request.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, request)
		}
	})
}

func readLocalProviderToken(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("read local provider token file: unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("read local provider token file: must be a regular non-symlink file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("read local provider token file: permissions must be owner-only")
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read local provider token file: unavailable")
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("read local provider token file: unavailable")
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return "", fmt.Errorf("read local provider token file: changed while being opened")
	}
	if openedInfo.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("read local provider token file: permissions must be owner-only")
	}
	contents, err := io.ReadAll(io.LimitReader(file, localProviderMaxTokenBytes+1))
	if err != nil {
		return "", fmt.Errorf("read local provider token file: unavailable")
	}
	token := strings.TrimSuffix(string(contents), "\n")
	token = strings.TrimSuffix(token, "\r")
	if len(token) < 32 || len(token) > localProviderMaxTokenBytes {
		return "", fmt.Errorf("read local provider token file: token must contain 32 to %d bytes", localProviderMaxTokenBytes)
	}
	for _, char := range token {
		if char <= ' ' || char > '~' {
			return "", fmt.Errorf("read local provider token file: token contains invalid characters")
		}
	}
	return token, nil
}

func validateLocalProviderCapabilities(provider config.LLMProvider, response *localproviderv1.GetCapabilitiesResponse) (LocalProviderCapabilities, error) {
	defaultID := response.GetDefaultModelId()
	if !strings.HasPrefix(defaultID, string(provider)+":") {
		return LocalProviderCapabilities{}, fmt.Errorf("local provider default model does not match configured provider")
	}
	parsed := LocalProviderCapabilities{
		ServerVersion:  response.GetServerVersion(),
		DefaultModelID: defaultID,
		Models:         make([]LocalProviderModel, 0, len(response.GetModels())),
	}
	seen := make(map[string]struct{}, len(response.GetModels()))
	defaultSupportsStructured := false
	for _, model := range response.GetModels() {
		if _, duplicate := seen[model.GetId()]; duplicate {
			return LocalProviderCapabilities{}, fmt.Errorf("local provider capabilities contain a duplicate model")
		}
		seen[model.GetId()] = struct{}{}
		if !strings.HasPrefix(model.GetId(), string(provider)+":") {
			return LocalProviderCapabilities{}, fmt.Errorf("local provider model does not match configured provider")
		}
		parsed.Models = append(parsed.Models, LocalProviderModel{
			ID:                 model.GetId(),
			SupportsStructured: model.GetSupportsStructured(),
		})
		if model.GetId() == defaultID {
			defaultSupportsStructured = model.GetSupportsStructured()
		}
	}
	if _, ok := seen[defaultID]; !ok {
		return LocalProviderCapabilities{}, fmt.Errorf("local provider default model is not advertised")
	}
	if !defaultSupportsStructured {
		return LocalProviderCapabilities{}, fmt.Errorf("local provider default model does not support structured generation")
	}
	return parsed, nil
}

func localProviderJSONSchema(value any) ([]byte, *jsonschema.Resolved, error) {
	if value == nil {
		return nil, nil, fmt.Errorf("schema is required")
	}

	var schema *jsonschema.Schema
	switch typed := value.(type) {
	case json.RawMessage:
		schema = &jsonschema.Schema{}
		if err := json.Unmarshal(typed, schema); err != nil {
			return nil, nil, err
		}
	case []byte:
		schema = &jsonschema.Schema{}
		if err := json.Unmarshal(typed, schema); err != nil {
			return nil, nil, err
		}
	case string:
		schema = &jsonschema.Schema{}
		if err := json.Unmarshal([]byte(typed), schema); err != nil {
			return nil, nil, err
		}
	case *jsonschema.Schema:
		if typed == nil {
			return nil, nil, fmt.Errorf("schema is required")
		}
		schema = typed
	default:
		var err error
		schema, err = jsonschema.ForType(reflect.TypeOf(value), nil)
		if err != nil {
			return nil, nil, err
		}
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, nil, err
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, nil, err
	}
	return encoded, resolved, nil
}

func localProviderUsage(usage *localproviderv1.Usage) Usage {
	if usage == nil {
		return Usage{}
	}
	return Usage{
		Model:        usage.GetModel(),
		InputTokens:  usage.GetInputTokens(),
		OutputTokens: usage.GetOutputTokens(),
	}
}

func normalizeLocalProviderError(err error) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return &LocalProviderError{Code: connect.CodeUnknown}
	}
	providerErr := &LocalProviderError{Code: connectErr.Code()}
	for _, detail := range connectErr.Details() {
		value, valueErr := detail.Value()
		if valueErr != nil {
			continue
		}
		typed, ok := value.(*localproviderv1.LocalProviderErrorDetail)
		if !ok || protovalidate.Validate(typed) != nil {
			continue
		}
		providerErr.Reason = typed.GetReason()
		providerErr.TurnStarted = typed.GetTurnStarted()
		providerErr.RetryAfter = time.Duration(typed.GetRetryAfterMs()) * time.Millisecond
		break
	}
	return providerErr
}

var _ Client = (*LocalProviderClient)(nil)
