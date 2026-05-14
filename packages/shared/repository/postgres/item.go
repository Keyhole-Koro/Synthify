package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
	"github.com/synthify/backend/packages/shared/repository/postgres/sqlcgen"
)

func (s *Store) GetItem(ctx context.Context, itemID string) (*domain.Item, error) {
	row, err := s.q().GetItem(ctx, itemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toItemFromGetRow(row), nil
}

func (s *Store) CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) *domain.Item {
	return s.createStructuredItemDirect(ctx, workspaceID, label, 0, description, "", createdBy, parentID)
}

func (s *Store) createStructuredItemDirect(ctx context.Context, workspaceID, label string, level int, description, summaryHTML, createdBy, parentID string) *domain.Item {
	createdAt := nowTime()
	item := &domain.Item{
		ItemID:      newID(),
		WorkspaceID: workspaceID,
		ParentID:    parentID,
		Title:       label,
		Level:       level,
		Description: description,
		Content:     summaryHTML,
		CreatedBy:   createdBy,
		CreatedAt:   createdAt.Format(time.RFC3339),
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	if err := s.q().WithTx(tx).CreateItem(ctx, sqlcgen.CreateItemParams{
		ID:          item.ItemID,
		WorkspaceID: workspaceID,
		ParentID: sql.NullString{
			String: parentID,
			Valid:  parentID != "",
		},
		Title:       item.Title,
		Level:       int32(item.Level),
		Description: item.Description,
		Content:     item.Content,
		CreatedBy:   item.CreatedBy,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return nil
	}
	_ = s.q().UpdateTreeTimestamp(ctx, sqlcgen.UpdateTreeTimestampParams{
		ID:        item.ItemID,
		UpdatedAt: nowTime(),
	})
	return item
}

func (s *Store) CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) *domain.Item {
	if !s.canMutateTree(capability, treev1.JobOperation_JOB_OPERATION_CREATE_ITEM, workspaceID, documentID) {
		return nil
	}
	if capability.MaxItemCreations > 0 && s.countJobMutations(ctx, jobID, "item") >= capability.MaxItemCreations {
		return nil
	}

	createdAt := nowTime()
	item := &domain.Item{
		ItemID:            newID(),
		WorkspaceID:       workspaceID,
		ParentID:          parentID,
		Title:             label,
		Level:             level,
		Description:       description,
		Content:           summaryHTML,
		OverrideCSS:       overrideCSS,
		CreatedBy:         createdBy,
		GovernanceState:   treev1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_SYSTEM_GENERATED,
		LastMutationJobID: jobID,
		CreatedAt:         createdAt.Format(time.RFC3339),
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	if err := qtx.CreateStructuredItem(ctx, sqlcgen.CreateStructuredItemParams{
		ID:          item.ItemID,
		WorkspaceID: workspaceID,
		ParentID: sql.NullString{
			String: parentID,
			Valid:  parentID != "",
		},
		Title:             item.Title,
		Level:             int32(item.Level),
		Description:       item.Description,
		Content:           item.Content,
		OverrideCss:       item.OverrideCSS,
		CreatedBy:         item.CreatedBy,
		GovernanceState:   "system_generated",
		LastMutationJobID: item.LastMutationJobID,
		CreatedAt:         createdAt,
	}); err != nil {
		return nil
	}
	if err := s.logMutationTx(ctx, tx, &domain.JobMutationLog{
		MutationID:     newID(),
		JobID:          jobID,
		CapabilityID:   capability.CapabilityID,
		WorkspaceID:    workspaceID,
		TargetType:     "item",
		TargetID:       item.ItemID,
		MutationType:   "append",
		RiskTier:       "tier_1",
		BeforeJSON:     "{}",
		AfterJSON:      mustJSON(item),
		ProvenanceJSON: mustJSON(map[string]any{"document_id": documentID, "source_chunk_ids": sourceChunkIDs}),
		CreatedAt:      createdAt.Format(time.RFC3339),
	}); err != nil {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return nil
	}
	return item
}

func (s *Store) UpsertItemSource(ctx context.Context, itemID, documentID, fileID, chunkID, sourceText string, confidence float64) error {
	return s.q().UpsertItemSource(ctx, sqlcgen.UpsertItemSourceParams{
		ItemID:     itemID,
		DocumentID: documentID,
		FileID:     fileID,
		ChunkID:    chunkID,
		SourceText: sourceText,
		Confidence: sql.NullFloat64{Float64: confidence, Valid: confidence > 0},
	})
}

func (s *Store) UpdateItemSummaryHTMLWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, summaryHTML string) error {
	if capability == nil || !capability.Allows(treev1.JobOperation_JOB_OPERATION_UPDATE_ITEM) || capability.IsExpired(nowTime()) {
		return fmt.Errorf("mutation not allowed by capability or expired")
	}

	row, err := s.q().GetItemSummaryUpdateContext(ctx, itemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ErrNotFound
		}
		return err
	}
	if capability.WorkspaceID != "" && capability.WorkspaceID != row.WorkspaceID {
		return fmt.Errorf("workspace mismatch")
	}
	if !capability.AllowsItem(itemID) {
		return fmt.Errorf("item not in capability scope")
	}
	if row.GovernanceState == "human_curated" || row.GovernanceState == "locked" {
		return fmt.Errorf("item is protected")
	}

	now := nowTime()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	affected, err := qtx.UpdateItemSummaryAndMutation(ctx, sqlcgen.UpdateItemSummaryAndMutationParams{
		ID:                itemID,
		Content:           summaryHTML,
		LastMutationJobID: jobID,
		UpdatedAt:         now,
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	if err := s.logMutationTx(ctx, tx, &domain.JobMutationLog{
		MutationID:     newID(),
		JobID:          jobID,
		CapabilityID:   capability.CapabilityID,
		WorkspaceID:    row.WorkspaceID,
		TargetType:     "item",
		TargetID:       itemID,
		MutationType:   "revise",
		RiskTier:       "tier_1",
		BeforeJSON:     mustJSON(map[string]string{"content": row.Content}),
		AfterJSON:      mustJSON(map[string]string{"content": summaryHTML}),
		ProvenanceJSON: mustJSON(map[string]any{"field": "content"}),
		CreatedAt:      now.Format(time.RFC3339),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ApproveAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error {
	if err := s.q().UpsertApprovedAlias(ctx, sqlcgen.UpsertApprovedAliasParams{
		WorkspaceID:     wsID,
		CanonicalItemID: canonicalItemID,
		AliasItemID:     aliasItemID,
		UpdatedAt:       nowTime(),
	}); err != nil {
		return fmt.Errorf("approve alias: %w", err)
	}
	return nil
}

func (s *Store) RejectAlias(ctx context.Context, wsID, canonicalItemID, aliasItemID string) error {
	if err := s.q().UpsertRejectedAlias(ctx, sqlcgen.UpsertRejectedAliasParams{
		WorkspaceID:     wsID,
		CanonicalItemID: canonicalItemID,
		AliasItemID:     aliasItemID,
		UpdatedAt:       nowTime(),
	}); err != nil {
		return fmt.Errorf("reject alias: %w", err)
	}
	return nil
}

func (s *Store) canMutateTree(capability *domain.JobCapability, op treev1.JobOperation, workspaceID, documentID string) bool {
	if capability == nil || capability.IsExpired(nowTime()) {
		return false
	}
	if !capability.Allows(op) {
		return false
	}
	if capability.WorkspaceID != "" && capability.WorkspaceID != workspaceID {
		return false
	}
	return capability.AllowsDocument(documentID)
}

func (s *Store) countJobMutations(ctx context.Context, jobID, targetType string) int {
	count, err := s.q().CountJobMutationsByTarget(ctx, sqlcgen.CountJobMutationsByTargetParams{
		JobID:      jobID,
		TargetType: targetType,
	})
	if err != nil {
		return 0
	}
	return int(count)
}

func (s *Store) logMutationTx(ctx context.Context, tx *sql.Tx, entry *domain.JobMutationLog) error {
	createdAt, err := time.Parse(time.RFC3339, entry.CreatedAt)
	if err != nil {
		createdAt = nowTime()
	}
	return s.q().WithTx(tx).InsertJobMutationLog(ctx, sqlcgen.InsertJobMutationLogParams{
		MutationID:     entry.MutationID,
		JobID:          entry.JobID,
		PlanID:         entry.PlanID,
		CapabilityID:   entry.CapabilityID,
		WorkspaceID:    entry.WorkspaceID,
		TargetType:     entry.TargetType,
		TargetID:       entry.TargetID,
		MutationType:   entry.MutationType,
		RiskTier:       entry.RiskTier,
		BeforeJson:     entry.BeforeJSON,
		AfterJson:      entry.AfterJSON,
		ProvenanceJson: entry.ProvenanceJSON,
		CreatedAt:      createdAt,
	})
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func toItemFromItemRow(row sqlcgen.ListItemsByWorkspaceRow) *domain.Item {
	return &domain.Item{
		ItemID:          row.ID,
		WorkspaceID:     row.WorkspaceID,
		ParentID:        row.ParentID.String,
		Title:           row.Title,
		Level:           int(row.Level),
		Description:     row.Description,
		Content:         row.Content,
		OverrideCSS:     row.OverrideCss,
		CreatedBy:       row.CreatedBy,
		GovernanceState: parseGovernanceState(row.GovernanceState),
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           treev1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

func toItemFromGetRow(row sqlcgen.GetItemRow) *domain.Item {
	return &domain.Item{
		ItemID:          row.ID,
		WorkspaceID:     row.WorkspaceID,
		ParentID:        row.ParentID.String,
		Title:           row.Title,
		Level:           int(row.Level),
		Description:     row.Description,
		Content:         row.Content,
		OverrideCSS:     row.OverrideCss,
		CreatedBy:       row.CreatedBy,
		GovernanceState: parseGovernanceState(row.GovernanceState),
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           treev1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

func toItemFromChildRow(row sqlcgen.ListChildItemsRow) *domain.Item {
	return &domain.Item{
		ItemID:          row.ID,
		WorkspaceID:     row.WorkspaceID,
		ParentID:        row.ParentID.String,
		Title:           row.Title,
		Level:           int(row.Level),
		Description:     row.Description,
		Content:         row.Content,
		OverrideCSS:     row.OverrideCss,
		CreatedBy:       row.CreatedBy,
		GovernanceState: parseGovernanceState(row.GovernanceState),
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           treev1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

func parseGovernanceState(s string) treev1.ItemGovernanceState {
	switch s {
	case "system_generated":
		return treev1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_SYSTEM_GENERATED
	case "pending_review":
		return treev1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_PENDING_REVIEW
	case "human_curated":
		return treev1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_HUMAN_CURATED
	case "locked":
		return treev1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_LOCKED
	default:
		return treev1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_UNSPECIFIED
	}
}
