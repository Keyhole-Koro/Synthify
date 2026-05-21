package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
)

// RecordCandidate persists a freshly generated tool with status=candidate and
// assigns Version = max(existing for (scope, origin_workspace_id, name)) + 1.
func (s *Store) RecordCandidate(ctx context.Context, tool *domain.DynamicTool) (*domain.DynamicTool, error) {
	if tool == nil {
		return nil, fmt.Errorf("nil tool")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	maxVersion := 0
	for _, existing := range s.dynamicTools {
		if existing.Scope == tool.Scope &&
			existing.OriginWorkspaceID == tool.OriginWorkspaceID &&
			existing.Name == tool.Name &&
			existing.Version > maxVersion {
			maxVersion = existing.Version
		}
	}

	s.dynamicToolSeq++
	stored := *tool
	stored.ToolID = fmt.Sprintf("stool-%d", s.dynamicToolSeq)
	stored.Version = maxVersion + 1
	stored.Status = domain.ToolStatusCandidate
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now().UTC().Add(time.Duration(s.dynamicToolSeq) * time.Nanosecond)
	}
	s.dynamicTools = append(s.dynamicTools, &stored)

	out := stored
	return &out, nil
}

// ListCandidates returns candidate-status tools oldest first, capped by limit.
func (s *Store) ListCandidates(ctx context.Context, limit int) ([]*domain.DynamicTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*domain.DynamicTool, 0)
	for _, tool := range s.dynamicTools {
		if tool.Status != domain.ToolStatusCandidate {
			continue
		}
		copied := *tool
		out = append(out, &copied)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, nil
}

// PromoteCandidate transitions a candidate to active (or held for tier_3).
func (s *Store) PromoteCandidate(ctx context.Context, toolID string, status domain.ToolStatus, promotedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool := s.findDynamicToolLocked(toolID)
	if tool == nil {
		return domain.ErrNotFound
	}
	tool.Status = status
	at := promotedAt
	tool.PromotedAt = &at
	return nil
}

// PromoteToGlobal flips an active workspace-scoped tool to global scope.
func (s *Store) PromoteToGlobal(ctx context.Context, toolID, reviewedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool := s.findDynamicToolLocked(toolID)
	if tool == nil {
		return domain.ErrNotFound
	}
	tool.Scope = domain.ToolScopeGlobal
	tool.ReviewedBy = reviewedBy
	return nil
}

// SetStatus is the kill switch / review-decision write path.
func (s *Store) SetStatus(ctx context.Context, toolID string, status domain.ToolStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool := s.findDynamicToolLocked(toolID)
	if tool == nil {
		return domain.ErrNotFound
	}
	tool.Status = status
	return nil
}

// ResolveActiveTools returns the active tools a job in workspaceID may load:
// that workspace's active tools plus global active tools. Default-deny.
func (s *Store) ResolveActiveTools(ctx context.Context, workspaceID string) ([]*domain.DynamicTool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*domain.DynamicTool, 0)
	for _, tool := range s.dynamicTools {
		if tool.Resolvable(workspaceID) {
			copied := *tool
			out = append(out, &copied)
		}
	}
	return out, nil
}

// IncrementUseCount bumps usage (ephemeral + promoted) for lifecycle/GC.
func (s *Store) IncrementUseCount(ctx context.Context, toolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tool := s.findDynamicToolLocked(toolID)
	if tool == nil {
		return domain.ErrNotFound
	}
	tool.UseCount++
	return nil
}

func (s *Store) findDynamicToolLocked(toolID string) *domain.DynamicTool {
	for _, tool := range s.dynamicTools {
		if tool.ToolID == toolID {
			return tool
		}
	}
	return nil
}
