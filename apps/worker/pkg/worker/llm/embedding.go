package llm

import (
	"context"
	"fmt"

	pgvector "github.com/pgvector/pgvector-go"
	"google.golang.org/genai"
)

// embeddingModel is the default Gemini embedding model used for vector index generation.
const embeddingModel = "gemini-embedding-2"

// EmbedText generates a 768-dimensional dense vector representation of the provided text.
func (c *GeminiClient) EmbedText(ctx context.Context, text string) (pgvector.Vector, error) {
	if c.client == nil {
		return pgvector.Vector{}, fmt.Errorf("gemini client not initialized")
	}
	contents := []*genai.Content{genai.NewContentFromText(text, genai.RoleUser)}
	dim := int32(768)
	cfg := &genai.EmbedContentConfig{OutputDimensionality: &dim}
	res, err := c.client.Models.EmbedContent(ctx, embeddingModel, contents, cfg)
	if err != nil {
		return pgvector.Vector{}, fmt.Errorf("embed content: %w", err)
	}
	if len(res.Embeddings) == 0 {
		return pgvector.Vector{}, fmt.Errorf("embed content: empty response")
	}
	return pgvector.NewVector(res.Embeddings[0].Values), nil
}
