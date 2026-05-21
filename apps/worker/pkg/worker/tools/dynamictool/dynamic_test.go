package dynamictool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
	"github.com/synthify/backend/packages/shared/domain"
)

type fakeEngine struct {
	lang transform.Language
	res  transform.ExecResult
}

func (e fakeEngine) Language() transform.Language {
	return e.lang
}

func (e fakeEngine) Execute(transform.ExecRequest) transform.ExecResult {
	return e.res
}

func testTool() *domain.DynamicTool {
	return &domain.DynamicTool{
		Name:        "normalize_title",
		Description: "normalize a title",
		Language:    string(transform.LangStarlark),
		Code:        "def transform(input): return input",
		IOSchema: `{
			"input": {"type": "object", "properties": {"title": {"type": "string"}}, "required": ["title"]},
			"output": {"type": "object", "properties": {"title": {"type": "string"}}, "required": ["title"]}
		}`,
		Status: domain.ToolStatusActive,
	}
}

func TestFromDomainBuildsRunnableTool(t *testing.T) {
	tool, err := FromDomain(testTool(), fakeEngine{
		lang: transform.LangStarlark,
		res:  transform.ExecResult{Status: transform.StatusOK, Output: `{"title":"HELLO"}`},
	})
	if err != nil {
		t.Fatalf("FromDomain: %v", err)
	}
	out, _, err := tool.Run(context.Background(), json.RawMessage(`{"title":"hello"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(out) != `{"title":"HELLO"}` {
		t.Fatalf("output = %s", out)
	}
}

func TestFromDomainRejectsInactiveTool(t *testing.T) {
	dt := testTool()
	dt.Status = domain.ToolStatusDisabled
	if _, err := FromDomain(dt, fakeEngine{lang: transform.LangStarlark}); err == nil {
		t.Fatal("expected inactive tool error")
	}
}

func TestFromDomainRejectsBadSchema(t *testing.T) {
	dt := testTool()
	dt.IOSchema = `{"input":`
	if _, err := FromDomain(dt, fakeEngine{lang: transform.LangStarlark}); err == nil {
		t.Fatal("expected schema error")
	}
}

func TestRunReportsEngineLanguageMismatch(t *testing.T) {
	tool, err := FromDomain(testTool(), fakeEngine{lang: transform.LangPython})
	if err != nil {
		t.Fatalf("FromDomain: %v", err)
	}
	_, _, err = tool.Run(context.Background(), json.RawMessage(`{"title":"hello"}`))
	if err == nil || !strings.Contains(err.Error(), "language") {
		t.Fatalf("Run error = %v, want language mismatch", err)
	}
}
