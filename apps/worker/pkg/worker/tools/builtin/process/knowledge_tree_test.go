package process

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

type knowledgeTreeFakeLLM struct {
	raw   json.RawMessage
	usage llm.Usage
	err   error
	req   llm.StructuredRequest
}

func (f *knowledgeTreeFakeLLM) GenerateStructured(_ context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	f.req = req
	return f.raw, f.usage, f.err
}

func (f *knowledgeTreeFakeLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "", llm.Usage{}, errors.New("unexpected GenerateText call")
}

func TestGenerateKnowledgeTree_ObjectResponseAndUsage(t *testing.T) {
	want := domain.GeneratedTreeItem{
		LocalID:        "item_1",
		Title:          "Authentication",
		Level:          1,
		Description:    "Token lifecycle",
		Content:        "<p>Token lifecycle</p>",
		SourceChunkIDs: []string{"doc_1_chunk_0"},
	}
	raw, err := json.Marshal(GenerateKnowledgeTreeResult{Items: []domain.GeneratedTreeItem{want}})
	if err != nil {
		t.Fatal(err)
	}
	fake := &knowledgeTreeFakeLLM{
		raw:   raw,
		usage: llm.Usage{Model: "fake-model", InputTokens: 12, OutputTokens: 34},
	}

	items, usage, err := GenerateKnowledgeTree(context.Background(), fake, GenerateKnowledgeTreeArgs{
		DocumentID: "doc_1",
		Chunks: []domain.Chunk{{ChunkIndex: 0, Heading: "Authentication", Text: "Tokens expire."}},
	})
	if err != nil {
		t.Fatalf("GenerateKnowledgeTree returned error: %v", err)
	}
	if len(items) != 1 || items[0].Title != want.Title || items[0].LocalID != want.LocalID {
		t.Fatalf("unexpected items: %#v", items)
	}
	if usage != fake.usage {
		t.Fatalf("usage = %#v, want %#v", usage, fake.usage)
	}
	if strings.TrimSpace(fake.req.SystemPrompt) == "" || strings.TrimSpace(fake.req.UserPrompt) == "" {
		t.Fatalf("rendered prompts must be non-empty: %#v", fake.req)
	}
	if fake.req.Schema == nil {
		t.Fatal("structured request must include an output schema")
	}
}

func TestGenerateKnowledgeTree_AcceptsTopLevelArray(t *testing.T) {
	want := []domain.GeneratedTreeItem{{
		LocalID:        "item_1",
		Title:          "認証",
		Level:          1,
		Description:    "認証方式",
		Content:        "<p>認証方式</p>",
		SourceChunkIDs: []string{"doc_1_chunk_0"},
	}}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	items, _, err := GenerateKnowledgeTree(context.Background(), &knowledgeTreeFakeLLM{raw: raw}, GenerateKnowledgeTreeArgs{
		DocumentID: "doc_1",
		Chunks:     []domain.Chunk{{ChunkIndex: 0, Heading: "認証", Text: "OAuthを使う。"}},
	})
	if err != nil {
		t.Fatalf("GenerateKnowledgeTree returned error: %v", err)
	}
	if len(items) != 1 || items[0].Title != "認証" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestGenerateKnowledgeTree_RejectsMalformedJSON(t *testing.T) {
	_, _, err := GenerateKnowledgeTree(context.Background(), &knowledgeTreeFakeLLM{raw: json.RawMessage(`{"items":`)}, GenerateKnowledgeTreeArgs{
		DocumentID: "doc_1",
		Chunks:     []domain.Chunk{{ChunkIndex: 0, Text: "content"}},
	})
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

func TestGenerateKnowledgeTree_RejectsEmptyItems(t *testing.T) {
	_, _, err := GenerateKnowledgeTree(context.Background(), &knowledgeTreeFakeLLM{raw: json.RawMessage(`{"items":[]}`)}, GenerateKnowledgeTreeArgs{
		DocumentID: "doc_1",
		Chunks:     []domain.Chunk{{ChunkIndex: 0, Text: "content"}},
	})
	if err == nil || !strings.Contains(err.Error(), "llm returned no items") {
		t.Fatalf("expected empty-items error, got %v", err)
	}
}

func TestGenerateKnowledgeTreeTool_FallsBackOnLLMError(t *testing.T) {
	fake := &knowledgeTreeFakeLLM{
		usage: llm.Usage{Model: "fake-model", InputTokens: 7, OutputTokens: 0},
		err:   errors.New("provider unavailable"),
	}
	tool, err := NewGenerateKnowledgeTreeTool(&base.Context{LLM: fake})
	if err != nil {
		t.Fatalf("NewGenerateKnowledgeTreeTool: %v", err)
	}
	input, err := json.Marshal(GenerateKnowledgeTreeArgs{
		DocumentID: "doc_1",
		Chunks: []domain.Chunk{{
			ChunkIndex: 0,
			Heading:    "",
			Text:       `<script>alert("x")</script>`,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, usage, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("tool.Run returned error: %v", err)
	}
	var out GenerateKnowledgeTreeResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("fallback item count = %d, want 1", len(out.Items))
	}
	item := out.Items[0]
	if item.Title != "Section 1" {
		t.Fatalf("fallback title = %q, want Section 1", item.Title)
	}
	if item.Content != `<p>&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;</p>` {
		t.Fatalf("fallback content was not escaped: %q", item.Content)
	}
	if len(item.SourceChunkIDs) != 1 || item.SourceChunkIDs[0] != "doc_1_chunk_0" {
		t.Fatalf("unexpected source chunk ids: %#v", item.SourceChunkIDs)
	}
	if usage.Model != fake.usage.Model || usage.InputTokens != fake.usage.InputTokens || usage.OutputTokens != fake.usage.OutputTokens {
		t.Fatalf("usage = %#v, want %#v", usage, fake.usage)
	}
}

func TestGenerateKnowledgeTree_ReturnsProviderErrorAndUsage(t *testing.T) {
	providerErr := errors.New("rate limited")
	fake := &knowledgeTreeFakeLLM{
		usage: llm.Usage{Model: "fake-model", InputTokens: 21},
		err:   providerErr,
	}

	_, usage, err := GenerateKnowledgeTree(context.Background(), fake, GenerateKnowledgeTreeArgs{
		DocumentID: "doc_1",
		Chunks:     []domain.Chunk{{ChunkIndex: 0, Text: "content"}},
	})
	if !errors.Is(err, providerErr) {
		t.Fatalf("error = %v, want provider error", err)
	}
	if usage != fake.usage {
		t.Fatalf("usage = %#v, want %#v", usage, fake.usage)
	}
}
