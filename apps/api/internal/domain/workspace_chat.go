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
	// ChatRoleUser represents a user-authored chat message.
	ChatRoleUser = "user"
	// ChatRoleAssistant represents an assistant-generated chat response.
	ChatRoleAssistant = "assistant"
)

const (
	// ChatMessageStatusComplete indicates successful generation and persistence of the message.
	ChatMessageStatusComplete = "complete"
	// ChatMessageStatusFailed indicates generation encountered an error.
	ChatMessageStatusFailed = "failed"
	// ChatMessageStatusCancelled indicates generation was cancelled prior to completion.
	ChatMessageStatusCancelled = "cancelled"
)

// Standard client-facing error codes matching workspace chat design specifications.
const (
	// ChatErrInvalidMessage indicates malformed or oversized message input.
	ChatErrInvalidMessage = "invalid_chat_message"
	// ChatErrGenerationFailed indicates downstream LLM generation failure.
	ChatErrGenerationFailed = "chat_generation_failed"
	// ChatErrWorkspaceForbidden indicates insufficient permission to access the workspace chat.
	ChatErrWorkspaceForbidden = "workspace_access_denied"
)

// ChatMessageMaxRunes is the maximum allowed rune count for a single chat message (8000 runes).
const ChatMessageMaxRunes = 8000

// ChatConversation represents a thread of chat messages within a workspace.
type ChatConversation struct {
	// ConversationID is the unique identifier for the conversation thread.
	ConversationID string `json:"conversation_id"`
	// WorkspaceID is the workspace enclosing this conversation.
	WorkspaceID string `json:"workspace_id"`
	// CreatedBy is the user ID who initiated the conversation.
	CreatedBy string `json:"created_by"`
	// Title is the conversation title or subject line.
	Title string `json:"title"`
	// CreatedAt is the ISO timestamp when the conversation was initiated.
	CreatedAt string `json:"created_at"`
	// UpdatedAt is the ISO timestamp when the conversation was last active.
	UpdatedAt string `json:"updated_at"`
}

// ChatSource represents citation provenance for an assistant response.
// Generated exclusively by the server and never constructed directly from client input.
type ChatSource struct {
	// DocumentID is the ID of the referenced document.
	DocumentID string `json:"document_id"`
	// ChunkID is the ID of the specific chunk referenced, if applicable.
	ChunkID string `json:"chunk_id,omitempty"`
	// ItemID is the ID of the knowledge tree item referenced, if applicable.
	ItemID string `json:"item_id,omitempty"`
	// Label is the human-readable display label for the citation source.
	Label string `json:"label"`
}

// ChatMessage represents an individual message entry in a conversation thread.
type ChatMessage struct {
	// MessageID is the unique identifier for the message.
	MessageID string `json:"message_id"`
	// ConversationID is the ID of the conversation thread this message belongs to.
	ConversationID string `json:"conversation_id"`
	// Role indicates the message author role ("user" or "assistant").
	Role string `json:"role"`
	// Content is the raw markdown/text body of the message.
	Content string `json:"content"`
	// Sources is the list of citation sources backing an assistant message.
	Sources []ChatSource `json:"sources,omitempty"`
	// ModelSelection identifies the LLM model that generated this message.
	ModelSelection string `json:"model_selection"`
	// Status is the generation status ("complete", "failed", "cancelled").
	Status string `json:"status"`
	// ErrorCode is the error code if generation failed.
	ErrorCode string `json:"error_code,omitempty"`
	// Grounded reports whether the response is anchored in workspace sources.
	// When false, the response relied solely on general model knowledge.
	Grounded bool `json:"grounded"`
	// CreatedAt is the ISO timestamp when the message was recorded.
	CreatedAt string `json:"created_at"`
}

