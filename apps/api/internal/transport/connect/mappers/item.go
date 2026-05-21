package mappers

import (
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func ToProtoSubtreeItem(item *domain.SubtreeItem) *appv1.SubtreeItem {
	if item == nil {
		return nil
	}
	return &appv1.SubtreeItem{
		Item:        ToProtoItem(&item.Item),
		HasChildren: item.HasChildren,
	}
}

func ToProtoItem(item *domain.Item) *appv1.Item {
	if item == nil {
		return nil
	}
	return &appv1.Item{
		Id:              item.ItemID,
		Title:           item.Title,
		Level:           int32(item.Level),
		Description:     item.Description,
		Content:         item.Content,
		CreatedAt:       item.CreatedAt,
		ParentId:        item.ParentID,
		ChildIds:        item.ChildIDs,
		Scope:           item.Scope,
		GovernanceState: item.GovernanceState,
	}
}
