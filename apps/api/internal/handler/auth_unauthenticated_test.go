package handler

// Sweep tests: every non-public Connect RPC rejects an unauthenticated request
// with CodeUnauthenticated. This is the "everyone forgets a guard" regression net.

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/synthify/backend/apps/api/internal/application"
	"github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// ── Workspace ────────────────────────────────────────────────────────────────

func TestWorkspaceHandler_CreateWorkspace_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestWorkspaceHandler(store)
	resp, err := h.CreateWorkspace(context.Background(), connect.NewRequest(&appv1.CreateWorkspaceRequest{Name: "x"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

// ── Document ─────────────────────────────────────────────────────────────────

func TestDocumentHandler_GetUploadURL_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestDocumentHandler(store)
	resp, err := h.GetUploadURL(context.Background(), connect.NewRequest(&appv1.GetUploadURLRequest{
		WorkspaceId: "ws", Filename: "f.pdf", MimeType: "application/pdf", FileSize: 100,
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestDocumentHandler_StartProcessing_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestDocumentHandler(store)
	resp, err := h.StartProcessing(context.Background(), connect.NewRequest(&appv1.StartProcessingRequest{DocumentId: "doc"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestDocumentHandler_ResumeProcessing_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestDocumentHandler(store)
	resp, err := h.ResumeProcessing(context.Background(), connect.NewRequest(&appv1.ResumeProcessingRequest{DocumentId: "doc"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

// ── Tree ─────────────────────────────────────────────────────────────────────

func TestTreeHandler_GetSubtree_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewTreeHandler(application.NewTreeService(application.TreeServiceDeps{
		Tree:       store,
		Workspaces: store,
		Logger:     nil,
	}))
	resp, err := h.GetSubtree(context.Background(), connect.NewRequest(&appv1.GetSubtreeRequest{
		WorkspaceId: "ws", ItemId: "i",
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestTreeHandler_FindPaths_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewTreeHandler(application.NewTreeService(application.TreeServiceDeps{
		Tree:       store,
		Workspaces: store,
		Logger:     nil,
	}))
	resp, err := h.FindPaths(context.Background(), connect.NewRequest(&appv1.FindPathsRequest{
		WorkspaceId: "ws", SourceItemId: "a", TargetItemId: "b",
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

// ── Item ─────────────────────────────────────────────────────────────────────

func TestItemHandler_GetTreeEntityDetail_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestItemHandler(store)
	resp, err := h.GetTreeEntityDetail(context.Background(), connect.NewRequest(&appv1.GetTreeEntityDetailRequest{
		TargetRef: &appv1.EntityRef{Id: "i", WorkspaceId: "ws"},
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestItemHandler_ApproveAlias_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestItemHandler(store)
	resp, err := h.ApproveAlias(context.Background(), connect.NewRequest(&appv1.ApproveAliasRequest{
		WorkspaceId: "ws", CanonicalItemId: "a", AliasItemId: "b",
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestItemHandler_RejectAlias_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := newTestItemHandler(store)
	resp, err := h.RejectAlias(context.Background(), connect.NewRequest(&appv1.RejectAliasRequest{
		WorkspaceId: "ws", CanonicalItemId: "a", AliasItemId: "b",
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

// ── Job ──────────────────────────────────────────────────────────────────────

func TestJobHandler_GetJobExecutionPlan_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.GetJobExecutionPlan(context.Background(), connect.NewRequest(&appv1.GetJobExecutionPlanRequest{JobId: "j"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_ListJobApprovalRequests_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.ListJobApprovalRequests(context.Background(), connect.NewRequest(&appv1.ListJobApprovalRequestsRequest{JobId: "j"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_ApproveJobApproval_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.ApproveJobApproval(context.Background(), connect.NewRequest(&appv1.ApproveJobApprovalRequest{ApprovalId: "a"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_RejectJobApproval_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.RejectJobApproval(context.Background(), connect.NewRequest(&appv1.RejectJobApprovalRequest{ApprovalId: "a"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_ListJobMutationLogs_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.ListJobMutationLogs(context.Background(), connect.NewRequest(&appv1.ListJobMutationLogsRequest{JobId: "j"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_ListJobLogs_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.ListJobLogs(context.Background(), connect.NewRequest(&appv1.ListJobLogsRequest{JobId: "j"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_SearchJobLogs_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.SearchJobLogs(context.Background(), connect.NewRequest(&appv1.SearchJobLogsRequest{WorkspaceId: "ws"}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

// ListAllJobs: 認証 + admin 必須。anonymous read 用パスを除く。
func TestJobHandler_ListAllJobs_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.ListAllJobs(context.Background(), connect.NewRequest(&appv1.ListAllJobsRequest{}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}

func TestJobHandler_ListAllJobs_NonAdminUser_ReturnsPermissionDenied(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "u1", Email: "u@example.com"})
	resp, err := h.ListAllJobs(ctx, connect.NewRequest(&appv1.ListAllJobsRequest{}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestJobHandler_ListAllJobs_AdminUser_Allowed(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{Kind: auth.PrincipalKindUser, SubjectID: "admin", Email: "a@example.com", Admin: true})
	resp, err := h.ListAllJobs(ctx, connect.NewRequest(&appv1.ListAllJobsRequest{}))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestJobHandler_ListRelatedJobLogs_Unauthenticated(t *testing.T) {
	store := mock.NewStore()
	h := NewJobHandler(store, store, store, store, store, nil)
	resp, err := h.ListRelatedJobLogs(context.Background(), connect.NewRequest(&appv1.ListRelatedJobLogsRequest{
		Scope: appv1.RelatedLogScope_RELATED_LOG_SCOPE_JOB,
		JobId: "j",
	}))
	assert.Nil(t, resp)
	assertConnectCode(t, err, connect.CodeUnauthenticated)
}
