// Package chat provides the API's grounded-answer clients for workspace chat.
//
// This deliberately re-implements a minimal Gemini client rather than importing
// apps/worker/pkg/worker/llm, which pulls in the ADK / agent / storage
// dependency tree. Chat needs one structured call, so a small standalone client
// keeps the API binary lean — the same tradeoff as
// apps/api/internal/infrastructure/worker/dispatcher.go.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/synthify/backend/apps/api/internal/application"
)

// GeminiAnswerer implements application.ChatAnswerer against the Gemini API.
type GeminiAnswerer struct {
	client *genai.Client
	model  string
}

type GeminiAnswererConfig struct {
	// APIKey uses the Gemini API backend when set; otherwise the client falls
	// back to Vertex AI with Project / Location.
	APIKey   string
	Project  string
	Location string
	Model    string
}

func NewGeminiAnswerer(ctx context.Context, cfg GeminiAnswererConfig) (*GeminiAnswerer, error) {
	var clientCfg *genai.ClientConfig
	if cfg.APIKey != "" {
		clientCfg = &genai.ClientConfig{Backend: genai.BackendGeminiAPI, APIKey: cfg.APIKey}
	} else {
		clientCfg = &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  cfg.Project,
			Location: cfg.Location,
		}
	}
	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("init gemini client: %w", err)
	}
	return &GeminiAnswerer{client: client, model: cfg.Model}, nil
}

// answerSchema constrains the model to an answer plus citation ids. The ids are
// still only claims — application.validateCitations checks them against the
// candidate set before any of them becomes a stored citation.
var answerSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"answer": {
			Type:        genai.TypeString,
			Description: "The answer to the user's question, in the language of the question.",
		},
		"source_ids": {
			Type:        genai.TypeArray,
			Items:       &genai.Schema{Type: genai.TypeString},
			Description: "source_id values from the provided sources that support the answer. Empty when answering from general knowledge.",
		},
	},
	Required: []string{"answer", "source_ids"},
}

const answerSystemPrompt = `You answer questions about a specific workspace.
Sources are excerpts from its documents and pages from its knowledge tree.

Rules:
- Prefer the provided sources. When they answer the question, cite them.
- Cite with the exact source_id values given. Never invent a source_id.
- When the sources do not cover the question, you may answer from general
  knowledge — but say so, and cite nothing rather than citing something
  loosely related.
- When no sources are provided at all, just answer from general knowledge.
- Answer in the same language the question was asked in.
- Be concise: a short paragraph unless the question needs more.`

func (a *GeminiAnswerer) Answer(ctx context.Context, req application.ChatAnswerRequest) (application.ChatAnswer, error) {
	if a.client == nil {
		return application.ChatAnswer{}, fmt.Errorf("gemini client not initialized")
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: answerSystemPrompt}}},
		ResponseMIMEType:  "application/json",
		ResponseSchema:    answerSchema,
		Temperature:       genai.Ptr(float32(0.2)),
	}

	contents := []*genai.Content{{
		Role:  "user",
		Parts: []*genai.Part{{Text: buildPrompt(req)}},
	}}

	res, err := a.client.Models.GenerateContent(ctx, a.model, contents, config)
	if err != nil {
		return application.ChatAnswer{}, fmt.Errorf("gemini generate: %w", err)
	}
	if res == nil || len(res.Candidates) == 0 || res.Candidates[0].Content == nil {
		return application.ChatAnswer{}, fmt.Errorf("gemini returned no content")
	}

	var raw strings.Builder
	for _, part := range res.Candidates[0].Content.Parts {
		raw.WriteString(part.Text)
	}

	var parsed struct {
		Answer    string   `json:"answer"`
		SourceIDs []string `json:"source_ids"`
	}
	if err := json.Unmarshal([]byte(raw.String()), &parsed); err != nil {
		return application.ChatAnswer{}, fmt.Errorf("parse gemini answer: %w", err)
	}

	return application.ChatAnswer{
		Text:           parsed.Answer,
		SourceIDs: parsed.SourceIDs,
	}, nil
}

// buildPrompt renders history and sources into the user turn. Source text is
// included here but must never be persisted into retrieval_snapshot_json.
func buildPrompt(req application.ChatAnswerRequest) string {
	var b strings.Builder

	if len(req.History) > 0 {
		b.WriteString("## Conversation so far\n\n")
		for _, msg := range req.History {
			b.WriteString(msg.Role)
			b.WriteString(": ")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(req.Candidates) == 0 {
		b.WriteString("## Sources\n\n(none — answer from general knowledge)\n\n")
	} else {
		b.WriteString("## Sources\n\n")
	}
	for _, c := range req.Candidates {
		b.WriteString("source_id: ")
		b.WriteString(c.SourceID())
		b.WriteString("\nsource: ")
		b.WriteString(c.Label())
		b.WriteString("\n")
		b.WriteString(c.Text)
		b.WriteString("\n\n")
	}

	b.WriteString("## Question\n\n")
	b.WriteString(req.Question)
	return b.String()
}

// ModelID reports the Gemini model that actually served the request.
func (a *GeminiAnswerer) ModelID() string { return a.model }
