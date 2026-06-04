package process

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

type GenerateKnowledgeTreeArgs struct {
	JobID       string         `json:"job_id"`
	DocumentID  string         `json:"document_id"`
	WorkspaceID string         `json:"workspace_id"`
	Chunks      []domain.Chunk `json:"chunks"`
	Instruction string         `json:"instruction,omitempty"`
	// ExistingNodes is the workspace's current knowledge tree (id + title +
	// description per node). The LLM consults it to decide whether a generated
	// concept should merge into an existing node (set merge_target_item_id) or
	// be created anew. Populated by the tool, not the agent.
	ExistingNodes []domain.ExistingNode `json:"existing_nodes,omitempty"`
}

type GenerateKnowledgeTreeResult struct {
	Items []domain.GeneratedTreeItem `json:"items"`
}

func NewGenerateKnowledgeTreeTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[GenerateKnowledgeTreeArgs, GenerateKnowledgeTreeResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args GenerateKnowledgeTreeArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		// Show the LLM the workspace's current tree so it can merge new
		// concepts into existing nodes instead of duplicating them. Best
		// effort: a fetch failure just means generation proceeds without
		// merge hints (all items become new nodes).
		if b != nil && b.Repo != nil && args.WorkspaceID != "" {
			if existing, terr := b.Repo.GetTreeByWorkspace(ctx, args.WorkspaceID); terr == nil {
				args.ExistingNodes = make([]domain.ExistingNode, 0, len(existing))
				for _, it := range existing {
					args.ExistingNodes = append(args.ExistingNodes, domain.ExistingNode{
						ID:          it.ItemID,
						Title:       it.Title,
						Description: it.Description,
					})
				}
			}
		}
		items, usage, err := GenerateKnowledgeTree(ctx, b.LLM, args)
		if err != nil {
			// Fallback to deterministic generation, matching the pre-道A behaviour
			// (the ADK callback chain was given a successful result built from
			// chunks so the agent could proceed).
			items = deterministicKnowledgeTree(args.DocumentID, args.Chunks)
		}
		out, mErr := json.Marshal(GenerateKnowledgeTreeResult{Items: items})
		return out, core.Usage{Model: usage.Model, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}, mErr
	}
	return core.Tool{
		Name:        "generate_knowledge_tree",
		Description: "Generates structured knowledge tree items from document chunks based on a brief and optional instructions.",
		IOSchema:    schema,
		Run:         run,
	}, nil
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
		DocumentID:    args.DocumentID,
		Instruction:   args.Instruction,
		Chunks:        args.Chunks,
		ExistingNodes: args.ExistingNodes,
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
