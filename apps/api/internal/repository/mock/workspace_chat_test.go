package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// seedSucceededDocument は chunk を持ち処理が succeeded した document を作る。
// 返り値は documentID。
func seedSucceededDocument(t testing.TB, ctx context.Context, store *Store, workspaceID, filename string, chunks []*domain.DocumentChunk) string {
	t.Helper()

	doc, _, err := store.CreateDocument(ctx, workspaceID, "owner", filename, "application/pdf", 100)
	require.NoError(t, err, "CreateDocument")

	for _, chunk := range chunks {
		chunk.DocumentID = doc.DocumentID
	}
	require.NoError(t, store.SaveDocumentChunks(ctx, doc.DocumentID, chunks), "SaveDocumentChunks")

	job, err := store.CreateProcessingJob(ctx, doc.DocumentID, workspaceID, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err, "CreateProcessingJob")
	require.NoError(t, store.CompleteProcessingJob(ctx, job.JobID), "CompleteProcessingJob")

	return doc.DocumentID
}

// createSecondWorkspace は fixture とは別の workspace を作る。
// mock の CreateWorkspace は名前から ID を導出し、CreateUserWorkspaceFixture は
// 常に "test-workspace" を使うので、user を変えるだけでは同じ workspace になる。
// document を置けるように account も先に用意する。
func createSecondWorkspace(t testing.TB, ctx context.Context, store *Store, userID string) *domain.Workspace {
	t.Helper()

	acct, err := store.GetOrCreateAccount(ctx, userID)
	require.NoError(t, err, "GetOrCreateAccount")

	ws, err := store.CreateWorkspace(ctx, acct.AccountID, "other-workspace")
	require.NoError(t, err, "CreateWorkspace")
	return ws
}

func TestCreateChatMessage_UnknownConversation_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	_, err := store.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: "does-not-exist",
		Role:           domain.ChatRoleUser,
		Content:        "hello",
	}, nil)

	// 実 DB では conversation_id の FK 違反になる経路。
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListChatMessages_ReturnsMessagesWithSourcesInOrder(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")

	conv, err := store.CreateChatConversation(ctx, fixture.Workspace.WorkspaceID, "owner", "新しい会話")
	require.NoError(t, err)

	_, err = store.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: conv.ConversationID,
		Role:           domain.ChatRoleUser,
		Content:        "結論は？",
	}, nil)
	require.NoError(t, err)

	_, err = store.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: conv.ConversationID,
		Role:           domain.ChatRoleAssistant,
		Content:        "結論はXです。",
		ModelSelection: "gemini",
		Sources: []domain.ChatSource{
			{DocumentID: "doc-1", ChunkID: "chunk-1", Label: "report.pdf / 結論"},
		},
	}, nil)
	require.NoError(t, err)

	msgs, err := store.ListChatMessages(ctx, conv.ConversationID, 50)
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	assert.Equal(t, domain.ChatRoleUser, msgs[0].Role)
	assert.Empty(t, msgs[0].Sources, "user message carries no citations")

	assert.Equal(t, domain.ChatRoleAssistant, msgs[1].Role)
	assert.Equal(t, domain.ChatMessageStatusComplete, msgs[1].Status, "status defaults to complete")
	require.Len(t, msgs[1].Sources, 1)
	assert.Equal(t, "chunk-1", msgs[1].Sources[0].ChunkID)
}

// 失敗した turn は行として残るが、次のリクエストの履歴には混ぜない。
func TestListRecentChatMessages_ExcludesNonCompleteMessages(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")

	conv, err := store.CreateChatConversation(ctx, fixture.Workspace.WorkspaceID, "owner", "")
	require.NoError(t, err)

	_, err = store.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: conv.ConversationID,
		Role:           domain.ChatRoleUser,
		Content:        "ok message",
	}, nil)
	require.NoError(t, err)

	_, err = store.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: conv.ConversationID,
		Role:           domain.ChatRoleAssistant,
		Content:        "",
		Status:         domain.ChatMessageStatusFailed,
		ErrorCode:      domain.ChatErrGenerationFailed,
	}, nil)
	require.NoError(t, err)

	all, err := store.ListChatMessages(ctx, conv.ConversationID, 50)
	require.NoError(t, err)
	assert.Len(t, all, 2, "the failed turn is still persisted")

	recent, err := store.ListRecentChatMessages(ctx, conv.ConversationID, 12)
	require.NoError(t, err)
	require.Len(t, recent, 1, "history replays only complete messages")
	assert.Equal(t, "ok message", recent[0].Content)
}

func TestListRecentChatMessages_KeepsNewestInChronologicalOrder(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")

	conv, err := store.CreateChatConversation(ctx, fixture.Workspace.WorkspaceID, "owner", "")
	require.NoError(t, err)

	for _, content := range []string{"m1", "m2", "m3", "m4"} {
		_, err := store.CreateChatMessage(ctx, &domain.ChatMessage{
			ConversationID: conv.ConversationID,
			Role:           domain.ChatRoleUser,
			Content:        content,
		}, nil)
		require.NoError(t, err)
	}

	recent, err := store.ListRecentChatMessages(ctx, conv.ConversationID, 2)
	require.NoError(t, err)
	require.Len(t, recent, 2)
	// 新しい 2 件を、古い順で返す。
	assert.Equal(t, "m3", recent[0].Content)
	assert.Equal(t, "m4", recent[1].Content)
}

