package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/internal/db/sqlcgen"
	"github.com/synthify/backend/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
	sharedstorage "github.com/synthify/backend/internal/platform/storage"
)

// ListDocuments returns all documents uploaded to the specified workspace, ordered by creation time descending.
func (s *Store) ListDocuments(ctx context.Context, wsID string) ([]*domain.Document, error) {
	rows, err := s.q().ListDocuments(ctx, wsID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	docs := make([]*domain.Document, 0, len(rows))
	for _, row := range rows {
		docs = append(docs, toDocument(row))
	}
	return docs, nil
}

// GetDocument retrieves a single document record by ID.
// Returns domain.ErrNotFound if the document does not exist.
func (s *Store) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	row, err := s.q().GetDocument(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get document: %w", err)
	}
	return toDocument(row), nil
}

// GetDocumentChunks retrieves all extracted text chunks associated with the specified document.
func (s *Store) GetDocumentChunks(ctx context.Context, documentID string) ([]*domain.DocumentChunk, error) {
	rows, err := s.q().ListDocumentChunks(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("list document chunks: %w", err)
	}

	chunks := make([]*domain.DocumentChunk, 0, len(rows))
	for _, row := range rows {
		chunks = append(chunks, &domain.DocumentChunk{
			ChunkID:    row.ChunkID,
			DocumentID: row.DocumentID,
			Heading:    row.Heading,
			Text:       row.Text,
			SourcePage: int(row.SourcePage.Int32),
		})
	}
	return chunks, nil
}

// GetJobPlanningSignals collects contextual statistics (duplicate items, approved/protected aliases)
// used by the planner to determine risk tier and execution plan constraints.
func (s *Store) GetJobPlanningSignals(ctx context.Context, documentID, workspaceID, treeID string) (*domain.JobPlanningSignals, error) {
	signals := &domain.JobPlanningSignals{
		DocumentID:  documentID,
		WorkspaceID: workspaceID,
	}
	if strings.TrimSpace(documentID) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(treeID) == "" {
		return signals, nil
	}

	sameDocumentItemCount, err := s.q().CountSameDocumentItems(ctx, sqlcgen.CountSameDocumentItemsParams{
		DocumentID:  documentID,
		WorkspaceID: treeID,
	})
	if err != nil {
		return nil, fmt.Errorf("count same document items: %w", err)
	}
	approvedAliasCount, err := s.q().CountApprovedAliases(ctx, sqlcgen.CountApprovedAliasesParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: treeID,
	})
	if err != nil {
		return nil, fmt.Errorf("count approved aliases: %w", err)
	}
	protectedAliasCount, err := s.q().CountProtectedAliases(ctx, sqlcgen.CountProtectedAliasesParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: treeID,
	})
	if err != nil {
		return nil, fmt.Errorf("count protected aliases: %w", err)
	}

	signals.SameDocumentItemCount = int(sameDocumentItemCount)
	signals.ApprovedAliasCount = int(approvedAliasCount)
	signals.ProtectedAliasCount = int(protectedAliasCount)
	return signals, nil
}

