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

type MergeCandidate struct {
	LocalID     string `json:"local_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type MergeArgs struct {
	Items []MergeCandidate `json:"items"`
}

type MergeResult struct {
	MergedID string `json:"merged_id"`
	Reason   string `json:"reason"`
}

func NewMergeTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[MergeArgs, MergeResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args MergeArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		var result MergeResult
		switch len(args.Items) {
		case 0:
			result = MergeResult{Reason: "no items provided"}
		case 1:
			result = MergeResult{MergedID: args.Items[0].LocalID, Reason: "only one candidate"}
		default:
			r, mErr := merge(ctx, b.LLM, args.Items)
			if mErr != nil {
				return nil, core.Usage{}, mErr
			}
			result = r
		}
		out, mErr := json.Marshal(result)
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "deduplicate_and_merge",
		Description: "Merges multiple items into a single canonical item to reduce redundancy in the knowledge tree.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

func merge(ctx context.Context, llmClient base.LLMClient, items []MergeCandidate) (MergeResult, error) {
	if llmClient == nil {
		return MergeResult{MergedID: items[0].LocalID, Reason: "llm not configured, selected first"}, nil
	}

	var sb strings.Builder
	for _, item := range items {
		fmt.Fprintf(&sb, "[%s] %s: %s\n", item.LocalID, item.Title, item.Description)
	}

	type llmResult struct {
		MergedID string `json:"merged_id"`
		Reason   string `json:"reason"`
	}

	raw, _, err := llmClient.GenerateStructured(ctx, llm.StructuredRequest{
		SystemPrompt: `You are a knowledge deduplication expert.
Given a list of knowledge tree items that may represent the same concept,
select the single most comprehensive and accurate one as the canonical item.
Return its local_id and a brief reason for your choice.`,
		UserPrompt: "Candidate items:\n" + sb.String(),
		Schema:     llmResult{},
	})
	if err != nil {
		return MergeResult{MergedID: items[0].LocalID, Reason: "llm error, selected first"}, nil
	}

	var res llmResult
	if err := json.Unmarshal(raw, &res); err != nil || res.MergedID == "" {
		return MergeResult{MergedID: items[0].LocalID, Reason: "parse error, selected first"}, nil
	}
	return MergeResult{MergedID: res.MergedID, Reason: res.Reason}, nil
}
