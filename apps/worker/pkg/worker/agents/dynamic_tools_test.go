package agents

import "testing"

func TestIsDynamicTool(t *testing.T) {
	o := &Orchestrator{}

	// No job active yet: nothing is dynamic, and the nil set must not panic.
	if o.isDynamicTool("anything") {
		t.Fatal("isDynamicTool should be false before any job sets the dynamic set")
	}

	names := map[string]struct{}{"promoted_transform": {}}
	o.dynamicToolNames.Store(&names)

	if !o.isDynamicTool("promoted_transform") {
		t.Error("dynamic tool name should report true")
	}
	if o.isDynamicTool("semantic_chunking") {
		t.Error("builtin tool name should report false")
	}
}
