package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
	"github.com/synthify/backend/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// GetItem retrieves a single knowledge tree item by its ID from PostgreSQL.
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

// CreateItem inserts a new knowledge tree node directly under a parent node or root.
func (s *Store) CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error) {
	return s.createStructuredItemDirect(ctx, workspaceID, label, 0, description, "", createdBy, parentID)
}

// createStructuredItemDirect inserts an item record into PostgreSQL without capability checks.
// Callers should wrap within Transactor.WithTx if atomic consistency is required.
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
	// Best-effort timestamp touch on parent tree.
	_ = s.q().UpdateTreeTimestamp(ctx, sqlcgen.UpdateTreeTimestampParams{
		ID:        item.ItemID,
		UpdatedAt: nowTime(),
	})
	return item, nil
}

// CreateStructuredItemWithCapability verifies job capability constraints, creates a structured knowledge item,
// and records an audit log entry in mutation_logs within PostgreSQL.
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

// UpsertItemSource links a document chunk source citation to a knowledge item in PostgreSQL.
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

// UpsertItemSourceRef stores or updates external provenance citations (URL, repository, line numbers) for an item.
func (s *Store) UpsertItemSourceRef(ctx context.Context, source *domain.ItemSourceRef) error {
	if source == nil {
		return nil
	}
	now := nowTime()
	sourceRefID := source.SourceRefID
	if sourceRefID == "" {
		sourceRefID = newID()
	}
	return s.q().UpsertItemSourceRef(ctx, sqlcgen.UpsertItemSourceRefParams{
		SourceRefID:  sourceRefID,
		ItemID:       source.ItemID,
		SourceType:   source.Type,
		Url:          source.URL,
		Repo:         source.Repo,
		Ref:          source.Ref,
		Path:         source.Path,
		LineStart:    nullInt32(source.LineStart),
		LineEnd:      nullInt32(source.LineEnd),
		Kind:         source.Kind,
		ExternalID:   source.ExternalID,
		Title:        source.Title,
		Confidence:   sql.NullFloat64{Float64: source.Confidence, Valid: source.Confidence > 0},
		SnapshotRef:  source.SnapshotRef,
		ContentHash:  source.ContentHash,
		MetadataJson: defaultMetadataJSON(source.MetadataJSON),
		CreatedAt:    now,
	})
}

// ListItemSources retrieves all document chunk source associations for an item.
func (s *Store) ListItemSources(ctx context.Context, itemID string) ([]*domain.ItemSource, error) {
	rows, err := s.q().ListItemSources(ctx, itemID)
	if err != nil {
		return nil, err
	}
	sources := make([]*domain.ItemSource, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, &domain.ItemSource{
			ItemID:     row.ItemID,
			DocumentID: row.DocumentID,
			FileID:     row.FileID,
			ChunkID:    row.ChunkID,
			SourceText: row.SourceText,
			Confidence: row.Confidence,
		})
	}
	return sources, nil
}

// ListItemSourceRefs retrieves all external and repository source references associated with an item.
func (s *Store) ListItemSourceRefs(ctx context.Context, itemID string) ([]*domain.ItemSourceRef, error) {
	rows, err := s.q().ListItemSourceRefs(ctx, itemID)
	if err != nil {
		return nil, err
	}
	sources := make([]*domain.ItemSourceRef, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, &domain.ItemSourceRef{
			SourceRefID:  row.SourceRefID,
			ItemID:       row.ItemID,
			Type:         row.SourceType,
			URL:          row.Url,
			Repo:         row.Repo,
			Ref:          row.Ref,
			Path:         row.Path,
			LineStart:    intFromNullInt32(row.LineStart),
			LineEnd:      intFromNullInt32(row.LineEnd),
			Kind:         row.Kind,
			ExternalID:   row.ExternalID,
			Title:        row.Title,
			Confidence:   row.Confidence,
			SnapshotRef:  row.SnapshotRef,
			ContentHash:  row.ContentHash,
			MetadataJSON: row.MetadataJson,
		})
	}
	return sources, nil
}

// UpdateItemSummaryHTMLWithCapability updates an item's rendered HTML content within the constraints of an active JobCapability.
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

// ApproveAlias accepts an alias relationship between canonicalItemID and aliasItemID in the workspace.
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

// RejectAlias records an explicit rejection of an alias suggestion between canonicalItemID and aliasItemID.
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

// canMutateTree verifies whether capability grants permission for op on the specified workspace and document.
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

// countJobMutations returns the number of mutations executed by jobID targeting targetType.
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

// logMutation writes a single audit row to mutation_logs using active transaction context if present.
func (s *Store) logMutation(ctx context.Context, entry *domain.JobMutationLog) error {
	return s.logMutationParams(ctx, s.q(), entry)
}

// logMutationTx writes an entry to mutation_logs scoped to an active database transaction.
func (s *Store) logMutationTx(ctx context.Context, tx *sql.Tx, entry *domain.JobMutationLog) error {
	return s.logMutationParams(ctx, s.q().WithTx(tx), entry)
}

// logMutationParams executes the SQL query to insert an entry into mutation_logs.
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

// nullInt32 wraps an int into a sql.NullInt32 if positive.
func nullInt32(value int) sql.NullInt32 {
	return sql.NullInt32{Int32: int32(value), Valid: value > 0}
}

// intFromNullInt32 extracts an int from sql.NullInt32 defaulting to 0 if null.
func intFromNullInt32(value sql.NullInt32) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int32)
}

// defaultMetadataJSON returns a valid empty JSON object "{}" if the input string is empty.
func defaultMetadataJSON(value string) string {
	if value == "" {
		return "{}"
	}
	return value
}

// mustJSON serializes a value into a JSON string, falling back to "{}" on serialization error.
func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// toItemFromItemRow converts a sqlc ListItemsByWorkspaceRow into a domain Item entity.
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

// toItemFromGetRow converts a sqlc GetItemRow into a domain Item entity.
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

// toItemFromChildRow converts a sqlc ListChildItemsRow into a domain Item entity.
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

// parseGovernanceState converts a string governance state into its appv1.ItemGovernanceState enum representation.
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
