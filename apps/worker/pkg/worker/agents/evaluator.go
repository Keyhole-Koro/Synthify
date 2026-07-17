package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
)

const evaluationPassScore = 70

type Evaluator struct {
	llm llm.Client
}

func NewEvaluator(client llm.Client) *Evaluator {
	return &Evaluator{llm: client}
}

type evaluationOutput struct {
	Score    int      `json:"score"`
	Passed   bool     `json:"passed"`
	Summary  string   `json:"summary"`
	Findings []string `json:"findings"`
}

func (e *Evaluator) EvaluateTree(ctx context.Context, treeData string) (*evaluationOutput, error) {
	if e.llm == nil {
		return &evaluationOutput{Score: 0, Passed: false, Summary: "LLM not configured"}, nil
	}

	raw, _, err := e.llm.GenerateStructured(ctx, llm.StructuredRequest{
		SystemPrompt: `You are a quality assurance engineer for a knowledge management system.
Evaluate the provided knowledge tree JSON and return a structured assessment.
Score from 0-100 based on:
- Coverage: key concepts captured from the source
- Accuracy: content grounded in the original, no hallucinations
- Structure: logical hierarchy suitable for human exploration
Set passed=true if score >= 70.
Return findings as a list of specific issues found.`,
		UserPrompt: fmt.Sprintf("Evaluate this knowledge tree:\n%s", treeData),
		Schema:     evaluationOutput{},
	})
	if err != nil {
		return nil, fmt.Errorf("evaluate tree: %w", err)
	}

	var out evaluationOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse evaluation result: %w", err)
	}
	if out.Score < 0 || out.Score > 100 {
		return nil, fmt.Errorf("invalid evaluation score %d: must be between 0 and 100", out.Score)
	}

	// Do not trust the model to keep its boolean consistent with the numeric
	// score. The score is the canonical value and the pass threshold is enforced
	// deterministically so contradictory model output cannot pass evaluation.
	out.Passed = out.Score >= evaluationPassScore
	return &out, nil
}
