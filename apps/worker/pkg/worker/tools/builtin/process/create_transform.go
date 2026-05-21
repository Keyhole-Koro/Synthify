package process

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
)

// CreateTransformArgs is the meta-tool the LLM calls to define a transform.
// See docs/improvements/dynamic-tool-synthesis.md "メタツール create_transform".
type CreateTransformArgs struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"` // "starlark" (stage 1) | "python" (later)
	Code        string `json:"code"`
	InputSample string `json:"input_sample"`
}

// CreateTransformResult is returned to the LLM. Status mirrors the executor
// contract; the worker maps each non-ok status to a fixed message so failures
// don't trigger blind retries (see "失敗の返し方" in the design doc).
type CreateTransformResult struct {
	Status    string `json:"status"`
	Output    string `json:"output,omitempty"`     // input_sample run result on ok
	Error     string `json:"error,omitempty"`      // diagnostic for the LLM on failure
	ToolName  string `json:"tool_name"`            // name to reuse within this job
	FloorTier string `json:"floor_tier,omitempty"` // machine-derived lower bound
}

// runCreateTransform is the pure core: analyze (parse + static analysis), then
// execute the input_sample on success. No persistence, no usage accounting —
// the functiontool wrapper handles those so this stays unit-testable.
func runCreateTransform(
	ctx context.Context,
	analyzer transform.Analyzer,
	engine transform.Engine,
	args CreateTransformArgs,
) CreateTransformResult {
	res := CreateTransformResult{ToolName: args.Name}

	// Stage 1 only wires Starlark. Python is dispatched to the executor
	// service in a later stage; reject it explicitly rather than silently.
	if transform.Language(args.Language) != engine.Language() {
		res.Status = string(transform.StatusNonzeroExit)
		res.Error = fmt.Sprintf("language %q is not available yet; only %q is supported", args.Language, engine.Language())
		return res
	}

	// Step 0/a: a single parse serves syntax validation and the floor walk.
	an := analyzer.Analyze(args.Code)
	switch {
	case an.SyntaxErr:
		res.Status = string(transform.StatusSyntaxError)
		res.Error = an.Reason
		return res
	case an.Rejected:
		res.Status = string(transform.StatusAnalyzerRejected)
		res.Error = an.Reason
		return res
	}
	res.FloorTier = string(an.FloorTier)

	out := engine.Execute(transform.ExecRequest{Code: args.Code, Input: args.InputSample})
	res.Status = string(out.Status)
	if out.Status == transform.StatusOK {
		res.Output = out.Output
	} else {
		res.Error = out.Stderr
	}
	return res
}

// NewCreateTransformTool registers create_transform as an ADK tool. The
// wrapper consumes the transform-creation budget (shared with syntax_error
// fix-regeneration so a malformed-code loop can't run unbounded) before doing
// any work.
func NewCreateTransformTool(b *base.Context, analyzer transform.Analyzer, engine transform.Engine) (core.Tool, error) {
	schema, err := core.IOSchemaFor[CreateTransformArgs, CreateTransformResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args CreateTransformArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		if err := b.IncrementTransformCreations(ctx); err != nil {
			// Budget exhausted: stop generating, fall back to existing tools.
			out, mErr := json.Marshal(CreateTransformResult{
				Status:   string(transform.StatusNonzeroExit),
				ToolName: args.Name,
				Error:    "transform creation budget exhausted for this job; use existing tools",
			})
			return out, core.Usage{}, mErr
		}
		result := runCreateTransform(ctx, analyzer, engine, args)
		out, mErr := json.Marshal(result)
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name: "create_transform",
		Description: "Define a one-off transform (Starlark) to reshape data the existing tools can't. " +
			"Provide code with a top-level function transform(input) -> string and an input_sample to verify it.",
		IOSchema: schema,
		Run:      run,
	}, nil
}
