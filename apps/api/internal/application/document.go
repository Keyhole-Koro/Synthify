package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	joblifecycle "github.com/synthify/backend/internal/platform/job/lifecycle"
	joblog "github.com/synthify/backend/internal/platform/job/log"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

// WorkerDispatcher abstracts dispatching asynchronous job planning and execution tasks to workers.
type WorkerDispatcher interface {
	GenerateExecutionPlan(ctx context.Context, req domain.ExecutePlanRequest) error
	ExecuteApprovedPlan(ctx context.Context, req domain.ExecutePlanRequest) error
}

// ObjectMetadataFetcher retrieves storage object metadata such as content type and byte size.
type ObjectMetadataFetcher interface {
	GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error)
}

// DocumentUsecase defines the use-case interface for document lifecycle management, uploads, and processing.
// Each method verifies workspace authorization using the user ID, returning domain.ErrForbidden on unauthorized access.
type DocumentUsecase interface {
	ListDocuments(ctx context.Context, workspaceID, userID string) ([]*domain.Document, error)
	GetDocument(ctx context.Context, documentID, userID string) (*domain.Document, error)
	CreateDocument(ctx context.Context, wsID, userID, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error)
	ConfirmUpload(ctx context.Context, documentID, userID string) (*domain.Document, error)
	StartProcessing(ctx context.Context, documentID, userID string, forceReprocess bool) (*domain.DocumentProcessingJob, error)
	ResumeProcessing(ctx context.Context, documentID, userID string) (*domain.DocumentProcessingJob, error)
	IssueImageURL(ctx context.Context, fileID, userID string) (repository.SignedURL, error)
}

const (
	autoResumeMaxRetries = 1
	autoResumeMaxAge     = 24 * time.Hour
	// stuckJobMinAge is how long a job may sit in QUEUED/RUNNING before it is
	// treated as stuck. A job wedged in RUNNING (worker crash mid-run, or Cloud
	// Tasks exhausting max_attempts and silently dropping the task) never emits
	// a JobFailed event, so without this sweep it is invisible.
	stuckJobMinAge = 30 * time.Minute
)

// DocumentService orchestrates document storage reservations, upload confirmations, and background worker jobs.
type DocumentService struct {
	repo             repository.DocumentRepository
	jobs             repository.JobRepository
	accounts         repository.AccountRepository
	workspaces       repository.WorkspaceRepository
	tree             repository.TreeRepository
	transactor       repository.Transactor
	sourceURLBuilder repository.DocumentSourceURLBuilder
	imageURLIssuer   repository.DocumentImageURLIssuer
	objectMetadata   ObjectMetadataFetcher
	objectStore      repository.DocumentObjectStore
	dispatcher       WorkerDispatcher
	lifecycle        joblifecycle.QueuedNotifier
	notifier         jobstatus.Notifier
	logger           *slog.Logger
	nrApp            *newrelic.Application
}

// DocumentServiceDeps defines dependencies required to instantiate a DocumentService.
type DocumentServiceDeps struct {
	Repo             repository.DocumentRepository
	Jobs             repository.JobRepository
	Accounts         repository.AccountRepository
	LifecycleRepo    joblifecycle.Repository
	Workspaces       repository.WorkspaceRepository
	Tree             repository.TreeRepository
	Transactor       repository.Transactor
	SourceURLBuilder repository.DocumentSourceURLBuilder
	ImageURLIssuer   repository.DocumentImageURLIssuer
	ObjectMetadata   ObjectMetadataFetcher
	ObjectStore      repository.DocumentObjectStore
	Dispatcher       WorkerDispatcher
	Notifier         jobstatus.Notifier
	Logger           *slog.Logger
	NRApp            *newrelic.Application
}