func TestListChatConversations_ScopedToWorkspace(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	mine := CreateUserWorkspaceFixture(t, ctx, store, "owner")
	other := createSecondWorkspace(t, ctx, store, "stranger")

	_, err := store.CreateChatConversation(ctx, mine.Workspace.WorkspaceID, "owner", "mine")
	require.NoError(t, err)
	_, err = store.CreateChatConversation(ctx, other.WorkspaceID, "stranger", "theirs")
	require.NoError(t, err)

	convs, err := store.ListChatConversations(ctx, mine.Workspace.WorkspaceID, 50)
	require.NoError(t, err)
	require.Len(t, convs, 1)
	assert.Equal(t, "mine", convs[0].Title)
}

// 会話は workspace のもので、作成者だけのものではない。
func TestListChatConversations_VisibleToAnyWorkspaceReader(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")

	_, err := store.CreateChatConversation(ctx, fixture.Workspace.WorkspaceID, "owner", "shared")
	require.NoError(t, err)

	// repository は user を引数に取らない: 可視性は workspace 単位。
	convs, err := store.ListChatConversations(ctx, fixture.Workspace.WorkspaceID, 50)
	require.NoError(t, err)
	require.Len(t, convs, 1)
	assert.Equal(t, "owner", convs[0].CreatedBy, "created_by stays as audit data")
}

func TestSearchChatSourceCandidates_OnlyReturnsSucceededDocuments(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")
	wsID := fixture.Workspace.WorkspaceID

	seedSucceededDocument(t, ctx, store, wsID, "done.pdf", []*domain.DocumentChunk{
		{ChunkID: "c1", Heading: "結論", Text: "結論はXです"},
	})

	// 処理中の document: chunk はあるが job は succeeded ではない。
	inflight, _, err := store.CreateDocument(ctx, wsID, "owner", "inflight.pdf", "application/pdf", 100)
	require.NoError(t, err)
	require.NoError(t, store.SaveDocumentChunks(ctx, inflight.DocumentID, []*domain.DocumentChunk{
		{ChunkID: "c2", DocumentID: inflight.DocumentID, Heading: "途中", Text: "結論はYです"},
	}))
	_, err = store.CreateProcessingJob(ctx, inflight.DocumentID, wsID, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)

	got, err := store.SearchChatSourceCandidates(ctx, wsID, nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 1, "in-flight documents must not be citable")
	assert.Equal(t, "c1", got[0].ChunkID)
}

func TestSearchChatSourceCandidates_ScopedToWorkspace(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	mine := CreateUserWorkspaceFixture(t, ctx, store, "owner")
	other := createSecondWorkspace(t, ctx, store, "stranger")

	seedSucceededDocument(t, ctx, store, mine.Workspace.WorkspaceID, "mine.pdf", []*domain.DocumentChunk{
		{ChunkID: "c1", Text: "秘密ではない"},
	})
	seedSucceededDocument(t, ctx, store, other.WorkspaceID, "theirs.pdf", []*domain.DocumentChunk{
		{ChunkID: "c2", Text: "他人の資料"},
	})

	got, err := store.SearchChatSourceCandidates(ctx, mine.Workspace.WorkspaceID, nil, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].ChunkID, "another workspace's chunks must never surface")
}

func TestSearchChatSourceCandidates_FiltersByQueryAndRespectsLimit(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")
	wsID := fixture.Workspace.WorkspaceID

	seedSucceededDocument(t, ctx, store, wsID, "doc.pdf", []*domain.DocumentChunk{
		{ChunkID: "c1", Heading: "序論", Text: "前置き"},
		{ChunkID: "c2", Heading: "結論", Text: "結論はXです"},
		{ChunkID: "c3", Heading: "付録", Text: "結論の補足"},
	})

	// 一致した chunk が先頭に来る。一致しない chunk も候補としては残す
	// (「該当語が無い」だけで回答不能にしないため)。
	matched, err := store.SearchChatSourceCandidates(ctx, wsID, []string{"結論"}, 10)
	require.NoError(t, err)
	require.Len(t, matched, 3)
	assert.Equal(t, "c2", matched[0].ChunkID)
	assert.Equal(t, "c3", matched[1].ChunkID)
	assert.Equal(t, "c1", matched[2].ChunkID, "non-matching chunks rank last, not dropped")

	limited, err := store.SearchChatSourceCandidates(ctx, wsID, nil, 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2, "limit bounds the candidate set")

	// 一致数が多い chunk ほど先に来る。
	ranked, err := store.SearchChatSourceCandidates(ctx, wsID, []string{"結論", "補足"}, 10)
	require.NoError(t, err)
	require.NotEmpty(t, ranked)
	assert.Equal(t, "c3", ranked[0].ChunkID, "two term hits outrank one")

	// どの語も一致しなくても候補は返る。
	none, err := store.SearchChatSourceCandidates(ctx, wsID, []string{"存在しない語"}, 10)
	require.NoError(t, err)
	assert.Len(t, none, 3, "a total miss still yields grounding context")
}

func TestCountChatSourceDocuments(t *testing.T) {
	ctx := context.Background()
	store := NewStore()
	fixture := CreateUserWorkspaceFixture(t, ctx, store, "owner")
	wsID := fixture.Workspace.WorkspaceID

	count, err := store.CountChatSourceDocuments(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "an empty workspace has nothing to answer from")

	seedSucceededDocument(t, ctx, store, wsID, "a.pdf", []*domain.DocumentChunk{{ChunkID: "c1", Text: "x"}})
	seedSucceededDocument(t, ctx, store, wsID, "b.pdf", []*domain.DocumentChunk{{ChunkID: "c2", Text: "y"}})

	count, err = store.CountChatSourceDocuments(ctx, wsID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
