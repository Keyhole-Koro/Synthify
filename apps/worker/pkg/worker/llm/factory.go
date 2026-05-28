package llm

import (
	"context"
	"log/slog"

	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	storage "github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// Init initializes the LLM clients (both ADK and custom Gemini client) based on the config.
// If the LLM config is disabled (missing API key), it returns nil for both clients.
func Init(ctx context.Context, cfg config.LLM, fs *storage.FileSystem, logger *slog.Logger) (model.LLM, *GeminiClient) {
	if !cfg.Enabled() {
		logger.Info("worker.gemini_disabled", "reason", "no api key")
		return nil, nil
	}

	clientCfg := buildClientConfig(cfg, logger)

	var adkModel model.LLM
	var adkErr error
	adkModel, adkErr = gemini.NewModel(ctx, cfg.GeminiModel, clientCfg)
	if adkErr != nil {
		logger.Error("worker.adk_model_init_failed", "error", adkErr.Error(), "model", cfg.GeminiModel)
	}

	embedder, err := NewGeminiClient(ctx, cfg, fs)
	if err != nil {
		logger.Error("worker.gemini_client_init_failed", "error", err.Error(), "model", cfg.GeminiModel)
	}

	return adkModel, embedder
}

// buildClientConfig selects the Vertex AI backend in production (uses GCP SA
// auth via GCP_PROJECT + VERTEX_LOCATION) and falls back to the Gemini API key
// flow for local development. Vertex is preferred when configured because the
// Gemini API free tier (20 req/day per model) is too restrictive even for low
// production traffic.
func buildClientConfig(cfg config.LLM, logger *slog.Logger) *genai.ClientConfig {
	if cfg.UseVertex() {
		logger.Info("worker.llm_backend", "backend", "vertex_ai",
			"project", cfg.GCPProject, "location", cfg.VertexLocation, "model", cfg.GeminiModel)
		return &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  cfg.GCPProject,
			Location: cfg.VertexLocation,
		}
	}
	logger.Info("worker.llm_backend", "backend", "gemini_api", "model", cfg.GeminiModel)
	return &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	}
}
