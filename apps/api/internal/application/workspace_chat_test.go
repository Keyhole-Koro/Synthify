package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// fakeAnswerer は ChatAnswerer の差し替え。実 LLM は呼ばない。
type fakeAnswerer struct {
	answer     ChatAnswer
	err        error
	lastReq    ChatAnswerRequest
	callCount  int
	claimedIDs []string
	modelID    string
}

func (f *fakeAnswerer) ModelID() string {
	if f.modelID == "" {
		return "fake-model"
	}
	return f.modelID
}

func (f *fakeAnswerer) Answer(ctx context.Context, req ChatAnswerRequest) (ChatAnswer, error) {
	f.callCount++
	f.lastReq = req
	if f.err != nil {
		return ChatAnswer{}, f.err
	}
	if f.claimedIDs != nil {
		return ChatAnswer{Text: f.answer.Text, SourceChunkIDs: f.claimedIDs}, nil
	}
	return f.answer, nil
}

type chatFixture struct {
	store     *mock.Store
	svc       *WorkspaceChatService
	answerer  *fakeAnswerer
	wsID      string
	userID    string
	documentA string
}

// newChatFixture は「処理済みドキュメントが1件ある workspace」を用意する。
func newChatFixture(t *testing.T) *chatFixture {
	t.Helper()
	ctx := context.Background()
	store := mock.NewStore()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")

	doc, _, err := store.CreateDocument(ctx, ws.Workspace.WorkspaceID, "owner", "report.pdf", "application/pdf", 100)
	require.NoError(t, err)
	require.NoError(t, store.SaveDocumentChunks(ctx, doc.DocumentID, []*domain.DocumentChunk{
		{ChunkID: "c1", DocumentID: doc.DocumentID, Heading: "序論", Text: "前置きの説明"},
		{ChunkID: "c2", DocumentID: doc.DocumentID, Heading: "結論", Text: "結論はXである"},
	}))
	job, err := store.CreateProcessingJob(ctx, doc.DocumentID, ws.Workspace.WorkspaceID, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NoError(t, store.CompleteProcessingJob(ctx, job.JobID))

	answerer := &fakeAnswerer{answer: ChatAnswer{Text: "結論はXです。"}}
	svc := NewWorkspaceChatService(WorkspaceChatServiceDeps{
		Repo:       store,
		Workspaces: store,
		Answerer:   answerer,
	})

	return &chatFixture{
		store:     store,
		svc:       svc,
		answerer:  answerer,
		wsID:      ws.Workspace.WorkspaceID,
		userID:    "owner",
		documentA: doc.DocumentID,
	}
}

func TestSendMessage_PersistsBothTurnsAndCreatesConversation(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	userMsg, assistantMsg, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err)

	assert.Equal(t, domain.ChatRoleUser, userMsg.Role)
	assert.Equal(t, "結論は？", userMsg.Content)
	assert.Equal(t, domain.ChatRoleAssistant, assistantMsg.Role)
	assert.Equal(t, "結論はXです。", assistantMsg.Content)
	assert.Equal(t, "fake-model", assistantMsg.ModelSelection,
		"the stored model must be the one that actually answered")
	assert.Equal(t, domain.ChatMessageStatusComplete, assistantMsg.Status)

	convs, _, err := f.svc.ListConversations(ctx, f.wsID, f.userID)
	require.NoError(t, err)
	require.Len(t, convs, 1, "the first message creates the conversation")
	assert.Equal(t, "結論は？", convs[0].Title)

	msgs, err := f.svc.ListMessages(ctx, f.wsID, convs[0].ConversationID, f.userID)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

func TestSendMessage_RejectsInvalidText(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		text string
	}{
		{name: "empty", text: ""},
		{name: "whitespace only", text: "   \n"},
		{name: "over the limit", text: strings.Repeat("あ", domain.ChatMessageMaxRunes+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newChatFixture(t)
			_, _, err := f.svc.SendMessage(ctx, f.wsID, "", tt.text, f.userID)
			assert.ErrorIs(t, err, domain.ErrInvalidArgument)
			assert.Zero(t, f.answerer.callCount, "invalid input must not reach the model")
		})
	}
}

func TestSendMessage_NonMember_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	_, _, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", "stranger")
	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.Zero(t, f.answerer.callCount, "an unauthorized caller must not reach the model")
}

// 資料が1件も無い workspace では、model を呼ぶ前に落とす。
func TestSendMessage_NoProcessedDocuments_ReturnsSourceUnavailable(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
	answerer := &fakeAnswerer{}
	svc := NewWorkspaceChatService(WorkspaceChatServiceDeps{Repo: store, Workspaces: store, Answerer: answerer})

	_, _, err := svc.SendMessage(ctx, ws.Workspace.WorkspaceID, "", "結論は？", "owner")
	assert.ErrorIs(t, err, domain.ErrChatSourceUnavailable)
	assert.Zero(t, answerer.callCount, "no sources means no model spend")
}

