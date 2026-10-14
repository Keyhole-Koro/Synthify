package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func TestExtractiveAnswererWithoutCandidates(t *testing.T) {
	answer, err := NewExtractiveAnswerer().Answer(context.Background(), application.ChatAnswerRequest{Question: "質問"})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if !strings.Contains(answer.Text, ExtractiveAnswerNotice) || !strings.Contains(answer.Text, "質問: 質問") {
		t.Fatalf("answer = %q", answer.Text)
	}
	if len(answer.SourceIDs) != 0 {
		t.Fatalf("source ids = %v, want none", answer.SourceIDs)
	}
	if got := NewExtractiveAnswerer().ModelID(); got != "extractive-dev" {
		t.Fatalf("ModelID() = %q", got)
	}
}

func TestExtractiveAnswererLimitsCandidatesAndTruncatesText(t *testing.T) {
	candidates := make([]domain.ChatSourceCandidate, 4)
	for i := range candidates {
		candidates[i] = domain.ChatSourceCandidate{
			ChunkID:  "chunk_" + string(rune('1'+i)),
			Filename: "notes.md",
			Heading:  "Section",
			Text:     strings.Repeat("あ", 250),
		}
	}
	answer, err := NewExtractiveAnswerer().Answer(context.Background(), application.ChatAnswerRequest{
		Question:   "質問",
		Candidates: candidates,
	})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if len(answer.SourceIDs) != 3 {
		t.Fatalf("source ids = %v, want first 3 candidates", answer.SourceIDs)
	}
	if strings.Contains(answer.Text, "chunk_4") || !strings.Contains(answer.Text, "…") {
		t.Fatalf("answer did not limit/truncate candidates: %q", answer.Text)
	}
}

func TestBuildPromptIncludesHistoryAndSources(t *testing.T) {
	prompt := buildPrompt(application.ChatAnswerRequest{
		Question: "What?",
		History:  []*domain.ChatMessage{{Role: "user", Content: "Earlier"}},
		Candidates: []domain.ChatSourceCandidate{{
			ChunkID:  "chunk_1",
			Filename: "doc.pdf",
			Text:     "Source text",
		}},
	})
	for _, want := range []string{"user: Earlier", "source_id: chunk_1", "Source text", "Question", "What?"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q: %q", want, prompt)
		}
	}
}

func TestGeminiAnswererWithoutClientReturnsError(t *testing.T) {
	answer, err := (&GeminiAnswerer{model: "test"}).Answer(context.Background(), application.ChatAnswerRequest{})
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("Answer() = %v, want not initialized error", err)
	}
	if answer.Text != "" {
		t.Fatalf("answer = %+v, want empty answer", answer)
	}
}
