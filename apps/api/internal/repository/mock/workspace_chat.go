package mock

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// Workspace chat の in-memory 実装。postgres.Store と同じ可視性ルールを守る:
// 会話は workspace 単位で見え、created_by による私的な絞り込みはしない。

func (s *Store) CreateChatConversation(ctx context.Context, workspaceID, createdBy, title string) (*domain.ChatConversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatSeq++
	now := s.chatNow()
	conv := &domain.ChatConversation{
		ConversationID: fmt.Sprintf("conv-%d", s.chatSeq),
		WorkspaceID:    workspaceID,
		CreatedBy:      createdBy,
		Title:          title,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.chatConversations[conv.ConversationID] = conv
	return copyChatConversation(conv), nil
}

func (s *Store) GetChatConversation(ctx context.Context, conversationID string) (*domain.ChatConversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.chatConversations[conversationID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return copyChatConversation(conv), nil
}

func (s *Store) ListChatConversations(ctx context.Context, workspaceID string, limit int) ([]*domain.ChatConversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.ChatConversation, 0)
	for _, conv := range s.chatConversations {
		if conv.WorkspaceID == workspaceID {
			out = append(out, copyChatConversation(conv))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].ConversationID > out[j].ConversationID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) TouchChatConversation(ctx context.Context, conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conv, ok := s.chatConversations[conversationID]
	if !ok {
		return domain.ErrNotFound
	}
	conv.UpdatedAt = s.chatNow()
	return nil
}

func (s *Store) CreateChatMessage(ctx context.Context, msg *domain.ChatMessage, retrievalSnapshot []byte) (*domain.ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chatConversations[msg.ConversationID]; !ok {
		// 実 DB では FK 違反になるケース。
		return nil, domain.ErrNotFound
	}
	s.chatSeq++
	stored := copyChatMessage(msg)
	if stored.MessageID == "" {
		stored.MessageID = fmt.Sprintf("msg-%d", s.chatSeq)
	}
	if stored.Status == "" {
		stored.Status = domain.ChatMessageStatusComplete
	}
	stored.CreatedAt = s.chatNow()
	s.chatMessages[msg.ConversationID] = append(s.chatMessages[msg.ConversationID], stored)
	return copyChatMessage(stored), nil
}

func (s *Store) ListChatMessages(ctx context.Context, conversationID string, limit int) ([]*domain.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.chatMessages[conversationID]
	out := make([]*domain.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, copyChatMessage(msg))
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) ListRecentChatMessages(ctx context.Context, conversationID string, limit int) ([]*domain.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.chatMessages[conversationID]
	// complete のみ (失敗した turn を履歴として再生しない)。
	complete := make([]*domain.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Status == domain.ChatMessageStatusComplete {
			complete = append(complete, msg)
		}
	}
	if limit > 0 && len(complete) > limit {
		complete = complete[len(complete)-limit:]
	}
	out := make([]*domain.ChatMessage, 0, len(complete))
	for _, msg := range complete {
		copied := copyChatMessage(msg)
		copied.Sources = nil // 履歴には citation を載せない (postgres 実装と同じ)
		out = append(out, copied)
	}
	return out, nil
}

// SearchChatSourceCandidates は postgres 実装と同じ順序規則を再現する:
// 一致した term 数の降順、同数なら (document_id, chunk_id) 昇順。一致ゼロでも
// 候補は落とさない。
func (s *Store) SearchChatSourceCandidates(ctx context.Context, workspaceID string, terms []string, limit int) ([]domain.ChatSourceCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docIDs := s.succeededDocumentIDsLocked(workspaceID)
	sort.Strings(docIDs)

	type scored struct {
		candidate domain.ChatSourceCandidate
		matches   int
	}

	rows := make([]scored, 0)
	for _, docID := range docIDs {
		doc := s.documents[docID]
		chunks := append([]*domain.DocumentChunk(nil), s.chunks[docID]...)
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].ChunkID < chunks[j].ChunkID })
		for _, chunk := range chunks {
			haystack := strings.ToLower(chunk.Text + " " + chunk.Heading)
			matches := 0
			for _, term := range terms {
				if term != "" && strings.Contains(haystack, strings.ToLower(term)) {
					matches++
				}
			}
			filename := ""
			if doc != nil {
				filename = doc.Filename
			}
			rows = append(rows, scored{
				candidate: domain.ChatSourceCandidate{
					ChunkID:    chunk.ChunkID,
					DocumentID: chunk.DocumentID,
					Filename:   filename,
					Heading:    chunk.Heading,
					Text:       chunk.Text,
					SourcePage: chunk.SourcePage,
				},
				matches: matches,
			})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].matches > rows[j].matches })

	out := make([]domain.ChatSourceCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.candidate)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// SearchChatSourceItems は postgres 実装と同じ順位付けを再現する。
func (s *Store) SearchChatSourceItems(ctx context.Context, workspaceID string, terms []string, limit int) ([]domain.ChatSourceCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*domain.Item, 0)
	for _, item := range s.items[workspaceID] {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ItemID < items[j].ItemID })

	type scored struct {
		candidate domain.ChatSourceCandidate
		matches   int
	}
	rows := make([]scored, 0, len(items))
	for _, item := range items {
		haystack := strings.ToLower(item.Title + " " + item.Description + " " + item.Content)
		matches := 0
		for _, term := range terms {
			if term != "" && strings.Contains(haystack, strings.ToLower(term)) {
				matches++
			}
		}
		text := item.Description
		if text == "" {
			text = item.Content
		}
		rows = append(rows, scored{
			candidate: domain.ChatSourceCandidate{
				ItemID:  item.ItemID,
				Heading: item.Title,
				Text:    text,
			},
			matches: matches,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].matches > rows[j].matches })

	out := make([]domain.ChatSourceCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.candidate)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Store) CountChatSourceItems(ctx context.Context, workspaceID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items[workspaceID]), nil
}

func (s *Store) CountChatSourceDocuments(ctx context.Context, workspaceID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.succeededDocumentIDsLocked(workspaceID)), nil
}

// succeededDocumentIDsLocked は処理が succeeded した document の id を返す。
// 呼び出し側が s.mu を保持していること。
func (s *Store) succeededDocumentIDsLocked(workspaceID string) []string {
	succeeded := make(map[string]bool)
	for _, job := range s.jobs {
		if job.WorkspaceID == workspaceID && job.Status == appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED {
			succeeded[job.DocumentID] = true
		}
	}
	out := make([]string, 0, len(succeeded))
	for docID := range succeeded {
		if doc, ok := s.documents[docID]; ok && doc.WorkspaceID == workspaceID {
			out = append(out, docID)
		}
	}
	return out
}

// chatNow は単調増加するタイムスタンプを返す。同一テスト内で連続して作られた
// 会話 / メッセージが同じ秒に落ちて順序が壊れるのを避ける。
// 呼び出し側が s.mu を保持していること。
func (s *Store) chatNow() string {
	s.chatClock++
	return time.Unix(0, 0).UTC().Add(time.Duration(s.chatClock) * time.Second).Format(time.RFC3339)
}

func copyChatConversation(conv *domain.ChatConversation) *domain.ChatConversation {
	copied := *conv
	return &copied
}

func copyChatMessage(msg *domain.ChatMessage) *domain.ChatMessage {
	copied := *msg
	if msg.Sources != nil {
		copied.Sources = append([]domain.ChatSource(nil), msg.Sources...)
	}
	return &copied
}