// 処理中のドキュメントしか無い workspace も同じく回答不能。
func TestSendMessage_OnlyInFlightDocuments_ReturnsSourceUnavailable(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")

	doc, _, err := store.CreateDocument(ctx, ws.Workspace.WorkspaceID, "owner", "wip.pdf", "application/pdf", 100)
	require.NoError(t, err)
	require.NoError(t, store.SaveDocumentChunks(ctx, doc.DocumentID, []*domain.DocumentChunk{
		{ChunkID: "c1", DocumentID: doc.DocumentID, Text: "処理中の中身"},
	}))
	_, err = store.CreateProcessingJob(ctx, doc.DocumentID, ws.Workspace.WorkspaceID, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)

	answerer := &fakeAnswerer{}
	svc := NewWorkspaceChatService(WorkspaceChatServiceDeps{Repo: store, Workspaces: store, Answerer: answerer})

	_, _, err = svc.SendMessage(ctx, ws.Workspace.WorkspaceID, "", "結論は？", "owner")
	assert.ErrorIs(t, err, domain.ErrChatSourceUnavailable)
}

// 生成が落ちても質問は残り、assistant 行は failed として永続化される。
func TestSendMessage_GenerationFailure_PersistsFailedTurn(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)
	f.answerer.err = errors.New("provider exploded")

	userMsg, assistantMsg, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err, "a provider failure is a message state, not an RPC error")

	assert.Equal(t, "結論は？", userMsg.Content, "the question survives a failed generation")
	assert.Equal(t, domain.ChatMessageStatusFailed, assistantMsg.Status)
	assert.Equal(t, domain.ChatErrGenerationFailed, assistantMsg.ErrorCode)
	assert.Empty(t, assistantMsg.Content)

	convs, _, err := f.svc.ListConversations(ctx, f.wsID, f.userID)
	require.NoError(t, err)
	msgs, err := f.svc.ListMessages(ctx, f.wsID, convs[0].ConversationID, f.userID)
	require.NoError(t, err)
	require.Len(t, msgs, 2, "the failed turn is not silently omitted")
}

// citation 検証: 候補集合に無い id は捨てる。
func TestSendMessage_DropsCitationsOutsideTheCandidateSet(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)
	f.answerer.claimedIDs = []string{"c2", "fabricated-chunk", "c2"}

	_, assistantMsg, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err)

	require.Len(t, assistantMsg.Sources, 1, "unknown and duplicate ids are dropped")
	assert.Equal(t, "c2", assistantMsg.Sources[0].ChunkID)
	assert.Equal(t, f.documentA, assistantMsg.Sources[0].DocumentID)
	assert.Equal(t, "report.pdf / 結論", assistantMsg.Sources[0].Label)
}

// 別 workspace の chunk id を主張されても引用にしない。
func TestSendMessage_RejectsCitationFromAnotherWorkspace(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	acct, err := f.store.GetOrCreateAccount(ctx, "stranger")
	require.NoError(t, err)
	otherWS, err := f.store.CreateWorkspace(ctx, acct.AccountID, "other-workspace")
	require.NoError(t, err)
	otherDoc, _, err := f.store.CreateDocument(ctx, otherWS.WorkspaceID, "stranger", "secret.pdf", "application/pdf", 100)
	require.NoError(t, err)
	require.NoError(t, f.store.SaveDocumentChunks(ctx, otherDoc.DocumentID, []*domain.DocumentChunk{
		{ChunkID: "secret-chunk", DocumentID: otherDoc.DocumentID, Text: "他人の秘密"},
	}))

	f.answerer.claimedIDs = []string{"secret-chunk"}

	_, assistantMsg, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err)
	assert.Empty(t, assistantMsg.Sources, "a chunk from another workspace can never become a citation")
}

func TestSendMessage_ContinuesExistingConversation(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	_, _, err := f.svc.SendMessage(ctx, f.wsID, "", "最初の質問", f.userID)
	require.NoError(t, err)

	convs, _, err := f.svc.ListConversations(ctx, f.wsID, f.userID)
	require.NoError(t, err)
	require.Len(t, convs, 1)
	convID := convs[0].ConversationID

	_, _, err = f.svc.SendMessage(ctx, f.wsID, convID, "二つ目の質問", f.userID)
	require.NoError(t, err)

	convs, _, err = f.svc.ListConversations(ctx, f.wsID, f.userID)
	require.NoError(t, err)
	assert.Len(t, convs, 1, "a follow-up must not create a second conversation")

	msgs, err := f.svc.ListMessages(ctx, f.wsID, convID, f.userID)
	require.NoError(t, err)
	assert.Len(t, msgs, 4)

	// 2回目の生成には1回目の履歴が渡る。
	assert.NotEmpty(t, f.answerer.lastReq.History, "the second turn sees prior history")
}

