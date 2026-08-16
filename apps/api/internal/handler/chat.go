package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/platform/util"
)

// Request size limits. Candidate papers carry whole paper bodies, so a request
// grows quickly; these caps reject or trim before Connect's own message limit
// turns it into an opaque transport error. Content over the per-item cap is
// truncated rather than rejected: losing the tail of one paper still leaves a
// usable turn, whereas failing the call loses the user's question.
const (
	maxChatCandidates       = 32
	maxChatCandidateBytes   = 8 * 1024
	maxChatCandidatesTotal  = 128 * 1024
	maxChatHistoryMessages  = 40
	maxChatHistoryTextBytes = 8 * 1024
)

// ChatTurnDispatcher hands a prepared turn to the worker. Chat dispatches
// synchronously rather than through Cloud Tasks: the queueing delay is
// invisible on a document job but reads as latency before the first token here.
type ChatTurnDispatcher interface {
	RunChatTurn(ctx context.Context, req domain.ChatTurnRequest) error
}

// ChatHandler triggers dialogue turns. It never calls the LLM — it
// authenticates, authorizes, allocates ids and dispatches — so the API service
// needs no Vertex AI credentials.
type ChatHandler struct {
	workspaces repository.WorkspaceRepository
	dispatcher ChatTurnDispatcher
	logger     *slog.Logger
}

func NewChatHandler(
	workspaceRepo repository.WorkspaceRepository,
	dispatcher ChatTurnDispatcher,
	logger *slog.Logger,
) *ChatHandler {
	return &ChatHandler{workspaces: workspaceRepo, dispatcher: dispatcher, logger: logger}
}

func (h *ChatHandler) PostChatTurn(
	ctx context.Context,
	req *connect.Request[appv1.PostChatTurnRequest],
) (*connect.Response[appv1.PostChatTurnResponse], error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}

	msg := req.Msg
	workspaceID := msg.GetWorkspaceId()
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	if msg.GetPaperId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("paper_id is required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, workspaceID); err != nil {
		return nil, err
	}

	history, err := normalizeHistory(msg.GetHistory())
	if err != nil {
		return nil, err
	}
	candidates, err := normalizeCandidates(msg.GetCandidates())
	if err != nil {
		return nil, err
	}

	// An empty chat_id starts a new conversation. A caller-supplied one is
	// taken at face value only for grouping: the turn is written under the
	// caller's own uid, so a foreign chat_id can at worst collide inside the
	// caller's own subtree and can never reach another user's conversation.
	chatID := msg.GetChatId()
	if chatID == "" {
		chatID = util.NewULID()
	}
	turnID := util.NewULID()

	if err := h.dispatcher.RunChatTurn(ctx, domain.ChatTurnRequest{
		UserID:            userID,
		WorkspaceID:       workspaceID,
		ChatID:            chatID,
		TurnID:            turnID,
		PaperID:           msg.GetPaperId(),
		History:           history,
		Candidates:        candidates,
		PinnedContextIDs:  msg.GetPinnedContextIds(),
		AutoSelectContext: msg.GetAutoSelectContext(),
	}); err != nil {
		h.logger.Error("chat.dispatch_failed",
			"error", err.Error(),
			"workspace_id", workspaceID,
			"chat_id", chatID,
			"turn_id", turnID,
		)
		return nil, toError(err)
	}

	return connect.NewResponse(&appv1.PostChatTurnResponse{
		ChatId: chatID,
		TurnId: turnID,
	}), nil
}

// normalizeHistory drops the oldest messages past the cap and truncates
// oversized ones. The newest entries are the user's current question and its
// immediate context, so the tail is what must survive.
func normalizeHistory(in []*appv1.ChatMessage) ([]domain.ChatMessage, error) {
	if len(in) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("history must contain at least the user message"))
	}
	if len(in) > maxChatHistoryMessages {
		in = in[len(in)-maxChatHistoryMessages:]
	}
	out := make([]domain.ChatMessage, 0, len(in))
	for _, m := range in {
		out = append(out, domain.ChatMessage{
			Role: chatRoleFromProto(m.GetRole()),
			Text: truncateBytes(m.GetText(), maxChatHistoryTextBytes),
		})
	}
	if out[len(out)-1].Role != domain.ChatRoleUser {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the last history entry must be the user message"))
	}
	return out, nil
}

func normalizeCandidates(in []*appv1.ContextPaper) ([]domain.ContextPaper, error) {
	if len(in) > maxChatCandidates {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("at most %d candidates are allowed, got %d", maxChatCandidates, len(in)))
	}
	out := make([]domain.ContextPaper, 0, len(in))
	total := 0
	for _, c := range in {
		content := truncateBytes(c.GetContent(), maxChatCandidateBytes)
		total += len(content)
		if total > maxChatCandidatesTotal {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("candidate content exceeds %d bytes in total", maxChatCandidatesTotal))
		}
		out = append(out, domain.ContextPaper{
			PaperID:     c.GetPaperId(),
			Title:       c.GetTitle(),
			Description: c.GetDescription(),
			Content:     content,
		})
	}
	return out, nil
}

// truncateBytes cuts at a UTF-8 boundary so a trimmed string never ends in a
// partial rune, which would corrupt the prompt.
func truncateBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

func chatRoleFromProto(r appv1.ChatRole) domain.ChatRole {
	switch r {
	case appv1.ChatRole_CHAT_ROLE_ASSISTANT:
		return domain.ChatRoleAssistant
	default:
		return domain.ChatRoleUser
	}
}
