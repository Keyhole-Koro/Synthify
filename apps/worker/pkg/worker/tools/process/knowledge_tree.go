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

type GenerateKnowledgeTreeArgs struct {
	JobID       string         `json:"job_id"`
	DocumentID  string         `json:"document_id"`
	WorkspaceID string         `json:"workspace_id"`
	Chunks      []domain.Chunk `json:"chunks"`
	Instruction string         `json:"instruction,omitempty"`
}

type GenerateKnowledgeTreeResult struct {
	Items []domain.GeneratedTreeItem `json:"items"`
}

func NewGenerateKnowledgeTreeTool(b *base.Context) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "generate_knowledge_tree",
		Description: "Generates structured knowledge tree items from document chunks based on a brief and optional instructions.",
	}, func(ctx tool.Context, args GenerateKnowledgeTreeArgs) (GenerateKnowledgeTreeResult, error) {
		items, _, err := GenerateKnowledgeTree(ctx, b.LLM, args)
		if err != nil {
			items = deterministicKnowledgeTree(args.DocumentID, args.Chunks)
		}
		return GenerateKnowledgeTreeResult{Items: items}, nil
	})
}

func generateKnowledgeTree(ctx context.Context, llmClient base.LLMClient, args GenerateKnowledgeTreeArgs) ([]domain.GeneratedTreeItem, error) {
	items, _, err := GenerateKnowledgeTree(ctx, llmClient, args)
	return items, err
}

// GenerateKnowledgeTree runs knowledge tree generation with the production
// embedded prompt.
func GenerateKnowledgeTree(ctx context.Context, llmClient base.LLMClient, args GenerateKnowledgeTreeArgs) ([]domain.GeneratedTreeItem, llm.Usage, error) {
	renderer, err := prompts.Default()
	if err != nil {
		return nil, llm.Usage{}, fmt.Errorf("load knowledge tree prompts: %w", err)
	}
	return GenerateKnowledgeTreeWithRenderer(ctx, llmClient, renderer, args)
}

// GenerateKnowledgeTreeWithRenderer runs knowledge tree generation with an
// explicit prompt renderer. The eval runner uses this to inject a variant
// renderer; production callers use GenerateKnowledgeTree, which supplies the
// embedded renderer.
func GenerateKnowledgeTreeWithRenderer(ctx context.Context, llmClient base.LLMClient, renderer *prompts.Renderer, args GenerateKnowledgeTreeArgs) ([]domain.GeneratedTreeItem, llm.Usage, error) {
	if llmClient == nil {
		return nil, llm.Usage{}, fmt.Errorf("llm not configured")
	}
	if renderer == nil {
		return nil, llm.Usage{}, fmt.Errorf("prompt renderer not configured")
	}

	prompt, err := renderer.Render(prompts.RenderInput{
		DocumentID:  args.DocumentID,
		Instruction: args.Instruction,
		Chunks:      args.Chunks,
	})
	if err != nil {
		return nil, llm.Usage{}, err
	}

	type llmOutput struct {
		Items []domain.GeneratedTreeItem `json:"items"`
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
		var items []domain.GeneratedTreeItem
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

func deterministicKnowledgeTree(documentID string, chunks []domain.Chunk) []domain.GeneratedTreeItem {
	items := make([]domain.GeneratedTreeItem, 0, len(chunks))
	for _, chunk := range chunks {
		title := strings.TrimSpace(chunk.Heading)
		if title == "" {
			title = fmt.Sprintf("Section %d", chunk.ChunkIndex+1)
		}
		description := base.SummarizePlainText(chunk.Text, 360)
		items = append(items, domain.GeneratedTreeItem{
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