// 別 workspace の conversation_id を渡しても、存在を漏らさず Forbidden。
func TestSendMessage_ConversationFromAnotherWorkspace_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	acct, err := f.store.GetOrCreateAccount(ctx, "stranger")
	require.NoError(t, err)
	otherWS, err := f.store.CreateWorkspace(ctx, acct.AccountID, "other-workspace")
	require.NoError(t, err)
	otherConv, err := f.store.CreateChatConversation(ctx, otherWS.WorkspaceID, "stranger", "theirs")
	require.NoError(t, err)

	_, _, err = f.svc.SendMessage(ctx, f.wsID, otherConv.ConversationID, "結論は？", f.userID)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListMessages_NonMember_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	_, _, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err)
	convs, _, err := f.svc.ListConversations(ctx, f.wsID, f.userID)
	require.NoError(t, err)

	_, err = f.svc.ListMessages(ctx, f.wsID, convs[0].ConversationID, "stranger")
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListConversations_NonMember_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	_, _, err := f.svc.ListConversations(ctx, f.wsID, "stranger")
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// 候補は workspace 内の succeeded document に限られる。
func TestSendMessage_CandidatesStayInsideTheWorkspace(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	acct, err := f.store.GetOrCreateAccount(ctx, "stranger")
	require.NoError(t, err)
	otherWS, err := f.store.CreateWorkspace(ctx, acct.AccountID, "other-workspace")
	require.NoError(t, err)
	otherDoc, _, err := f.store.CreateDocument(ctx, otherWS.WorkspaceID, "stranger", "secret.pdf", "application/pdf", 100)
	require.NoError(t, err)
	require.NoError(t, f.store.SaveDocumentChunks(ctx, otherDoc.DocumentID, []*domain.DocumentChunk{
		{ChunkID: "secret-chunk", DocumentID: otherDoc.DocumentID, Text: "他人の秘密"},
	}))
	otherJob, err := f.store.CreateProcessingJob(ctx, otherDoc.DocumentID, otherWS.WorkspaceID, "stranger", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NoError(t, f.store.CompleteProcessingJob(ctx, otherJob.JobID))

	_, _, err = f.svc.SendMessage(ctx, f.wsID, "", "秘密", f.userID)
	require.NoError(t, err)

	for _, c := range f.answerer.lastReq.Candidates {
		assert.Equal(t, f.documentA, c.DocumentID, "no candidate may come from another workspace")
	}
}

// retrieval が空振りしても回答不能にせず、workspace 全体から候補を作る。
func TestSendMessage_FallsBackWhenLexicalSearchMisses(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)

	_, _, err := f.svc.SendMessage(ctx, f.wsID, "", "まったく一致しない語句", f.userID)
	require.NoError(t, err)

	assert.NotEmpty(t, f.answerer.lastReq.Candidates, "a lexical miss still yields grounding context")
}

// retrieval snapshot は再現用の識別子だけを持ち、本文を含めない。
func TestSnapshotJSON_ContainsOnlyBoundedIdentifiers(t *testing.T) {
	candidates := []domain.ChatSourceCandidate{
		{ChunkID: "c1", DocumentID: "d1", Text: "機密の本文", Heading: "見出し"},
	}

	raw := snapshotJSON(candidates, "gemini")

	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(raw, &snapshot))

	assert.Equal(t, "lexical_v1", snapshot["strategy"])
	assert.Equal(t, "gemini", snapshot["effective_model"])
	assert.Contains(t, string(raw), "c1")
	assert.NotContains(t, string(raw), "機密の本文", "chunk text must never enter the snapshot")
	assert.NotContains(t, string(raw), "見出し")
}

func TestValidateCitations(t *testing.T) {
	candidates := []domain.ChatSourceCandidate{
		{ChunkID: "c1", DocumentID: "d1", Filename: "a.pdf"},
		{ChunkID: "c2", DocumentID: "d1", Filename: "a.pdf", Heading: "結論"},
	}

	tests := []struct {
		name     string
		claimed  []string
		wantIDs  []string
		wantNote string
	}{
		{name: "all valid", claimed: []string{"c1", "c2"}, wantIDs: []string{"c1", "c2"}},
		{name: "unknown dropped", claimed: []string{"c1", "nope"}, wantIDs: []string{"c1"}},
		{name: "duplicates collapsed", claimed: []string{"c2", "c2"}, wantIDs: []string{"c2"}},
		{name: "none claimed", claimed: nil, wantIDs: []string{}},
		{name: "all invalid", claimed: []string{"x", "y"}, wantIDs: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateCitations(tt.claimed, candidates)
			ids := make([]string, 0, len(got))
			for _, s := range got {
				ids = append(ids, s.ChunkID)
			}
			assert.Equal(t, tt.wantIDs, ids)
		})
	}
}

