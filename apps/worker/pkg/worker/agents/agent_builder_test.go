package agents

import (
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

func TestBuildAgentRejectsDynamicBuiltinCollision(t *testing.T) {
	o := &Orchestrator{
		builtinTools: []core.Tool{{Name: "semantic_chunking"}},
	}
	_, err := o.buildAgent([]core.Tool{{Name: "semantic_chunking"}})
	if err == nil || !strings.Contains(err.Error(), "collides with a builtin") {
		t.Fatalf("buildAgent error = %v, want collision", err)
	}
}
