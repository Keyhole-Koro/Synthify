package mappers

import (
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func ToProtoWorkspace(ws *domain.Workspace) *appv1.Workspace {
	if ws == nil {
		return nil
	}
	return &appv1.Workspace{
		WorkspaceId:       ws.WorkspaceID,
		Name:              ws.Name,
		OwnerId:           ws.AccountID,
		Plan:              toProtoWorkspacePlan(ws.Plan),
		StorageUsedBytes:  ws.StorageUsedBytes,
		StorageQuotaBytes: ws.StorageQuotaBytes,
		MaxFileSizeBytes:  ws.MaxFileSizeBytes,
		MaxUploadsPerDay:  ws.MaxUploadsPerWeek,
		CreatedAt:         ws.CreatedAt,
	}
}

func toProtoWorkspacePlan(plan string) appv1.WorkspacePlan {
	switch plan {
	case "pro":
		return appv1.WorkspacePlan_WORKSPACE_PLAN_PRO
	case "free", "":
		return appv1.WorkspacePlan_WORKSPACE_PLAN_FREE
	default:
		return appv1.WorkspacePlan_WORKSPACE_PLAN_UNSPECIFIED
	}
}