func TestBoundContext_StopsAtTheCharacterBudget(t *testing.T) {
	candidates := []domain.ChatSourceCandidate{
		{ChunkID: "c1", Text: strings.Repeat("a", 100)},
		{ChunkID: "c2", Text: strings.Repeat("b", 100)},
		{ChunkID: "c3", Text: strings.Repeat("c", 100)},
	}

	assert.Len(t, boundContext(candidates, 1000), 3, "everything fits under a generous budget")
	assert.Len(t, boundContext(candidates, 150), 1, "the budget truncates the candidate list")
	assert.Empty(t, boundContext(candidates, 10), "a budget smaller than the first chunk yields nothing")
}

func TestConversationTitle_TruncatesLongQuestions(t *testing.T) {
	assert.Equal(t, "短い質問", conversationTitle("  短い質問  "))

	long := strings.Repeat("あ", 60)
	title := conversationTitle(long)
	assert.Equal(t, 41, len([]rune(title)), "40 runes plus the ellipsis")
	assert.True(t, strings.HasSuffix(title, "…"))
}

// message 行の model_selection は「実際に答えた主体」でなければならない。
// 固定値 "gemini" を書くと、モデルを増やしたときに監査が成立しなくなる。
func TestSendMessage_RecordsTheAnswererThatActuallyAnswered(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)
	f.answerer.modelID = "extractive-dev"

	_, assistantMsg, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err)
	assert.Equal(t, "extractive-dev", assistantMsg.ModelSelection)
}

// 生成が失敗した行にも、試みた主体を記録する。
func TestSendMessage_FailedTurnRecordsTheAnswerer(t *testing.T) {
	ctx := context.Background()
	f := newChatFixture(t)
	f.answerer.modelID = "gemini-3-flash-preview"
	f.answerer.err = errors.New("boom")

	_, assistantMsg, err := f.svc.SendMessage(ctx, f.wsID, "", "結論は？", f.userID)
	require.NoError(t, err)
	assert.Equal(t, domain.ChatMessageStatusFailed, assistantMsg.Status)
	assert.Equal(t, "gemini-3-flash-preview", assistantMsg.ModelSelection)
}

// UI が「質問できるか」を自前で推測しないよう、判定はサーバーが返す。
// retrieval と同じ条件 (succeeded job を持つ document) でなければならない。
func TestListConversations_ReportsWhetherTheWorkspaceCanAnswer(t *testing.T) {
	ctx := context.Background()

	t.Run("workspace with a processed document", func(t *testing.T) {
		f := newChatFixture(t)
		_, hasSources, err := f.svc.ListConversations(ctx, f.wsID, f.userID)
		require.NoError(t, err)
		assert.True(t, hasSources)
	})

	t.Run("empty workspace", func(t *testing.T) {
		store := mock.NewStore()
		ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
		svc := NewWorkspaceChatService(WorkspaceChatServiceDeps{
			Repo: store, Workspaces: store, Answerer: &fakeAnswerer{},
		})

		_, hasSources, err := svc.ListConversations(ctx, ws.Workspace.WorkspaceID, "owner")
		require.NoError(t, err)
		assert.False(t, hasSources)
	})

	t.Run("only an in-flight document", func(t *testing.T) {
		store := mock.NewStore()
		ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
		doc, _, err := store.CreateDocument(ctx, ws.Workspace.WorkspaceID, "owner", "wip.pdf", "application/pdf", 100)
		require.NoError(t, err)
		require.NoError(t, store.SaveDocumentChunks(ctx, doc.DocumentID, []*domain.DocumentChunk{
			{ChunkID: "c1", DocumentID: doc.DocumentID, Text: "処理中"},
		}))
		_, err = store.CreateProcessingJob(ctx, doc.DocumentID, ws.Workspace.WorkspaceID, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
		require.NoError(t, err)

		svc := NewWorkspaceChatService(WorkspaceChatServiceDeps{
			Repo: store, Workspaces: store, Answerer: &fakeAnswerer{},
		})

		_, hasSources, err := svc.ListConversations(ctx, ws.Workspace.WorkspaceID, "owner")
		require.NoError(t, err)
		assert.False(t, hasSources, "an in-flight upload is not answerable yet")
	})
}