// ChatSourceCandidate represents a retrieved snippet candidate from documents or knowledge tree items
// for grounding an assistant response. The LLM's returned source IDs are validated against this set (design §5).
type ChatSourceCandidate struct {
	// ChunkID is the ID of the document chunk, if this candidate originates from a document chunk.
	ChunkID string
	// ItemID is the ID of the tree item, if this candidate originates from the knowledge tree.
	ItemID string
	// DocumentID is the ID of the document containing the candidate chunk.
	DocumentID string
	// Filename is the display filename of the document.
	Filename string
	// SubPath is the file sub-path if nested within an archive.
	SubPath string
	// Heading is the section heading or item title.
	Heading string
	// Text is the extracted text content of the candidate chunk or item.
	Text string
	// SourcePage is the 1-based page number where the chunk occurs.
	SourcePage int
}

// SourceID returns the unique identifier for the citation candidate (ItemID for tree items, ChunkID for document chunks).
func (c ChatSourceCandidate) SourceID() string {
	if c.ItemID != "" {
		return c.ItemID
	}
	return c.ChunkID
}

// Label constructs a user-facing compact source label for the citation candidate.
func (c ChatSourceCandidate) Label() string {
	if c.ItemID != "" {
		// Tree item headings serve as standalone source names without a filename prefix.
		return c.Heading
	}
	name := c.Filename
	if c.SubPath != "" {
		name = c.SubPath
	}
	if c.Heading != "" {
		return name + " / " + c.Heading
	}
	return name
}

// ValidateChatMessageText validates message content for SendMessage.
// Preserves leading/trailing whitespace, but rejects empty or whitespace-only messages,
// invalid UTF-8, and messages exceeding ChatMessageMaxRunes.
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

// ChatSearchTermSeparator is the delimiter used when joining search terms into a single string for SQL queries.
// Uses an ASCII unit separator character not found in standard user text.
const ChatSearchTermSeparator = "\x1f"

// chatSearchMaxTerms is the upper bound on search terms extracted from a single chat query.
const chatSearchMaxTerms = 20

// ChatSearchTerms extracts lexical search terms from a user question.
// In addition to whitespace-delimited tokens, generates 2-3 character n-gram windows
// for contiguous CJK sequences to enable accurate lexical matching in languages without explicit spaces.
func ChatSearchTerms(question string) []string {
	seen := make(map[string]bool)
	terms := make([]string, 0, chatSearchMaxTerms)

	add := func(term string) bool {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 || seen[term] {
			return true
		}
		// Drop terms containing the separator to prevent SQL fragmentation.
		if strings.Contains(term, ChatSearchTermSeparator) {
			return true
		}
		seen[term] = true
		terms = append(terms, term)
		return len(terms) < chatSearchMaxTerms
	}

	for _, field := range strings.FieldsFunc(question, isChatTermBoundary) {
		runes := []rune(field)
		// Include alphanumeric / space-delimited words directly.
		if !add(field) {
			return terms
		}
		// Break down CJK sequences using sliding 2- and 3-character windows.
		if !containsCJK(runes) {
			continue
		}
		// Emit 2-character windows first as they capture most Japanese content words.
		for size := 2; size <= 3; size++ {
			for i := 0; i+size <= len(runes); i++ {
				if !add(string(runes[i : i+size])) {
					return terms
				}
			}
		}
	}
	return terms
}

// isChatTermBoundary reports whether rune r should be treated as a token boundary when extracting search terms.
func isChatTermBoundary(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '　', '?', '？', '。', '、', '.', ',', '!', '！',
		'「', '」', '(', ')', '（', '）', ':', '：', ';', '；':
		return true
	}
	return false
}

// containsCJK reports whether the rune slice contains any Hiragana, Katakana, or CJK Unified Ideograph characters.
func containsCJK(runes []rune) bool {
	for _, r := range runes {
		switch {
		case r >= 0x3040 && r <= 0x30ff, // Hiragana and Katakana
			r >= 0x4e00 && r <= 0x9fff: // CJK Unified Ideographs
			return true
		}
	}
	return false
}
