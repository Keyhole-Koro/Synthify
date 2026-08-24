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
		// 出典ゼロでも回答自体は返す。サービス側が grounded=false として
		// 記録し、UI が「資料に基づかない回答」と明示する。
		return application.ChatAnswer{
			Text: ExtractiveAnswerNotice + "\n\nこのワークスペースにはまだ参照できる資料もページもありません。" +
				"モデルを設定すると、一般知識にもとづく回答を返せます。\n\n質問: " + req.Question,
		}, nil
	}

	// 候補は retrieval が一致数の多い順に並べて渡してくるので、ここでは
	// 並べ替えず先頭から採る。独自にトークン化し直すと、空白の無い日本語で
	// 質問文全体が1トークンになり一致ゼロになる (retrieval 側と同じ罠)。
	matched := make([]int, 0, 3)
	for i := range req.Candidates {
		matched = append(matched, i)
		if len(matched) == 3 {
			break
		}
	}

	var b strings.Builder
	b.WriteString(ExtractiveAnswerNotice)
	b.WriteString("\n\n")
	b.WriteString("質問: ")
	b.WriteString(req.Question)
	b.WriteString("\n\n関連する記述:\n")

	sourceIDs := make([]string, 0, len(matched))
	for _, i := range matched {
		c := req.Candidates[i]
		b.WriteString("\n・")
		b.WriteString(c.Label())
		b.WriteString("\n  ")
		b.WriteString(truncateRunes(c.Text, 200))
		b.WriteString("\n")
		sourceIDs = append(sourceIDs, c.SourceID())
	}

	return application.ChatAnswer{Text: b.String(), SourceIDs: sourceIDs}, nil
}

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

// ModelID reports that no model was involved, so a stored message can never
// claim Gemini answered when it did not.
func (a *ExtractiveAnswerer) ModelID() string { return "extractive-dev" }
