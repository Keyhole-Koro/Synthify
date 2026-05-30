package metering

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type capturingReporter struct {
	events []domain.UsageEvent
}

func (c *capturingReporter) RecordUsage(_ context.Context, ev domain.UsageEvent) error {
	c.events = append(c.events, ev)
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func taggedCtx() context.Context {
	return WithTag(context.Background(), Tag{AccountID: "acc-1", WorkspaceID: "ws-1", JobID: "job-1"})
}

func completeResp() *model.LLMResponse {
	return &model.LLMResponse{
		ModelVersion: "gemini-3-flash-preview",
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     100,
			CandidatesTokenCount: 25,
		},
	}
}

func TestReportAgentUsage_ReportsCompleteResponse(t *testing.T) {
	// 目的: ADK 経路の完了レスポンスのトークンが課金イベントとして reporter に
	// 届くことを確認する。これが漏れると agent の生成トークンが課金から落ちる。
	rep := &capturingReporter{}
	reportAgentUsage(taggedCtx(), rep, discardLogger(), completeResp(), nil)

	require.Len(t, rep.events, 1)
	ev := rep.events[0]
	assert.Equal(t, "acc-1", ev.AccountID)
	assert.Equal(t, "ws-1", ev.WorkspaceID)
	assert.Equal(t, "job-1", ev.JobID)
	assert.Equal(t, "gemini-3-flash-preview", ev.Model)
	assert.Equal(t, int64(100), ev.InputTokens)
	assert.Equal(t, int64(25), ev.OutputTokens)
	assert.NotEmpty(t, ev.EventID)
}

func TestReportAgentUsage_SkipsNonReportable(t *testing.T) {
	partial := completeResp()
	partial.Partial = true

	noUsage := &model.LLMResponse{ModelVersion: "gemini-3-flash-preview"}

	cases := []struct {
		name string
		resp *model.LLMResponse
		err  error
	}{
		// 部分(partial)レスポンスは最終イベントと usage が重複するため二重計上を防ぐ。
		{"partial", partial, nil},
		// エラー時は失敗呼び出しなので課金しない。
		{"error", completeResp(), errors.New("boom")},
		// レスポンス無しは計上しようがない。
		{"nil response", nil, nil},
		// usage メタデータ欠落時は計上しない(ゼロ計上を防ぐ)。
		{"no usage metadata", noUsage, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := &capturingReporter{}
			reportAgentUsage(taggedCtx(), rep, discardLogger(), tc.resp, tc.err)
			assert.Empty(t, rep.events)
		})
	}
}

func TestReportAgentUsage_MissingTagDropsEvent(t *testing.T) {
	// タグ未設定時は warn ログのみで落とし、呼び出し自体は壊さない(LLMClient と同じ挙動)。
	rep := &capturingReporter{}
	reportAgentUsage(context.Background(), rep, discardLogger(), completeResp(), nil)
	assert.Empty(t, rep.events)
}
