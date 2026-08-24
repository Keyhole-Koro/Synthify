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
	// Grounded は回答が workspace 内の出典に基づくか。false は
	// 「モデルの一般知識で答えた」を意味し、UI で明示する。
	Grounded  bool   `json:"grounded"`
	CreatedAt string `json:"created_at"`
}

// ChatSourceCandidate は retrieval が返す引用候補。model が返した source id は
// この集合に対してのみ検証され、集合外の id は捨てられる (設計 §5)。
//
// 候補はドキュメントの chunk か、ナレッジツリーの item のどちらか。item 由来の
// 候補は DocumentID が空になる。
type ChatSourceCandidate struct {
	ChunkID    string
	ItemID     string
	DocumentID string
	Filename   string
	SubPath    string
	Heading    string
	Text       string
	SourcePage int
}

// SourceID は候補を一意に指す id。model にはこの値を出典として返させ、
// 検証もこれで行う。
func (c ChatSourceCandidate) SourceID() string {
	if c.ItemID != "" {
		return c.ItemID
	}
	return c.ChunkID
}

// Label は citation に出す短い source label を組み立てる。
func (c ChatSourceCandidate) Label() string {
	if c.ItemID != "" {
		// item は見出しそのものが名前なので、ファイル名は付けない。
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

// ChatSearchTermSeparator は検索語をひとつの文字列に詰めるときの区切り。
// 通常の文章に現れない制御文字を使う。
const ChatSearchTermSeparator = "\x1f"

// chatSearchMaxTerms は 1 クエリあたりの検索語数の上限。
const chatSearchMaxTerms = 20

// ChatSearchTerms は質問文から lexical 検索語を作る。
//
// 空白区切りの語をそのまま使うと日本語の質問では語が1つ (文全体) になり、
// ILIKE がまず一致しない。そこで空白区切りの語に加えて、CJK 文字が続く
// 範囲から 2〜3 文字の窓を切り出す。形態素解析ではないので過剰に語を作るが、
// 一致数で順位付けする前提なので、余分な語は順位にほとんど影響しない。
//
// vector 検索が使えるようになればこの関数ごと不要になる。
func ChatSearchTerms(question string) []string {
	seen := make(map[string]bool)
	terms := make([]string, 0, chatSearchMaxTerms)

	add := func(term string) bool {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 || seen[term] {
			return true
		}
		// 区切り文字を含む語は SQL 側で分割されてしまうので落とす。
		if strings.Contains(term, ChatSearchTermSeparator) {
			return true
		}
		seen[term] = true
		terms = append(terms, term)
		return len(terms) < chatSearchMaxTerms
	}

	for _, field := range strings.FieldsFunc(question, isChatTermBoundary) {
		runes := []rune(field)
		// 空白区切りで意味のある語 (英数字など) はそのまま使う。
		if !add(field) {
			return terms
		}
		// CJK を含む語は窓で刻む。「ワークスペースの権限」から「権限」を拾う。
		if !containsCJK(runes) {
			continue
		}
		// 2 文字窓を先に出す。日本語の内容語 (権限・結論・概要) はたいてい
		// 2 文字で、3 文字窓を先に埋めると上限に達して 2 文字窓が出なくなる。
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

func isChatTermBoundary(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '　', '?', '？', '。', '、', '.', ',', '!', '！',
		'「', '」', '(', ')', '（', '）', ':', '：', ';', '；':
		return true
	}
	return false
}

func containsCJK(runes []rune) bool {
	for _, r := range runes {
		switch {
		case r >= 0x3040 && r <= 0x30ff, // ひらがな・カタカナ
			r >= 0x4e00 && r <= 0x9fff: // CJK 統合漢字
			return true
		}
	}
	return false
}
