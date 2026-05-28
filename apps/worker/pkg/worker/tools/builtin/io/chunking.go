package io

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	shareddocument "github.com/synthify/backend/apps/worker/pkg/worker/document"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

type ChunkingArgs struct {
	DocumentID string `json:"document_id"`
	FileID     string `json:"file_id"`
	RawText    string `json:"raw_text"`
}

type ChunkingResult struct {
	Chunks  []domain.Chunk `json:"chunks"`
	Outline []string       `json:"outline"`
}

func NewChunkingTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[ChunkingArgs, ChunkingResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args ChunkingArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		text := strings.TrimSpace(args.RawText)
		if text == "" {
			out, err := json.Marshal(ChunkingResult{})
			return out, core.Usage{}, err
		}

		sections := shareddocument.SplitSections(text)
		chunks := make([]domain.Chunk, 0, len(sections))
		outline := make([]string, 0, len(sections))
		for i, section := range sections {
			heading := section.Heading
			if heading == "" {
				heading = fmt.Sprintf("Section %d", i+1)
			}
			chunks = append(chunks, domain.Chunk{ChunkIndex: i, Heading: heading, Text: section.Text})
			outline = append(outline, heading)
		}

		if b != nil && b.Repo != nil {
			if b.Embedder == nil {
				return nil, core.Usage{}, fmt.Errorf("embedder is required: configure Vertex AI")
			}
			domainChunks := make([]*domain.DocumentChunk, 0, len(chunks))
			for _, chunk := range chunks {
				vec, err := b.Embedder.EmbedText(ctx, chunk.Heading+" "+chunk.Text)
				if err != nil {
					return nil, core.Usage{}, fmt.Errorf("embed chunk %d: %w", chunk.ChunkIndex, err)
				}
				domainChunks = append(domainChunks, &domain.DocumentChunk{
					ChunkID:    fmt.Sprintf("%s_chunk_%d", args.DocumentID, chunk.ChunkIndex),
					DocumentID: args.DocumentID,
					FileID:     args.FileID,
					Heading:    chunk.Heading,
					Text:       chunk.Text,
					Embedding:  vec.Slice(),
				})
			}
			if err := b.Repo.SaveDocumentChunks(ctx, args.DocumentID, domainChunks); err != nil {
				return nil, core.Usage{}, fmt.Errorf("save chunks: %w", err)
			}
		}
		out, err := json.Marshal(ChunkingResult{Chunks: chunks, Outline: outline})
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "semantic_chunking",
		Description: "Splits raw document text into semantically coherent chunks and generates an outline.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}
