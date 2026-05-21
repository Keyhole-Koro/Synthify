// Package adkadapter is the single bridge between the worker's 道A Tool
// abstraction (core.Tool) and the ADK runtime that drives the agent
// loop. The orchestrator builds tools as core.Tool, and only at the
// boundary that hands the tool list to llmagent.New does it call Wrap to
// produce ADK-compatible tool.Tool values.
//
// Why a dedicated package: keeping the ADK type leak in one place lets the
// rest of the worker stay tool.Tool-free, and makes "what to change when ADK
// is eventually replaced" obvious — only this package.
//
// Schema policy: ADK can auto-derive a JSON schema from the handler's TArgs
// generic, but we override it with the core.Tool's IOSchema instead.
// That way the single source of truth is the Go struct used by the tool
// (via jsonschema.For[T]) and the LLM sees exactly the schema the tool
// validates against. Without this override the ADK-derived schema and the
// 道A schema can drift and the LLM would call against one shape while the
// tool validates the other.
//
// Usage handling: core.RunFn returns a Usage value, but the ADK handler
// signature only carries (result, error). The adapter currently drops Usage.
// This is safe because LLM-level token accounting already happens inside
// metering.LLMClient (apps/worker/pkg/worker/metering/llmclient.go), so the
// adapter never has to relay it. If a future change moves token accounting
// out of the LLM client, this is the seam to thread it through.
package adkadapter

import (
	"encoding/json"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

// Wrap converts a 道A core.Tool into an ADK tool.Tool. The handler
// translates ADK's map[string]any args to JSON, delegates to the underlying
// RunFn, and translates the JSON output back to a map for the ADK callback
// chain.
//
// The wrapper uses map[string]any for both TArgs and TResults so a single
// generic instantiation works for every tool — without this, every Wrap call
// site would need to specialise on the tool's Args/Result Go types and the
// builtin/dynamic uniformity would break (dynamic tools have no Go types).
// The schema override above carries the actual structural contract to the
// LLM, so the loss of generic type information at the handler boundary does
// not weaken what the model sees.
func Wrap(t core.Tool) (tool.Tool, error) {
	if t.IOSchema.Input == nil || t.IOSchema.Output == nil {
		return nil, fmt.Errorf("adkadapter: tool %q is missing IOSchema (input or output)", t.Name)
	}
	if t.Run == nil {
		return nil, fmt.Errorf("adkadapter: tool %q has nil Run", t.Name)
	}
	cfg := functiontool.Config{
		Name:         t.Name,
		Description:  t.Description,
		InputSchema:  t.IOSchema.Input.Schema(),
		OutputSchema: t.IOSchema.Output.Schema(),
	}
	handler := func(ctx tool.Context, args map[string]any) (map[string]any, error) {
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("adkadapter %q: marshal input: %w", t.Name, err)
		}
		// Usage is dropped here intentionally — see package doc.
		out, _, runErr := t.Run(ctx, raw)
		if runErr != nil {
			return nil, runErr
		}
		// Some tools may legitimately return nil/empty output; surface as
		// an empty map so the ADK callback chain still has a map to inspect.
		if len(out) == 0 {
			return map[string]any{}, nil
		}
		var result map[string]any
		if err := json.Unmarshal(out, &result); err != nil {
			return nil, fmt.Errorf("adkadapter %q: unmarshal output: %w", t.Name, err)
		}
		return result, nil
	}
	return functiontool.New[map[string]any, map[string]any](cfg, handler)
}
