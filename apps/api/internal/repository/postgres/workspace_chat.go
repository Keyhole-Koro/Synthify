package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
)

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

func (s *Store) TouchChatConversation(ctx context.Context, conversationID string) error {
	return s.q().TouchWorkspaceChatConversation(ctx, sqlcgen.TouchWorkspaceChatConversationParams{
		ConversationID: conversationID,
		UpdatedAt:      nowTime(),
	})
}

// CreateChatMessage は message 行と、検証済みの sources を書き込む。
// 呼び出し側が Transactor.WithTx で包めば message + sources が atomic になる。
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
		RetrievalSnapshotJson: retrievalSnapshot,
		CreatedAt:             nowTime(),
	})
	if err != nil {
		return nil, err
	}

	for i, src := range msg.Sources {
		if err := s.q().CreateWorkspaceChatMessageSource(ctx, sqlcgen.CreateWorkspaceChatMessageSourceParams{
			MessageID:  messageID,
			Ordinal:    int32(i),
			DocumentID: src.DocumentID,
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
		CreatedAt:      row.CreatedAt.Format(time.RFC3339),
	}, nil
}

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
	// sources は (message_id, ordinal) 順で返るので、message ごとにまとめ直す。
	byMessage := make(map[string][]domain.ChatSource, len(sourceRows))
	for _, src := range sourceRows {
		byMessage[src.MessageID] = append(byMessage[src.MessageID], domain.ChatSource{
			DocumentID: src.DocumentID,
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
			CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ListRecentChatMessages は最新 limit 件を取得し、時系列 (古い順) に直して返す。
// SQL 側は新しい順に取るので、ここで反転させる。
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
			CreatedAt:      row.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// SearchChatSourceCandidates は引用候補を返す。terms に一致した数が多い chunk
// を先に返し、どれにも一致しなくても候補は返す (回答不能にしないため)。
//
// 現状は lexical 経路のみを使う。vector 経路 (SearchWorkspaceChatSourceChunks)
// は pgvector が必要で、ローカル開発スタックの CockroachDB v24.2 には
// vector_cosine_distance が無いため呼ばない。API 側に embedding client が
// 無いことも併せて、embedding が実際に使えるようになった時点で切り替える。
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

func (s *Store) CountChatSourceDocuments(ctx context.Context, workspaceID string) (int, error) {
	count, err := s.q().CountWorkspaceChatSourceDocuments(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

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