// CreateDocument initializes a document record and temporary upload reservation,
// validating storage quotas before issuing a signed upload URL target.
func (s *Store) CreateDocument(ctx context.Context, wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error) {
	createdAt := nowTime()
	docID := newID()
	reservationID := newID()
	expiresAt := createdAt.Add(15 * time.Minute)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.Error("repository.create_document_tx_failed", "error", err.Error(), "workspace_id", wsID, "filename", filename)
		return nil, repository.DocumentUploadTarget{}, err
	}
	defer tx.Rollback()

	account, err := lockWorkspaceAccount(ctx, tx, wsID)
	if err != nil {
		s.logger.Error("repository.create_document_account_failed", "error", err.Error(), "workspace_id", wsID, "filename", filename)
		return nil, repository.DocumentUploadTarget{}, err
	}
	if err := validateUploadSize(account, fileSize); err != nil {
		s.logger.Warn("repository.create_document_quota_rejected", "error", err.Error(), "workspace_id", wsID, "filename", filename, "file_size", fileSize)
		return nil, repository.DocumentUploadTarget{}, err
	}
	reserved, err := activeReservedBytes(ctx, tx, account.AccountID, createdAt)
	if err != nil {
		s.logger.Error("repository.create_document_reservation_sum_failed", "error", err.Error(), "account_id", account.AccountID)
		return nil, repository.DocumentUploadTarget{}, err
	}
	if account.StorageUsedBytes+reserved+fileSize > account.StorageQuotaBytes {
		s.logger.Warn("repository.create_document_quota_rejected", "error", domain.ErrStorageQuotaExceeded.Error(), "account_id", account.AccountID, "file_size", fileSize)
		return nil, repository.DocumentUploadTarget{}, domain.ErrStorageQuotaExceeded
	}

	qtx := s.q().WithTx(tx)
	if err := qtx.CreateDocument(ctx, sqlcgen.CreateDocumentParams{
		DocumentID:  docID,
		WorkspaceID: wsID,
		UploadedBy:  uploadedBy,
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		CreatedAt:   createdAt,
	}); err != nil {
		s.logger.Error("repository.create_document_failed", "error", err.Error(), "workspace_id", wsID, "filename", filename)
		return nil, repository.DocumentUploadTarget{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO upload_reservations (
  reservation_id, account_id, workspace_id, document_id, expected_size_bytes,
  actual_size_bytes, status, expires_at, created_at
)
VALUES ($1, $2, $3, $4, $5, 0, 'reserved', $6, $7)
	`, reservationID, account.AccountID, wsID, docID, fileSize, expiresAt, createdAt); err != nil {
		s.logger.Error("repository.create_upload_reservation_failed", "error", err.Error(), "workspace_id", wsID, "document_id", docID)
		return nil, repository.DocumentUploadTarget{}, err
	}
	// Upload the original to {doc}/source/original.{ext}. The document root is a
	// directory; the worker reads from source/ and expands into extracted/.
	objectName := sharedstorage.SourceObjectName(docID, filename)
	target, err := s.uploadURLIssuer.IssueDocumentUploadURL(ctx, wsID, objectName, mimeType, fileSize)
	if err != nil {
		s.logger.Error("repository.issue_document_upload_url_failed", "error", err.Error(), "workspace_id", wsID, "document_id", docID)
		return nil, repository.DocumentUploadTarget{}, err
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("repository.create_document_commit_failed", "error", err.Error(), "workspace_id", wsID, "document_id", docID)
		return nil, repository.DocumentUploadTarget{}, err
	}

	return &domain.Document{
		DocumentID:  docID,
		WorkspaceID: wsID,
		UploadedBy:  uploadedBy,
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		CreatedAt:   createdAt.Format(time.RFC3339),
	}, target, nil
}

// ConfirmDocumentUpload verifies the uploaded file size matches the reservation,
// ensures storage quotas are maintained, marks the reservation confirmed, and increments used storage.
func (s *Store) ConfirmDocumentUpload(ctx context.Context, documentID string, actualSize int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var accountID string
	var expectedSize int64
	var status string
	var expiresAt time.Time
	if err := tx.QueryRowContext(ctx, `
SELECT account_id, expected_size_bytes, status, expires_at
FROM upload_reservations
WHERE document_id = $1
FOR UPDATE
`, documentID).Scan(&accountID, &expectedSize, &status, &expiresAt); err != nil {
		if err == sql.ErrNoRows {
			return domain.ErrUploadNotConfirmed
		}
		return err
	}
	if status == "confirmed" {
		return nil
	}
	if status != "reserved" || nowTime().After(expiresAt) {
		return domain.ErrUploadNotConfirmed
	}
	if expectedSize != actualSize {
		_, _ = tx.ExecContext(ctx, `
UPDATE upload_reservations
SET status = 'failed', actual_size_bytes = $2
WHERE document_id = $1
`, documentID, actualSize)
		return domain.ErrUploadSizeMismatch
	}
	var storageUsed int64
	var storageQuota int64
	if err := tx.QueryRowContext(ctx, `
SELECT storage_used_bytes, storage_quota_bytes
FROM accounts
WHERE account_id = $1
FOR UPDATE
`, accountID).Scan(&storageUsed, &storageQuota); err != nil {
		if err == sql.ErrNoRows {
			return domain.ErrNotFound
		}
		return err
	}
	if storageUsed+actualSize > storageQuota {
		return domain.ErrStorageQuotaExceeded
	}
	now := nowTime()
	if _, err := tx.ExecContext(ctx, `
UPDATE accounts
SET storage_used_bytes = storage_used_bytes + $2,
    updated_at = $3
WHERE account_id = $1
`, accountID, actualSize, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE upload_reservations
SET status = 'confirmed',
    actual_size_bytes = $2,
    confirmed_at = $3
WHERE document_id = $1
`, documentID, actualSize, now); err != nil {
		return err
	}
	return tx.Commit()
}

