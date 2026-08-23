package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"

	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// WorkspaceChatUsecase is the application API required by WorkspaceChatHandler.
type WorkspaceChatUsecase interface {
	ListConversations(ctx context.Context, workspaceID, userID string) ([]*domain.ChatConversation, error)
	ListMessages(ctx context.Context, workspaceID, conversationID, userID string) ([]*domain.ChatMessage, error)
	SendMessage(ctx context.Context, workspaceID, conversationID, text, userID string) (*domain.ChatMessage, *domain.ChatMessage, error)
}

type WorkspaceChatHandler struct {
	service WorkspaceChatUsecase
}

func NewWorkspaceChatHandler(service WorkspaceChatUsecase) *WorkspaceChatHandler {
	return &WorkspaceChatHandler{service: service}
}

func (h *WorkspaceChatHandler) ListWorkspaceChatConversations(
	ctx context.Context,
	req *connect.Request[appv1.ListWorkspaceChatConversationsRequest],
) (*connect.Response[appv1.ListWorkspaceChatConversationsResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, err := h.service.ListConversations(ctx, req.Msg.GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	out := make([]*appv1.WorkspaceChatConversation, 0, len(convs))
	for _, conv := range convs {
		out = append(out, toProtoChatConversation(conv))
	}
	return connect.NewResponse(&appv1.ListWorkspaceChatConversationsResponse{Conversations: out}), nil
}

func (h *WorkspaceChatHandler) ListWorkspaceChatMessages(
	ctx context.Context,
	req *connect.Request[appv1.ListWorkspaceChatMessagesRequest],
) (*connect.Response[appv1.ListWorkspaceChatMessagesResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetConversationId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and conversation_id are required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := h.service.ListMessages(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetConversationId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	out := make([]*appv1.WorkspaceChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, toProtoChatMessage(msg))
	}
	return connect.NewResponse(&appv1.ListWorkspaceChatMessagesResponse{Messages: out}), nil
}

func (h *WorkspaceChatHandler) SendWorkspaceChatMessage(
	ctx context.Context,
	req *connect.Request[appv1.SendWorkspaceChatMessageRequest],
) (*connect.Response[appv1.SendWorkspaceChatMessageResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	userMsg, assistantMsg, err := h.service.SendMessage(
		ctx,
		req.Msg.GetWorkspaceId(),
		req.Msg.GetConversationId(),
		req.Msg.GetText(),
		userID,
	)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.SendWorkspaceChatMessageResponse{
		ConversationId:   userMsg.ConversationID,
		UserMessage:      toProtoChatMessage(userMsg),
		AssistantMessage: toProtoChatMessage(assistantMsg),
	}), nil
}

func toProtoChatConversation(conv *domain.ChatConversation) *appv1.WorkspaceChatConversation {
	return &appv1.WorkspaceChatConversation{
		ConversationId: conv.ConversationID,
		WorkspaceId:    conv.WorkspaceID,
		CreatedBy:      conv.CreatedBy,
		Title:          conv.Title,
		CreatedAt:      conv.CreatedAt,
		UpdatedAt:      conv.UpdatedAt,
	}
}

func toProtoChatMessage(msg *domain.ChatMessage) *appv1.WorkspaceChatMessage {
	if msg == nil {
		return nil
	}
	sources := make([]*appv1.WorkspaceChatSource, 0, len(msg.Sources))
	for _, src := range msg.Sources {
		sources = append(sources, &appv1.WorkspaceChatSource{
			DocumentId: src.DocumentID,
			ChunkId:    src.ChunkID,
			ItemId:     src.ItemID,
			Label:      src.Label,
		})
	}
	return &appv1.WorkspaceChatMessage{
		MessageId:      msg.MessageID,
		ConversationId: msg.ConversationID,
		Role:           msg.Role,
		Content:        msg.Content,
		Sources:        sources,
		ModelSelection: msg.ModelSelection,
		CreatedAt:      msg.CreatedAt,
		Status:         toProtoChatMessageStatus(msg.Status),
		ErrorCode:      msg.ErrorCode,
	}
}

func toProtoChatMessageStatus(status string) appv1.WorkspaceChatMessageStatus {
	switch status {
	case domain.ChatMessageStatusComplete:
		return appv1.WorkspaceChatMessageStatus_WORKSPACE_CHAT_MESSAGE_STATUS_COMPLETE
	case domain.ChatMessageStatusFailed:
		return appv1.WorkspaceChatMessageStatus_WORKSPACE_CHAT_MESSAGE_STATUS_FAILED
	case domain.ChatMessageStatusCancelled:
		return appv1.WorkspaceChatMessageStatus_WORKSPACE_CHAT_MESSAGE_STATUS_CANCELLED
	default:
		return appv1.WorkspaceChatMessageStatus_WORKSPACE_CHAT_MESSAGE_STATUS_UNSPECIFIED
	}
}
