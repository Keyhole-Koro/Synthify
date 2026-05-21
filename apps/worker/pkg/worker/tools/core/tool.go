package core

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

// Tool is the single tool concept ("道A"). Builtin and dynamic tools are the
// same type; see package doc for the design rationale.
type Tool struct {
	Name        string
	Description string
	IOSchema    IOSchema
	Run         RunFn
}

// RunFn runs one tool: input JSON -> (output JSON, usage, toolErr). Builtin
// and dynamic share this exact signature — that sameness is the point of 道A.
//
// err here is a TOOL-LEVEL failure (LLM error, no items, transform non-zero
// exit). The orchestrator surfaces it through the ADK callback chain as the
// tool result error; it is NOT a Go-level panic or runner abort.
//
// Renderer / closure dependencies (LLM client, brief memory, transform engine,
// etc.) are NOT in this signature. Each builtin captures whatever it needs in
// the closure returned by its factory. Putting tool-specific deps in RunFn
// would add parameters meaningless to other tools and break "every tool is the
// same type".
type RunFn func(ctx context.Context, in json.RawMessage) (out json.RawMessage, usage Usage, err error)

// Usage is the per-run model/token accounting common to every tool. Builtin
// tools that call the LLM populate it; tools that don't (memory, pure
// transforms) leave it zero. LLM-level metering already happens inside
// metering.LLMClient, so this Usage is informational only at the moment and
// the ADK adapter drops it on the floor (see adkadapter doc).
type Usage struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
}

// IOSchema holds resolved input/output validators. It is resolved once when
// the Tool is built and reused for every call (resolving per call would not
// scale — same policy as the eval runner's prompt renderer caching).
type IOSchema struct {
	Input  *jsonschema.Resolved
	Output *jsonschema.Resolved
}

// ResolveTool looks up a Tool by name from a per-run table. Unknown name is
// surfaced as an error so the caller can map it to the right exit / logging.
func ResolveTool(table map[string]Tool, name string) (Tool, error) {
	t, ok := table[name]
	if !ok {
		return Tool{}, &UnsupportedToolError{Name: name}
	}
	return t, nil
}

// UnsupportedToolError is returned when a tool name has no entry in the table.
type UnsupportedToolError struct {
	Name string
}

func (e *UnsupportedToolError) Error() string {
	return "unsupported tool: " + e.Name
}
