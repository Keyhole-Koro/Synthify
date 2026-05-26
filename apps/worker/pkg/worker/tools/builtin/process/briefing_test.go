package process

import (
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

func TestSuggestedWorkspaceName_UsesBriefTopic(t *testing.T) {
	got := suggestedWorkspaceName(domain.DocumentBrief{
		Topic:   "  Clinical   Trial Protocol  ",
		Outline: []string{"Fallback"},
	})

	if got != "Clinical Trial Protocol" {
		t.Fatalf("got %q, want %q", got, "Clinical Trial Protocol")
	}
}

func TestSuggestedWorkspaceName_FallsBackToFirstOutlineHeading(t *testing.T) {
	got := suggestedWorkspaceName(domain.DocumentBrief{
		Outline: []string{"  OAuth Migration Plan  "},
	})

	if got != "OAuth Migration Plan" {
		t.Fatalf("got %q, want %q", got, "OAuth Migration Plan")
	}
}

func TestSuggestedWorkspaceName_TruncatesTo64Runes(t *testing.T) {
	got := suggestedWorkspaceName(domain.DocumentBrief{
		Topic: strings.Repeat("あ", 80),
	})

	if len([]rune(got)) != 64 {
		t.Fatalf("rune length = %d, want 64", len([]rune(got)))
	}
}