// ExpireUploadReservations scans for pending reservations past their expiration timestamp,
// transitions them to 'expired', and returns their identifiers for cleanup.
func (s *Store) ExpireUploadReservations(ctx context.Context, now time.Time) ([]domain.ExpiredReservation, error) {
	// RETURNING returns expired document/workspace IDs so caller can handle GCS object deletion.
	// Atomic single update ensures no race conditions or duplicate cleanups.
	rows, err := s.db.QueryContext(ctx, `
UPDATE upload_reservations
SET status = 'expired'
WHERE status = 'reserved' AND expires_at <= $1
RETURNING document_id, workspace_id
`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var expired []domain.ExpiredReservation
	for rows.Next() {
		var entry domain.ExpiredReservation
		if err := rows.Scan(&entry.DocumentID, &entry.WorkspaceID); err != nil {
			return nil, err
		}
		expired = append(expired, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return expired, nil
}

// lockWorkspaceAccount locks and retrieves the account associated with the specified workspace.
func lockWorkspaceAccount(ctx context.Context, tx *sql.Tx, workspaceID string) (*domain.Account, error) {
	var account domain.Account
	var maxUploadsPer5h int32
	var maxUploadsPerWeek int32
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `
SELECT a.account_id, a.name, a.plan, a.storage_quota_bytes, a.storage_used_bytes,
       a.max_file_size_bytes, a.max_uploads_per_5h, a.max_uploads_per_1week,
       a.stripe_customer_id, a.stripe_subscription_id, a.created_at
FROM workspaces w
JOIN accounts a ON a.account_id = w.account_id
WHERE w.workspace_id = $1 AND w.deleted_at IS NULL
FOR UPDATE OF a
`, workspaceID).Scan(
		&account.AccountID,
		&account.Name,
		&account.Plan,
		&account.StorageQuotaBytes,
		&account.StorageUsedBytes,
		&account.MaxFileSizeBytes,
		&maxUploadsPer5h,
		&maxUploadsPerWeek,
		&account.StripeCustomerID,
		&account.StripeSubscriptionID,
		&createdAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	account.MaxUploadsPerFiveH = int64(maxUploadsPer5h)
	account.MaxUploadsPerWeek = int64(maxUploadsPerWeek)
	account.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &account, nil
}

// activeReservedBytes sums up bytes currently reserved for unexpired uploads of an account.
func activeReservedBytes(ctx context.Context, tx *sql.Tx, accountID string, now time.Time) (int64, error) {
	var reserved sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(expected_size_bytes), 0)
FROM upload_reservations
WHERE account_id = $1 AND status = 'reserved' AND expires_at > $2
`, accountID, now).Scan(&reserved); err != nil {
		return 0, err
	}
	if !reserved.Valid {
		return 0, nil
	}
	return reserved.Int64, nil
}

// validateUploadSize checks whether the upload file size fits within plan limits and quotas.
func validateUploadSize(account *domain.Account, fileSize int64) error {
	if fileSize <= 0 {
		return domain.ErrUploadSizeMismatch
	}
	if account.MaxFileSizeBytes > 0 && fileSize > account.MaxFileSizeBytes {
		return domain.ErrFileTooLarge
	}
	if account.StorageQuotaBytes > 0 && account.StorageUsedBytes+fileSize > account.StorageQuotaBytes {
		return domain.ErrStorageQuotaExceeded
	}
	return nil
}

// CreateDocumentFile registers an extracted or source file under a document.
// The operation is idempotent on (document_id, path).
func (s *Store) CreateDocumentFile(ctx context.Context, docID, path, mimeType string, fileSize int64) (*domain.DocumentFile, error) {
	// Idempotent on (document_id, path): a re-registration returns the existing
	// file_id instead of violating the unique index.
	row, err := s.q().CreateDocumentFile(ctx, sqlcgen.CreateDocumentFileParams{
		FileID:     newID(),
		DocumentID: docID,
		Path:       path,
		MimeType:   mimeType,
		FileSize:   fileSize,
		CreatedAt:  nowTime(),
	})
	if err != nil {
		return nil, fmt.Errorf("create document file: %w", err)
	}
	return &domain.DocumentFile{
		FileID:     row.FileID,
		DocumentID: docID,
		Path:       path,
		MimeType:   mimeType,
		FileSize:   fileSize,
		CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// ListDocumentFiles returns metadata for all files associated with a given document.
func (s *Store) ListDocumentFiles(ctx context.Context, docID string) ([]*domain.DocumentFile, error) {
	rows, err := s.q().ListDocumentFiles(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("list document files: %w", err)
	}
	res := make([]*domain.DocumentFile, 0, len(rows))
	for _, row := range rows {
		res = append(res, &domain.DocumentFile{
			FileID:     row.FileID,
			DocumentID: row.DocumentID,
			Path:       row.Path,
			MimeType:   row.MimeType,
			FileSize:   row.FileSize,
			CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return res, nil
}

// GetDocumentFileByPath retrieves a file record by document ID and relative storage path.
// Returns domain.ErrNotFound if the file record does not exist.
func (s *Store) GetDocumentFileByPath(ctx context.Context, docID, path string) (*domain.DocumentFile, error) {
	row, err := s.q().GetDocumentFileByPath(ctx, sqlcgen.GetDocumentFileByPathParams{
		DocumentID: docID,
		Path:       path,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get document file: %w", err)
	}
	return &domain.DocumentFile{
		FileID:     row.FileID,
		DocumentID: row.DocumentID,
		Path:       row.Path,
		MimeType:   row.MimeType,
		FileSize:   row.FileSize,
		CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// GetDocumentFileForDelivery retrieves file location metadata needed to deliver or stream the file content.
// Returns domain.ErrNotFound if the file does not exist.
func (s *Store) GetDocumentFileForDelivery(ctx context.Context, fileID string) (*domain.DocumentFileLocation, error) {
	row, err := s.q().GetDocumentFileForDelivery(ctx, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get document file for delivery: %w", err)
	}
	return &domain.DocumentFileLocation{
		FileID:      row.FileID,
		DocumentID:  row.DocumentID,
		WorkspaceID: row.WorkspaceID,
		Path:        row.Path,
		MimeType:    row.MimeType,
	}, nil
}

// GetLatestProcessingJob retrieves the most recently created processing job for a document.
// Returns domain.ErrNotFound if no jobs exist for the document.
func (s *Store) GetLatestProcessingJob(ctx context.Context, docID string) (*domain.DocumentProcessingJob, error) {
	row, err := s.q().GetLatestProcessingJob(ctx, docID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get latest job: %w", err)
	}
	return toJob(row), nil
}

// GetProcessingJob retrieves a document processing job by its job ID.
// Returns domain.ErrNotFound if the job does not exist.
func (s *Store) GetProcessingJob(ctx context.Context, jobID string) (*domain.DocumentProcessingJob, error) {
	row, err := s.q().GetProcessingJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return toJob(row), nil
}

// GetJobCapability retrieves permission capabilities and execution quotas granted to a job.
// Returns domain.ErrNotFound if no capability row is found.
func (s *Store) GetJobCapability(ctx context.Context, jobID string) (*domain.JobCapability, error) {
	row, err := s.q().GetJobCapability(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get capability: %w", err)
	}
	var allowedDocs, allowedItems []string
	var allowedOperations []appv1.JobOperation
	_ = json.Unmarshal([]byte(row.AllowedDocumentIdsJson), &allowedDocs)
	_ = json.Unmarshal([]byte(row.AllowedItemIdsJson), &allowedItems)
	_ = json.Unmarshal([]byte(row.AllowedOperationsJson), &allowedOperations)

	return &domain.JobCapability{
		CapabilityID:          row.CapabilityID,
		JobID:                 row.JobID,
		WorkspaceID:           row.WorkspaceID,
		MaxLLMCalls:           int(row.MaxLlmCalls),
		MaxToolRuns:           int(row.MaxToolRuns),
		MaxItemCreations:      int(row.MaxItemCreations),
		MaxTransformCreations: int(row.MaxTransformCreations),
		MaxTransformRuns:      int(row.MaxTransformRuns),
		AllowedDocumentIDs:    allowedDocs,
		AllowedItemIDs:        allowedItems,
		AllowedOperations:     allowedOperations,
		ExpiresAt:             row.ExpiresAt.UTC().Format(time.RFC3339),
	}, nil
}

// GetJobExecutionPlan retrieves the execution plan record associated with a job.
// Returns domain.ErrNotFound if no plan exists for the job.
func (s *Store) GetJobExecutionPlan(ctx context.Context, jobID string) (*domain.JobExecutionPlan, error) {
	row, err := s.q().GetJobExecutionPlan(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get execution plan: %w", err)
	}
	return &domain.JobExecutionPlan{
		PlanID:    row.PlanID,
		JobID:     row.JobID,
		Status:    row.Status,
		Summary:   row.Summary,
		PlanJSON:  row.PlanJson,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// UpsertJobExecutionPlan creates or updates the execution plan for a processing job.
func (s *Store) UpsertJobExecutionPlan(ctx context.Context, jobID string, plan *domain.JobExecutionPlan) error {
	createdAt, _ := time.Parse(time.RFC3339, plan.CreatedAt)
	if createdAt.IsZero() {
		createdAt = nowTime()
	}
	if err := s.q().UpsertJobExecutionPlan(ctx, sqlcgen.UpsertJobExecutionPlanParams{
		PlanID:    plan.PlanID,
		JobID:     jobID,
		Status:    plan.Status,
		Summary:   plan.Summary,
		PlanJson:  plan.PlanJSON,
		CreatedBy: plan.CreatedBy,
		CreatedAt: createdAt,
		UpdatedAt: nowTime(),
	}); err != nil {
		return fmt.Errorf("upsert execution plan: %w", err)
	}
	return nil
}

// UpsertJobEvaluation updates the evaluation status of a processing job.
func (s *Store) UpsertJobEvaluation(ctx context.Context, jobID string, result *domain.JobEvaluationResult) error {
	if err := s.q().UpdateProcessingJobEvaluationState(ctx, sqlcgen.UpdateProcessingJobEvaluationStateParams{
		JobID:            jobID,
		EvaluationStatus: result.Status,
		UpdatedAt:        nowTime(),
	}); err != nil {
		return fmt.Errorf("upsert job evaluation: %w", err)
	}
	return nil
}

// EvaluateJob constructs a JobEvaluationResult from the current status of the processing job.
func (s *Store) EvaluateJob(ctx context.Context, jobID string) (*domain.JobEvaluationResult, error) {
	job, err := s.GetProcessingJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return &domain.JobEvaluationResult{
		JobID:   job.JobID,
		Status:  job.EvaluationStatus,
		Passed:  job.EvaluationStatus == "passed",
		Summary: "evaluation from job status",
	}, nil
}

// ListJobApprovalRequests lists all approval requests created for a processing job.
func (s *Store) ListJobApprovalRequests(ctx context.Context, jobID string) ([]*domain.JobApprovalRequest, error) {
	rows, err := s.q().ListJobApprovalRequests(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("list job approval requests: %w", err)
	}
	res := make([]*domain.JobApprovalRequest, 0, len(rows))
	for _, r := range rows {
		res = append(res, &domain.JobApprovalRequest{
			ApprovalID:  r.ApprovalID,
			JobID:       r.JobID,
			Status:      r.Status,
			RequestedBy: r.RequestedBy,
			Reason:      r.Reason,
			ReviewedBy:  r.ReviewedBy,
			ReviewedAt:  r.ReviewedAt.Time.Format(time.RFC3339),
			RequestedAt: r.RequestedAt.UTC().Format(time.RFC3339),
		})
	}
	return res, nil
}

// RequestJobApproval submits a request for human review and approval before executing an execution plan.
func (s *Store) RequestJobApproval(ctx context.Context, jobID, requestedBy, reason string) (*domain.JobApprovalRequest, error) {
	plan, err := s.GetJobExecutionPlan(ctx, jobID)
	if err != nil {
		return nil, err
	}

	var planData map[string]any
	if err := json.Unmarshal([]byte(plan.PlanJSON), &planData); err != nil {
		return nil, err
	}
	requestedOperations, _ := planData["steps"].([]any)
	requestedOpsJSON, _ := json.Marshal(requestedOperations)

	now := nowTime()
	approvalID := "apr_" + newID()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	if err := qtx.CreateJobApprovalRequest(ctx, sqlcgen.CreateJobApprovalRequestParams{
		ApprovalID:              approvalID,
		JobID:                   jobID,
		PlanID:                  plan.PlanID,
		Status:                  "pending",
		RequestedOperationsJson: string(requestedOpsJSON),
		Reason:                  firstNonEmptyNonSQL(reason, "approval required before execution"),
		RiskTier:                plan.HighestRiskTier(),
		RequestedBy:             firstNonEmptyNonSQL(requestedBy, "system"),
		ReviewedBy:              "",
		RequestedAt:             now,
		ReviewedAt:              sql.NullTime{},
	}); err != nil {
		return nil, fmt.Errorf("create approval request: %w", err)
	}
	if _, err := qtx.UpdateJobExecutionPlanStatus(ctx, sqlcgen.UpdateJobExecutionPlanStatusParams{
		PlanID:    plan.PlanID,
		Status:    string(domain.PlanStatusPendingApproval),
		UpdatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("update plan status: %w", err)
	}
	if err := qtx.UpdateProcessingJobPlanState(ctx, sqlcgen.UpdateProcessingJobPlanStateParams{
		JobID:           jobID,
		ExecutionPlanID: plan.PlanID,
		PlanStatus:      string(domain.PlanStatusPendingApproval),
		UpdatedAt:       now,
	}); err != nil {
		return nil, fmt.Errorf("update job plan state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &domain.JobApprovalRequest{
		ApprovalID:  approvalID,
		JobID:       jobID,
		PlanID:      plan.PlanID,
		Status:      "pending",
		RequestedBy: requestedBy,
		Reason:      reason,
		RequestedAt: now.UTC().Format(time.RFC3339),
	}, nil
}

// ApproveJobApproval marks an approval request as approved and transitions the associated plan to approved.
func (s *Store) ApproveJobApproval(ctx context.Context, jobID, approvalID, reviewedBy string) error {
	now := nowTime()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	planID, err := qtx.GetJobApprovalPlanID(ctx, sqlcgen.GetJobApprovalPlanIDParams{
		JobID:      jobID,
		ApprovalID: approvalID,
	})
	if err != nil {
		return fmt.Errorf("get plan id: %w", err)
	}
	if _, err := qtx.ApproveJobApproval(ctx, sqlcgen.ApproveJobApprovalParams{
		JobID:      jobID,
		ApprovalID: approvalID,
		ReviewedBy: firstNonEmptyNonSQL(reviewedBy, "reviewer"),
		ReviewedAt: sql.NullTime{Time: now, Valid: true},
	}); err != nil {
		return fmt.Errorf("approve approval: %w", err)
	}
	if _, err := qtx.UpdateJobExecutionPlanStatus(ctx, sqlcgen.UpdateJobExecutionPlanStatusParams{
		PlanID:    planID,
		Status:    string(domain.PlanStatusApproved),
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("update plan status: %w", err)
	}
	if err := qtx.UpdateProcessingJobPlanState(ctx, sqlcgen.UpdateProcessingJobPlanStateParams{
		JobID:           jobID,
		ExecutionPlanID: planID,
		PlanStatus:      string(domain.PlanStatusApproved),
		UpdatedAt:       now,
	}); err != nil {
		return fmt.Errorf("update job plan state: %w", err)
	}

	return tx.Commit()
}

// RejectJobApproval marks an approval request as rejected along with an optional reason.
func (s *Store) RejectJobApproval(ctx context.Context, jobID, approvalID, reviewedBy, reason string) error {
	now := nowTime()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	planID, err := qtx.GetJobApprovalPlanID(ctx, sqlcgen.GetJobApprovalPlanIDParams{
		JobID:      jobID,
		ApprovalID: approvalID,
	})
	if err != nil {
		return fmt.Errorf("get plan id: %w", err)
	}
	if _, err := qtx.RejectJobApproval(ctx, sqlcgen.RejectJobApprovalParams{
		JobID:      jobID,
		ApprovalID: approvalID,
		ReviewedBy: firstNonEmptyNonSQL(reviewedBy, "reviewer"),
		ReviewedAt: sql.NullTime{Time: now, Valid: true},
		Column5:    strings.TrimSpace(reason),
	}); err != nil {
		return fmt.Errorf("reject approval: %w", err)
	}
	if _, err := qtx.UpdateJobExecutionPlanStatus(ctx, sqlcgen.UpdateJobExecutionPlanStatusParams{
		PlanID:    planID,
		Status:    string(domain.PlanStatusRejected),
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("update plan status: %w", err)
	}
	if err := qtx.UpdateProcessingJobPlanState(ctx, sqlcgen.UpdateProcessingJobPlanStateParams{
		JobID:           jobID,
		ExecutionPlanID: planID,
		PlanStatus:      string(domain.PlanStatusRejected),
		UpdatedAt:       now,
	}); err != nil {
		return fmt.Errorf("update job plan state: %w", err)
	}

	return tx.Commit()
}

// CreateProcessingJob inserts one job_capabilities row and one processing_jobs row.
// It relies on any surrounding transaction provided by Transactor.WithTx via q().
func (s *Store) CreateProcessingJob(ctx context.Context, docID, workspaceID, requestedBy string, jobType appv1.JobType) (*domain.DocumentProcessingJob, error) {
	createdAt := nowTime()
	jobID := newID()
	doc, err := s.GetDocument(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	capability := domain.DefaultJobCapability(jobID, doc.WorkspaceID, docID, createdAt)
	allowedDocumentIDsJSON, err := json.Marshal(capability.AllowedDocumentIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal allowed document ids: %w", err)
	}
	allowedItemIDsJSON, err := json.Marshal(capability.AllowedItemIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal allowed item ids: %w", err)
	}
	allowedOperationsJSON, err := json.Marshal(capability.AllowedOperations)
	if err != nil {
		return nil, fmt.Errorf("marshal allowed operations: %w", err)
	}

	if err := s.q().CreateProcessingJob(ctx, sqlcgen.CreateProcessingJobParams{
		JobID:        jobID,
		DocumentID:   docID,
		WorkspaceID:  doc.WorkspaceID,
		JobType:      jobstatus.JobTypeToDB(jobType),
		Status:       jobstatus.LifecycleStateToDB(appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED),
		RequestedBy:  requestedBy,
		CapabilityID: capability.CapabilityID,
		CreatedAt:    createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create processing job: %w", err)
	}

	if err := s.q().CreateJobCapability(ctx, sqlcgen.CreateJobCapabilityParams{
		CapabilityID:           capability.CapabilityID,
		JobID:                  jobID,
		WorkspaceID:            doc.WorkspaceID,
		MaxLlmCalls:            int32(capability.MaxLLMCalls),
		MaxToolRuns:            int32(capability.MaxToolRuns),
		MaxItemCreations:       int32(capability.MaxItemCreations),
		MaxTransformCreations:  int32(capability.MaxTransformCreations),
		MaxTransformRuns:       int32(capability.MaxTransformRuns),
		AllowedDocumentIdsJson: string(allowedDocumentIDsJSON),
		AllowedItemIdsJson:     string(allowedItemIDsJSON),
		AllowedOperationsJson:  string(allowedOperationsJSON),
		ExpiresAt:              createdAt.Add(24 * time.Hour),
		CreatedAt:              createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create job capability: %w", err)
	}

	return &domain.DocumentProcessingJob{
		JobID:        jobID,
		DocumentID:   docID,
		WorkspaceID:  doc.WorkspaceID,
		JobType:      jobType,
		Status:       appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
		RequestedBy:  requestedBy,
		CapabilityID: capability.CapabilityID,
		CreatedAt:    createdAt.Format(time.RFC3339),
		UpdatedAt:    createdAt.Format(time.RFC3339),
	}, nil
}

// MarkProcessingJobRunning sets a job's status to running.
// Returns domain.ErrNotFound if the job does not exist.
func (s *Store) MarkProcessingJobRunning(ctx context.Context, jobID string) error {
	rowsAffected, err := s.q().MarkProcessingJobRunning(ctx, sqlcgen.MarkProcessingJobRunningParams{
		JobID:     jobID,
		UpdatedAt: nowTime(),
	})
	if err != nil {
		return fmt.Errorf("mark job running: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateProcessingJobStage updates the current execution stage name of a processing job.
func (s *Store) UpdateProcessingJobStage(ctx context.Context, jobID, stage string) error {
	if err := s.q().UpdateProcessingJobStage(ctx, sqlcgen.UpdateProcessingJobStageParams{
		JobID:        jobID,
		CurrentStage: stage,
		UpdatedAt:    nowTime(),
	}); err != nil {
		return fmt.Errorf("update job stage: %w", err)
	}
	return nil
}

// FailProcessingJob transitions a processing job to failed state with an error message.
// Returns domain.ErrNotFound if the job does not exist.
func (s *Store) FailProcessingJob(ctx context.Context, jobID, errorMessage string) error {
	rowsAffected, err := s.q().FailProcessingJob(ctx, sqlcgen.FailProcessingJobParams{
		JobID:        jobID,
		ErrorMessage: errorMessage,
		UpdatedAt:    nowTime(),
	})
	if err != nil {
		return fmt.Errorf("fail job: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetProcessingJobRetryCount sets the retry attempt counter for a processing job.
// Returns domain.ErrNotFound if the job does not exist.
func (s *Store) SetProcessingJobRetryCount(ctx context.Context, jobID string, retryCount int) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE document_processing_jobs
SET retry_count = $2, updated_at = $3
WHERE job_id = $1
`, jobID, retryCount, nowTime())
	if err != nil {
		return fmt.Errorf("set job retry count: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set job retry count rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CompleteProcessingJob marks a processing job as succeeded and clears any failure messages.
// Returns domain.ErrNotFound if the job does not exist.
func (s *Store) CompleteProcessingJob(ctx context.Context, jobID string) error {
	rowsAffected, err := s.q().CompleteProcessingJob(ctx, sqlcgen.CompleteProcessingJobParams{
		JobID:     jobID,
		UpdatedAt: nowTime(),
	})
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpsertStageRunning records that a pipeline stage has started running for a job.
func (s *Store) UpsertStageRunning(ctx context.Context, jobID, stage string) error {
	return s.q().UpsertJobStageCheckpoint(ctx, sqlcgen.UpsertJobStageCheckpointParams{
		JobID:     jobID,
		Stage:     stage,
		Status:    "running",
		GcsRef:    "",
		UpdatedAt: nowTime(),
	})
}

// MarkStageSucceeded records the successful completion of a stage and attaches its GCS artifact reference.
func (s *Store) MarkStageSucceeded(ctx context.Context, jobID, stage, gcsRef string) error {
	return s.q().UpsertJobStageCheckpoint(ctx, sqlcgen.UpsertJobStageCheckpointParams{
		JobID:     jobID,
		Stage:     stage,
		Status:    "succeeded",
		GcsRef:    gcsRef,
		UpdatedAt: nowTime(),
	})
}

// MarkStageFailed records that a pipeline stage failed execution.
func (s *Store) MarkStageFailed(ctx context.Context, jobID, stage, errorMessage string) error {
	// We could store the error message in the envelope if needed,
	// for the DB index we just mark it failed.
	return s.q().UpsertJobStageCheckpoint(ctx, sqlcgen.UpsertJobStageCheckpointParams{
		JobID:     jobID,
		Stage:     stage,
		Status:    "failed",
		GcsRef:    "",
		UpdatedAt: nowTime(),
	})
}

// ListStageCheckpoints returns all stage execution checkpoints recorded for a job.
func (s *Store) ListStageCheckpoints(ctx context.Context, jobID string) ([]domain.JobStageCheckpoint, error) {
	rows, err := s.q().ListJobStageCheckpoints(ctx, jobID)
	if err != nil {
		return nil, err
	}
	res := make([]domain.JobStageCheckpoint, 0, len(rows))
	for _, row := range rows {
		res = append(res, domain.JobStageCheckpoint{
			JobID:     row.JobID,
			Stage:     row.Stage,
			Status:    row.Status,
			GCSRef:    row.GcsRef,
			UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return res, nil
}

// SaveDocumentChunks atomically replaces all chunks for a document with the provided slice.
func (s *Store) SaveDocumentChunks(ctx context.Context, documentID string, chunks []*domain.DocumentChunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.q().WithTx(tx)
	if err := qtx.DeleteDocumentChunks(ctx, documentID); err != nil {
		return err
	}

	for _, chunk := range chunks {
		if err := qtx.CreateDocumentChunk(ctx, sqlcgen.CreateDocumentChunkParams{
			ChunkID:    chunk.ChunkID,
			DocumentID: documentID,
			FileID:     sql.NullString{String: chunk.FileID, Valid: chunk.FileID != ""},
			Heading:    chunk.Heading,
			Text:       chunk.Text,
			SourcePage: sql.NullInt32{Int32: int32(chunk.SourcePage), Valid: chunk.SourcePage > 0},
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ListJobMutationLogs retrieves all mutation audit log entries produced during job execution.
func (s *Store) ListJobMutationLogs(ctx context.Context, jobID string) ([]*domain.JobMutationLog, error) {
	rows, err := s.q().ListJobMutationLogs(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("list job mutation logs: %w", err)
	}
	res := make([]*domain.JobMutationLog, 0, len(rows))
	for _, r := range rows {
		res = append(res, &domain.JobMutationLog{
			MutationID:     r.MutationID,
			JobID:          r.JobID,
			PlanID:         r.PlanID,
			CapabilityID:   r.CapabilityID,
			WorkspaceID:    r.WorkspaceID,
			TargetType:     r.TargetType,
			TargetID:       r.TargetID,
			MutationType:   r.MutationType,
			RiskTier:       r.RiskTier,
			BeforeJSON:     r.BeforeJson,
			AfterJSON:      r.AfterJson,
			ProvenanceJSON: r.ProvenanceJson,
			CreatedAt:      r.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return res, nil
}

// ListAllJobs returns all document processing jobs in the system, ordered by creation time descending.
func (s *Store) ListAllJobs(ctx context.Context) ([]*domain.DocumentProcessingJob, error) {
	rows, err := s.q().ListAllJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all jobs: %w", err)
	}
	res := make([]*domain.DocumentProcessingJob, 0, len(rows))
	for _, row := range rows {
		res = append(res, toJob(row))
	}
	return res, nil
}

// LogToolCall records a mutation log entry for a dynamic tool invocation within a job.
func (s *Store) LogToolCall(ctx context.Context, jobID, toolName, inputJSON, outputJSON string, durationMs int64) error {
	now := nowTime()
	return s.q().CreateJobMutationLog(ctx, sqlcgen.CreateJobMutationLogParams{
		MutationID:     newID(),
		JobID:          jobID,
		WorkspaceID:    "", // Will be updated by job status if needed
		TargetType:     "tool_call",
		TargetID:       toolName,
		MutationType:   "execute",
		RiskTier:       "tier_0",
		BeforeJson:     inputJSON,
		AfterJson:      outputJSON,
		ProvenanceJson: fmt.Sprintf(`{"duration_ms": %d}`, durationMs),
		CreatedAt:      now,
	})
}

// SearchRelatedChunksByVector queries document chunks in the workspace using cosine similarity on embeddings.
func (s *Store) SearchRelatedChunksByVector(ctx context.Context, workspaceID string, embedding []float32, limit int) ([]*domain.DocumentChunk, error) {
	rows, err := s.q().SearchWorkspaceDocumentChunksByVector(ctx, sqlcgen.SearchWorkspaceDocumentChunksByVectorParams{
		WorkspaceID:    workspaceID,
		QueryEmbedding: embedding,
		MinSimilarity:  0.0, // Adjust threshold as needed
		ResultLimit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	res := make([]*domain.DocumentChunk, 0, len(rows))
	for _, r := range rows {
		res = append(res, &domain.DocumentChunk{
			ChunkID:    r.ChunkID,
			DocumentID: r.DocumentID,
			Heading:    r.Heading,
			Text:       r.Text,
			SourcePage: int(r.SourcePage.Int32),
		})
	}
	return res, nil
}

// toDocument converts a sqlcgen.Document row to a domain.Document entity.
func toDocument(row sqlcgen.Document) *domain.Document {
	return &domain.Document{
		DocumentID:  row.DocumentID,
		WorkspaceID: row.WorkspaceID,
		UploadedBy:  row.UploadedBy,
		Filename:    row.Filename,
		MimeType:    row.MimeType,
		FileSize:    row.FileSize,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// toJob converts a sqlcgen.DocumentProcessingJob query row into a domain.DocumentProcessingJob.
func toJob(row sqlcgen.DocumentProcessingJob) *domain.DocumentProcessingJob {
	return &domain.DocumentProcessingJob{
		JobID:            row.JobID,
		DocumentID:       row.DocumentID,
		WorkspaceID:      row.WorkspaceID,
		CapabilityID:     row.CapabilityID,
		ExecutionPlanID:  row.ExecutionPlanID,
		JobType:          jobstatus.JobTypeFromDB(row.JobType),
		Status:           jobstatus.LifecycleStateFromDB(row.Status),
		CurrentStage:     row.CurrentStage,
		PlanStatus:       row.PlanStatus,
		EvaluationStatus: row.EvaluationStatus,
		ErrorMessage:     row.ErrorMessage,
		ParamsJSON:       row.ParamsJson,
		RequestedBy:      row.RequestedBy,
		RetryCount:       int(row.RetryCount),
		BudgetJSON:       row.BudgetJson,
		CreatedAt:        row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// firstNonEmptyNonSQL returns the first non-empty string or the fallback if v is blank.
func firstNonEmptyNonSQL(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
