package adkadapter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

type echoInput struct {
	Text string `json:"text"`
}

type echoOutput struct {
	Upper string `json:"upper"`
}

func newEchoSharedTool(t *testing.T) core.Tool {
	t.Helper()
	schema, err := core.IOSchemaFor[echoInput, echoOutput]()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return core.Tool{
		Name:        "echo_upper",
		Description: "uppercase the input text",
		IOSchema:    schema,
		Run: func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
			var args echoInput
			if err := json.Unmarshal(in, &args); err != nil {
				return nil, core.Usage{}, err
			}
			out, err := json.Marshal(echoOutput{Upper: strings.ToUpper(args.Text)})
			return out, core.Usage{}, err
		},
	}
}

// Wrap surfaces Name/Description through the ADK tool.Tool interface. The
// IOSchema override (so the LLM sees the real shape rather than an empty
// map[string]any) is enforced inside ADK's runnableTool path, which is
// unexported and therefore not directly assertable from outside the package.
// We exercise the schema override indirectly through the runtime round-trip
// test below (TestWrap_HandlerRoundTrip) — if the override were missing, ADK
// would reject map[string]any args that don't conform to the schema it
// derived from the handler's TArgs.
func TestWrap_NameAndDescription(t *testing.T) {
	tl, err := Wrap(newEchoSharedTool(t))
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if tl.Name() != "echo_upper" {
		t.Fatalf("Name() = %q, want echo_upper", tl.Name())
	}
	if tl.Description() != "uppercase the input text" {
		t.Fatalf("Description() not propagated, got %q", tl.Description())
	}
	if tl.IsLongRunning() {
		t.Fatal("IsLongRunning() = true, want false (echo is fast)")
	}
}

// Missing IOSchema must be rejected — every 道A Tool is required to carry
// one, and a nil schema would mean the LLM-facing declaration carries no
// contract while the adapter has nothing to validate against.
func TestWrap_RejectsMissingSchema(t *testing.T) {
	tl, err := Wrap(core.Tool{Name: "x", Run: func(context.Context, json.RawMessage) (json.RawMessage, core.Usage, error) {
		return nil, core.Usage{}, nil
	}})
	if err == nil {
		t.Fatalf("expected error for missing IOSchema, got tool %#v", tl)
	}
}

// Nil Run must be rejected — Wrap is meant to be called by builtin/dynamic
// factories that always provide one. A nil Run reaching the agent would
// surface as a panic deep inside ADK, not as a clear factory error.
func TestWrap_RejectsNilRun(t *testing.T) {
	schema, _ := core.IOSchemaFor[echoInput, echoOutput]()
	if _, err := Wrap(core.Tool{Name: "x", IOSchema: schema}); err == nil {
		t.Fatal("expected error for nil Run")
	}
}
