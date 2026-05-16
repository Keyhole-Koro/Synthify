package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
	"github.com/synthify/backend/packages/shared/repository/postgres/sqlcgen"
)

func toDynamicTool(r sqlcgen.DynamicTool) *domain.DynamicTool {
	t := &domain.DynamicTool{
		ToolID:            r.ToolID,
		Name:              r.Name,
		Version:           int(r.Version),
		Scope:             domain.ToolScope(r.Scope),
		OriginWorkspaceID: r.OriginWorkspaceID,
		OriginJobID:       r.OriginJobID,
		Description:       r.Description,
		Language:          r.Language,
		Code:              r.Code,
		IOSchema:          r.IoSchema,
		DeclaredTier:      r.DeclaredTier,
		FloorTier:         r.FloorTier,
		RiskTier:          r.RiskTier,
		Status:            domain.ToolStatus(r.Status),
		InputSample:       r.InputSample,
		RuntimeVersion:    r.RuntimeVersion,
		UseCount:          int(r.UseCount),
		CreatedAt:         r.CreatedAt,
		ReviewedBy:        r.ReviewedBy,
	}
	if r.PromotedAt.Valid {
		at := r.PromotedAt.Time
		t.PromotedAt = &at
	}
	return t
}

// RecordCandidate persists a freshly generated tool with status=candidate.
// Version is MAX(version)+1 for the (scope, origin_workspace_id, name) tuple;
// the DB unique index uq_dynamic_tools_identity is the source of truth.
func (s *Store) RecordCandidate(ctx context.Context, tool *domain.DynamicTool) (*domain.DynamicTool, error) {
	if tool == nil {
		return nil, fmt.Errorf("nil tool")
	}

	maxVersion, err := s.q().MaxDynamicToolVersion(ctx, sqlcgen.MaxDynamicToolVersionParams{
		Scope:             string(tool.Scope),
		OriginWorkspaceID: tool.OriginWorkspaceID,
		Name:              tool.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("dynamic_tools max version: %w", err)
	}

	stored := *tool
	stored.ToolID = newID()
	stored.Version = int(maxVersion) + 1
	stored.Status = domain.ToolStatusCandidate
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = nowTime()
	}

	if err := s.q().InsertDynamicTool(ctx, sqlcgen.InsertDynamicToolParams{
		ToolID:            stored.ToolID,
		Name:              stored.Name,
		Version:           int32(stored.Version),
		Scope:             string(stored.Scope),
		OriginWorkspaceID: stored.OriginWorkspaceID,
		OriginJobID:       stored.OriginJobID,
		Description:       stored.Description,
		Language:          stored.Language,
		Code:              stored.Code,
		IoSchema:          stored.IOSchema,
		DeclaredTier:      stored.DeclaredTier,
		FloorTier:         stored.FloorTier,
		RiskTier:          stored.RiskTier,
		Status:            string(stored.Status),
		InputSample:       stored.InputSample,
		CreatedAt:         stored.CreatedAt,
	}); err != nil {
		return nil, fmt.Errorf("insert dynamic_tool: %w", err)
	}

	out := stored
	return &out, nil
}

// ListCandidates returns candidate-status tools oldest first, capped by limit.
func (s *Store) ListCandidates(ctx context.Context, limit int) ([]*domain.DynamicTool, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.q().ListDynamicToolCandidates(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list dynamic_tool candidates: %w", err)
	}
	out := make([]*domain.DynamicTool, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDynamicTool(r))
	}
	return out, nil
}

// PromoteCandidate transitions a candidate to active (or held for tier_3).
func (s *Store) PromoteCandidate(ctx context.Context, toolID string, status domain.ToolStatus, promotedAt time.Time) error {
	n, err := s.q().PromoteDynamicToolCandidate(ctx, sqlcgen.PromoteDynamicToolCandidateParams{
		ToolID:     toolID,
		Status:     string(status),
		PromotedAt: sql.NullTime{Time: promotedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("promote dynamic_tool: %w", err)
	}
	return requireAffected(n)
}

// PromoteToGlobal flips an active workspace-scoped tool to global scope.
func (s *Store) PromoteToGlobal(ctx context.Context, toolID, reviewedBy string) error {
	n, err := s.q().PromoteDynamicToolToGlobal(ctx, sqlcgen.PromoteDynamicToolToGlobalParams{
		ToolID:     toolID,
		ReviewedBy: reviewedBy,
	})
	if err != nil {
		return fmt.Errorf("promote dynamic_tool to global: %w", err)
	}
	return requireAffected(n)
}

// SetStatus is the kill switch / review-decision write path.
func (s *Store) SetStatus(ctx context.Context, toolID string, status domain.ToolStatus) error {
	n, err := s.q().SetDynamicToolStatus(ctx, sqlcgen.SetDynamicToolStatusParams{
		ToolID: toolID,
		Status: string(status),
	})
	if err != nil {
		return fmt.Errorf("set dynamic_tool status: %w", err)
	}
	return requireAffected(n)
}

// ResolveActiveTools returns the active tools a job in workspaceID may load:
// that workspace's active tools plus global active tools. Default-deny.
func (s *Store) ResolveActiveTools(ctx context.Context, workspaceID string) ([]*domain.DynamicTool, error) {
	rows, err := s.q().ResolveActiveDynamicTools(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve active dynamic_tools: %w", err)
	}
	out := make([]*domain.DynamicTool, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDynamicTool(r))
	}
	return out, nil
}

// IncrementUseCount bumps usage (ephemeral + promoted) for lifecycle/GC.
func (s *Store) IncrementUseCount(ctx context.Context, toolID string) error {
	n, err := s.q().IncrementDynamicToolUseCount(ctx, toolID)
	if err != nil {
		return fmt.Errorf("increment dynamic_tool use_count: %w", err)
	}
	return requireAffected(n)
}

func requireAffected(n int64) error {
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
