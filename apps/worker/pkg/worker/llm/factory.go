package llm

import (
	"context"
	"log/slog"

	connect "connectrpc.com/connect"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	storage "github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// Init initializes the LLM clients (both ADK and custom Gemini client) based on the config.
// If the LLM config is disabled (missing GCP project), it returns nil for both clients.
func Init(ctx context.Context, cfg config.LLM, fs *storage.FileSystem, logger *slog.Logger, nrApp *newrelic.Application) (model.LLM, *GeminiClient) {
	if !cfg.Enabled() {
		logger.Info("worker.gemini_disabled", "reason", "no gcp project")
		return nil, nil
	}

	clientCfg := buildClientConfig(cfg, logger)

	var adkModel model.LLM
	var adkErr error
	adkModel, adkErr = gemini.NewModel(ctx, cfg.GeminiModel, clientCfg)
	if adkErr != nil {
		logger.Error("worker.adk_model_init_failed", "error", adkErr.Error(), "model", cfg.GeminiModel)
	}
	if adkModel != nil {
		q := QuotaFor(cfg.GeminiModel)
		logger.Info("worker.llm_quota_reference",
			"model", q.Model, "tier", q.Tier, "rpm", q.RPM)
		adkModel = NewRetryingModel(adkModel, RetryConfig{}, logger, nrApp)
	}

	embedder, err := NewGeminiClient(ctx, cfg, fs)
	if err != nil {
		logger.Error("worker.gemini_client_init_failed", "error", err.Error(), "model", cfg.GeminiModel)
	}

	return adkModel, embedder
}

// InitProcessClient selects the direct process-tool generation client. ADK and
// embeddings remain on the Gemini clients initialized by Init.
func InitProcessClient(ctx context.Context, cfg config.LLM, geminiClient *GeminiClient, httpClient connect.HTTPClient, logger *slog.Logger, opts ...connect.ClientOption) (Client, error) {
	if err := cfg.ValidateProcessProvider(); err != nil {
		return nil, err
	}
	if !cfg.UsesLocalProvider() {
		return geminiClient, nil
	}
	client, err := NewLocalProviderClient(ctx, cfg, httpClient, opts...)
	if err != nil {
		return nil, err
	}
	logger.Info("worker.process_llm_provider_initialized",
		"provider", cfg.Provider,
		"model", client.Capabilities().DefaultModelID,
	)
	return client, nil
}

// buildClientConfig selects the Vertex AI backend. Project and location may be
// configured explicitly for local runs; production detects them from Cloud Run
// metadata when env vars are omitted.
func buildClientConfig(cfg config.LLM, logger *slog.Logger) *genai.ClientConfig {
	if cfg.APIKey != "" {
		logger.Info("worker.llm_backend", "backend", "gemini_api", "model", cfg.GeminiModel, "api_key_configured", true)
		return clientConfig(cfg)
	}
	logger.Info("worker.llm_backend", "backend", "vertex_ai",
		"project", cfg.GCPProject, "location", cfg.VertexLocation, "model", cfg.GeminiModel)
	return clientConfig(cfg)
}

func clientConfig(cfg config.LLM) *genai.ClientConfig {
	if cfg.APIKey != "" {
		return &genai.ClientConfig{
			Backend: genai.BackendGeminiAPI,
			APIKey:  cfg.APIKey,
		}
	}
	return &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  cfg.GCPProject,
		Location: cfg.VertexLocation,
	}
}
