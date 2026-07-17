package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
)

type evaluatorFakeLLM struct {
	raw json.RawMessage
	err error
	req llm.StructuredRequest
}

func (f *evaluatorFakeLLM) GenerateStructured(_ context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	f.req = req
	return f.raw, llm.Usage{Model: "fake-evaluator"}, f.err
}

func (f *evaluatorFakeLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "", llm.Usage{}, errors.New("unexpected GenerateText call")
}

func TestEvaluateTree_DerivesPassedFromScore(t *testing.T) {
	tests := []struct {
		name        string
		score       int
		modelPassed bool
		wantPassed  bool
	}{
		{name: "below threshold overrides true", score: 69, modelPassed: true, wantPassed: false},
		{name: "threshold overrides false", score: 70, modelPassed: false, wantPassed: true},
		{name: "above threshold passes", score: 95, modelPassed: true, wantPassed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(evaluationOutput{
				Score:    tt.score,
				Passed:   tt.modelPassed,
				Summary:  "summary",
				Findings: []string{"finding"},
			})
			if err != nil {
				t.Fatal(err)
			}
			fake := &evaluatorFakeLLM{raw: raw}

			out, err := NewEvaluator(fake).EvaluateTree(context.Background(), `{"items":[]}`)
			if err != nil {
				t.Fatalf("EvaluateTree returned error: %v", err)
			}
			if out.Passed != tt.wantPassed {
				t.Fatalf("Passed = %v, want %v (score=%d, model passed=%v)", out.Passed, tt.wantPassed, tt.score, tt.modelPassed)
			}
			if out.Score != tt.score {
				t.Fatalf("Score = %d, want %d", out.Score, tt.score)
			}
			if !strings.Contains(fake.req.UserPrompt, `{"items":[]}`) {
				t.Fatalf("tree data missing from user prompt: %q", fake.req.UserPrompt)
			}
			if _, ok := fake.req.Schema.(evaluationOutput); !ok {
				t.Fatalf("Schema type = %T, want evaluationOutput", fake.req.Schema)
			}
		})
	}
}

func TestEvaluateTree_RejectsOutOfRangeScore(t *testing.T) {
	for _, score := range []int{-1, 101} {
		t.Run(string(rune(score+128)), func(t *testing.T) {
			raw, err := json.Marshal(evaluationOutput{Score: score})
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewEvaluator(&evaluatorFakeLLM{raw: raw}).EvaluateTree(context.Background(), `{}`)
			if err == nil || !strings.Contains(err.Error(), "must be between 0 and 100") {
				t.Fatalf("expected range error for score %d, got %v", score, err)
			}
		})
	}
}

func TestEvaluateTree_RejectsMalformedJSON(t *testing.T) {
	_, err := NewEvaluator(&evaluatorFakeLLM{raw: json.RawMessage(`{"score":`)}).EvaluateTree(context.Background(), `{}`)
	if err == nil || !strings.Contains(err.Error(), "parse evaluation result") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestEvaluateTree_WrapsLLMError(t *testing.T) {
	_, err := NewEvaluator(&evaluatorFakeLLM{err: errors.New("provider unavailable")}).EvaluateTree(context.Background(), `{}`)
	if err == nil || !strings.Contains(err.Error(), "evaluate tree: provider unavailable") {
		t.Fatalf("expected wrapped provider error, got %v", err)
	}
}

func TestEvaluateTree_NilClientReturnsExplicitFailure(t *testing.T) {
	out, err := NewEvaluator(nil).EvaluateTree(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("EvaluateTree returned error: %v", err)
	}
	if out.Score != 0 || out.Passed || out.Summary != "LLM not configured" {
		t.Fatalf("unexpected nil-client result: %#v", out)
	}
}
