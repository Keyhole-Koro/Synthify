package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/process"
	"github.com/synthify/backend/packages/shared/domain"
)

type ktInput struct {
	DocumentID  string         `json:"document_id"`
	Instruction string         `json:"instruction,omitempty"`
	Chunks      []domain.Chunk `json:"chunks"`
}

type ktOutput struct {
	Items []domain.GeneratedTreeItem `json:"items"`
}

// newKnowledgeTreeTool adapts the production process tool to the 道A Tool
// shape. The renderer is captured here so RunFn remains meaningful for both
// builtin and dynamic tools: it only receives JSON input and returns JSON
// output.
func newKnowledgeTreeTool(llm base.LLMClient, renderer *prompts.Renderer) Tool {
	ioSchema := knowledgeTreeIOSchema()
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, Usage, error) {
		var args ktInput
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, Usage{}, err
		}
		items, usage, err := process.GenerateKnowledgeTreeWithRenderer(ctx, llm, renderer, process.GenerateKnowledgeTreeArgs{
			DocumentID:  args.DocumentID,
			Chunks:      args.Chunks,
			Instruction: args.Instruction,
		})
		u := Usage{Model: usage.Model, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}
		if err != nil {
			return nil, u, err
		}
		out, err := json.Marshal(ktOutput{Items: items})
		if err != nil {
			return nil, u, err
		}
		return out, u, nil
	}
	return Tool{Name: ToolKnowledgeTree, IOSchema: ioSchema, Run: run}
}

var (
	knowledgeTreeSchemaOnce sync.Once
	knowledgeTreeSchema     IOSchema
	knowledgeTreeSchemaErr  error
)

func knowledgeTreeIOSchema() IOSchema {
	knowledgeTreeSchemaOnce.Do(func() {
		input, err := resolvedSchemaFor[ktInput]()
		if err != nil {
			knowledgeTreeSchemaErr = fmt.Errorf("knowledge_tree input schema: %w", err)
			return
		}
		output, err := resolvedSchemaFor[ktOutput]()
		if err != nil {
			knowledgeTreeSchemaErr = fmt.Errorf("knowledge_tree output schema: %w", err)
			return
		}
		knowledgeTreeSchema = IOSchema{Input: input, Output: output}
	})
	if knowledgeTreeSchemaErr != nil {
		panic(knowledgeTreeSchemaErr)
	}
	return knowledgeTreeSchema
}

func resolvedSchemaFor[T any]() (*jsonschema.Resolved, error) {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, err
	}
	return s.Resolve(nil)
}
