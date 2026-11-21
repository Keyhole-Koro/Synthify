package handler

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// Requesting, approving and rejecting a job's approval decide whether the job
// runs. A viewer only reads the workspace, so these need the owner or an
// editor; reading the job stays open to every member.
func TestJobHandler_ApprovalsNeedWriteAccess(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithProcessingJobFixture(t, ctx, store, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	wsID, jobID := fixture.Workspace.WorkspaceID, fixture.Job.JobID
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, "viewer-user", "viewer", "owner"))
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, "editor-user", "editor", "owner"))
	h := NewJobHandler(store, store, store, store, store, nil)

	as := func(userID string) context.Context {
		return auth.ContextWithPrincipal(ctx, auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: userID, Email: userID + "@example.com"})
	}
	calls := map[string]func(context.Context) error{
		"request": func(c context.Context) error {
			_, err := h.RequestJobApproval(c, connect.NewRequest(&appv1.RequestJobApprovalRequest{JobId: jobID, Reason: "check"}))
			return err
		},
		"approve": func(c context.Context) error {
			_, err := h.ApproveJobApproval(c, connect.NewRequest(&appv1.ApproveJobApprovalRequest{JobId: jobID, ApprovalId: "approval-" + jobID}))
			return err
		},
		"reject": func(c context.Context) error {
			_, err := h.RejectJobApproval(c, connect.NewRequest(&appv1.RejectJobApprovalRequest{JobId: jobID, ApprovalId: "approval-" + jobID, Reason: "no"}))
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			assertConnectCode(t, call(as("viewer-user")), connect.CodePermissionDenied)
			assertConnectCode(t, call(as("stranger")), connect.CodePermissionDenied)
			assert.NoError(t, call(as("editor-user")), "an editor")
			assert.NoError(t, call(as("owner")), "the owner")
		})
	}

	_, err := h.GetJobStatus(as("viewer-user"), connect.NewRequest(&appv1.GetJobStatusRequest{JobId: jobID}))
	assert.NoError(t, err, "a viewer still reads the job")
}
