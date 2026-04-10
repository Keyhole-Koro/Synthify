package handler

import (
	graphv1 "github.com/synthify/backend/gen/synthify/graph/v1"
	"github.com/synthify/backend/internal/domain"
)

func workspacePlanToProto(plan string) graphv1.WorkspacePlan {
	switch plan {
	case "free":
		return graphv1.WorkspacePlan_WORKSPACE_PLAN_FREE
	case "pro":
		return graphv1.WorkspacePlan_WORKSPACE_PLAN_PRO
	default:
		return graphv1.WorkspacePlan_WORKSPACE_PLAN_UNSPECIFIED
	}
}

func workspaceRoleToProto(role domain.WorkspaceRole) graphv1.WorkspaceRole {
	switch role {
	case domain.WorkspaceRoleOwner:
		return graphv1.WorkspaceRole_WORKSPACE_ROLE_OWNER
	case domain.WorkspaceRoleEditor:
		return graphv1.WorkspaceRole_WORKSPACE_ROLE_EDITOR
	case domain.WorkspaceRoleViewer:
		return graphv1.WorkspaceRole_WORKSPACE_ROLE_VIEWER
	default:
		return graphv1.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED
	}
}

func workspaceRoleToDomain(role graphv1.WorkspaceRole) string {
	switch role {
	case graphv1.WorkspaceRole_WORKSPACE_ROLE_OWNER:
		return "owner"
	case graphv1.WorkspaceRole_WORKSPACE_ROLE_EDITOR:
		return "editor"
	case graphv1.WorkspaceRole_WORKSPACE_ROLE_VIEWER:
		return "viewer"
	default:
		return "viewer"
	}
}

func documentStatusToProto(status domain.DocumentLifecycleState) graphv1.DocumentLifecycleState {
	switch status {
	case domain.DocumentLifecycleUploaded:
		return graphv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UPLOADED
	case domain.DocumentLifecyclePendingNormalization:
		return graphv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_PENDING_NORMALIZATION
	case domain.DocumentLifecycleProcessing:
		return graphv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_PROCESSING
	case domain.DocumentLifecycleCompleted:
		return graphv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_COMPLETED
	case domain.DocumentLifecycleFailed:
		return graphv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_FAILED
	default:
		return graphv1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UNSPECIFIED
	}
}

func extractionDepthToProto(depth string) graphv1.ExtractionDepth {
	switch depth {
	case "full":
		return graphv1.ExtractionDepth_EXTRACTION_DEPTH_FULL
	case "summary":
		return graphv1.ExtractionDepth_EXTRACTION_DEPTH_SUMMARY
	default:
		return graphv1.ExtractionDepth_EXTRACTION_DEPTH_UNSPECIFIED
	}
}

func extractionDepthToDomain(depth graphv1.ExtractionDepth) string {
	switch depth {
	case graphv1.ExtractionDepth_EXTRACTION_DEPTH_SUMMARY:
		return "summary"
	case graphv1.ExtractionDepth_EXTRACTION_DEPTH_FULL:
		return "full"
	default:
		return ""
	}
}

func nodeCategoryToProto(category string) graphv1.NodeCategory {
	switch category {
	case "concept":
		return graphv1.NodeCategory_NODE_CATEGORY_CONCEPT
	case "entity":
		return graphv1.NodeCategory_NODE_CATEGORY_ENTITY
	case "claim":
		return graphv1.NodeCategory_NODE_CATEGORY_CLAIM
	case "evidence":
		return graphv1.NodeCategory_NODE_CATEGORY_EVIDENCE
	case "counter":
		return graphv1.NodeCategory_NODE_CATEGORY_COUNTER
	default:
		return graphv1.NodeCategory_NODE_CATEGORY_UNSPECIFIED
	}
}

func nodeCategoryToDomain(category graphv1.NodeCategory) string {
	switch category {
	case graphv1.NodeCategory_NODE_CATEGORY_CONCEPT:
		return "concept"
	case graphv1.NodeCategory_NODE_CATEGORY_ENTITY:
		return "entity"
	case graphv1.NodeCategory_NODE_CATEGORY_CLAIM:
		return "claim"
	case graphv1.NodeCategory_NODE_CATEGORY_EVIDENCE:
		return "evidence"
	case graphv1.NodeCategory_NODE_CATEGORY_COUNTER:
		return "counter"
	default:
		return ""
	}
}

func nodeEntityTypeToProto(entityType string) graphv1.NodeEntityType {
	switch entityType {
	case "organization":
		return graphv1.NodeEntityType_NODE_ENTITY_TYPE_ORGANIZATION
	case "person":
		return graphv1.NodeEntityType_NODE_ENTITY_TYPE_PERSON
	case "metric":
		return graphv1.NodeEntityType_NODE_ENTITY_TYPE_METRIC
	case "date":
		return graphv1.NodeEntityType_NODE_ENTITY_TYPE_DATE
	case "location":
		return graphv1.NodeEntityType_NODE_ENTITY_TYPE_LOCATION
	default:
		return graphv1.NodeEntityType_NODE_ENTITY_TYPE_UNSPECIFIED
	}
}
