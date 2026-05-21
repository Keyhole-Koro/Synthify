package process

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

type CritiqueArgs struct {
	TargetData string `json:"target_data"`
	Criteria   string `json:"criteria"`
}

type CritiqueResult struct {
	Valid       bool     `json:"valid"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

func NewCritiqueTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[CritiqueArgs, CritiqueResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args CritiqueArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		if strings.TrimSpace(args.TargetData) == "" {
			out, mErr := json.Marshal(CritiqueResult{Valid: false, Issues: []string{"target data is empty"}})
			return out, core.Usage{}, mErr
		}
		result, cerr := critique(ctx, b.LLM, args.TargetData, args.Criteria)
		if cerr != nil {
			return nil, core.Usage{}, cerr
		}
		out, mErr := json.Marshal(result)
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "quality_critique",
		Description: "Critiques the generated output for potential hallucinations, logical gaps, or inaccuracies. Use this before finalizing results.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

func critique(ctx context.Context, llmClient base.LLMClient, targetData, criteria string) (CritiqueResult, error) {
	if llmClient == nil {
		return CritiqueResult{Valid: true}, nil
	}

	userPrompt := fmt.Sprintf("Criteria: %s\n\nContent to evaluate:\n%s", criteria, targetData)

	raw, _, err := llmClient.GenerateStructured(ctx, llm.StructuredRequest{
		SystemPrompt: `You are a quality assurance reviewer for a knowledge management system.
Evaluate the provided content for:
- Hallucinations (claims not grounded in the source)
- Logical gaps or missing context
- Inaccurate or misleading descriptions
- Structural inconsistencies (e.g. broken parent-child relationships)

Return JSON with:
- valid: true only if no significant issues found
- issues: list of specific problems found (empty array if none)
- suggestions: list of concrete fixes (empty array if none)`,
		UserPrompt: userPrompt,
		Schema:     CritiqueResult{},
	})
	if err != nil {
		return CritiqueResult{Valid: true}, nil
	}

	var result CritiqueResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CritiqueResult{Valid: true}, nil
	}
	if result.Issues == nil {
		result.Issues = []string{}
	}
	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}
	return result, nil
}