// NewDocumentService creates a new DocumentService instance.
func NewDocumentService(deps DocumentServiceDeps) *DocumentService {
	return &DocumentService{
		repo:             deps.Repo,
		jobs:             deps.Jobs,
		accounts:         deps.Accounts,
		workspaces:       deps.Workspaces,
		tree:             deps.Tree,
		transactor:       deps.Transactor,
		sourceURLBuilder: deps.SourceURLBuilder,
		imageURLIssuer:   deps.ImageURLIssuer,
		objectMetadata:   deps.ObjectMetadata,
		objectStore:      deps.ObjectStore,
		dispatcher:       deps.Dispatcher,
		lifecycle:        joblifecycle.New(deps.LifecycleRepo, deps.Notifier, deps.Logger, deps.NRApp),
		notifier:         deps.Notifier,
		logger:           deps.Logger,
		nrApp:            deps.NRApp,
	}
}

// authorizeWorkspace verifies that the specified user has access to the workspace.
func (s *DocumentService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// IssueImageURL resolves an extracted file identifier into a short-lived signed image delivery URL.
func (s *DocumentService) IssueImageURL(ctx context.Context, fileID, userID string) (repository.SignedURL, error) {
	if s.imageURLIssuer == nil {
		return repository.SignedURL{}, domain.ErrNotFound
	}
	loc, err := s.repo.GetDocumentFileForDelivery(ctx, fileID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return repository.SignedURL{}, domain.ErrForbidden
		}
		return repository.SignedURL{}, err
	}
	if err := s.authorizeWorkspace(ctx, loc.WorkspaceID, userID); err != nil {
		return repository.SignedURL{}, err
	}
	return s.imageURLIssuer.IssueDocumentImageURL(ctx, loc.WorkspaceID, loc.DocumentID, loc.Path)
}

// authorizeWrite verifies that the user holds editor or owner role in the workspace.
func (s *DocumentService) authorizeWrite(ctx context.Context, workspaceID, userID string) error {
	role, err := s.workspaces.GetWorkspaceRole(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !role.CanWrite() {
		return domain.ErrForbidden
	}
	return nil
}

// ensureBudgetAvailable verifies that the user's account has not exceeded its monthly spending budget limit.
func (s *DocumentService) ensureBudgetAvailable(ctx context.Context, userID string) error {
	if s.accounts == nil {
		return nil
	}
	account, err := s.accounts.GetAccountByUser(ctx, userID)
	if err != nil {
		return nil
	}
	if account.BudgetExceeded {
		return domain.ErrBillingBudgetExceeded
	}
	return nil
}

// authorizeDocument retrieves a document while verifying workspace read accessibility.
func (s *DocumentService) authorizeDocument(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	doc, err := s.repo.GetDocument(ctx, documentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrForbidden
		}
		return nil, err
	}
	if err := s.authorizeWorkspace(ctx, doc.WorkspaceID, userID); err != nil {
		return nil, err
	}
	return doc, nil
}

// authorizeDocumentWrite retrieves a document while verifying workspace write accessibility.
func (s *DocumentService) authorizeDocumentWrite(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	doc, err := s.repo.GetDocument(ctx, documentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrForbidden
		}
		return nil, err
	}
	if err := s.authorizeWrite(ctx, doc.WorkspaceID, userID); err != nil {
		return nil, err
	}
	return doc, nil
}

// ListDocuments returns all documents in a workspace after verifying authorization.
func (s *DocumentService) ListDocuments(ctx context.Context, workspaceID, userID string) ([]*domain.Document, error) {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListDocuments(ctx, workspaceID)
}

// GetDocument retrieves a single document by its identifier after authorization.
func (s *DocumentService) GetDocument(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	return s.authorizeDocument(ctx, documentID, userID)
}

// CreateDocument initializes a document record and temporary upload reservation after verifying quotas and allowed mime types.
func (s *DocumentService) CreateDocument(ctx context.Context, wsID, userID, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error) {
	if err := s.authorizeWrite(ctx, wsID, userID); err != nil {
		return nil, repository.DocumentUploadTarget{}, err
	}
	if err := domain.ValidateDocumentUploadType(filename, mimeType); err != nil {
		s.reportCreateDocumentRejection(wsID, userID, filename, fileSize, err)
		return nil, repository.DocumentUploadTarget{}, err
	}
	doc, target, err := s.repo.CreateDocument(ctx, wsID, userID, filename, mimeType, fileSize)
	if err != nil {
		s.reportCreateDocumentRejection(wsID, userID, filename, fileSize, err)
	}
	return doc, target, err
}

