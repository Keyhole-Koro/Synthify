package core

import (
	"context"

	"github.com/synthify/backend/packages/shared/domain"
)

// DynamicToolSource is the seam between the worker and the dynamic tool store.
// The orchestrator never talks to the DB directly; it asks this interface for
// "what dynamic tools may a job in this workspace use". Implementations live
// in repository packages; tests inject fakes.
//
// Only tools with Status=active are returned. The interface returns the
// raw domain object; converting it into a core.Tool is the job of the
// dynamic builder (see apps/worker/pkg/worker/tools/dynamictool).
type DynamicToolSource interface {
	ResolveActive(ctx context.Context, workspaceID string) ([]*domain.DynamicTool, error)
}
