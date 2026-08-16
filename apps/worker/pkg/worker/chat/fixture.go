package chat

import (
	"context"
	"fmt"
	"log/slog"

	chatstatus "github.com/synthify/backend/internal/platform/chat/status"
)

// FixtureGenerator produces a deterministic turn without calling a model.
//
// It exists so the dialogue UI can be exercised end to end locally and in e2e,
// where there is no Vertex credential. Enabling it is gated by the same
// FixtureMode flag as the document processor, which configuration rejects
// outside local/test/dev.
//
// It writes through the same Writer and the same two-phase shape as the real
// generator — start, progress per section, then succeed — so what the UI
// receives here matches production apart from the words.
type FixtureGenerator struct {
	writer chatstatus.Writer
	logger *slog.Logger
}

func NewFixtureGenerator(writer chatstatus.Writer, logger *slog.Logger) *FixtureGenerator {
	return &FixtureGenerator{writer: writer, logger: logger}
}

func (g *FixtureGenerator) Run(ctx context.Context, req Request) error {
	ref := chatstatus.Ref{
		UserID:      req.UserID,
		WorkspaceID: req.WorkspaceID,
		ChatID:      req.ChatID,
		TurnID:      req.TurnID,
		PaperID:     req.PaperID,
	}
	if err := g.writer.Start(ctx, ref); err != nil {
		return fmt.Errorf("start turn: %w", err)
	}

	question := ""
	if n := len(req.History); n > 0 {
		question = req.History[n-1].Text
	}

	// Two sections, written separately, so the progressive fill-in is visible
	// rather than appearing all at once.
	first := []ContentNode{
		{"type": "section", "title": "回答", "children": []any{
			map[string]any{"type": "text", "value": "「" + question + "」についてのフィクスチャ応答です。"},
		}},
	}
	firstJSON, err := marshalContent(first)
	if err != nil {
		return err
	}
	if err := g.writer.Progress(ctx, ref, firstJSON, 1); err != nil {
		g.logger.Warn("chat.fixture_progress_failed", "error", err.Error())
	}

	all := append(first, ContentNode{
		"type": "list",
		"items": []any{
			[]any{map[string]any{"type": "text", "value": "セクション単位で書き込まれます"}},
			[]any{map[string]any{"type": "text", "value": "content は毎回全体が入ります"}},
		},
	})

	// A long run with no spaces. Japanese does not break on whitespace and a
	// model can return a long identifier, so this is the case that exposes a
	// container which grows instead of wrapping.
	all = append(all, ContentNode{
		"type": "paragraph",
		"children": []any{
			map[string]any{
				"type":  "text",
				"value": "長い文章が折り返されることを確認するための行です。空白を含まない日本語の文章はコンテナ幅を超えても改行されないことがあるため、ここで実際に折り返しを検証します。",
			},
		},
	})

	// A reference to a real candidate proves the link path renders and is
	// clickable; the sanitizer would demote an invented id to plain text.
	selected := []string{}
	if len(req.Candidates) > 0 {
		target := req.Candidates[0]
		all = append(all, ContentNode{
			"type":    "paper-link",
			"paperId": target.PaperID,
			"label":   target.Title,
		})
		selected = append(selected, target.PaperID)
	}

	allJSON, err := marshalContent(all)
	if err != nil {
		return err
	}

	// Run the proposal through the real classifier rather than hand-writing the
	// two fields, so the fixture cannot drift from the rule it is demonstrating.
	var proposed []Command
	if len(req.Candidates) > 0 {
		proposed = append(proposed, Command{
			"type":     "CREATE_CHILD_NODE",
			"parentId": req.Candidates[0].PaperID,
			"title":    "フィクスチャが提案する子 paper",
		})
	}
	auto, proposals := ClassifyCommands(proposed, allowedPaperIDs(req.Candidates))
	commandsJSON, err := marshalCommands(auto)
	if err != nil {
		return err
	}
	proposalsJSON, err := marshalCommands(proposals)
	if err != nil {
		return err
	}

	return g.writer.Succeed(ctx, ref, chatstatus.Success{
		ContentJSON:        allJSON,
		CommandsJSON:       commandsJSON,
		ProposalsJSON:      proposalsJSON,
		SelectedContextIDs: selected,
		SectionCount:       2,
		FinishReason:       chatstatus.FirestoreChatTurnFinishReasonStop,
	})
}