// reportCreateDocumentRejection records a custom event in New Relic when an upload request is rejected due to policy violations.
func (s *DocumentService) reportCreateDocumentRejection(workspaceID, userID, filename string, fileSize int64, cause error) {
	if s.nrApp == nil {
		return
	}
	var reason string
	switch {
	case errors.Is(cause, domain.ErrFileTooLarge):
		reason = "file_too_large"
	case errors.Is(cause, domain.ErrStorageQuotaExceeded):
		reason = "storage_quota_exceeded"
	case errors.Is(cause, domain.ErrUploadSizeMismatch):
		reason = "size_mismatch"
	case errors.Is(cause, domain.ErrUnsupportedDocumentType):
		reason = "unsupported_document_type"
	default:
		// Non-quota errors (DB outage, etc.) are already covered by the Connect
		// interceptor — skip to avoid double-counting.
		return
	}
	s.nrApp.RecordCustomEvent("UploadRejected", map[string]any{
		"workspace_id": workspaceID,
		"user_id":      userID,
		"filename":     filename,
		"file_size":    fileSize,
		"reason":       reason,
	})
}

// ConfirmUpload confirms the successful upload of a document payload by validating object presence and storage size.
func (s *DocumentService) ConfirmUpload(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	doc, err := s.authorizeDocumentWrite(ctx, documentID, userID)
	if err != nil {
		return nil, err
	}
	if err := s.confirmUploadedObject(ctx, doc); err != nil {
		return nil, err
	}
	return s.repo.GetDocument(ctx, documentID)
}

// ExpireUploadReservations identifies expired pending reservations, transitions them to expired, and cleans up orphaned objects.
func (s *DocumentService) ExpireUploadReservations(ctx context.Context, now time.Time) (int64, error) {
	expired, err := s.repo.ExpireUploadReservations(ctx, now)
	if err != nil {
		s.logger.Error("document.upload_reservations.expire_failed", "error", err.Error())
		return 0, err
	}
	if len(expired) > 0 {
		s.logger.Info("document.upload_reservations.expired", "count", len(expired))
		for _, entry := range expired {
			s.deleteOrphanedObject(ctx, entry.WorkspaceID, entry.DocumentID, "reservation_expired")
		}
	}
	return int64(len(expired)), nil
}

// deleteOrphanedObject deletes orphaned cloud storage objects on a best-effort basis without interrupting main workflows.
func (s *DocumentService) deleteOrphanedObject(ctx context.Context, workspaceID, documentID, reason string) {
	if s.objectStore == nil {
		return
	}
	if err := s.objectStore.DeleteDocumentObject(ctx, workspaceID, documentID); err != nil {
		s.logger.Warn("document.orphan_object_delete_failed",
			"error", err.Error(),
			"workspace_id", workspaceID,
			"document_id", documentID,
			"reason", reason,
		)
		s.reportUploadIncident("OrphanObjectDeleteFailed", workspaceID, documentID, reason, err)
		return
	}
	s.logger.Info("document.orphan_object_deleted",
		"workspace_id", workspaceID,
		"document_id", documentID,
		"reason", reason,
	)
}

// reportUploadIncident records upload policy violations and anomalies to New Relic custom events and error notices.
func (s *DocumentService) reportUploadIncident(eventName, workspaceID, documentID, reason string, cause error) {
	if s.nrApp == nil {
		return
	}
	attrs := map[string]any{
		"workspace_id": workspaceID,
		"document_id":  documentID,
		"reason":       reason,
	}
	if cause != nil {
		attrs["error"] = cause.Error()
	}
	s.nrApp.RecordCustomEvent(eventName, attrs)
	if cause == nil {
		return
	}
	tx := s.nrApp.StartTransaction("document.upload_incident")
	tx.AddAttribute("workspace_id", workspaceID)
	tx.AddAttribute("document_id", documentID)
	tx.AddAttribute("reason", reason)
	tx.AddAttribute("event", eventName)
	tx.NoticeError(cause)
	tx.End()
}

