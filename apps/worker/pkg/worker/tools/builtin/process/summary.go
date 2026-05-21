package process

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/packages/shared/domain"
)

type SummaryArgs struct {
	Item domain.GeneratedTreeItem `json:"item"`
}

type SummaryResult struct {
	HTML string `json:"html"`
}

func NewSummaryTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[SummaryArgs, SummaryResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args SummaryArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		if b == nil || b.LLM == nil {
			out, mErr := json.Marshal(SummaryResult{HTML: args.Item.Content})
			return out, core.Usage{}, mErr
		}
		html, usage, lerr := b.LLM.GenerateText(ctx, llm.TextRequest{
			SystemPrompt: `You are a Technical Writer. Convert the provided item details into a professional, rich HTML summary.

Rules:
- NO MARKDOWN: Never use #, **, etc. Use HTML tags only.
- RICH CLASSES: Use available styles to enhance readability:
  - <p class="lede">: for the opening summary.
  - <div class="stat-card">: for key numbers or metrics.
  - <div class="tip-box">: for insights.
  - <table>: for structured data.
- INTERNAL LINKS: Retain all existing <a data-paper-id="..."> links if present in the input.
- Conciseness: Be deep but efficient. 1-4 paragraphs max.`,
			UserPrompt: fmt.Sprintf("Title: %s\nDescription: %s\nRaw Content: %s", args.Item.Title, args.Item.Description, args.Item.Content),
		})
		if lerr != nil {
			// Match the pre-道A fallback: return the original content so the
			// agent can keep going on an LLM failure.
			out, mErr := json.Marshal(SummaryResult{HTML: args.Item.Content})
			return out, core.Usage{Model: usage.Model, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}, mErr
		}
		out, mErr := json.Marshal(SummaryResult{HTML: strings.TrimSpace(html)})
		return out, core.Usage{Model: usage.Model, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}, mErr
	}
	return core.Tool{
		Name:        "generate_html_summary",
		Description: "Generates a high-quality, formatted HTML summary for a specific knowledge tree item using the LLM.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}
