package application

import "context"

// JobStatusReaders mirrors who may access a workspace into Firestore, where
// the web app reads the workspace's job status documents directly
// (workspaces/{workspaceId}/jobs/{jobId}). firestore.rules lets a user read
// them only while a reader document exists for that user; see
// jobstatus.ReaderAccess, which implements this interface.
type JobStatusReaders interface {
	Grant(ctx context.Context, userID string, workspaceIDs ...string) error
	Revoke(ctx context.Context, userID, workspaceID string) error
	RevokeAll(ctx context.Context, workspaceID string) error
}

// The workspace service keeps the reader documents in step with workspace
// access:
//
//   - a user is granted a workspace when they create it, are invited to it,
//     list their workspaces or open one. Listing runs at every sign-in, so
//     access that predates the reader documents (or was missed) is filled in
//     before the web app subscribes to the job status documents;
//   - a removed member loses the workspace unless they still reach it through
//     its account, and a deleted workspace loses every reader.
//
// Mirroring is best effort: a failure is logged and the call it belongs to
// still succeeds. A missed grant only hides job progress until the next one;
// a missed revoke leaves a removed member able to read the job status
// documents, so it is logged as an error.

func (s *WorkspaceService) grantJobStatusReads(ctx context.Context, userID string, workspaceIDs ...string) {
	if s.jobReaders == nil || userID == "" || len(workspaceIDs) == 0 {
		return
	}
	if err := s.jobReaders.Grant(ctx, userID, workspaceIDs...); err != nil && s.logger != nil {
		s.logger.Warn("workspace.job_status_grant_failed", "error", err.Error(), "user_id", userID, "workspaces", len(workspaceIDs))
	}
}

func (s *WorkspaceService) revokeJobStatusReads(ctx context.Context, userID, workspaceID string) {
	if s.jobReaders == nil {
		return
	}
	if ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID); err == nil && ok {
		return
	}
	if err := s.jobReaders.Revoke(ctx, userID, workspaceID); err != nil && s.logger != nil {
		s.logger.Error("workspace.job_status_revoke_failed", "error", err.Error(), "user_id", userID, "workspace_id", workspaceID)
	}
}

func (s *WorkspaceService) revokeAllJobStatusReads(ctx context.Context, workspaceID string) {
	if s.jobReaders == nil {
		return
	}
	if err := s.jobReaders.RevokeAll(ctx, workspaceID); err != nil && s.logger != nil {
		s.logger.Error("workspace.job_status_revoke_all_failed", "error", err.Error(), "workspace_id", workspaceID)
	}
}
