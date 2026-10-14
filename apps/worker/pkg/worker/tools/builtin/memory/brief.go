package memory

import (
	"strings"
	"sync"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// Brief holds the working memory block for high-level document topic and claim summaries.
type Brief struct {
	mu    sync.RWMutex
	brief *domain.DocumentBrief
}

// NewBrief creates a new uninitialized Brief memory container.
func NewBrief() *Brief {
	return &Brief{}
}

// Set stores the document brief safely under write lock.
func (b *Brief) Set(brief domain.DocumentBrief) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.brief = &brief
}

// RenderForPrompt generates a Markdown representation of the brief suitable for LLM prompt injection.
func (b *Brief) RenderForPrompt() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.brief == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("### Document Brief\n")
	sb.WriteString("- **Topic**: ")
	sb.WriteString(b.brief.Topic)
	sb.WriteString("\n- **Summary**: ")
	sb.WriteString(b.brief.ClaimSummary)
	sb.WriteString("\n")
	return sb.String()
}
