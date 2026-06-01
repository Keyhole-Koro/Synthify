package memory

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// documentMapSnippetRunes bounds the per-chunk preview. A heading plus ~1-2
// lines is enough for the agent to decide which chunk it needs without a
// round-trip, while keeping the whole map small (a few hundred chunks still fit
// in a few KB) so it does not crowd out the rest of the context.
const documentMapSnippetRunes = 120

// DocumentMap is the Working Memory block that lists the document's chunks
// (heading + chunk_id + a short snippet) so the agent knows what the source
// contains from turn one. It exists to remove the exploration loop where the
// agent would blindly re-call extract_text/grep_search trying to discover the
// content — the orchestrator extracts and chunks the document deterministically,
// then populates this map. Read access is auto-injected (never a tool), matching
// the Brief/Glossary/Journal contract.
type DocumentMap struct {
	mu     sync.RWMutex
	chunks []*domain.DocumentChunk
}

func NewDocumentMap() *DocumentMap {
	return &DocumentMap{}
}

func (d *DocumentMap) Set(chunks []*domain.DocumentChunk) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.chunks = chunks
}

func (d *DocumentMap) RenderForPrompt() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.chunks) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "### Document Map\nThe source has already been extracted and split into %d chunk(s). Use these chunk_ids directly — there is no need to call extract_text.\n", len(d.chunks))
	for _, c := range d.chunks {
		heading := strings.TrimSpace(c.Heading)
		if heading == "" {
			heading = c.ChunkID
		}
		fmt.Fprintf(&sb, "- `%s` — %s", c.ChunkID, heading)
		if snippet := previewSnippet(c.Text); snippet != "" {
			fmt.Fprintf(&sb, ": %s", snippet)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// previewSnippet collapses whitespace and truncates to documentMapSnippetRunes
// runes so headings with weak signal ("Section 1") still carry a hint of the
// chunk's content.
func previewSnippet(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	flat := strings.Join(fields, " ")
	if utf8.RuneCountInString(flat) <= documentMapSnippetRunes {
		return flat
	}
	runes := []rune(flat)
	return string(runes[:documentMapSnippetRunes]) + "…"
}
