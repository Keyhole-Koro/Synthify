package process

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/builtin/memory"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/internal/domain"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

// BriefArgs specifies the section outline used to infer document themes and topic.
type BriefArgs struct {
	Outline []string `json:"outline"`
}

// BriefResult contains the synthesized document brief.
type BriefResult struct {
	Brief domain.DocumentBrief `json:"brief"`
}

// NewBriefTool creates a core.Tool for generating high-level document briefs and updating the suggested workspace title.
func NewBriefTool(b *base.Context, mem *memory.Brief) (core.Tool, error) {
	schema, err := core.IOSchemaFor[BriefArgs, BriefResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args BriefArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		brief, gerr := generateBrief(ctx, b.LLM, args.Outline)
		if gerr != nil {
			brief = fallbackBrief(args.Outline)
		}
		if b.Status != nil && b.Job != nil {
			title := suggestedWorkspaceName(brief)
			if title != "" {
				_ = b.Status.UpdateFields(ctx, b.Job.WorkspaceID, b.Job.JobID, jobstatus.UpdateFields{
					SuggestedWorkspaceName:       jobstatus.String(title),
					SuggestedWorkspaceNameSource: jobstatus.String(string(jobstatus.FirestoreJobStatusSuggestedWorkspaceNameSourceBriefTopic)),
				})
			}
		}
		mem.Set(brief)
		out, mErr := json.Marshal(BriefResult{Brief: brief})
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "generate_brief",
		Description: "Analyzes the document outline to generate a high-level summary and key themes.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// generateBrief prompts the LLM to infer the main topic, claim summary, key entities, and level01 hints from outline headings.
func generateBrief(ctx context.Context, llmClient base.LLMClient, outline []string) (domain.DocumentBrief, error) {
	if llmClient == nil {
		return domain.DocumentBrief{}, fmt.Errorf("llm not configured")
	}

	var sb strings.Builder
	for i, heading := range outline {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, heading)
	}

	raw, _, err := llmClient.GenerateStructured(ctx, llm.StructuredRequest{
		SystemPrompt: `You are a document analyst. Given a list of section headings, infer:
- topic: the main subject of the document (short phrase)
- claim_summary: the core argument or purpose in one sentence
- entities: key named concepts, systems, or terms (up to 10)
- level01_hints: suggested top-level labels for a knowledge tree (up to 5)
- outline: the original headings unchanged

Rules:
- NO MARKDOWN: Never use #, **, or other Markdown symbols. Use plain text.
- JSON: Respond only with valid JSON matching the schema.`,
		UserPrompt: "Section headings:\n" + sb.String(),
		Schema:     domain.DocumentBrief{},
	})
	if err != nil {
		return domain.DocumentBrief{}, err
	}

	var brief domain.DocumentBrief
	if err := json.Unmarshal(raw, &brief); err != nil {
		return domain.DocumentBrief{}, err
	}
	brief.Outline = append([]string(nil), outline...)
	return brief, nil
}

// suggestedWorkspaceName produces a clean, length-bounded workspace title proposal from the brief topic or first heading.
func suggestedWorkspaceName(brief domain.DocumentBrief) string {
	title := strings.TrimSpace(brief.Topic)
	if title == "" && len(brief.Outline) > 0 {
		title = strings.TrimSpace(brief.Outline[0])
	}
	title = strings.Join(strings.Fields(title), " ")
	if len([]rune(title)) > 64 {
		title = string([]rune(title)[:64])
	}
	return title
}

// fallbackBrief constructs a heuristic brief when the LLM brief generation fails.
func fallbackBrief(outline []string) domain.DocumentBrief {
	topic := "Document"
	if len(outline) > 0 && strings.TrimSpace(outline[0]) != "" {
		topic = strings.TrimSpace(outline[0])
	}
	return domain.DocumentBrief{
		Topic:        topic,
		ClaimSummary: "Document organized around: " + strings.Join(outline, ", "),
		Outline:      append([]string(nil), outline...),
	}
}
