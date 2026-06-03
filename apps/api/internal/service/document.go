package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	joblifecycle "github.com/synthify/backend/internal/platform/job/lifecycle"
	joblog "github.com/synthify/backend/internal/platform/job/log"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

type WorkerDispatcher interface {
	GenerateExecutionPlan(ctx context.Context, req domain.ExecutePlanRequest) error
	ExecuteApprovedPlan(ctx context.Context, req domain.ExecutePlanRequest) error
}

type ObjectMetadataFetcher interface {
	GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error)
}

// DocumentUsecase は handler が依存する DocumentService の API 表面。
// 各メソッドは userID を受け取り、内部で workspace authz を実施する。
// 未認可は domain.ErrForbidden を返す。
type DocumentUsecase interface {
	ListDocuments(ctx context.Context, workspaceID, userID string) ([]*domain.Document, error)
	GetDocument(ctx context.Context, documentID, userID string) (*domain.Document, error)
	CreateDocument(ctx context.Context, wsID, userID, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error)
	ConfirmUpload(ctx context.Context, documentID, userID string) (*domain.Document, error)
	StartProcessing(ctx context.Context, documentID, userID string, forceReprocess bool) (*domain.DocumentProcessingJob, error)
	ResumeProcessing(ctx context.Context, documentID, userID string) (*domain.DocumentProcessingJob, error)
}

const (
	autoResumeMaxRetries = 1
	autoResumeMaxAge     = 24 * time.Hour
)

type DocumentService struct {
	repo             repository.DocumentRepository
	jobs             repository.JobRepository
	workspaces       repository.WorkspaceRepository
	tree             repository.TreeRepository
	transactor       repository.Transactor
	sourceURLBuilder repository.DocumentSourceURLBuilder
	objectMetadata   ObjectMetadataFetcher
	objectStore      repository.DocumentObjectStore
	dispatcher       WorkerDispatcher
	lifecycle        joblifecycle.QueuedNotifier
	notifier         jobstatus.Notifier
	logger           *slog.Logger
	nrApp            *newrelic.Application
}

type DocumentServiceDeps struct {
	Repo             repository.DocumentRepository
	Jobs             repository.JobRepository
	LifecycleRepo    joblifecycle.Repository
	Workspaces       repository.WorkspaceRepository
	Tree             repository.TreeRepository
	Transactor       repository.Transactor
	SourceURLBuilder repository.DocumentSourceURLBuilder
	ObjectMetadata   ObjectMetadataFetcher
	ObjectStore      repository.DocumentObjectStore
	Dispatcher       WorkerDispatcher
	Notifier         jobstatus.Notifier
	Logger           *slog.Logger
	NRApp            *newrelic.Application
}

func NewDocumentService(deps DocumentServiceDeps) *DocumentService {
	return &DocumentService{
		repo:             deps.Repo,
		jobs:             deps.Jobs,
		workspaces:       deps.Workspaces,
		tree:             deps.Tree,
		transactor:       deps.Transactor,
		sourceURLBuilder: deps.SourceURLBuilder,
		objectMetadata:   deps.ObjectMetadata,
		objectStore:      deps.ObjectStore,
		dispatcher:       deps.Dispatcher,
		lifecycle:        joblifecycle.New(deps.LifecycleRepo, deps.Notifier, deps.Logger, deps.NRApp),
		notifier:         deps.Notifier,
		logger:           deps.Logger,
		nrApp:            deps.NRApp,
	}
}

// authorizeWorkspace は userID が workspace にアクセスできるかチェックする。
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

// authorizeDocument は document を取得しつつ workspace authz をする。
// document 不在も認可前に Forbidden を返す (存在を漏らさないため)。
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

func (s *DocumentService) ListDocuments(ctx context.Context, workspaceID, userID string) ([]*domain.Document, error) {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListDocuments(ctx, workspaceID)
}

func (s *DocumentService) GetDocument(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	return s.authorizeDocument(ctx, documentID, userID)
}

func (s *DocumentService) CreateDocument(ctx context.Context, wsID, userID, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error) {
	if err := s.authorizeWorkspace(ctx, wsID, userID); err != nil {
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

func (s *DocumentService) ConfirmUpload(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	doc, err := s.authorizeDocument(ctx, documentID, userID)
	if err != nil {
		return nil, err
	}
	if err := s.confirmUploadedObject(ctx, doc); err != nil {
		return nil, err
	}
	return s.repo.GetDocument(ctx, documentID)
}

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

// deleteOrphanedObject はサイズ不一致や予約期限切れで「もう正規ルートでは
// 使われなくなった」ドキュメントオブジェクトを GCS から消す。ベストエフォート:
// 失敗してもクライアントには影響させず、ログと NR にだけ残して呼び出し元の
// メイン経路を止めない。
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

// reportUploadIncident はクオータ違反やサイズ不一致など、アップロード周辺で
// 発生した「攻撃 / 誤用 / バグの兆候」を NR に流す。slog だけだと長期トレンドや
// アカウント別分析が組めないので、NRQL 集計用に CustomEvent + NoticeError の
// 両方に乗せる。NR 未設定のときは no-op。
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

func (s *DocumentService) StartProcessing(ctx context.Context, documentID, userID string, forceReprocess bool) (*domain.DocumentProcessingJob, error) {
	doc, err := s.authorizeDocument(ctx, documentID, userID)
	if err != nil {
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

func (s *DocumentService) ResumeProcessing(ctx context.Context, documentID, userID string) (*domain.DocumentProcessingJob, error) {
	doc, err := s.authorizeDocument(ctx, documentID, userID)
	if err != nil {
		return nil, err
	}
	return s.startProcessingJob(ctx, doc, userID, appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT, true)
}

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
	// job_capabilities + processing_jobs を 1 tx で挿入。
	// 旧実装は repo 内で tx を張りつつ全エラーを nil 化していたため、
	// service には ErrNotFound として漏れ、原因不明の not_found が
	// クライアントに返っていた。tx 境界を service に上げ、
	// 本来のエラーを伝播する。
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

func (s *DocumentService) runConfirmAndHandleMismatch(ctx context.Context, doc *domain.Document, actualSize int64) error {
	err := s.repo.ConfirmDocumentUpload(ctx, doc.DocumentID, actualSize)
	if err == nil {
		return nil
	}
	// Content-Length 強制をすり抜けた、あるいは fake-gcs 上で意図的に申告サイズと
	// 違うアップロードが入った場合、GCS には実体が残っているので消す。サイズ違反は
	// 攻撃か計装バグの兆候なので NR にも投げて可視化する。
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

func documentJobPayload(job *domain.DocumentProcessingJob, documentID, wsID, treeID string) jobstatus.Payload {
	return jobstatus.Payload{
		JobID:       job.JobID,
		JobType:     job.JobType.String(),
		DocumentID:  documentID,
		WorkspaceID: wsID,
		TreeID:      treeID,
	}
}

func (s *DocumentService) GetLatestProcessingJob(ctx context.Context, documentID string) (*domain.DocumentProcessingJob, error) {
	job, err := s.jobs.GetLatestProcessingJob(ctx, documentID)
	if err != nil {
		return nil, err
	}
	return job, nil
}
