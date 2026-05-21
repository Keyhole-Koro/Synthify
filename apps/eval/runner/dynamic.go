package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/transform"
	"github.com/synthify/backend/packages/shared/domain"
)

// DynamicToolSource is the eval-side seam for promoted tools. The current
// phase only proves dynamic tools fit the same Tool shape; actual transform
// execution and dynamic io_schema parsing are intentionally left for the next
// phase.
type DynamicToolSource interface {
	Resolve(ctx context.Context, name string) (domain.DynamicTool, bool, error)
}

func newDynamicTool(dt domain.DynamicTool, _ transform.Engine) Tool {
	name := dt.Name
	run := func(context.Context, json.RawMessage) (json.RawMessage, Usage, error) {
		return nil, Usage{}, fmt.Errorf("dynamic eval tool %q is not implemented yet", name)
	}
	return Tool{Name: name, Run: run}
}

var _ RunFn = newDynamicTool(domain.DynamicTool{}, nil).Run
