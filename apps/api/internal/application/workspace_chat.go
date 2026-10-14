package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// Workspace chat の application service。
// 設計は docs/architecture/workspace-chat-design.md。
//
// この service は Worker / Orchestrator / DynamicToolRepository / mutation 系
// repository に依存しない。chat が write 可能な agent に育つのを防ぐ主な
// 構造的ガードがこれなので、依存を足すときは §7 を読むこと。

// Initial limits and bounds for workspace chat retrieval and generation.
const (
	chatMaxRetrievedChunks  = 12
	chatMaxRetrievedItems   = 8
	chatMaxContextChars     = 24000
	chatMaxHistoryMessages  = 12
	chatGenerationTimeout   = 60 * time.Second
	chatMaxConversations    = 50
	chatMaxMessagesPerFetch = 200
)

// chatModelUnknown is the fallback model name when an answerer does not supply a specific identifier.
const chatModelUnknown = "unknown"

// ChatAnswerRequest defines the prompt inputs for grounded question answering.
type ChatAnswerRequest struct {
	Question   string
	History    []*domain.ChatMessage
	Candidates []domain.ChatSourceCandidate
}

// ChatAnswer represents the generative model's answer text and claimed source citations.
type ChatAnswer struct {
	Text      string
	SourceIDs []string
}

// ChatAnswerer abstracts the generation of grounded answers from retrieved context.
type ChatAnswerer interface {
	// Answer generates a grounded response for the given request.
	Answer(ctx context.Context, req ChatAnswerRequest) (ChatAnswer, error)
	// ModelID returns the identifier of the model actually used for answer generation.
	ModelID() string
}

// WorkspaceChatService manages conversations, retrieval-augmented generation, and citation validation.
type WorkspaceChatService struct {
	repo       repository.WorkspaceChatRepository
	workspaces repository.WorkspaceRepository
	answerer   ChatAnswerer
	logger     *slog.Logger
}

// WorkspaceChatServiceDeps defines the dependencies required to instantiate WorkspaceChatService.
type WorkspaceChatServiceDeps struct {
	Repo       repository.WorkspaceChatRepository
	Workspaces repository.WorkspaceRepository
	Answerer   ChatAnswerer
	Logger     *slog.Logger
}

// NewWorkspaceChatService creates a new WorkspaceChatService instance.
func NewWorkspaceChatService(deps WorkspaceChatServiceDeps) *WorkspaceChatService {
	return &WorkspaceChatService{
		repo:       deps.Repo,
		workspaces: deps.Workspaces,
		answerer:   deps.Answerer,
		logger:     deps.Logger,
	}
}

// authorizeRead verifies that the user has read access to the workspace.
func (s *WorkspaceChatService) authorizeRead(ctx context.Context, workspaceID, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// ListConversations returns recent chat conversations in a workspace and indicates whether sources are available.
func (s *WorkspaceChatService) ListConversations(ctx context.Context, workspaceID, userID string) ([]*domain.ChatConversation, bool, error) {
	if err := s.authorizeRead(ctx, workspaceID, userID); err != nil {
		return nil, false, err
	}
	conversations, err := s.repo.ListChatConversations(ctx, workspaceID, chatMaxConversations)
	if err != nil {
		return nil, false, err
	}
	docCount, err := s.repo.CountChatSourceDocuments(ctx, workspaceID)
	if err != nil {
		return nil, false, err
	}
	itemCount, err := s.repo.CountChatSourceItems(ctx, workspaceID)
	if err != nil {
		return nil, false, err
	}
	return conversations, docCount > 0 || itemCount > 0, nil
}

// ListMessages retrieves the message history of a conversation up to chatMaxMessagesPerFetch.
func (s *WorkspaceChatService) ListMessages(ctx context.Context, workspaceID, conversationID, userID string) ([]*domain.ChatMessage, error) {
	if err := s.authorizeRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if _, err := s.resolveConversation(ctx, workspaceID, conversationID); err != nil {
		return nil, err
	}
	return s.repo.ListChatMessages(ctx, conversationID, chatMaxMessagesPerFetch)
}

// resolveConversation verifies that the conversation belongs to the specified workspace.
func (s *WorkspaceChatService) resolveConversation(ctx context.Context, workspaceID, conversationID string) (*domain.ChatConversation, error) {
	conv, err := s.repo.GetChatConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv.WorkspaceID != workspaceID {
		return nil, domain.ErrForbidden
	}
	return conv, nil
}

// SendMessage persists the user's question, generates a grounded answer via the configured model,
// records citations, and touches the conversation's updated timestamp.
func (s *WorkspaceChatService) SendMessage(ctx context.Context, workspaceID, conversationID, text, userID string) (*domain.ChatMessage, *domain.ChatMessage, error) {
	if err := domain.ValidateChatMessageText(text); err != nil {
		return nil, nil, err
	}
	if err := s.authorizeRead(ctx, workspaceID, userID); err != nil {
		return nil, nil, err
	}

	// Always generate an answer even if no documents exist in the workspace;
	// zero sources results in an ungrounded general knowledge answer (Grounded=false).
	conv, err := s.ensureConversation(ctx, workspaceID, conversationID, text, userID)
	if err != nil {
		return nil, nil, err
	}

	history, err := s.repo.ListRecentChatMessages(ctx, conv.ConversationID, chatMaxHistoryMessages)
	if err != nil {
		return nil, nil, err
	}

	userMsg, err := s.repo.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: conv.ConversationID,
		Role:           domain.ChatRoleUser,
		Content:        text,
		Status:         domain.ChatMessageStatusComplete,
	}, nil)
	if err != nil {
		return nil, nil, err
	}

	assistantMsg, err := s.generateAnswer(ctx, workspaceID, conv.ConversationID, text, history)
	if err != nil {
		return nil, nil, err
	}

	if err := s.repo.TouchChatConversation(ctx, conv.ConversationID); err != nil {
		return nil, nil, err
	}
	return userMsg, assistantMsg, nil
}

