package chat

import (
	"context"
	"strings"

	"github.com/synthify/backend/apps/api/internal/application"
)

// ExtractiveAnswerer answers without a model, by quoting the retrieved chunks.
//
// It exists so local development and e2e runs exercise the real retrieval,
// citation-validation, and persistence paths without a Gemini key or per-run
// API spend. It is NOT a fallback for production: bootstrap selects it only in
// non-production environments, and it labels its own output so a reader can
// never mistake it for a generated answer.
type ExtractiveAnswerer struct{}

func NewExtractiveAnswerer() *ExtractiveAnswerer { return &ExtractiveAnswerer{} }

// ExtractiveAnswerNotice prefixes every extractive answer. Tests assert on it,
// and the UI shows it verbatim, so it must stay recognisable.
const ExtractiveAnswerNotice = "[ローカル開発用の抽出回答 / no model configured]"

func (a *ExtractiveAnswerer) Answer(ctx context.Context, req application.ChatAnswerRequest) (application.ChatAnswer, error) {
	if len(req.Candidates) == 0 {
		return application.ChatAnswer{
			Text: ExtractiveAnswerNotice + "\n\n該当する資料が見つかりませんでした。",
		}, nil
	}

	// 質問語を含む chunk を優先し、無ければ先頭から使う。
	query := strings.ToLower(req.Question)
	matched := make([]int, 0, len(req.Candidates))
	for i, c := range req.Candidates {
		if containsAnyToken(strings.ToLower(c.Text)+" "+strings.ToLower(c.Heading), query) {
			matched = append(matched, i)
		}
	}
	if len(matched) == 0 {
		for i := range req.Candidates {
			matched = append(matched, i)
		}
	}
	if len(matched) > 3 {
		matched = matched[:3]
	}

	var b strings.Builder
	b.WriteString(ExtractiveAnswerNotice)
	b.WriteString("\n\n")
	b.WriteString("質問: ")
	b.WriteString(req.Question)
	b.WriteString("\n\n関連する記述:\n")

	chunkIDs := make([]string, 0, len(matched))
	for _, i := range matched {
		c := req.Candidates[i]
		b.WriteString("\n・")
		b.WriteString(c.Label())
		b.WriteString("\n  ")
		b.WriteString(truncateRunes(c.Text, 200))
		b.WriteString("\n")
		chunkIDs = append(chunkIDs, c.ChunkID)
	}

	return application.ChatAnswer{Text: b.String(), SourceChunkIDs: chunkIDs}, nil
}

// containsAnyToken reports whether text contains any whitespace-separated token
// of query that is at least two characters long.
func containsAnyToken(text, query string) bool {
	for _, token := range strings.Fields(query) {
		if len([]rune(token)) < 2 {
			continue
		}
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}
