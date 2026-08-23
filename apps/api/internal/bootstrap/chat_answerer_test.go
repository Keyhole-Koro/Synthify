package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/synthify/backend/apps/api/internal/config"
	apichat "github.com/synthify/backend/apps/api/internal/infrastructure/chat"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// 本番で model 設定が無い場合は起動を失敗させる。抽出回答に黙って
// 降格させると、生成された回答だとユーザーが誤解する。
func TestNewChatAnswerer_ProductionWithoutModel_FailsStartup(t *testing.T) {
	for _, env := range []string{"production", "staging"} {
		t.Run(env, func(t *testing.T) {
			_, err := newChatAnswerer(context.Background(), config.API{Env: env}, quietLogger())
			if err == nil {
				t.Fatalf("expected startup failure in %s with no model configured", env)
			}
		})
	}
}

func TestNewChatAnswerer_NonProductionWithoutModel_UsesExtractive(t *testing.T) {
	for _, env := range []string{"local", "dev", "test"} {
		t.Run(env, func(t *testing.T) {
			answerer, err := newChatAnswerer(context.Background(), config.API{Env: env}, quietLogger())
			if err != nil {
				t.Fatalf("expected extractive fallback in %s, got error: %v", env, err)
			}
			if _, ok := answerer.(*apichat.ExtractiveAnswerer); !ok {
				t.Fatalf("expected *ExtractiveAnswerer in %s, got %T", env, answerer)
			}
		})
	}
}

// key があれば本番でも Gemini クライアントを選ぶ (この時点では API 呼び出しはしない)。
func TestNewChatAnswerer_WithAPIKey_UsesGemini(t *testing.T) {
	cfg := config.API{
		Env: "production",
		Chat: config.Chat{
			GeminiAPIKey: "test-key-not-used-for-a-real-call",
			GeminiModel:  "gemini-3-flash-preview",
		},
	}

	answerer, err := newChatAnswerer(context.Background(), cfg, quietLogger())
	if err != nil {
		t.Fatalf("expected gemini answerer, got error: %v", err)
	}
	if _, ok := answerer.(*apichat.GeminiAnswerer); !ok {
		t.Fatalf("expected *GeminiAnswerer, got %T", answerer)
	}
}
