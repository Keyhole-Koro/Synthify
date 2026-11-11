package io

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// SearchArgs specifies parameters for semantic search across workspace documents.
//
// @property WorkspaceID - Workspace scope for vector search.
// @property Query - Natural-language query to embed.
// @property Scope - Optional caller-provided narrower scope descriptor.
type SearchArgs struct {
	// WorkspaceID scopes vector search to a workspace.
	WorkspaceID string `json:"workspace_id"`
	// Query is the natural-language text to embed and search for.
	Query string `json:"query"`
	// Scope optionally describes a narrower search scope for callers; repository filtering currently uses WorkspaceID.
	Scope string `json:"scope"`
}

// SearchResult contains the ordered list of semantic search result items.
//
// @property Results - Closest matching chunks in repository order.
type SearchResult struct {
	// Results contains the closest matching chunks in repository search order.
	Results []SearchResultItem `json:"results"`
}

// SearchResultItem represents an individual text chunk matched by vector similarity.
//
// @property DocumentID - Source document identifier.
// @property ChunkID - Matched persisted chunk identifier.
// @property Text - Matched chunk content.
// @property Score - Normalized display score based on result rank.
type SearchResultItem struct {
	// DocumentID identifies the source document containing the chunk.
	DocumentID string `json:"document_id"`
	// ChunkID identifies the matched persisted chunk.
	ChunkID string `json:"chunk_id"`
	// Text contains the matched chunk content.
	Text string `json:"text"`
	// Score is a normalized rank score derived from the repository result position.
	Score float64 `json:"score"`
}

// NewSearchTool creates a core.Tool for semantic vector search across document chunks in a workspace.
//
// The tool embeds the query with the configured embedder and returns up to the
// repository's configured result limit, preserving repository order while
// assigning a normalized display score.
//
// Parameters:
//   - b: worker dependencies used for embedding and vector repository queries.
//
// Returns:
//   - a configured semantic_search tool;
//   - an error when the input/output schema cannot be generated.
//
// @param b - Worker dependencies used for embedding and vector repository queries.
// @return Configured semantic_search tool and optional schema-generation error.
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
			return nil, core.Usage{}, fmt.Errorf("embedder is required: configure Vertex AI")
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
