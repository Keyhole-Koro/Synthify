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
