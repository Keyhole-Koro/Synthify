package handler

import (
	graphv1 "github.com/synthify/backend/gen/synthify/graph/v1"
)

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
