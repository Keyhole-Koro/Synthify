package agents

import (
	"context"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/dynamictool"
)

// resolveDynamicTools returns the dynamic tool list for the given workspace.
// A source-level failure disables dynamic tools for that run; individual tool
// build failures are logged and skipped so other tools remain available.
func (o *Orchestrator) resolveDynamicTools(ctx context.Context, workspaceID string) []core.Tool {
	if o.dynamicSource == nil || workspaceID == "" {
		return nil
	}
	rows, err := o.dynamicSource.ResolveActive(ctx, workspaceID)
	if err != nil {
		if o.base != nil && o.base.Logger != nil {
			o.base.Logger.Warn("orchestrator.resolve_dynamic_tools_failed", "error", err.Error(), "workspace_id", workspaceID)
		}
		return nil
	}

	out := make([]core.Tool, 0, len(rows))
	for _, dt := range rows {
		t, err := dynamictool.FromDomain(dt, o.dynamicEngine)
		if err != nil {
			if o.base != nil && o.base.Logger != nil {
				o.base.Logger.Warn("orchestrator.dynamic_tool_build_failed", "error", err.Error(), "workspace_id", workspaceID, "tool", dt.Name)
			}
			continue
		}
		out = append(out, t)
	}
	return out
}

// isDynamicTool reports whether name is one of the current job's dynamic
// (transform-backed) tools. The set is rebuilt per job in ProcessDocument;
// when no job is active (nil set) nothing is dynamic.
func (o *Orchestrator) isDynamicTool(name string) bool {
	names := o.dynamicToolNames.Load()
	if names == nil {
		return false
	}
	_, ok := (*names)[name]
	return ok
}
