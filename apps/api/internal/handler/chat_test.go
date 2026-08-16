package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type fakeChatDispatcher struct {
	called bool
	got    domain.ChatTurnRequest
	err    error
}

func (f *fakeChatDispatcher) RunChatTurn(_ context.Context, req domain.ChatTurnRequest) error {
	f.called = true
	f.got = req
	return f.err
}

func newChatHandlerForTest(t testing.TB) (*ChatHandler, *mock.Store, *fakeChatDispatcher) {
	t.Helper()
	store := mock.NewStore()
	dispatcher := &fakeChatDispatcher{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewChatHandler(store, dispatcher, logger), store, dispatcher
}

func userCtx(userID string) context.Context {
	return auth.ContextWithPrincipal(context.Background(), auth.Principal{
		Kind:      auth.PrincipalKindUser,
		SubjectID: userID,
		Email:     userID + "@example.com",
	})
}

func userMessage(text string) *appv1.ChatMessage {
	return &appv1.ChatMessage{Role: appv1.ChatRole_CHAT_ROLE_USER, Text: text}
}

func validRequest(workspaceID string) *appv1.PostChatTurnRequest {
	return &appv1.PostChatTurnRequest{
		WorkspaceId: workspaceID,
		PaperId:     "paper-1",
		History:     []*appv1.ChatMessage{userMessage("what is this?")},
	}
}

func TestPostChatTurn_Unauthenticated(t *testing.T) {
	h, _, dispatcher := newChatHandlerForTest(t)

	_, err := h.PostChatTurn(context.Background(), connect.NewRequest(validRequest("ws-1")))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	assert.False(t, dispatcher.called, "must not dispatch without a caller")
}

func TestPostChatTurn_ForeignWorkspaceIsDenied(t *testing.T) {
	h, store, dispatcher := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	_, err := h.PostChatTurn(userCtx("stranger"), connect.NewRequest(validRequest(ws)))

	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	assert.False(t, dispatcher.called, "must not dispatch for an unauthorized workspace")
}

func TestPostChatTurn_AllocatesIDsAndDispatches(t *testing.T) {
	h, store, dispatcher := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	res, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(validRequest(ws)))

	require.NoError(t, err)
	assert.NotEmpty(t, res.Msg.GetChatId(), "a new chat gets an id")
	assert.NotEmpty(t, res.Msg.GetTurnId())
	require.True(t, dispatcher.called)
	// The worker addresses the turn document at users/{uid}/..., so the owner
	// has to travel with the request.
	assert.Equal(t, "owner", dispatcher.got.UserID)
	assert.Equal(t, res.Msg.GetChatId(), dispatcher.got.ChatID)
	assert.Equal(t, res.Msg.GetTurnId(), dispatcher.got.TurnID)
}

func TestPostChatTurn_KeepsSuppliedChatIDAndAllocatesNewTurn(t *testing.T) {
	h, store, _ := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	req := validRequest(ws)
	req.ChatId = "existing-chat"
	res, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))

	require.NoError(t, err)
	assert.Equal(t, "existing-chat", res.Msg.GetChatId(), "continuing a chat keeps its id")
	assert.NotEqual(t, "existing-chat", res.Msg.GetTurnId(), "each turn gets a fresh id")
}

func TestPostChatTurn_RejectsMissingFields(t *testing.T) {
	h, store, _ := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	cases := map[string]func(*appv1.PostChatTurnRequest){
		"no workspace": func(r *appv1.PostChatTurnRequest) { r.WorkspaceId = "" },
		"no paper":     func(r *appv1.PostChatTurnRequest) { r.PaperId = "" },
		"no history":   func(r *appv1.PostChatTurnRequest) { r.History = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validRequest(ws)
			mutate(req)
			_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

func TestPostChatTurn_RejectsHistoryNotEndingWithUser(t *testing.T) {
	h, store, _ := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	req := validRequest(ws)
	req.History = append(req.History, &appv1.ChatMessage{
		Role: appv1.ChatRole_CHAT_ROLE_ASSISTANT,
		Text: "an answer",
	})

	_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPostChatTurn_TrimsHistoryToTheNewestMessages(t *testing.T) {
	h, store, dispatcher := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	req := validRequest(ws)
	req.History = nil
	for i := 0; i < maxChatHistoryMessages+10; i++ {
		req.History = append(req.History, userMessage("m"))
	}
	req.History[len(req.History)-1] = userMessage("newest")

	_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))

	require.NoError(t, err)
	assert.Len(t, dispatcher.got.History, maxChatHistoryMessages)
	// The tail is the current question, so that is the end that must survive.
	assert.Equal(t, "newest", dispatcher.got.History[len(dispatcher.got.History)-1].Text)
}

func TestPostChatTurn_RejectsTooManyCandidates(t *testing.T) {
	h, store, _ := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	req := validRequest(ws)
	for i := 0; i < maxChatCandidates+1; i++ {
		req.Candidates = append(req.Candidates, &appv1.ContextPaper{PaperId: "p"})
	}

	_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPostChatTurn_RejectsOversizedCandidateTotal(t *testing.T) {
	h, store, _ := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	req := validRequest(ws)
	// Each is trimmed to the per-item cap, so enough of them still breach the total.
	for i := 0; i < maxChatCandidates; i++ {
		req.Candidates = append(req.Candidates, &appv1.ContextPaper{
			PaperId: "p",
			Content: strings.Repeat("x", maxChatCandidateBytes),
		})
	}

	_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPostChatTurn_TruncatesOversizedCandidateContent(t *testing.T) {
	h, store, dispatcher := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID

	req := validRequest(ws)
	req.Candidates = []*appv1.ContextPaper{{
		PaperId: "p",
		Content: strings.Repeat("x", maxChatCandidateBytes*2),
	}}

	_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(req))

	require.NoError(t, err, "one oversized paper is trimmed, not rejected")
	assert.LessOrEqual(t, len(dispatcher.got.Candidates[0].Content), maxChatCandidateBytes)
}

// Truncation must not split a rune: a trailing partial byte sequence would
// corrupt the prompt the worker builds.
func TestTruncateBytes_CutsOnRuneBoundary(t *testing.T) {
	// 3 bytes per character, so a 10-byte limit lands mid-rune.
	got := truncateBytes(strings.Repeat("あ", 10), 10)

	assert.True(t, len(got) <= 10)
	assert.Equal(t, "あああ", got, "cuts back to the last whole rune")
}

func TestPostChatTurn_SurfacesDispatchFailure(t *testing.T) {
	h, store, dispatcher := newChatHandlerForTest(t)
	ctx := context.Background()
	ws := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	dispatcher.err = errors.New("worker unreachable")

	_, err := h.PostChatTurn(userCtx("owner"), connect.NewRequest(validRequest(ws)))

	require.Error(t, err, "a turn that was never dispatched must not report success")
}
