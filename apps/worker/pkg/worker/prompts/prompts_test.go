package prompts

import (
	"fmt"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// legacySystemPrompt is the verbatim system prompt that knowledge tree generation.go embedded
// as a raw string literal before prompt externalization. The migration must
// reproduce it byte-for-byte (contract §2.2). Do not "fix" this string to
// match the template; if they diverge the template changed behaviour.
const legacySystemPrompt = `You are a Lead Knowledge Architect. Convert document chunks into a high-fidelity, hierarchical knowledge tree.

Rules for "content" (STRICT):
- NO MARKDOWN: Never use #, ##, **, or [text](url). Use HTML tags only.
- RICH HTML: Use a variety of structural tags and CSS classes to make the content "alive":
  - <p class="lede">: for important introductory paragraphs.
  - <p class="eyebrow">: for small, bold labels at the top of sections.
  - <div class="hero-block">: for featured summaries with a visual punch.
  - <div class="callout-grid">: for 2-column comparison or fact grids.
  - <div class="stat-card">: inside a grid or alone, use <strong>Number</strong> <span>Label</span>.
  - <div class="tip-box">: for helpful tips or additional context.
  - <blockquote>: for direct quotes from the source.
  - <table>: for technical data or side-by-side specs.
  - <details><summary>: for technical deep-dives that should be hidden by default.
  - <a data-paper-id="{local_id}">: to link to child items. Use the EXACT local_id.
- COMPOSITION: Combine these elements to create a professional technical report feel.

Rules for Structure:
- Use parent_local_id to express relationships. Root-level items have empty parent_local_id.
- Assign local_id as "item_1", "item_2", etc.
- description: a very short, plain-text summary for list views.
- source_chunk_ids: list of chunk IDs referenced (format: "{document_id}_chunk_{index}").`

// legacyUserPrompt reproduces the previous fmt.Sprintf construction in
// knowledge tree generation.go: the "none" default for empty instruction and the
// "[%d] %s\n%s\n\n" chunk block.
func legacyUserPrompt(documentID, instruction string, chunks []domain.Chunk) string {
	var sb strings.Builder
	for _, chunk := range chunks {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n\n", chunk.ChunkIndex, chunk.Heading, chunk.Text)
	}
	if instruction == "" {
		instruction = "none"
	}
	return fmt.Sprintf("document_id: %s\nInstruction: %s\n\nChunks:\n%s", documentID, instruction, sb.String())
}

func TestDefaultRendererMatchesLegacyPrompt(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatalf("Default(): %v", err)
	}

	cases := []RenderInput{
		{
			DocumentID:  "doc_api_spec",
			Instruction: "技術仕様書として整理して",
			Chunks: []domain.Chunk{
				{ChunkIndex: 0, Heading: "Overview", Text: "An API spec."},
				{ChunkIndex: 1, Heading: "Auth", Text: "Bearer token."},
			},
		},
		{
			// empty instruction must fall back to "none"
			DocumentID:  "doc_empty_instruction",
			Instruction: "",
			Chunks: []domain.Chunk{
				{ChunkIndex: 0, Heading: "H", Text: "T"},
			},
		},
		{
			// no chunks: Chunks block must be empty, trailing newlines absent
			DocumentID:  "doc_no_chunks",
			Instruction: "do it",
			Chunks:      nil,
		},
	}

	for _, in := range cases {
		t.Run(in.DocumentID, func(t *testing.T) {
			got, err := r.Render(in)
			if err != nil {
				t.Fatalf("Render(): %v", err)
			}
			if got.System != legacySystemPrompt {
				t.Errorf("system prompt diverged from legacy.\n--- got ---\n%q\n--- want ---\n%q", got.System, legacySystemPrompt)
			}
			wantUser := legacyUserPrompt(in.DocumentID, in.Instruction, in.Chunks)
			if got.User != wantUser {
				t.Errorf("user prompt diverged from legacy.\n--- got ---\n%q\n--- want ---\n%q", got.User, wantUser)
			}
		})
	}
}
