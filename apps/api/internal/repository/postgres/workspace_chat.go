package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
)

// CreateChatConversation inserts a new conversation thread record in PostgreSQL.
func (s *Store) CreateChatConversation(ctx context.Context, workspaceID, createdBy, title string) (*domain.ChatConversation, error) {
	row, err := s.q().CreateWorkspaceChatConversation(ctx, sqlcgen.CreateWorkspaceChatConversationParams{
		ConversationID: newID(),
		WorkspaceID:    workspaceID,
		CreatedBy:      createdBy,
		Title:          title,
		CreatedAt:      nowTime(),
	})
	if err != nil {
		return nil, err
	}
	return toChatConversation(row), nil
}

// GetChatConversation retrieves a conversation thread record by its unique ID.
func (s *Store) GetChatConversation(ctx context.Context, conversationID string) (*domain.ChatConversation, error) {
	row, err := s.q().GetWorkspaceChatConversation(ctx, conversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toChatConversation(row), nil
}

// ListChatConversations queries conversation threads in a workspace ordered by updated_at descending.
func (s *Store) ListChatConversations(ctx context.Context, workspaceID string, limit int) ([]*domain.ChatConversation, error) {
	rows, err := s.q().ListWorkspaceChatConversations(ctx, sqlcgen.ListWorkspaceChatConversationsParams{
		WorkspaceID: workspaceID,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.ChatConversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChatConversation(row))
	}
	return out, nil
}

// TouchChatConversation updates the updated_at timestamp of a conversation thread.
func (s *Store) TouchChatConversation(ctx context.Context, conversationID string) error {
	return s.q().TouchWorkspaceChatConversation(ctx, sqlcgen.TouchWorkspaceChatConversationParams{
		ConversationID: conversationID,
		UpdatedAt:      nowTime(),
	})
}

// CreateChatMessage inserts a message row and associated citation sources into PostgreSQL.
// Callers should wrap within Transactor.WithTx for atomic commit.
func (s *Store) CreateChatMessage(ctx context.Context, msg *domain.ChatMessage, retrievalSnapshot []byte) (*domain.ChatMessage, error) {
	if len(retrievalSnapshot) == 0 {
		retrievalSnapshot = []byte("{}")
	}
	messageID := msg.MessageID
	if messageID == "" {
		messageID = newID()
	}
	status := msg.Status
	if status == "" {
		status = domain.ChatMessageStatusComplete
	}
	row, err := s.q().CreateWorkspaceChatMessage(ctx, sqlcgen.CreateWorkspaceChatMessageParams{
		MessageID:             messageID,
		ConversationID:        msg.ConversationID,
		Role:                  msg.Role,
		Content:               msg.Content,
		ModelSelection:        msg.ModelSelection,
		Status:                status,
		ErrorCode:             msg.ErrorCode,
		Grounded:              msg.Grounded,
		RetrievalSnapshotJson: retrievalSnapshot,
		CreatedAt:             nowTime(),
	})
	if err != nil {
		return nil, err
	}

	for i, src := range msg.Sources {
		if err := s.q().CreateWorkspaceChatMessageSource(ctx, sqlcgen.CreateWorkspaceChatMessageSourceParams{
			MessageID: messageID,
			Ordinal:   int32(i),
			// Tree item sources lack an associated document; set NULL to avoid FK error.
			DocumentID: sql.NullString{String: src.DocumentID, Valid: src.DocumentID != ""},
			ChunkID:    src.ChunkID,
			ItemID:     src.ItemID,
			Label:      src.Label,
		}); err != nil {
			return nil, err
		}
	}

	return &domain.ChatMessage{
		MessageID:      row.MessageID,
		ConversationID: row.ConversationID,
		Role:           row.Role,
		Content:        row.Content,
		Sources:        msg.Sources,
		ModelSelection: row.ModelSelection,
		Status:         row.Status,
		ErrorCode:      row.ErrorCode,
		Grounded:       row.Grounded,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
	}, nil
}

// ListChatMessages returns messages for the given conversation ID with associated sources.
func (s *Store) ListChatMessages(ctx context.Context, conversationID string, limit int) ([]*domain.ChatMessage, error) {
	rows, err := s.q().ListWorkspaceChatMessages(ctx, sqlcgen.ListWorkspaceChatMessagesParams{
		ConversationID: conversationID,
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, err
	}
	sourceRows, err := s.q().ListWorkspaceChatMessageSources(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	// Group sources by message ID.
	byMessage := make(map[string][]domain.ChatSource, len(sourceRows))
	for _, src := range sourceRows {
		byMessage[src.MessageID] = append(byMessage[src.MessageID], domain.ChatSource{
			DocumentID: src.DocumentID.String,
			ChunkID:    src.ChunkID,
			ItemID:     src.ItemID,
			Label:      src.Label,
		})
	}

	out := make([]*domain.ChatMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.ChatMessage{
			MessageID:      row.MessageID,
			ConversationID: row.ConversationID,
			Role:           row.Role,
			Content:        row.Content,
			Sources:        byMessage[row.MessageID],
			ModelSelection: row.ModelSelection,
			Status:         row.Status,
			ErrorCode:      row.ErrorCode,
			Grounded:       row.Grounded,
			CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ListRecentChatMessages retrieves the latest limit messages reversed into chronological (oldest-first) order for prompt history.
func (s *Store) ListRecentChatMessages(ctx context.Context, conversationID string, limit int) ([]*domain.ChatMessage, error) {
	rows, err := s.q().ListRecentWorkspaceChatMessages(ctx, sqlcgen.ListRecentWorkspaceChatMessagesParams{
		ConversationID: conversationID,
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.ChatMessage, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		out = append(out, &domain.ChatMessage{
			MessageID:      row.MessageID,
			ConversationID: row.ConversationID,
			Role:           row.Role,
			Content:        row.Content,
			ModelSelection: row.ModelSelection,
			Status:         row.Status,
			ErrorCode:      row.ErrorCode,
			Grounded:       row.Grounded,
			CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// SearchChatSourceCandidates retrieves candidate chunks from succeeded documents matching search terms lexical queries.
func (s *Store) SearchChatSourceCandidates(ctx context.Context, workspaceID string, terms []string, limit int) ([]domain.ChatSourceCandidate, error) {
	rows, err := s.q().ListWorkspaceChatSourceChunksLexical(ctx, sqlcgen.ListWorkspaceChatSourceChunksLexicalParams{
		WorkspaceID: workspaceID,
		Terms:       strings.Join(terms, domain.ChatSearchTermSeparator),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChatSourceCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ChatSourceCandidate{
			ChunkID:    row.ChunkID,
			DocumentID: row.DocumentID,
			Filename:   row.Filename,
			SubPath:    row.SubPath.String,
			Heading:    row.Heading,
			Text:       row.Text,
			SourcePage: int(row.SourcePage.Int32),
		})
	}
	return out, nil
}

// SearchChatSourceItems retrieves candidate knowledge tree items in the workspace matching search terms.
func (s *Store) SearchChatSourceItems(ctx context.Context, workspaceID string, terms []string, limit int) ([]domain.ChatSourceCandidate, error) {
	rows, err := s.q().ListWorkspaceChatSourceItems(ctx, sqlcgen.ListWorkspaceChatSourceItemsParams{
		WorkspaceID: workspaceID,
		Terms:       strings.Join(terms, domain.ChatSearchTermSeparator),
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChatSourceCandidate, 0, len(rows))
	for _, row := range rows {
		// Prefer description over HTML content for prompts.
		text := row.Description
		if text == "" {
			text = row.Content
		}
		out = append(out, domain.ChatSourceCandidate{
			ItemID:  row.ID,
			Heading: row.Title,
			Text:    text,
		})
	}
	return out, nil
}

// CountChatSourceItems returns the number of knowledge tree items in the workspace.
func (s *Store) CountChatSourceItems(ctx context.Context, workspaceID string) (int, error) {
	count, err := s.q().CountWorkspaceChatSourceItems(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// CountChatSourceDocuments returns the number of successfully processed documents available for citation.
func (s *Store) CountChatSourceDocuments(ctx context.Context, workspaceID string) (int, error) {
	count, err := s.q().CountWorkspaceChatSourceDocuments(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// toChatConversation converts a sqlc WorkspaceChatConversation row into a domain ChatConversation entity.
func toChatConversation(row sqlcgen.WorkspaceChatConversation) *domain.ChatConversation {
	return &domain.ChatConversation{
		ConversationID: row.ConversationID,
		WorkspaceID:    row.WorkspaceID,
		CreatedBy:      row.CreatedBy,
		Title:          row.Title,
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      row.UpdatedAt.Format(time.RFC3339),
	}
}
