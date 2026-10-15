package io

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	shareddocument "github.com/synthify/backend/apps/worker/pkg/worker/document"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/internal/domain"
)

// ChunkingArgs defines input text and identifiers for semantic document chunking.
//
// @property DocumentID - Source document identifier.
// @property FileID - Source file identifier.
// @property RawText - Extracted text to split into semantic sections.
type ChunkingArgs struct {
	// DocumentID identifies the source document whose text is being chunked.
	DocumentID string `json:"document_id"`
	// FileID identifies the source file associated with the extracted text.
	FileID string `json:"file_id"`
	// RawText contains the extracted document text to split into semantic sections.
	RawText string `json:"raw_text"`
}

// ChunkingResult returns parsed text chunks and an outline of extracted headings.
//
// @property Chunks - Lightweight chunk records for the caller.
// @property Outline - Normalized headings in source order.
type ChunkingResult struct {
	// Chunks contains lightweight chunk records returned to the tool caller.
	Chunks []domain.Chunk `json:"chunks"`
	// Outline contains the normalized heading for each chunk in source order.
	Outline []string `json:"outline"`
}

// NewChunkingTool builds the semantic_chunking tool for splitting raw text into structured chunks.
//
// The tool parses ChunkingArgs, delegates to ChunkAndSave, and serializes the
// lightweight chunk and outline views into ChunkingResult. When a repository is
// configured, ChunkAndSave also creates embeddings and persists document
// chunks through the supplied context.
//
// Parameters:
//   - b: worker dependencies used for optional embedding and persistence.
//
// Returns:
//   - a configured semantic_chunking tool;
//   - an error when the input/output schema cannot be generated.
//
// @param b - Worker dependencies used for optional embedding and persistence.
// @return Configured semantic_chunking tool and optional schema-generation error.
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
		chunks, outline, _, err := ChunkAndSave(ctx, b, args.DocumentID, args.FileID, args.RawText)
		if err != nil {
			return nil, core.Usage{}, err
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

// ChunkAndSave splits rawText into sections, embeds each, and persists them as
// DocumentChunks. It returns the lightweight chunk/outline views (for the
// semantic_chunking tool response) plus the persisted DocumentChunks (which the
// orchestrator feeds into the Document Map). Shared by the tool and the
// orchestrator's deterministic pre-pass so both produce identical chunk_ids.
//
// Empty rawText yields no chunks (not an error): a media/image document may have
// no extractable text. When b.Repo is set, b.Embedder must be configured.
//
// Parameters:
//   - ctx: request context used for embedding and repository operations.
//   - b: worker dependencies; nil or a nil repository disables persistence.
//   - documentID: source document identifier used for persisted chunk IDs.
//   - fileID: source file identifier stored with each persisted chunk.
//   - rawText: extracted text to split and optionally embed.
//
// Returns:
//   - chunks: lightweight chunk records for the tool response;
//   - outline: normalized section headings in source order;
//   - persisted: repository records when persistence is enabled, otherwise nil;
//   - error: validation, embedding, or persistence failure.
//
// @param ctx - Request context used for embedding and repository operations.
// @param b - Worker dependencies; nil or a nil repository disables persistence.
// @param documentID - Source document identifier used for persisted chunk IDs.
// @param fileID - Source file identifier stored with each persisted chunk.
// @param rawText - Extracted text to split and optionally embed.
// @return Lightweight chunks, outline, persisted records, and an optional error.
func ChunkAndSave(ctx context.Context, b *base.Context, documentID, fileID, rawText string) ([]domain.Chunk, []string, []*domain.DocumentChunk, error) {
	text := strings.TrimSpace(rawText)
	if text == "" {
		return nil, nil, nil, nil
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

	var domainChunks []*domain.DocumentChunk
	if b != nil && b.Repo != nil {
		if b.Embedder == nil {
			return nil, nil, nil, fmt.Errorf("embedder is required: configure Vertex AI")
		}
		domainChunks = make([]*domain.DocumentChunk, 0, len(chunks))
		for _, chunk := range chunks {
			vec, err := b.Embedder.EmbedText(ctx, chunk.Heading+" "+chunk.Text)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("embed chunk %d: %w", chunk.ChunkIndex, err)
			}
			domainChunks = append(domainChunks, &domain.DocumentChunk{
				ChunkID:    fmt.Sprintf("%s_chunk_%d", documentID, chunk.ChunkIndex),
				DocumentID: documentID,
				FileID:     fileID,
				Heading:    chunk.Heading,
				Text:       chunk.Text,
				Embedding:  vec.Slice(),
			})
		}
		if err := b.Repo.SaveDocumentChunks(ctx, documentID, domainChunks); err != nil {
			return nil, nil, nil, fmt.Errorf("save chunks: %w", err)
		}
	}
	return chunks, outline, domainChunks, nil
}