// generateAnswer performs retrieval, prompts the model for an answer, validates citations against retrieved candidates, and persists the assistant message.
func (s *WorkspaceChatService) generateAnswer(
	ctx context.Context,
	workspaceID, conversationID, question string,
	history []*domain.ChatMessage,
) (*domain.ChatMessage, error) {
	terms := domain.ChatSearchTerms(question)

	candidates, err := s.repo.SearchChatSourceCandidates(ctx, workspaceID, terms, chatMaxRetrievedChunks)
	if err != nil {
		return nil, err
	}
	// Knowledge tree items can also serve as source candidates if documents are absent.
	itemCandidates, err := s.repo.SearchChatSourceItems(ctx, workspaceID, terms, chatMaxRetrievedItems)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, itemCandidates...)
	candidates = boundContext(candidates, chatMaxContextChars)

	genCtx, cancel := context.WithTimeout(ctx, chatGenerationTimeout)
	defer cancel()

	model := s.answerer.ModelID()
	if model == "" {
		model = chatModelUnknown
	}

	answer, genErr := s.answerer.Answer(genCtx, ChatAnswerRequest{
		Question:   question,
		History:    history,
		Candidates: candidates,
	})
	if genErr != nil {
		if s.logger != nil {
			s.logger.Error("chat.generation_failed",
				"workspace_id", workspaceID,
				"conversation_id", conversationID,
				"error", genErr.Error(),
			)
		}
		// Record failed generation as an assistant message so conversation history is preserved.
		failed, err := s.repo.CreateChatMessage(ctx, &domain.ChatMessage{
			ConversationID: conversationID,
			Role:           domain.ChatRoleAssistant,
			Content:        "",
			ModelSelection: model,
			Status:         domain.ChatMessageStatusFailed,
			ErrorCode:      domain.ChatErrGenerationFailed,
		}, snapshotJSON(candidates, model))
		if err != nil {
			return nil, err
		}
		return failed, nil
	}

	sources := validateCitations(answer.SourceIDs, candidates)

	return s.repo.CreateChatMessage(ctx, &domain.ChatMessage{
		ConversationID: conversationID,
		Role:           domain.ChatRoleAssistant,
		Content:        answer.Text,
		Sources:        sources,
		ModelSelection: model,
		Status:         domain.ChatMessageStatusComplete,
		// If no sources matched, the answer was not grounded in workspace materials.
		Grounded: len(sources) > 0,
	}, snapshotJSON(candidates, model))
}

// ensureConversation returns an existing conversation or creates a new one using the initial message text as title.
func (s *WorkspaceChatService) ensureConversation(ctx context.Context, workspaceID, conversationID, firstText, userID string) (*domain.ChatConversation, error) {
	if conversationID != "" {
		return s.resolveConversation(ctx, workspaceID, conversationID)
	}
	return s.repo.CreateChatConversation(ctx, workspaceID, userID, conversationTitle(firstText))
}

// validateCitations filters claimed citation source IDs against candidates retrieved in the context window.
func validateCitations(claimed []string, candidates []domain.ChatSourceCandidate) []domain.ChatSource {
	bySourceID := make(map[string]domain.ChatSourceCandidate, len(candidates))
	for _, c := range candidates {
		bySourceID[c.SourceID()] = c
	}

	seen := make(map[string]bool, len(claimed))
	sources := make([]domain.ChatSource, 0, len(claimed))
	for _, sourceID := range claimed {
		candidate, ok := bySourceID[sourceID]
		if !ok || seen[sourceID] {
			continue
		}
		seen[sourceID] = true
		sources = append(sources, domain.ChatSource{
			DocumentID: candidate.DocumentID,
			ChunkID:    candidate.ChunkID,
			ItemID:     candidate.ItemID,
			Label:      candidate.Label(),
		})
	}
	return sources
}

// boundContext trims the candidate slice to prevent prompt tokens from exceeding character limits.
func boundContext(candidates []domain.ChatSourceCandidate, maxChars int) []domain.ChatSourceCandidate {
	total := 0
	for i, c := range candidates {
		total += len(c.Text)
		if total > maxChars {
			return candidates[:i]
		}
	}
	return candidates
}

// conversationTitle produces a default truncated conversation title from the first query string.
func conversationTitle(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) > 40 {
		return string(runes[:40]) + "…"
	}
	return trimmed
}

// snapshotJSON encodes source candidate identifiers and retrieval metadata into a reproducible JSON snapshot.
func snapshotJSON(candidates []domain.ChatSourceCandidate, model string) []byte {
	sourceIDs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		sourceIDs = append(sourceIDs, c.SourceID())
	}
	snapshot := struct {
		Strategy            string   `json:"strategy"`
		CandidateSourceIDs  []string `json:"candidate_source_ids"`
		EffectiveModel      string   `json:"effective_model"`
		CandidateCountLimit int      `json:"candidate_count_limit"`
	}{
		Strategy:            "lexical_v2_chunks_and_items",
		CandidateSourceIDs:  sourceIDs,
		EffectiveModel:      model,
		CandidateCountLimit: chatMaxRetrievedChunks,
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return []byte("{}")
	}
	return encoded
}
