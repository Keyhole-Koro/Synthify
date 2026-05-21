package llm

import (
	"context"

	"github.com/synthify/backend/internal/platform/applog"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	storage "github.com/synthify/backend/apps/worker/pkg/worker/storage"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// Init initializes the LLM clients (both ADK and custom Gemini client) based on the config.
// If the LLM config is disabled (missing API key), it returns nil for both clients.
func Init(ctx context.Context, cfg config.LLM, fs *storage.FileSystem, logger applog.Logger) (model.LLM, *GeminiClient) {
	if !cfg.Enabled() {
		logger.Info(ctx, "worker.gemini_disabled", map[string]any{"reason": "no api key"})
		return nil, nil
	}

	var adkModel model.LLM
	var adkErr error
	adkModel, adkErr = gemini.NewModel(ctx, cfg.GeminiModel, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if adkErr != nil {
		logger.Error(ctx, "worker.adk_model_init_failed", adkErr, map[string]any{"model": cfg.GeminiModel})
	}

	embedder, err := NewGeminiClient(ctx, cfg, fs)
	if err != nil {
		logger.Error(ctx, "worker.gemini_client_init_failed", err, map[string]any{"model": cfg.GeminiModel})
	}

	return adkModel, embedder
}
