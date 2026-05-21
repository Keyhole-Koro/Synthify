package io

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

type SearchArgs struct {
	WorkspaceID string `json:"workspace_id"`
	Query       string `json:"query"`
	Scope       string `json:"scope"`
}

type SearchResult struct {
	Results []SearchResultItem `json:"results"`
}

type SearchResultItem struct {
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

func NewSearchTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[SearchArgs, SearchResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args SearchArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		if b == nil || b.Repo == nil {
			return nil, core.Usage{}, fmt.Errorf("repository is not configured")
		}
		if b.Embedder == nil {
			return nil, core.Usage{}, fmt.Errorf("embedder is required: configure GEMINI_API_KEY")
		}
		vec, err := b.Embedder.EmbedText(ctx, args.Query)
		if err != nil {
			return nil, core.Usage{}, fmt.Errorf("embed query: %w", err)
		}
		chunks, err := b.Repo.SearchRelatedChunksByVector(ctx, args.WorkspaceID, vec.Slice(), 8)
		if err != nil {
			return nil, core.Usage{}, fmt.Errorf("vector search: %w", err)
		}
		results := make([]SearchResultItem, 0, len(chunks))
		for i, chunk := range chunks {
			score := 1.0 - float64(i)/float64(len(chunks)+1)
			results = append(results, SearchResultItem{
				DocumentID: chunk.DocumentID,
				ChunkID:    chunk.ChunkID,
				Text:       chunk.Text,
				Score:      score,
			})
		}
		out, err := json.Marshal(SearchResult{Results: results})
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "semantic_search",
		Description: "Performs workspace-wide RAG. Finds relevant information across the current document or all documents in the same workspace.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}
