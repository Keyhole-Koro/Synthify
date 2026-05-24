package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/job/lifecycle"
	"github.com/synthify/backend/apps/api/internal/repository"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	"github.com/synthify/backend/internal/platform/applog"
	"github.com/synthify/backend/internal/platform/job/log"
	"github.com/synthify/backend/internal/platform/job/status"
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

type DocumentService struct {
	repo             repository.DocumentRepository
	jobs             repository.JobRepository
	workspaces       repository.WorkspaceRepository
	tree             repository.TreeRepository
	sourceURLBuilder repository.DocumentSourceURLBuilder
	objectMetadata   ObjectMetadataFetcher
	dispatcher       WorkerDispatcher
	lifecycle        *joblifecycle.Service
	notifier         jobstatus.Notifier
	logger           applog.Logger
}

func NewDocumentService(
	repo repository.DocumentRepository,
	jobs repository.JobRepository,
	lifecycleRepo joblifecycle.Repository,
	workspaces repository.WorkspaceRepository,
	tree repository.TreeRepository,
	sourceURLBuilder repository.DocumentSourceURLBuilder,
	objectMetadata ObjectMetadataFetcher,
	dispatcher WorkerDispatcher,
	notifier jobstatus.Notifier,
	logger applog.Logger,
) *DocumentService {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return &DocumentService{
		repo:             repo,
		jobs:             jobs,
		workspaces:       workspaces,
		tree:             tree,
		sourceURLBuilder: sourceURLBuilder,
		objectMetadata:   objectMetadata,
		dispatcher:       dispatcher,
		lifecycle:        joblifecycle.New(lifecycleRepo, notifier, logger),
		notifier:         notifier,
		logger:           logger,
	}
}

// authorizeWorkspace は userID が workspace にアクセスできるかチェックする。
func (s *DocumentService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	if !s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID) {
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
	return s.repo.ListDocuments(ctx, workspaceID), nil
}

func (s *DocumentService) GetDocument(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	return s.authorizeDocument(ctx, documentID, userID)
}

func (s *DocumentService) CreateDocument(ctx context.Context, wsID, userID, filename, mimeType string, fileSize int64) (*domain.Document, repository.DocumentUploadTarget, error) {
	if err := s.authorizeWorkspace(ctx, wsID, userID); err != nil {
		return nil, repository.DocumentUploadTarget{}, err
	}
	return s.repo.CreateDocument(ctx, wsID, userID, filename, mimeType, fileSize)
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
		s.logger.Error(ctx, "document.upload_reservations.expire_failed", err, map[string]any{})
		return 0, err
	}
	if expired > 0 {
		s.logger.Info(ctx, "document.upload_reservations.expired", map[string]any{"count": expired})
	}
	return expired, nil
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

func (s *DocumentService) startProcessingJob(ctx context.Context, doc *domain.Document, requestedBy string, jobType appv1.JobType, resumeExisting bool) (*domain.DocumentProcessingJob, error) {
	wsID := doc.WorkspaceID
	documentID := doc.DocumentID
	if err := s.confirmUploadedObject(ctx, doc); err != nil {
		return nil, err
	}
	if resumeExisting {
		if latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID); err == nil {
			switch latest.Status {
			case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
				appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED:
				return latest, nil
			}
		}
	}
	tree, err := s.tree.GetOrCreateTree(ctx, wsID)
	if err != nil {
		return nil, err
	}
	job := s.jobs.CreateProcessingJob(ctx, documentID, wsID, requestedBy, jobType)
	if job == nil {
		return nil, domain.ErrNotFound
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
		return s.repo.ConfirmDocumentUpload(ctx, doc.DocumentID, doc.FileSize)
	}
	metadata, err := s.objectMetadata.GetObjectMetadata(ctx, doc.WorkspaceID, doc.DocumentID)
	if err != nil {
		return err
	}
	if metadata == nil {
		return domain.ErrUploadNotConfirmed
	}
	return s.repo.ConfirmDocumentUpload(ctx, doc.DocumentID, metadata.Size)
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
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:       job.JobID,
		WorkspaceID: wsID,
		DocumentID:  documentID,
		Level:       joblog.ERROR,
		Event:       "job.dispatch_failed",
		Message:     fmt.Sprintf("job dispatch failed: %v", dispatchErr),
		Detail:      map[string]any{"error": dispatchErr.Error()},
	})
	s.lifecycle.TryFail(ctx, payload, dispatchErr.Error())
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
