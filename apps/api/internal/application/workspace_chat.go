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

// 設計 §5 の初期 bound。
const (
	chatMaxRetrievedChunks  = 12
	chatMaxRetrievedItems   = 8
	chatMaxContextChars     = 24000
	chatMaxHistoryMessages  = 12
	chatGenerationTimeout   = 60 * time.Second
	chatMaxConversations    = 50
	chatMaxMessagesPerFetch = 200
)

// chatModelUnknown は answerer が自身のモデル ID を返さなかった場合の値。
const chatModelUnknown = "unknown"

// ChatAnswerRequest は grounded answer 1 回分の入力。
type ChatAnswerRequest struct {
	Question   string
	History    []*domain.ChatMessage
	Candidates []domain.ChatSourceCandidate
}

// ChatAnswer は model の出力。SourceIDs は「model が引用したと主張した」
// 値であって、検証前の生データである。service 側で候補集合に対して検証する。
// chunk id と item id の両方を取りうる。
type ChatAnswer struct {
	Text           string
	SourceIDs []string
}

// ChatAnswerer は grounded answer を生成する port。
//
// 意図的に狭い: API は apps/worker/pkg/worker の ADK / LLM ツリーを取り込まない
// (infrastructure/worker/dispatcher.go と同じ方針)。実装は infrastructure 側に置く。
type ChatAnswerer interface {
	Answer(ctx context.Context, req ChatAnswerRequest) (ChatAnswer, error)
	// ModelID は実際に回答を生成した主体の識別子を返す。message 行と
	// retrieval snapshot にそのまま記録するので、"gemini" のような固定値では
	// なく、実装が本当に使ったものを返すこと。後からモデルを増やしたときに
	// 「どれが答えたか」を遡れることがこのフィールドの唯一の存在理由なので、
	// 嘘をつくと監査価値が消える。
	ModelID() string
}

type WorkspaceChatService struct {
	repo       repository.WorkspaceChatRepository
	workspaces repository.WorkspaceRepository
	answerer   ChatAnswerer
	logger     *slog.Logger
}

type WorkspaceChatServiceDeps struct {
	Repo       repository.WorkspaceChatRepository
	Workspaces repository.WorkspaceRepository
	Answerer   ChatAnswerer
	Logger     *slog.Logger
}

func NewWorkspaceChatService(deps WorkspaceChatServiceDeps) *WorkspaceChatService {
	return &WorkspaceChatService{
		repo:       deps.Repo,
		workspaces: deps.Workspaces,
		answerer:   deps.Answerer,
		logger:     deps.Logger,
	}
}

// authorizeRead は workspace への read 権限を確認する。会話・メッセージ・chunk・
// item label のいずれかを読む前に必ず通す (設計 §7)。
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

// ListConversations は会話一覧と、この workspace に出典になりうるものが
// あるかを返す。
//
// このフラグは入力欄を塞ぐためのものではない (出典が無くても質問はできる)。
// 「出典付きで答えられる見込みがあるか」を UI に伝えるだけで、判定条件は
// retrieval と一致していなければならないのでサーバーが持つ。
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

func (s *WorkspaceChatService) ListMessages(ctx context.Context, workspaceID, conversationID, userID string) ([]*domain.ChatMessage, error) {
	if err := s.authorizeRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if _, err := s.resolveConversation(ctx, workspaceID, conversationID); err != nil {
		return nil, err
	}
	return s.repo.ListChatMessages(ctx, conversationID, chatMaxMessagesPerFetch)
}

// resolveConversation は conversation が指定 workspace のものであることを確認する。
// 別 workspace の conversation_id を渡された場合は Forbidden を返し、存在の
// 有無を漏らさない。
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

// SendMessage はユーザーの質問を永続化し、grounded answer を生成して返す。
//
// 順序が重要: user メッセージは生成を始める前に必ず永続化する。生成が落ちても
// 質問が消えないようにするため (設計 §4)。
func (s *WorkspaceChatService) SendMessage(ctx context.Context, workspaceID, conversationID, text, userID string) (*domain.ChatMessage, *domain.ChatMessage, error) {
	if err := domain.ValidateChatMessageText(text); err != nil {
		return nil, nil, err
	}
	if err := s.authorizeRead(ctx, workspaceID, userID); err != nil {
		return nil, nil, err
	}

	// 出典が無くても回答は行う。ドキュメント未投入の workspace でも使いたい
	// という要求があり、出典ゼロは「回答不能」ではなく「一般知識で答えた」と
	// して表現する (grounded=false)。
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

// generateAnswer は retrieval → 生成 → citation 検証 → 永続化を行う。
// 生成が失敗した場合も assistant 行を failed として残し、turn が黙って
// 消えないようにする。
func (s *WorkspaceChatService) generateAnswer(
	ctx context.Context,
	workspaceID, conversationID, question string,
	history []*domain.ChatMessage,
) (*domain.ChatMessage, error) {
	// SQL 側が一致数で順位付けし、一致ゼロでも候補を返すので、空振り用の
	// 二度目の問い合わせは要らない。
	terms := domain.ChatSearchTerms(question)

	candidates, err := s.repo.SearchChatSourceCandidates(ctx, workspaceID, terms, chatMaxRetrievedChunks)
	if err != nil {
		return nil, err
	}
	// ツリー item も出典候補にする。document が1件も無い workspace で
	// 回答の根拠になるのはこちらだけ。
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
		// 失敗も行として残す。中身 (質問文・生成文) はログに出さない。
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
		// 出典が1件も付かなかった回答は workspace の中身に基づいていない。
		// 本文からは後追いで判定できないので行に残し、UI で明示する。
		Grounded: len(sources) > 0,
	}, snapshotJSON(candidates, model))
}

func (s *WorkspaceChatService) ensureConversation(ctx context.Context, workspaceID, conversationID, firstText, userID string) (*domain.ChatConversation, error) {
	if conversationID != "" {
		return s.resolveConversation(ctx, workspaceID, conversationID)
	}
	return s.repo.CreateChatConversation(ctx, workspaceID, userID, conversationTitle(firstText))
}

// validateCitations は model が返した chunk id を候補集合に突き合わせる。
// 候補に無い id は捨てる。重複も 1 件に畳む。これが「model が prompt に無い
// 出典をでっち上げる」経路を塞ぐ唯一の場所 (設計 §5)。
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

// boundContext は候補列を累計文字数で切る。prompt コストを無制限にしないため。
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

// conversationTitle は最初の質問から会話タイトルを作る。v1 では LLM による
// 自動命名はしない (設計 §1)。
func conversationTitle(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) > 40 {
		return string(runes[:40]) + "…"
	}
	return trimmed
}

// snapshotJSON は retrieval の再現用記録を作る。
// source id / strategy / 実効モデルのみを入れる。資格情報、endpoint、
// 生の prompt、chunk 本文は入れない (設計 §3, §7)。
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