// StartProcessing initiates asynchronous document extraction and parsing by dispatching a job.
func (s *DocumentService) StartProcessing(ctx context.Context, documentID, userID string, forceReprocess bool) (*domain.DocumentProcessingJob, error) {
	doc, err := s.authorizeDocumentWrite(ctx, documentID, userID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureBudgetAvailable(ctx, userID); err != nil {
		return nil, err
	}
	if !forceReprocess {
		if latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID); err == nil {
			switch latest.Status {
			case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED,
				appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
				appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED:
				return latest, nil
			}
		}
	}

	jobType := appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT
	if forceReprocess {
		jobType = appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT
	}
	return s.startProcessingJob(ctx, doc, userID, jobType, false)
}

// ResumeProcessing resumes a failed document processing job, incrementing its retry count.
func (s *DocumentService) ResumeProcessing(ctx context.Context, documentID, userID string) (*domain.DocumentProcessingJob, error) {
	doc, err := s.authorizeDocumentWrite(ctx, documentID, userID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureBudgetAvailable(ctx, userID); err != nil {
		return nil, err
	}
	return s.startProcessingJob(ctx, doc, userID, appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT, true)
}

// AutoResumeFailedJobs periodically retries eligible failed processing jobs that have not exceeded max retries.
func (s *DocumentService) AutoResumeFailedJobs(ctx context.Context) (int, error) {
	jobs, err := s.jobs.ListAllJobs(ctx)
	if err != nil {
		return 0, err
	}
	seenDocuments := make(map[string]struct{}, len(jobs))
	resumed := 0
	cutoff := time.Now().UTC().Add(-autoResumeMaxAge)
	for _, job := range jobs {
		if job == nil {
			continue
		}
		if _, seen := seenDocuments[job.DocumentID]; seen {
			continue
		}
		seenDocuments[job.DocumentID] = struct{}{}
		if job.Status != appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED {
			continue
		}
		if job.RetryCount >= autoResumeMaxRetries || job.RequestedBy == "" {
			continue
		}
		if createdAt, err := time.Parse(time.RFC3339, job.CreatedAt); err == nil && createdAt.Before(cutoff) {
			continue
		}
		s.logger.Info("job.auto_resume_starting",
			"job_id", job.JobID,
			"document_id", job.DocumentID,
			"workspace_id", job.WorkspaceID,
			"retry_count", job.RetryCount,
		)
		if _, err := s.ResumeProcessing(ctx, job.DocumentID, job.RequestedBy); err != nil {
			s.logger.Error("job.auto_resume_failed",
				"error", err.Error(),
				"job_id", job.JobID,
				"document_id", job.DocumentID,
				"workspace_id", job.WorkspaceID,
			)
			continue
		}
		resumed++
	}
	return resumed, nil
}

// ReportStuckJobs scans for jobs wedged in QUEUED/RUNNING past stuckJobMinAge
// and emits a JobStuck custom event per stale job. These are the failures that
// JobFailed cannot catch — a worker crash mid-run or a Cloud Tasks task dropped
// after exhausting retries leaves the job in RUNNING with no terminal event, so
// to the user it just never finishes. Emitting per job_id lets the NR alert use
// uniqueCount(), which dedups across API instances and the alert window.
// Returns the number of stuck jobs found. Best-effort: NR-disabled is a no-op.
func (s *DocumentService) ReportStuckJobs(ctx context.Context) (int, error) {
	jobs, err := s.jobs.ListAllJobs(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().Add(-stuckJobMinAge)
	stuck := 0
	for _, job := range jobs {
		if job == nil {
			continue
		}
		switch job.Status {
		case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
			appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING:
		default:
			continue
		}
		// UpdatedAt moves on every state/stage transition, so a job still making
		// progress stays fresh; only a genuinely wedged one crosses the cutoff.
		updatedAt, err := time.Parse(time.RFC3339, job.UpdatedAt)
		if err != nil || !updatedAt.Before(cutoff) {
			continue
		}
		stuck++
		s.logger.Warn("job.stuck_detected",
			"job_id", job.JobID,
			"document_id", job.DocumentID,
			"workspace_id", job.WorkspaceID,
			"status", job.Status.String(),
			"age_seconds", int(time.Since(updatedAt).Seconds()),
		)
		if s.nrApp == nil {
			continue
		}
		s.nrApp.RecordCustomEvent("JobStuck", map[string]any{
			"job_id":       job.JobID,
			"document_id":  job.DocumentID,
			"workspace_id": job.WorkspaceID,
			"job_type":     job.JobType.String(),
			"status":       job.Status.String(),
			"age_seconds":  int(time.Since(updatedAt).Seconds()),
		})
	}
	return stuck, nil
}

// startProcessingJob provisions a processing job transactionally, publishes a queued event, and dispatches the worker execution plan.
func (s *DocumentService) startProcessingJob(ctx context.Context, doc *domain.Document, requestedBy string, jobType appv1.JobType, resumeExisting bool) (*domain.DocumentProcessingJob, error) {
	wsID := doc.WorkspaceID
	documentID := doc.DocumentID
	if err := s.confirmUploadedObject(ctx, doc); err != nil {
		return nil, err
	}
	retryCount := 0
	if resumeExisting {
		if latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID); err == nil {
			switch latest.Status {
			case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
				appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED:
				return latest, nil
			case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_FAILED:
				retryCount = latest.RetryCount + 1
			}
		}
	}
	tree, err := s.tree.GetTree(ctx, wsID)
	if err != nil {
		return nil, err
	}
	// Insert job_capabilities and processing_jobs in a single transaction.
	var job *domain.DocumentProcessingJob
	if err := s.transactor.WithTx(ctx, func(repos repository.Repositories) error {
		var createErr error
		job, createErr = repos.CreateProcessingJob(ctx, documentID, wsID, requestedBy, jobType)
		return createErr
	}); err != nil {
		return nil, fmt.Errorf("create processing job: %w", err)
	}
	if retryCount > 0 {
		if err := s.jobs.SetProcessingJobRetryCount(ctx, job.JobID, retryCount); err != nil {
			return nil, fmt.Errorf("set processing job retry count: %w", err)
		}
		job.RetryCount = retryCount
	}
	payload := documentJobPayload(job, documentID, wsID, tree.TreeID)
	s.lifecycle.NotifyQueued(ctx, payload)
	s.logJobQueued(ctx, job, wsID, documentID)
	if s.dispatcher != nil {
		dispatchReq := s.buildExecutePlanRequest(job, doc, wsID, tree.TreeID)
		if err := s.dispatcher.GenerateExecutionPlan(ctx, dispatchReq); err != nil {
			return s.handleDispatchFailure(ctx, job, payload, wsID, documentID, err, !resumeExisting), nil
		}
		if err := s.dispatcher.ExecuteApprovedPlan(ctx, dispatchReq); err != nil {
			if errors.Is(err, domain.ErrApprovalRequired) || errors.Is(err, domain.ErrPlanRejected) {
				if latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID); err == nil {
					return latest, nil
				}
				return job, nil
			}
			return s.handleDispatchFailure(ctx, job, payload, wsID, documentID, err, true), nil
		}
	}
	if latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID); err == nil {
		job = latest
	}
	return job, nil
}

