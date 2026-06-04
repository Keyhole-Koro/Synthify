package prompts

import (
	"fmt"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// expectedSystemPrompt mirrors templates/knowledge_tree.system.tmpl. If they
// diverge the template changed behaviour — update both deliberately.
const expectedSystemPrompt = `You are a Lead Knowledge Architect. Convert document chunks into a high-fidelity, hierarchical knowledge tree.

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
- source_chunk_ids: list of chunk IDs referenced (format: "{document_id}_chunk_{index}").

Rules for Merging with the Existing Workspace Tree:
- The workspace already holds ONE shared knowledge tree, listed under "Existing workspace nodes". Documents are sources that feed this single tree, not separate trees.
- When a concept you are generating is essentially the SAME as an existing node, MERGE into it: set "merge_target_item_id" to that node's id (the first column). Then write title/description/content as an IMPROVED, combined version that incorporates BOTH the existing knowledge and the new document's contribution — do not discard the existing node's information.
- When a concept is genuinely new, leave "merge_target_item_id" empty and create a new node as usual.
- Merge conservatively: only merge when you are confident two concepts are the same. When unsure, create a new node.
- A merged item still lists its source_chunk_ids from THIS document; parent_local_id is ignored for merged items (the existing node keeps its place in the tree).`

// expectedUserPrompt mirrors templates/knowledge_tree.user.tmpl: the "none"
// defaults for empty instruction / no existing nodes, the chunk block, and the
// existing-nodes block.
func expectedUserPrompt(documentID, instruction string, chunks []domain.Chunk, existing []domain.ExistingNode) string {
	var sb strings.Builder
	for _, chunk := range chunks {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n\n", chunk.ChunkIndex, chunk.Heading, chunk.Text)
	}
	if instruction == "" {
		instruction = "none"
	}
	existingNodes := "none"
	if len(existing) > 0 {
		var eb strings.Builder
		for _, n := range existing {
			fmt.Fprintf(&eb, "- %s | %s | %s\n", n.ID, n.Title, n.Description)
		}
		existingNodes = eb.String()
	}
	return fmt.Sprintf("document_id: %s\nInstruction: %s\n\nExisting workspace nodes (id | title | description):\n%s\nChunks:\n%s", documentID, instruction, existingNodes, sb.String())
}

func TestDefaultRendererProducesExpectedPrompt(t *testing.T) {
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
		{
			// existing nodes are rendered for merge guidance
			DocumentID:  "doc_with_existing",
			Instruction: "merge where possible",
			Chunks: []domain.Chunk{
				{ChunkIndex: 0, Heading: "Intro", Text: "About X."},
			},
			ExistingNodes: []domain.ExistingNode{
				{ID: "nd_1", Title: "序論", Description: "Introduction"},
				{ID: "nd_2", Title: "手法", Description: "Method"},
			},
		},
	}

	for _, in := range cases {
		t.Run(in.DocumentID, func(t *testing.T) {
			got, err := r.Render(in)
			if err != nil {
				t.Fatalf("Render(): %v", err)
			}
			if got.System != expectedSystemPrompt {
				t.Errorf("system prompt diverged.\n--- got ---\n%q\n--- want ---\n%q", got.System, expectedSystemPrompt)
			}
			wantUser := expectedUserPrompt(in.DocumentID, in.Instruction, in.Chunks, in.ExistingNodes)
			if got.User != wantUser {
				t.Errorf("user prompt diverged.\n--- got ---\n%q\n--- want ---\n%q", got.User, wantUser)
			}
		})
	}
}
