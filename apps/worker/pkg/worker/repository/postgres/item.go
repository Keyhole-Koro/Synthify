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

// createStructuredItemDirect は items テーブルに 1 行挿入する。
// atomic 性が必要なら呼び出し側を Transactor.WithTx で包むこと。
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
	// updated_at の touch は best-effort。失敗しても item 作成自体は成功扱い。
	_ = s.q().UpdateTreeTimestamp(ctx, sqlcgen.UpdateTreeTimestampParams{
		ID:        item.ItemID,
		UpdatedAt: nowTime(),
	})
	return item, nil
}

// CreateStructuredItemWithCapability は capability 検証 + item 挿入 + mutation log を行う。
// capability 違反は domain.ErrForbidden を返す。atomic 性が必要なら呼び出し側を
// Transactor.WithTx で包むこと。
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

// CreateDocumentRootItemWithCapability inserts the document_root tree_item
// for a document and adds the matching document_tree_links row in the same
// transaction. The UNIQUE constraint on (document_id) ensures we cannot
// silently overwrite an existing root for the same document — a second
// call surfaces as ErrDocumentRootAlreadyExists via the FK violation path.
func (s *Store) CreateDocumentRootItemWithCapability(ctx context.Context, capability *domain.JobCapability, jobID, documentID, workspaceID, label, description, workspaceRootItemID string) (*domain.Item, error) {
	if !s.canMutateTree(capability, appv1.JobOperation_JOB_OPERATION_CREATE_ITEM, workspaceID, documentID) {
		return nil, fmt.Errorf("capability denies item creation: %w", domain.ErrForbidden)
	}
	if capability.MaxItemCreations > 0 && s.countJobMutations(ctx, jobID, "item") >= capability.MaxItemCreations {
		return nil, fmt.Errorf("max item creations reached for job: %w", domain.ErrForbidden)
	}

	createdAt := nowTime()
	itemID := newID()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	if err := qtx.CreateDocumentRootItem(ctx, sqlcgen.CreateDocumentRootItemParams{
		ID:          itemID,
		WorkspaceID: workspaceID,
		ParentID: sql.NullString{
			String: workspaceRootItemID,
			Valid:  workspaceRootItemID != "",
		},
		Title:             label,
		Description:       description,
		LastMutationJobID: jobID,
		CreatedAt:         createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create document root item: %w", err)
	}
	if err := qtx.CreateDocumentTreeLink(ctx, sqlcgen.CreateDocumentTreeLinkParams{
		DocumentID:  documentID,
		RootItemID:  itemID,
		WorkspaceID: workspaceID,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create document tree link: %w", err)
	}

	item := &domain.Item{
		ItemID:            itemID,
		WorkspaceID:       workspaceID,
		ParentID:          workspaceRootItemID,
		Title:             label,
		Level:             1,
		Description:       description,
		CreatedBy:         "worker",
		GovernanceState:   appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_SYSTEM_GENERATED,
		Kind:              appv1.ItemKind_ITEM_KIND_DOCUMENT_ROOT,
		LastMutationJobID: jobID,
		CreatedAt:         createdAt.Format(time.RFC3339),
	}

	if err := s.logMutationTx(ctx, tx, &domain.JobMutationLog{
		MutationID:     newID(),
		JobID:          jobID,
		CapabilityID:   capability.CapabilityID,
		WorkspaceID:    workspaceID,
		TargetType:     "item",
		TargetID:       itemID,
		MutationType:   "append",
		RiskTier:       "tier_1",
		BeforeJSON:     "{}",
		AfterJSON:      mustJSON(item),
		ProvenanceJSON: mustJSON(map[string]any{"document_id": documentID, "kind": "document_root"}),
		CreatedAt:      createdAt.Format(time.RFC3339),
	}); err != nil {
		return nil, fmt.Errorf("log mutation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit document root creation: %w", err)
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

// logMutation は mutation_logs に 1 行追記する。Store が Transactor.WithTx 経由で
// tx-bound されていれば、その tx の queries が使われる。
func (s *Store) logMutation(ctx context.Context, entry *domain.JobMutationLog) error {
	return s.logMutationParams(ctx, s.q(), entry)
}

func (s *Store) logMutationTx(ctx context.Context, tx *sql.Tx, entry *domain.JobMutationLog) error {
	return s.logMutationParams(ctx, s.q().WithTx(tx), entry)
}

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
		Kind:            parseItemKind(row.Kind),
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           appv1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
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
		Kind:            parseItemKind(row.Kind),
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           appv1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
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
		Kind:            parseItemKind(row.Kind),
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		Scope:           appv1.TreeProjectionScope_TREE_PROJECTION_SCOPE_DOCUMENT,
	}
}

// parseItemKind mirrors the api-side mapper. Keeping the two in sync is
// deliberate; the schema defines the canonical strings, and we accept the
// duplication until the worker/api domain split is collapsed.
func parseItemKind(s string) appv1.ItemKind {
	switch s {
	case "workspace_root":
		return appv1.ItemKind_ITEM_KIND_WORKSPACE_ROOT
	case "document_root":
		return appv1.ItemKind_ITEM_KIND_DOCUMENT_ROOT
	case "node":
		return appv1.ItemKind_ITEM_KIND_NODE
	default:
		return appv1.ItemKind_ITEM_KIND_UNSPECIFIED
	}
}

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