// confirmUploadedObject verifies document object existence and size against cloud storage metadata.
func (s *DocumentService) confirmUploadedObject(ctx context.Context, doc *domain.Document) error {
	if s.objectMetadata == nil {
		return s.runConfirmAndHandleMismatch(ctx, doc, doc.FileSize)
	}
	metadata, err := s.objectMetadata.GetObjectMetadata(ctx, doc.WorkspaceID, doc.DocumentID)
	if err != nil {
		return err
	}
	if metadata == nil {
		return domain.ErrUploadNotConfirmed
	}
	if err := domain.ValidateDocumentUploadType(doc.Filename, metadata.ContentType); err != nil {
		s.reportUploadIncident("UnsupportedDocumentUpload", doc.WorkspaceID, doc.DocumentID, "unsupported_document_type", err)
		s.deleteOrphanedObject(ctx, doc.WorkspaceID, doc.DocumentID, "unsupported_document_type")
		return err
	}
	return s.runConfirmAndHandleMismatch(ctx, doc, metadata.Size)
}

// runConfirmAndHandleMismatch checks for upload size mismatches and purges invalid artifacts on failure.
func (s *DocumentService) runConfirmAndHandleMismatch(ctx context.Context, doc *domain.Document, actualSize int64) error {
	err := s.repo.ConfirmDocumentUpload(ctx, doc.DocumentID, actualSize)
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrUploadSizeMismatch) {
		s.logger.Warn("document.upload_size_mismatch",
			"document_id", doc.DocumentID,
			"workspace_id", doc.WorkspaceID,
			"expected", doc.FileSize,
			"actual", actualSize,
		)
		s.reportUploadIncident("UploadSizeMismatch", doc.WorkspaceID, doc.DocumentID, "size_mismatch", err)
		s.deleteOrphanedObject(ctx, doc.WorkspaceID, doc.DocumentID, "size_mismatch")
	}
	return err
}

