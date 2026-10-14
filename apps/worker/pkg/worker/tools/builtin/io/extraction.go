// Package io provides builtin tools for document text extraction, chunking, search, encoding repairs, and table parsing.
package io

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/sourceio"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// ExtractionArgs defines inputs for document text extraction.
//
// @property FileURI - Source object or mounted-file identifier.
// @property MimeType - MIME type selecting the extraction strategy.
// @property WorkspaceID - Workspace scope, optionally inherited from the job.
// @property DocumentID - Document scope, optionally inherited from the job.
type ExtractionArgs struct {
	// FileURI identifies the source object or mounted file to read.
	FileURI string `json:"file_uri"`
	// MimeType selects the extraction path, such as archive, text, image, or media.
	MimeType string `json:"mime_type"`
	// WorkspaceID scopes filesystem and repository operations; the job context is used when empty.
	WorkspaceID string `json:"workspace_id,omitempty"`
	// DocumentID identifies the document; the job context is used when empty.
	DocumentID string `json:"document_id,omitempty"`
}

// ExtractionResult holds the extracted raw text content of a source document.
//
// @property RawText - Normalized text, transcription, or image-derived content.
type ExtractionResult struct {
	// RawText contains normalized text, transcription, or image-derived content.
	RawText string `json:"raw_text"`
}

// NewExtractionTool creates the extract_text builtin tool for parsing text from documents and archives.
//
// The tool delegates to the same extraction path used by the deterministic
// worker pre-pass, so tool-driven and pre-pass extraction produce consistent
// source records and extracted files.
//
// Parameters:
//   - b: worker dependencies, including filesystem, repository, and optional LLM clients.
//
// Returns:
//   - a configured extract_text tool;
//   - an error when the input/output schema cannot be generated.
//
// @param b - Worker dependencies, including filesystem, repository, and optional LLM clients.
// @return Configured extract_text tool and optional schema-generation error.
func NewExtractionTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[ExtractionArgs, ExtractionResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args ExtractionArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		result, err := runExtraction(ctx, b, args)
		if err != nil {
			return nil, core.Usage{}, err
		}
		out, err := json.Marshal(result)
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "extract_text",
		Description: "Extract text from a document or archive file.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// Extract extracts text from a file without going through the agent tool path.
// Split out so the orchestrator can perform a deterministic extract pass at the start of a job
// (the agent no longer discovers the source by guessing extract_text args). It
// is the same code path the extract_text tool drives.
//
// Parameters:
//   - ctx: request context propagated to storage, repository, and LLM operations.
//   - b: worker dependencies used to read and persist extraction artifacts.
//   - fileURI: source object or mounted-file identifier.
//   - mimeType: source MIME type used to select the extraction strategy.
//   - workspaceID: workspace scope, used when the job context does not provide it.
//   - documentID: document scope, used when the job context does not provide it.
//
// Returns the extracted text, or an error when the source cannot be loaded or
// the selected extractor fails.
//
// @param ctx - Request context propagated to storage, repository, and LLM operations.
// @param b - Worker dependencies used to read and persist extraction artifacts.
// @param fileURI - Source object or mounted-file identifier.
// @param mimeType - Source MIME type used to select the extraction strategy.
// @param workspaceID - Workspace scope, used when the job context does not provide it.
// @param documentID - Document scope, used when the job context does not provide it.
// @return Extracted text and an optional extraction error.
func Extract(ctx context.Context, b *base.Context, fileURI, mimeType, workspaceID, documentID string) (string, error) {
	res, err := runExtraction(ctx, b, ExtractionArgs{
		FileURI:     fileURI,
		MimeType:    mimeType,
		WorkspaceID: workspaceID,
		DocumentID:  documentID,
	})
	if err != nil {
		return "", err
	}
	return res.RawText, nil
}

// runExtraction is the unwrapped extraction logic. Split from the factory so
// the unified Run closure stays small and the existing per-file-shape branches
// (zip / media / text) remain testable independently.
func runExtraction(ctx context.Context, b *base.Context, args ExtractionArgs) (ExtractionResult, error) {
	wsID := args.WorkspaceID
	if wsID == "" && b != nil && b.Job != nil {
		wsID = b.Job.WorkspaceID
	}
	docID := args.DocumentID
	if docID == "" && b != nil && b.Job != nil {
		docID = b.Job.DocumentID
	}

	// Filename is intentionally left empty: the worker reads from the FUSE mount,
	// not from FileURI, so the authoritative name is the on-disk one under
	// source/ (original.{ext}). PopulateSourceFile adopts it. Deriving the name
	// from FileURI would yield "{document_id}" and corrupt the extracted path and
	// the document_files record.
	source := domain.SourceFile{
		URI:         args.FileURI,
		MimeType:    args.MimeType,
		WorkspaceID: wsID,
		DocumentID:  docID,
	}
	if err := sourceio.Load(ctx, b.FS, &source); err != nil {
		return ExtractionResult{}, err
	}

	// Working tree is extracted/, kept separate from the read-only source/ where
	// the original lives, so the document root is always a directory.
	if b.FS != nil {
		dir := b.FS.ExtractedDir(wsID, docID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return ExtractionResult{}, fmt.Errorf("failed to create extracted dir: %w", err)
		}
	}

	if isZip(source.MimeType, source.Filename) {
		return processZip(ctx, b, source)
	}

	destPath := ""
	if b.FS != nil {
		destPath = filepath.Join(b.FS.ExtractedDir(wsID, docID), source.Filename)
		if err := os.WriteFile(destPath, source.Content, 0644); err != nil {
			b.Logger.Error("extraction.save_single_file_failed", "error", err.Error(), "filename", source.Filename)
		}
	}

	var fileRecord *domain.DocumentFile
	if b != nil && b.Repo != nil {
		var err error
		fileRecord, err = b.Repo.CreateDocumentFile(ctx, source.DocumentID, source.Filename, source.MimeType, int64(len(source.Content)))
		if err != nil {
			b.Logger.Error("extraction.create_db_record_failed", "error", err.Error(), "filename", source.Filename)
		}
	}

	if isMediaFile(source.MimeType) {
		return processMedia(ctx, b, source, fileRecord)
	}
	if isImageFile(source.MimeType) {
		return processImage(ctx, b, source, fileRecord)
	}
	return processSingleTextFile(ctx, b, source, fileRecord)
}

func isZip(mimeType, filename string) bool {
	return mimeType == "application/zip" ||
		mimeType == "application/x-zip-compressed" ||
		strings.HasSuffix(strings.ToLower(filename), ".zip")
}

func isMediaFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/") || strings.HasPrefix(mimeType, "video/")
}

func isImageFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func processSingleTextFile(ctx context.Context, b *base.Context, source domain.SourceFile, record *domain.DocumentFile) (ExtractionResult, error) {
	text := string(source.Content)
	text = strings.ReplaceAll(text, "\x00", "")

	fileID := ""
	if record != nil {
		fileID = record.FileID
	}

	resultText := strings.TrimSpace(text)
	if fileID != "" {
		resultText = fmt.Sprintf("--- File: %s (ID: %s) ---\n%s", source.Filename, fileID, resultText)
	}

	return ExtractionResult{RawText: resultText}, nil
}

func processMedia(ctx context.Context, b *base.Context, source domain.SourceFile, record *domain.DocumentFile) (ExtractionResult, error) {
	if b == nil || b.LLM == nil {
		return ExtractionResult{}, fmt.Errorf("%w: LLM client not configured for transcription", domain.ErrCritical)
	}

	text, _, err := b.LLM.GenerateText(ctx, llm.TextRequest{
		SystemPrompt: "You are an expert transcription and video analysis assistant. Your task is to provide a high-quality, verbatim transcription of the provided media file. If it is a video, describe key visual transitions with timestamps as well.",
		UserPrompt:   "Please transcribe this media file. Use MM:SS format for timestamps.",
		SourceFiles:  []domain.SourceFile{source},
	})
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("transcription failed: %w", err)
	}

	fileID := ""
	if record != nil {
		fileID = record.FileID
	}

	resultText := strings.TrimSpace(text)
	if fileID != "" {
		resultText = fmt.Sprintf("--- Media File: %s (ID: %s) ---\n%s", source.Filename, fileID, resultText)
	}

	return ExtractionResult{RawText: resultText}, nil
}

// processImage extracts text from an image (PNG, JPEG, diagrams, screenshots)
// by handing the raw bytes to the vision-capable LLM. Without this branch images
// fall through to processSingleTextFile, which stringifies the binary and returns
// noise — the agent then re-calls extract_text in a loop trying to get usable
// text, hitting the Cloud Run request timeout. Mirrors processMedia: same
// SourceFiles plumbing, same fileID labeling, only the prompt differs.
func processImage(ctx context.Context, b *base.Context, source domain.SourceFile, record *domain.DocumentFile) (ExtractionResult, error) {
	if b == nil || b.LLM == nil {
		return ExtractionResult{}, fmt.Errorf("%w: LLM client not configured for image extraction", domain.ErrCritical)
	}

	text, _, err := b.LLM.GenerateText(ctx, llm.TextRequest{
		SystemPrompt: "You are an expert at reading images. Extract all legible text verbatim. If the image is a diagram, chart, or screenshot, also describe its structure (nodes, edges, labels, relationships) in plain text so the content can be reconstructed. Do not invent content that is not visible.",
		UserPrompt:   "Extract the text and structural content from this image.",
		SourceFiles:  []domain.SourceFile{source},
	})
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("image extraction failed: %w", err)
	}

	fileID := ""
	if record != nil {
		fileID = record.FileID
	}

	// Register the image so HTML-generation tools can embed it via an [[img:N]]
	// placeholder, which Render turns into an <img data-file-id> marker. The web
	// client later exchanges the file id for a short-lived signed URL. The alt
	// text is the extracted content, doubling as a prompt-hint description.
	// Registration is best-effort: without a file id the image is simply not
	// offered for embedding.
	if fileID != "" {
		b.Images.Register(fileID, imageAltText(source.Filename, text))
	}

	resultText := strings.TrimSpace(text)
	if fileID != "" {
		resultText = fmt.Sprintf("--- Image File: %s (ID: %s) ---\n%s", source.Filename, fileID, resultText)
	}

	return ExtractionResult{RawText: resultText}, nil
}

