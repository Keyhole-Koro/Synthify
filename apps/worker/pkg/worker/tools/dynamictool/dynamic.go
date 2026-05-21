// Package dynamictool builds core.Tool values from promoted dynamic tool rows
// (transforms that the LLM generated via create_transform and a human
// approved). The resolution and IOSchema parsing live here so the
// orchestrator stays uniform: builtin tools come from the static registry,
// dynamic tools come from this package, and both end up as core.Tool
// values that adkadapter.Wrap converts for ADK.
//
// Status filtering: only DynamicTool with Status=active resolves. The DB
// query (DynamicToolRepository.ResolveActiveTools) already enforces this; we
// double-check in FromDomain so a stale row never reaches the agent.
//
// IOSchema convention: domain.DynamicTool.IOSchema is a single JSON Schema
// string with two top-level properties named "input" and "output", each
// itself a JSON Schema. See core.IOSchemaFromJSON for the parser.
package dynamictool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
)

// FromDomain turns one DynamicTool row into a core.Tool whose Run executes
// the stored Code through the given transform.Engine. The engine's Language
// must match dt.Language; mismatches surface as a tool-level error in Run
// (not a Go error here) so the agent loop can keep going with other tools.
func FromDomain(dt *domain.DynamicTool, eng transform.Engine) (core.Tool, error) {
	if dt == nil {
		return core.Tool{}, fmt.Errorf("dynamic: nil DynamicTool")
	}
	if dt.Status != domain.ToolStatusActive {
		return core.Tool{}, fmt.Errorf("dynamic: %q is not active (status=%s)", dt.Name, dt.Status)
	}
	schema, err := core.IOSchemaFromJSON(dt.IOSchema)
	if err != nil {
		return core.Tool{}, fmt.Errorf("dynamic %q: %w", dt.Name, err)
	}
	name := dt.Name
	code := dt.Code
	expectedLang := transform.Language(dt.Language)
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		if eng == nil {
			return nil, core.Usage{}, fmt.Errorf("dynamic %q: no transform engine configured", name)
		}
		if eng.Language() != expectedLang {
			return nil, core.Usage{}, fmt.Errorf("dynamic %q: language %q is not supported (engine speaks %q)", name, expectedLang, eng.Language())
		}
		res := eng.Execute(transform.ExecRequest{Code: code, Input: string(in)})
		if res.Status != transform.StatusOK {
			return nil, core.Usage{}, fmt.Errorf("dynamic %q: %s: %s", name, res.Status, res.Stderr)
		}
		// Output is a JSON document by IOSchema convention; we surface it as
		// json.RawMessage without re-marshalling so the caller sees the
		// engine's bytes verbatim.
		return json.RawMessage(res.Output), core.Usage{}, nil
	}
	return core.Tool{
		Name:        name,
		Description: dt.Description,
		IOSchema:    schema,
		Run:         run,
	}, nil
}