// buildExecutePlanRequest populates the worker execution payload from document and job models.
func (s *DocumentService) buildExecutePlanRequest(job *domain.DocumentProcessingJob, doc *domain.Document, wsID, treeID string) domain.ExecutePlanRequest {
	return domain.ExecutePlanRequest{
		JobID:       job.JobID,
		JobType:     job.JobType.String(),
		DocumentID:  doc.DocumentID,
		WorkspaceID: wsID,
		TreeID:      treeID,
		FileURI:     s.sourceURLBuilder(wsID, doc.DocumentID),
		Filename:    doc.Filename,
		MimeType:    doc.MimeType,
	}
}

// logJobQueued records an informational job log event indicating the job has entered the queue.
func (s *DocumentService) logJobQueued(ctx context.Context, job *domain.DocumentProcessingJob, wsID, documentID string) {
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       job.JobID,
		WorkspaceID: wsID,
		DocumentID:  documentID,
		Level:       joblog.INFO,
		Event:       "job.queued",
		Message:     fmt.Sprintf("job queued: doc=%s type=%s", documentID, job.JobType),
		Detail:      map[string]any{"type": job.JobType.String()},
	})
}

// handleDispatchFailure handles dispatcher communication errors by logging, failing the job, and updating status.
func (s *DocumentService) handleDispatchFailure(ctx context.Context, job *domain.DocumentProcessingJob, payload jobstatus.Payload, wsID, documentID string, dispatchErr error, reloadLatest bool) *domain.DocumentProcessingJob {
	s.logger.Error("job.dispatch_failed",
		"error", dispatchErr.Error(),
		"job_id", job.JobID,
		"job_type", job.JobType.String(),
		"workspace_id", wsID,
		"document_id", documentID,
	)
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       job.JobID,
		WorkspaceID: wsID,
		DocumentID:  documentID,
		Level:       joblog.ERROR,
		Event:       "job.dispatch_failed",
		Message:     fmt.Sprintf("job dispatch failed: %v", dispatchErr),
		Detail:      map[string]any{"error": dispatchErr.Error()},
	})
	s.lifecycle.TryFail(ctx, payload, dispatchErr)
	if reloadLatest {
		if latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID); err == nil {
			return latest
		}
	}
	return job
}

// documentJobPayload constructs standard job status notification payload parameters.
func documentJobPayload(job *domain.DocumentProcessingJob, documentID, wsID, treeID string) jobstatus.Payload {
	return jobstatus.Payload{
		JobID:       job.JobID,
		JobType:     job.JobType.String(),
		DocumentID:  documentID,
		WorkspaceID: wsID,
		TreeID:      treeID,
	}
}

// GetLatestProcessingJob fetches the most recent processing job metadata for a document.
func (s *DocumentService) GetLatestProcessingJob(ctx context.Context, documentID string) (*domain.DocumentProcessingJob, error) {
	job, err := s.jobs.GetLatestProcessingJob(ctx, documentID)
	if err != nil {
		return nil, err
	}
	return job, nil
}
