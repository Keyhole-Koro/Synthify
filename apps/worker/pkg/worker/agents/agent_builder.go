package agents

import (
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/adkadapter"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/tool"
)

// buildAgent constructs the llmagent for a job. The builtin tool list is
// always first and dynamic tools are appended after workspace resolution.
func (o *Orchestrator) buildAgent(dyn []core.Tool) (agent.Agent, error) {
	combined := make([]core.Tool, 0, len(o.builtinTools)+len(dyn))
	combined = append(combined, o.builtinTools...)

	builtinNames := make(map[string]struct{}, len(o.builtinTools))
	for _, t := range o.builtinTools {
		builtinNames[t.Name] = struct{}{}
	}
	for _, d := range dyn {
		if _, dup := builtinNames[d.Name]; dup {
			return nil, fmt.Errorf("dynamic tool %q collides with a builtin", d.Name)
		}
		combined = append(combined, d)
	}

	toolList := make([]tool.Tool, 0, len(combined))
	for _, st := range combined {
		wrapped, err := adkadapter.Wrap(st)
		if err != nil {
			return nil, fmt.Errorf("wrap tool %q for ADK: %w", st.Name, err)
		}
		toolList = append(toolList, wrapped)
	}

	return llmagent.New(llmagent.Config{
		Name:                 "orchestrator",
		Model:                o.model,
		Instruction:          o.instruction,
		Tools:                toolList,
		BeforeModelCallbacks: o.beforeModelCBs,
		AfterModelCallbacks:  o.afterModelCBs,
		BeforeToolCallbacks:  o.beforeToolCBs,
		AfterToolCallbacks:   o.afterToolCBs,
	})
}
