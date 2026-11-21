package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
	sharedstorage "github.com/synthify/backend/internal/platform/storage"
)

// DeleteDocument deletes a document for good: first its stored files (the
// uploaded original, the extracted copy, the worker's cache and every job's
// checkpoints), then its rows and the tree nodes made only from it (see
// repository.DocumentDeletionRepository). If the files cannot be deleted the
// rows stay, so the document stays listed and the user can try again.
//
// Owners may delete any document of the workspace, editors only the ones they
// uploaded. A document whose upload or processing job is still in flight is
// refused with ErrDocumentBusy: either would keep writing for it.
func (s *DocumentService) DeleteDocument(ctx context.Context, documentID, userID string) error {
	if s.deletion == nil || s.objectPrefixes == nil {
		return domain.ErrNotImplemented
	}
	doc, err := s.authorizeDocumentDelete(ctx, documentID, userID)
	if err != nil {
		return err
	}
	state, err := s.deletion.GetDocumentDeletionState(ctx, documentID)
	if err != nil {
		return err
	}
	if state.UploadPending {
		return domain.ErrDocumentBusy
	}
	latest, err := s.jobs.GetLatestProcessingJob(ctx, documentID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
	case err != nil:
		return err
	case processingInFlight(latest, time.Now().UTC()):
		return domain.ErrDocumentBusy
	}

	for _, prefix := range documentStoragePrefixes(doc.WorkspaceID, documentID, state.JobIDs) {
		if _, err := s.objectPrefixes.DeleteObjectsWithPrefix(ctx, prefix); err != nil {
			return fmt.Errorf("delete stored files of document %s: %w", documentID, err)
		}
	}
	deletedItems, err := s.deletion.DeleteDocumentRows(ctx, documentID)
	if err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("document.deleted",
			"document_id", documentID,
			"workspace_id", doc.WorkspaceID,
			"user_id", userID,
			"deleted_items", deletedItems,
			"jobs", len(state.JobIDs),
		)
	}
	return nil
}

// authorizeDocumentDelete lets owners delete any document of the workspace and
// editors only their own uploads. A missing document is reported as Forbidden,
// like the other document RPCs, so its existence does not leak.
func (s *DocumentService) authorizeDocumentDelete(ctx context.Context, documentID, userID string) (*domain.Document, error) {
	doc, err := s.repo.GetDocument(ctx, documentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrForbidden
		}
		return nil, err
	}
	role, err := s.workspaces.GetWorkspaceRole(ctx, doc.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}
	switch {
	case role == domain.WorkspaceRoleOwner:
	case role == domain.WorkspaceRoleEditor && userID != "" && doc.UploadedBy == userID:
	default:
		return nil, domain.ErrForbidden
	}
	return doc, nil
}

// processingInFlight reports whether a job is queued, running or requeued
// after the worker ran out of budget (RETRYABLE), and has moved within
// stuckJobMinAge. A job silent for longer is stuck (ReportStuckJobs alerts on
// it) and no longer blocks deletion; otherwise a crashed worker would make its
// document undeletable.
func processingInFlight(job *domain.DocumentProcessingJob, now time.Time) bool {
	switch job.Status {
	case appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_QUEUED,
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING,
		appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RETRYABLE:
	default:
		return false
	}
	updatedAt, err := time.Parse(time.RFC3339, job.UpdatedAt)
	if err != nil {
		return true // unknown age: it may still be running
	}
	return now.Sub(updatedAt) < stuckJobMinAge
}

// documentStoragePrefixes lists where a document's data is stored.
func documentStoragePrefixes(workspaceID, documentID string, jobIDs []string) []string {
	prefixes := []string{
		sharedstorage.DocumentObjectPrefix(workspaceID, documentID),
		sharedstorage.DocumentCachePrefix(documentID),
	}
	for _, jobID := range jobIDs {
		prefixes = append(prefixes, sharedstorage.JobCheckpointPrefix(jobID))
	}
	return prefixes
}
