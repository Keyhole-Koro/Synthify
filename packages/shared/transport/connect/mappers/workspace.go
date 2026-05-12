package mappers

import (
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

func ToProtoWorkspace(ws *domain.Workspace) *treev1.Workspace {
	if ws == nil {
		return nil
	}
	return &treev1.Workspace{
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

func toProtoWorkspacePlan(plan string) treev1.WorkspacePlan {
	switch plan {
	case "pro":
		return treev1.WorkspacePlan_WORKSPACE_PLAN_PRO
	case "free", "":
		return treev1.WorkspacePlan_WORKSPACE_PLAN_FREE
	default:
		return treev1.WorkspacePlan_WORKSPACE_PLAN_UNSPECIFIED
	}
}
