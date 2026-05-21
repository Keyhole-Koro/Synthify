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
			o.base.Logger.Warn(ctx, "orchestrator.resolve_dynamic_tools_failed", err, map[string]any{"workspace_id": workspaceID})
		}
		return nil
	}

	out := make([]core.Tool, 0, len(rows))
	for _, dt := range rows {
		t, err := dynamictool.FromDomain(dt, o.dynamicEngine)
		if err != nil {
			if o.base != nil && o.base.Logger != nil {
				o.base.Logger.Warn(ctx, "orchestrator.dynamic_tool_build_failed", err, map[string]any{"workspace_id": workspaceID, "tool": dt.Name})
			}
			continue
		}
		out = append(out, t)
	}
	return out
}