// imageAltText derives concise alt text for a registered image: the filename
// plus a trimmed lead of the extracted content, so the prompt hint and the
// rendered <img alt> describe what the image shows.
func imageAltText(filename, extracted string) string {
	lead := strings.TrimSpace(extracted)
	if len(lead) > 160 {
		lead = strings.TrimSpace(lead[:160])
	}
	switch {
	case filename != "" && lead != "":
		return filename + ": " + lead
	case lead != "":
		return lead
	default:
		return filename
	}
}

func processZip(ctx context.Context, b *base.Context, source domain.SourceFile) (ExtractionResult, error) {
	if b.FS == nil || b.FS.MountPath == "" {
		return ExtractionResult{}, fmt.Errorf("%w: FUSE mount required for ZIP extraction", domain.ErrCritical)
	}

	// Expand into extracted/, separate from the source/ archive. Previously this
	// targeted {ws}/{doc} directly, colliding with the source object of the same
	// path; the source/extracted split removes that collision.
	extractDir := b.FS.ExtractedDir(source.WorkspaceID, source.DocumentID)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return ExtractionResult{}, fmt.Errorf("failed to create extraction dir: %w", err)
	}

	r, err := zip.NewReader(bytes.NewReader(source.Content), int64(len(source.Content)))
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("%w: invalid zip file format", domain.ErrJobError)
	}

	var combinedText strings.Builder
	extractedCount := 0

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		relPath := f.Name
		destPath := filepath.Join(extractDir, relPath)

		// Create parent dirs
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			b.Logger.Error("extraction.create_parent_dir_failed", "error", err.Error(), "path", relPath)
			continue
		}

		// Extract file
		rc, err := f.Open()
		if err != nil {
			b.Logger.Error("extraction.open_zip_file_failed", "error", err.Error(), "path", relPath)
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			b.Logger.Error("extraction.read_zip_file_failed", "error", err.Error(), "path", relPath)
			continue
		}

		if err := os.WriteFile(destPath, content, 0644); err != nil {
			b.Logger.Error("extraction.write_extracted_file_failed", "error", err.Error(), "path", relPath)
			continue
		}

		extractedCount++

		// Detect MIME and Record to DB
		mimeType := http.DetectContentType(content)
		var fileRecord *domain.DocumentFile
		if b != nil && b.Repo != nil {
			var err error
			fileRecord, err = b.Repo.CreateDocumentFile(ctx, source.DocumentID, relPath, mimeType, int64(len(content)))
			if err != nil {
				b.Logger.Error("extraction.create_db_record_failed", "error", err.Error(), "path", relPath)
			}
		}

		// Only append text files to the combined result for the LLM's initial view
		if isLikelyText(content, mimeType) {
			fileID := ""
			if fileRecord != nil {
				fileID = fileRecord.FileID
			}
			combinedText.WriteString(fmt.Sprintf("\n--- File: %s (ID: %s) ---\n", relPath, fileID))
			combinedText.Write(content)
			combinedText.WriteString("\n")
		}
	}

	if extractedCount == 0 {
		return ExtractionResult{}, fmt.Errorf("%w: zip file contained no valid files to extract", domain.ErrJobError)
	}

	return ExtractionResult{RawText: strings.TrimSpace(combinedText.String())}, nil
}

func isLikelyText(content []byte, mimeType string) bool {
	if len(content) == 0 {
		return true
	}

	// 1. Binary heuristic: scan first 512 bytes for NULL bytes.
	checkLen := 512
	if len(content) < checkLen {
		checkLen = len(content)
	}
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return false // Likely binary
		}
	}

	// 2. Known text types are usually text
	if strings.HasPrefix(mimeType, "text/") ||
		strings.Contains(mimeType, "json") ||
		strings.Contains(mimeType, "markdown") {
		return true
	}

	// 3. Last resort: must be valid UTF-8
	return utf8.Valid(content)
}
