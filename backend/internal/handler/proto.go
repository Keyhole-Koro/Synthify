package handler

import (
	graphv1 "github.com/synthify/backend/gen/synthify/graph/v1"
)

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
