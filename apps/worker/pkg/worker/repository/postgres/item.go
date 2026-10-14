package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/postgres/sqlcgen"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
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

func (s *Store) CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error) {
	return s.createStructuredItemDirect(ctx, workspaceID, label, 0, description, "", createdBy, parentID)
}

// createStructuredItemDirect inserts a single row into the items table.
// Wrap with Transactor.WithTx if atomic execution is needed.
func (s *Store) createStructuredItemDirect(ctx context.Context, workspaceID, label string, level int, description, summaryHTML, createdBy, parentID string) (*domain.Item, error) {
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

	if err := s.q().CreateItem(ctx, sqlcgen.CreateItemParams{
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
		return nil, fmt.Errorf("create item: %w", err)
	}
	// Best-effort touch of parent tree timestamp; failure does not abort item creation.
	_ = s.q().UpdateTreeTimestamp(ctx, sqlcgen.UpdateTreeTimestampParams{
		ID:        item.ItemID,
		UpdatedAt: nowTime(),
	})
	return item, nil
}

// CreateStructuredItemWithCapability performs capability verification, item insertion, and mutation logging.
// Capability violations return domain.ErrForbidden. Wrap with Transactor.WithTx if atomic execution is needed.
func (s *Store) CreateStructuredItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label string, level int, description, summaryHTML, overrideCSS, createdBy, parentID string, sourceChunkIDs []string) (*domain.Item, error) {
	if !s.canMutateTree(capability, appv1.JobOperation_JOB_OPERATION_CREATE_ITEM, workspaceID, documentID) {
		return nil, fmt.Errorf("capability denies item creation: %w", domain.ErrForbidden)
	}
	if capability.MaxItemCreations > 0 && s.countJobMutations(ctx, jobID, "item") >= capability.MaxItemCreations {
		return nil, fmt.Errorf("max item creations reached for job: %w", domain.ErrForbidden)
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
		GovernanceState:   appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_SYSTEM_GENERATED,
		LastMutationJobID: jobID,
		CreatedAt:         createdAt.Format(time.RFC3339),
	}

	if err := s.q().CreateStructuredItem(ctx, sqlcgen.CreateStructuredItemParams{
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
		return nil, fmt.Errorf("create structured item: %w", err)
	}
	if err := s.logMutation(ctx, &domain.JobMutationLog{
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
		return nil, fmt.Errorf("log mutation: %w", err)
	}
	return item, nil
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
	if capability == nil || !capability.Allows(appv1.JobOperation_JOB_OPERATION_UPDATE_ITEM) || capability.IsExpired(nowTime()) {
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
	if row.GovernanceState == string(domain.GovernanceStateHumanCurated) || row.GovernanceState == string(domain.GovernanceStateLocked) {
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

// MergeItemWithCapability folds a document's knowledge into an existing tree
// node: it rewrites the node's title/description/content and marks it
// cross_document. Callers pair this with UpsertItemSource to record the new
// document's provenance. Capability checks mirror the other mutating methods.
func (s *Store) MergeItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, itemID, title, description, content string) (*domain.Item, error) {
	if capability == nil || !capability.Allows(appv1.JobOperation_JOB_OPERATION_UPDATE_ITEM) || capability.IsExpired(nowTime()) {
		return nil, fmt.Errorf("mutation not allowed by capability or expired")
	}
	if !capability.AllowsItem(itemID) {
		return nil, fmt.Errorf("item not in capability scope: %w", domain.ErrForbidden)
	}

	existing, err := s.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if capability.WorkspaceID != "" && capability.WorkspaceID != existing.WorkspaceID {
		return nil, fmt.Errorf("workspace mismatch")
	}

	now := nowTime()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	affected, err := qtx.UpdateItemForMerge(ctx, sqlcgen.UpdateItemForMergeParams{
		ID:                itemID,
		Title:             title,
		Description:       description,
		Content:           content,
		LastMutationJobID: jobID,
		UpdatedAt:         now,
	})
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, domain.ErrNotFound
	}

	merged := *existing
	merged.Title = title
	merged.Description = description
	merged.Content = content
	merged.CrossDocument = true
	merged.LastMutationJobID = jobID

	if err := s.logMutationTx(ctx, tx, &domain.JobMutationLog{
		MutationID:     newID(),
		JobID:          jobID,
		CapabilityID:   capability.CapabilityID,
		WorkspaceID:    existing.WorkspaceID,
		TargetType:     "item",
		TargetID:       itemID,
		MutationType:   "merge",
		RiskTier:       "tier_1",
		BeforeJSON:     mustJSON(map[string]string{"title": existing.Title, "description": existing.Description, "content": existing.Content}),
		AfterJSON:      mustJSON(map[string]string{"title": title, "description": description, "content": content}),
		ProvenanceJSON: mustJSON(map[string]any{"merge": true}),
		CreatedAt:      now.Format(time.RFC3339),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &merged, nil
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

// canMutateTree verifies whether capability allows performing the given tree mutation operation.
func (s *Store) canMutateTree(capability *domain.JobCapability, op appv1.JobOperation, workspaceID, documentID string) bool {
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

// countJobMutations counts the number of mutations executed by a job on a given target type.
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

// logMutation appends a record to the mutation_logs table. If Store is bound to a
// transaction via Transactor.WithTx, that transaction's queries are used.
func (s *Store) logMutation(ctx context.Context, entry *domain.JobMutationLog) error {
	return s.logMutationParams(ctx, s.q(), entry)
}

// logMutationTx appends a record to the mutation_logs table within the specified transaction.
func (s *Store) logMutationTx(ctx context.Context, tx *sql.Tx, entry *domain.JobMutationLog) error {
	return s.logMutationParams(ctx, s.q().WithTx(tx), entry)
}

// logMutationParams executes the database insertion for a job mutation log entry.
func (s *Store) logMutationParams(ctx context.Context, q *sqlcgen.Queries, entry *domain.JobMutationLog) error {
	createdAt, err := time.Parse(time.RFC3339, entry.CreatedAt)
	if err != nil {
		createdAt = nowTime()
	}
	return q.InsertJobMutationLog(ctx, sqlcgen.InsertJobMutationLogParams{
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

// mustJSON serializes an arbitrary value to a JSON string, returning "{}" on failure.
func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// toItemFromItemRow maps a sqlc ListItemsByWorkspaceRow to a domain Item model.
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
		CrossDocument:   row.CrossDocument,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           appv1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

// toItemFromGetRow maps a sqlc GetItemRow to a domain Item model.
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
		CrossDocument:   row.CrossDocument,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           appv1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

// toItemFromChildRow maps a sqlc ListChildItemsRow to a domain Item model.
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
		CrossDocument:   row.CrossDocument,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           appv1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

// parseGovernanceState converts a governance state database string to an enum value.
func parseGovernanceState(s string) appv1.ItemGovernanceState {
	switch s {
	case "system_generated":
		return appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_SYSTEM_GENERATED
	case "pending_review":
		return appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_PENDING_REVIEW
	case "human_curated":
		return appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_HUMAN_CURATED
	case "locked":
		return appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_LOCKED
	default:
		return appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_UNSPECIFIED
	}
}
