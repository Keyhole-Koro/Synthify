package memory

import (
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

func TestDocumentMap_EmptyRendersNothing(t *testing.T) {
	dm := NewDocumentMap()
	if got := dm.RenderForPrompt(); got != "" {
		t.Errorf("empty map rendered %q, want empty", got)
	}
}

func TestDocumentMap_RendersChunkIDsHeadingsAndHint(t *testing.T) {
	dm := NewDocumentMap()
	dm.Set([]*domain.DocumentChunk{
		{ChunkID: "doc_chunk_0", Heading: "Intro", Text: "Mermaid is a diagramming tool."},
		{ChunkID: "doc_chunk_1", Heading: "Flowcharts", Text: "Declared with graph or flowchart."},
	})
	got := dm.RenderForPrompt()

	for _, want := range []string{"doc_chunk_0", "doc_chunk_1", "Intro", "Flowcharts"} {
		if !strings.Contains(got, want) {
			t.Errorf("render missing %q:\n%s", want, got)
		}
	}
	// The whole point of the map: steer the agent away from the extract_text
	// guessing loop.
	if !strings.Contains(got, "extract_text") {
		t.Errorf("render should hint not to call extract_text:\n%s", got)
	}
}

func TestDocumentMap_FallsBackToChunkIDWhenHeadingBlank(t *testing.T) {
	dm := NewDocumentMap()
	dm.Set([]*domain.DocumentChunk{{ChunkID: "doc_chunk_0", Heading: "", Text: "body"}})
	if got := dm.RenderForPrompt(); !strings.Contains(got, "doc_chunk_0") {
		t.Errorf("blank heading should fall back to chunk_id:\n%s", got)
	}
}

func TestDocumentMap_SnippetTruncates(t *testing.T) {
	long := strings.Repeat("word ", 200) // far over the snippet bound
	dm := NewDocumentMap()
	dm.Set([]*domain.DocumentChunk{{ChunkID: "doc_chunk_0", Heading: "H", Text: long}})
	got := dm.RenderForPrompt()
	if !strings.Contains(got, "…") {
		t.Errorf("long text should be truncated with ellipsis:\n%s", got)
	}
	// A single chunk line must stay small so the map does not crowd the context.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "doc_chunk_0") && len([]rune(line)) > documentMapSnippetRunes+80 {
			t.Errorf("chunk line too long (%d runes): %s", len([]rune(line)), line)
		}
	}
}
