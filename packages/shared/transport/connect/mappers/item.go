package mappers

import (
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

func ToProtoSubtreeItem(item *domain.SubtreeItem) *treev1.SubtreeItem {
	if item == nil {
		return nil
	}
	return &treev1.SubtreeItem{
		Item:        ToProtoItem(&item.Item),
		HasChildren: item.HasChildren,
	}
}

func ToProtoItem(item *domain.Item) *treev1.Item {
	if item == nil {
		return nil
	}
	return &treev1.Item{
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
