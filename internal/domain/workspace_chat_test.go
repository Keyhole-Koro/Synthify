package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateChatMessageText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{name: "typical question", text: "この資料の結論は？", wantErr: false},
		{name: "empty", text: "", wantErr: true},
		{name: "spaces only", text: "   ", wantErr: true},
		{name: "newlines and tabs only", text: "\n\t \r\n", wantErr: true},
		{name: "ideographic space only", text: "　　", wantErr: true},
		// 前後の空白は保持する = 中身があれば valid。
		{name: "padded but non-empty", text: "  hello  ", wantErr: false},
		{name: "at the limit", text: strings.Repeat("あ", ChatMessageMaxRunes), wantErr: false},
		{name: "one rune over the limit", text: strings.Repeat("あ", ChatMessageMaxRunes+1), wantErr: true},
		{name: "invalid utf8", text: string([]byte{0xff, 0xfe}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChatMessageText(tt.text)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidArgument)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// 上限はバイト数ではなく Unicode code point で数える。マルチバイト文字だけの
// メッセージがバイト長で弾かれないことを固定する。
func TestValidateChatMessageText_CountsRunesNotBytes(t *testing.T) {
	text := strings.Repeat("あ", ChatMessageMaxRunes)
	assert.Greater(t, len(text), ChatMessageMaxRunes, "test premise: multibyte text is longer in bytes")
	assert.NoError(t, ValidateChatMessageText(text))
}

func TestChatSourceCandidateLabel(t *testing.T) {
	tests := []struct {
		name      string
		candidate ChatSourceCandidate
		want      string
	}{
		{
			name:      "filename only",
			candidate: ChatSourceCandidate{Filename: "report.pdf"},
			want:      "report.pdf",
		},
		{
			name:      "filename with heading",
			candidate: ChatSourceCandidate{Filename: "report.pdf", Heading: "結論"},
			want:      "report.pdf / 結論",
		},
		{
			name:      "sub path wins over filename",
			candidate: ChatSourceCandidate{Filename: "archive.zip", SubPath: "docs/intro.md"},
			want:      "docs/intro.md",
		},
		{
			name:      "sub path with heading",
			candidate: ChatSourceCandidate{Filename: "archive.zip", SubPath: "docs/intro.md", Heading: "概要"},
			want:      "docs/intro.md / 概要",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.candidate.Label())
		})
	}
}

func TestChatSearchTerms(t *testing.T) {
	tests := []struct {
		name        string
		question    string
		wantContain []string
		wantAbsent  []string
	}{
		{
			// 本来の狙い: 空白の無い日本語の質問から意味のある語を取り出す。
			name:        "japanese question yields substrings",
			question:    "ワークスペースの権限について教えて",
			wantContain: []string{"権限"},
		},
		{
			name:        "ascii words survive whole",
			question:    "what is the knowledge tree",
			wantContain: []string{"what", "knowledge", "tree"},
		},
		{
			name:       "single characters are dropped",
			question:   "a b c",
			wantAbsent: []string{"a", "b", "c"},
		},
		{
			name:        "punctuation splits terms",
			question:    "結論は？",
			wantContain: []string{"結論"},
			wantAbsent:  []string{"結論は？"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terms := ChatSearchTerms(tt.question)
			for _, want := range tt.wantContain {
				assert.Contains(t, terms, want)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, terms, absent)
			}
		})
	}
}

func TestChatSearchTerms_IsBounded(t *testing.T) {
	terms := ChatSearchTerms(strings.Repeat("あいうえお", 100))
	assert.LessOrEqual(t, len(terms), chatSearchMaxTerms, "term count must stay bounded")
}

// 区切り文字を含む語を作ってしまうと SQL 側で誤って分割される。
func TestChatSearchTerms_NeverEmitsTheSeparator(t *testing.T) {
	terms := ChatSearchTerms("結論" + ChatSearchTermSeparator + "です")
	for _, term := range terms {
		assert.NotContains(t, term, ChatSearchTermSeparator)
	}
}

func TestChatSearchTerms_EmptyQuestion(t *testing.T) {
	assert.Empty(t, ChatSearchTerms(""))
	assert.Empty(t, ChatSearchTerms("   "))
}
