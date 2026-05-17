package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/eval/runner"
	"github.com/synthify/backend/packages/shared/domain"
)

func TestWriteJSON(t *testing.T) {
	results := []runner.Result{{
		CaseName:     "case_1",
		Tool:         runner.ToolSynthesis,
		Passed:       true,
		SchemaValid:  true,
		ItemCount:    2,
		MaxDepth:     1,
		DurationMS:   123,
		Model:        "fake",
		InputTokens:  10,
		OutputTokens: 20,
		Items: []domain.SynthesizedItem{{
			LocalID: "item_1",
			Title:   "Authentication",
			Level:   1,
		}},
	}}

	var buf bytes.Buffer
	if err := Write(&buf, "json", results); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	var decoded []runner.Result
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(decoded) != 1 || decoded[0].CaseName != "case_1" {
		t.Fatalf("unexpected json report: %#v", decoded)
	}
	if len(decoded[0].Items) != 1 || decoded[0].Items[0].Title != "Authentication" {
		t.Fatalf("items missing from json report: %#v", decoded[0].Items)
	}
}

func TestWriteJSONIncludesFailedInput(t *testing.T) {
	results := []runner.Result{{
		CaseName: "case_1",
		Passed:   false,
		FailedInput: &runner.InputSnapshot{
			DocumentID: "doc_1",
			ChunksPath: "chunks.json",
		},
	}}

	var buf bytes.Buffer
	if err := Write(&buf, "json", results); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if !strings.Contains(buf.String(), `"failed_input"`) || !strings.Contains(buf.String(), `"document_id": "doc_1"`) {
		t.Fatalf("failed input missing from json report: %s", buf.String())
	}
}

func TestWriteJSONDoesNotEscapeHTML(t *testing.T) {
	results := []runner.Result{{
		CaseName: "case_1",
		Items: []domain.SynthesizedItem{{
			LocalID: "item_1",
			Title:   "HTML",
			Content: `<p class="lede">A & B</p>`,
		}},
	}}

	var buf bytes.Buffer
	if err := Write(&buf, "json", results); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, `\u003c`) || strings.Contains(out, `\u003e`) || strings.Contains(out, `\u0026`) {
		t.Fatalf("html was escaped in json report: %s", out)
	}
	if !strings.Contains(out, `<p class=\"lede\">A & B</p>`) {
		t.Fatalf("html missing from json report: %s", out)
	}
}

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "table", []runner.Result{{CaseName: "case_1", Passed: true}})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CASE") || !strings.Contains(out, "case_1") {
		t.Fatalf("unexpected table output: %q", out)
	}
}

func TestWriteTableIncludesFailedInputSummary(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "table", []runner.Result{{
		CaseName: "case_1",
		Passed:   false,
		FailedInput: &runner.InputSnapshot{
			DocumentID: "doc_1",
			ChunksPath: "chunks.json",
		},
	}})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "FAILED_INPUT") || !strings.Contains(out, "document_id=doc_1") {
		t.Fatalf("failed input missing from table output: %q", out)
	}
}

func TestWriteUnsupportedFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, "xml", nil); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
