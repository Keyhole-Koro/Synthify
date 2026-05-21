package runner

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

// Tool is the single tool concept ("道A"). A builtin tool (knowledge_tree etc.,
// which calls process.X) and a dynamic tool (a promoted transform run via
// transform.Engine) are the SAME type. The runner never sees the
// builtin/dynamic distinction.
//
// Design rationale (kept here because the eval-multitool design doc was
// deleted — "the contract is the Go type"):
//
//   - Every tool is unified as "input JSON -> output JSON + io_schema". A
//     builtin tool's Go-typed output is JSON-marshalled so all judgement runs
//     on JSON, uniformly. This removes the old Items/Text union and the
//     generation/transform kind split: schema conformance of the output
//     JSON is the one judgement common to every tool.
//   - io_schema is mandatory for every tool. builtin schemas are derived from
//     their Go types (jsonschema.For); dynamic tools carry io_schema in the DB.
//   - builtin vs dynamic differ only in registration path (static table vs
//     runtime DB resolution); from eval's view the type is identical.
//   - No Go interface. The run logic is one function per tool with a uniform
//     signature, so polymorphism is unnecessary; a data record + a value
//     function (RunFn) is the whole abstraction. (An interface with one
//     implementer is over-abstraction — this was deliberately rejected.)
type Tool struct {
	Name     string
	IOSchema IOSchema
	Run      RunFn
}

// RunFn runs one tool for one case: input JSON -> (output JSON, usage,
// toolErr). builtin and dynamic share this exact signature — that sameness is
// the point of 道A.
//
// err here is a TOOL-LEVEL failure (LLM error, no items, transform non-zero
// exit). It is NOT a runner error: the runner records it on the Result and
// continues with other cases. Runner-level errors (bad case input, unreadable
// fixture) are produced outside RunFn, in the prepare layer.
//
// The prompt renderer (--variant) is NOT a RunFn parameter. The knowledge_tree
// builtin captures its renderer in a closure. Putting renderer in the
// signature would add a parameter meaningless to dynamic tools and break
// "every tool is the same type".
type RunFn func(ctx context.Context, in json.RawMessage) (out json.RawMessage, usage Usage, err error)

// Usage is the per-run model/token accounting, common to every tool.
type Usage struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
}

// IOSchema holds resolved input/output validators. It is resolved once when
// the Tool is built and reused for every case (resolving per case would not
// scale — same policy as the prompt renderer caching).
type IOSchema struct {
	Input  *jsonschema.Resolved
	Output *jsonschema.Resolved
}

// resolveTool returns the Tool for a tool key from the run's tool table, or an
// error the runner maps to exit 2 (same contract as the single-tool eval:
// unknown tool is not a silent skip).
func resolveTool(table map[string]Tool, name string) (Tool, error) {
	t, ok := table[name]
	if !ok {
		return Tool{}, errUnsupportedTool(name)
	}
	return t, nil
}

func errUnsupportedTool(name string) error {
	return &unsupportedToolError{name: name}
}

type unsupportedToolError struct {
	name string
}

func (e *unsupportedToolError) Error() string {
	return "unsupported tool: " + e.name
}
