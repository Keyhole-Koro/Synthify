package runner

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
)

// chunkHeadingLLM is a prompt-driven fake: it echoes the chunk headings the
// renderer embedded in the user prompt back as one level-1 tree item each. It
// takes no case knowledge — whatever titles actually reached the model are the
// titles it returns.
//
// This makes the smoke test a real end-to-end plumbing check of the committed
// cases WITHOUT a live LLM: YAML parse, fixture path resolution, input schema
// validation, prompt render (chunks embedded), output schema validation, and
// the expect.json rules (count_gte / tree_depth_lte / contains_all) all run
// against the real apps/eval/cases + apps/eval/testdata. It does NOT judge
// model quality — only that the harness wires together and that each case's
// expectations are satisfiable from the fixture headings.
type chunkHeadingLLM struct{}

var chunkLine = regexp.MustCompile(`(?m)^\[\d+\]\s+(.+?)\s*$`)

func (chunkHeadingLLM) GenerateStructured(_ context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	matches := chunkLine.FindAllStringSubmatch(req.UserPrompt, -1)
	items := make([]domain.GeneratedTreeItem, 0, len(matches))
	for i, m := range matches {
		items = append(items, domain.GeneratedTreeItem{
			LocalID: "item_" + itoa(i+1),
			Title:   m[1],
			Level:   1,
		})
	}
	raw, err := json.Marshal(map[string]any{"items": items})
	return raw, llm.Usage{Model: "fake-chunk-heading", InputTokens: 1, OutputTokens: 1}, err
}

func (chunkHeadingLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "", llm.Usage{}, nil
}

func itoa(i int) string {
	return string(rune('0'+i%10)) // single-digit ids are enough for the fixtures
}

// TestCommittedCasesPassWithFakeLLM runs every real case in apps/eval/cases
// through the runner with the prompt-driven fake and asserts all pass. It
// guards against a case file, fixture, or expect rule drifting into a state the
// harness cannot even satisfy offline (e.g. a contains_all value no fixture
// heading matches, a bad JSON path, an unloadable chunks fixture).
func TestCommittedCasesPassWithFakeLLM(t *testing.T) {
	casesDir := filepath.Join("..", "cases")
	files, err := CaseFiles(casesDir)
	if err != nil {
		t.Fatalf("CaseFiles(%s): %v", casesDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no committed cases found under %s", casesDir)
	}

	results, err := Runner{LLM: chunkHeadingLLM{}}.RunCaseFiles(context.Background(), files)
	if err != nil {
		t.Fatalf("RunCaseFiles: %v", err)
	}
	if len(results) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(results))
	}
	for _, res := range results {
		if !res.SchemaValid {
			t.Errorf("case %q: output failed schema validation", res.CaseName)
		}
		if !res.Passed {
			t.Errorf("case %q: not passed (error=%q, output=%s)", res.CaseName, res.Error, res.Output)
		}
		t.Logf("case %q: passed=%t schema_valid=%t prompt_source=%s", res.CaseName, res.Passed, res.SchemaValid, res.PromptSource)
	}
}
