package process

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/base"
	"github.com/synthify/backend/packages/shared/domain"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type SynthesisArgs struct {
	JobID       string         `json:"job_id"`
	DocumentID  string         `json:"document_id"`
	WorkspaceID string         `json:"workspace_id"`
	Chunks      []domain.Chunk `json:"chunks"`
	Instruction string         `json:"instruction,omitempty"`
}

type SynthesisResult struct {
	Items []domain.SynthesizedItem `json:"items"`
}

func NewSynthesisTool(b *base.Context) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "goal_driven_synthesis",
		Description: "Synthesizes a structured knowledge tree from document chunks based on a brief and optional instructions.",
	}, func(ctx tool.Context, args SynthesisArgs) (SynthesisResult, error) {
		items, _, err := Synthesize(ctx, b.LLM, args)
		if err != nil {
			items = deterministicSynthesis(args.DocumentID, args.Chunks)
		}
		return SynthesisResult{Items: items}, nil
	})
}

func synthesize(ctx context.Context, llmClient base.LLMClient, args SynthesisArgs) ([]domain.SynthesizedItem, error) {
	items, _, err := Synthesize(ctx, llmClient, args)
	return items, err
}

// Synthesize runs synthesis with the production (embedded) prompt.
func Synthesize(ctx context.Context, llmClient base.LLMClient, args SynthesisArgs) ([]domain.SynthesizedItem, llm.Usage, error) {
	provider, err := prompts.Default()
	if err != nil {
		return nil, llm.Usage{}, fmt.Errorf("load synthesis prompts: %w", err)
	}
	return SynthesizeWithProvider(ctx, llmClient, provider, args)
}

// SynthesizeWithProvider runs synthesis with an explicit prompt provider. The
// eval runner uses this to inject a variant provider; production callers use
// Synthesize, which supplies the embedded provider.
func SynthesizeWithProvider(ctx context.Context, llmClient base.LLMClient, provider *prompts.Provider, args SynthesisArgs) ([]domain.SynthesizedItem, llm.Usage, error) {
	if llmClient == nil {
		return nil, llm.Usage{}, fmt.Errorf("llm not configured")
	}
	if provider == nil {
		return nil, llm.Usage{}, fmt.Errorf("prompt provider not configured")
	}

	prompt, err := provider.Synthesis(prompts.SynthesisInput{
		DocumentID:  args.DocumentID,
		Instruction: args.Instruction,
		Chunks:      args.Chunks,
	})
	if err != nil {
		return nil, llm.Usage{}, err
	}

	type llmOutput struct {
		Items []domain.SynthesizedItem `json:"items"`
	}

	raw, usage, err := llmClient.GenerateStructured(ctx, llm.StructuredRequest{
		SystemPrompt: prompt.System,
		UserPrompt:   prompt.User,
		Schema:       llmOutput{},
	})
	if err != nil {
		return nil, usage, err
	}

	var out llmOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		var items []domain.SynthesizedItem
		if arrayErr := json.Unmarshal(raw, &items); arrayErr != nil {
			return nil, usage, err
		}
		out.Items = items
	}
	if len(out.Items) == 0 {
		return nil, usage, fmt.Errorf("llm returned no items")
	}
	return out.Items, usage, nil
}

func deterministicSynthesis(documentID string, chunks []domain.Chunk) []domain.SynthesizedItem {
	items := make([]domain.SynthesizedItem, 0, len(chunks))
	for _, chunk := range chunks {
		title := strings.TrimSpace(chunk.Heading)
		if title == "" {
			title = fmt.Sprintf("Section %d", chunk.ChunkIndex+1)
		}
		description := base.SummarizePlainText(chunk.Text, 360)
		items = append(items, domain.SynthesizedItem{
			LocalID:        fmt.Sprintf("chunk_%d", chunk.ChunkIndex),
			Title:          title,
			Level:          1,
			Description:    description,
			Content:        "<p>" + base.HtmlEscape(description) + "</p>",
			SourceChunkIDs: []string{fmt.Sprintf("%s_chunk_%d", documentID, chunk.ChunkIndex)},
		})
	}
	return items
}
