package domain

import (
	"strings"
	"unicode/utf8"
)

// Workspace chat の domain 型。設計は docs/architecture/workspace-chat-design.md。
//
// chat は read-only capability である。この package には tree / document を
// 変更する型を置かない。

const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"
)

const (
	ChatMessageStatusComplete  = "complete"
	ChatMessageStatusFailed    = "failed"
	ChatMessageStatusCancelled = "cancelled"
)

// 安定した client 向けエラーコード。設計 §4 の error mapping 表に対応する。
const (
	ChatErrInvalidMessage     = "invalid_chat_message"
	ChatErrSourceUnavailable  = "chat_source_unavailable"
	ChatErrGenerationFailed   = "chat_generation_failed"
	ChatErrWorkspaceForbidden = "workspace_access_denied"
)

// ChatMessageMaxRunes は 1 メッセージの上限 (設計 §4)。
const ChatMessageMaxRunes = 8000

type ChatConversation struct {
	ConversationID string `json:"conversation_id"`
	WorkspaceID    string `json:"workspace_id"`
	CreatedBy      string `json:"created_by"`
	Title          string `json:"title"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// ChatSource は assistant メッセージの引用根拠。常にサーバー生成であり、
// client 入力から作ってはならない。
type ChatSource struct {
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id,omitempty"`
	ItemID     string `json:"item_id,omitempty"`
	Label      string `json:"label"`
}

type ChatMessage struct {
	MessageID      string       `json:"message_id"`
	ConversationID string       `json:"conversation_id"`
	Role           string       `json:"role"`
	Content        string       `json:"content"`
	Sources        []ChatSource `json:"sources,omitempty"`
	ModelSelection string       `json:"model_selection"`
	Status         string       `json:"status"`
	ErrorCode      string       `json:"error_code,omitempty"`
	CreatedAt      string       `json:"created_at"`
}

// ChatSourceCandidate は retrieval が返す引用候補。model が返した source id は
// この集合に対してのみ検証され、集合外の id は捨てられる (設計 §5)。
type ChatSourceCandidate struct {
	ChunkID    string
	DocumentID string
	Filename   string
	SubPath    string
	Heading    string
	Text       string
	SourcePage int
}

// Label は citation に出す短い source label を組み立てる。
func (c ChatSourceCandidate) Label() string {
	name := c.Filename
	if c.SubPath != "" {
		name = c.SubPath
	}
	if c.Heading != "" {
		return name + " / " + c.Heading
	}
	return name
}

// ValidateChatMessageText は SendMessage の text を検証する。
// 前後の空白は保持するが、全て空白のメッセージは invalid (設計 §4)。
func ValidateChatMessageText(text string) error {
	if !utf8.ValidString(text) {
		return ErrInvalidArgument
	}
	if strings.TrimSpace(text) == "" {
		return ErrInvalidArgument
	}
	if utf8.RuneCountInString(text) > ChatMessageMaxRunes {
		return ErrInvalidArgument
	}
	return nil
}
