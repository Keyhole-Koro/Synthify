package handler

import (
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func toProtoSubtreeItem(item *domain.SubtreeItem) *appv1.SubtreeItem {
	if item == nil {
		return nil
	}
	return &appv1.SubtreeItem{
		Item:        toProtoItem(&item.Item),
		HasChildren: item.HasChildren,
	}
}

func toProtoItem(item *domain.Item) *appv1.Item {
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
		CrossDocument:   item.CrossDocument,
	}
}

func toProtoItemEvidence(evidence *domain.ItemEvidence) *appv1.TreeEntityEvidence {
	if evidence == nil {
		return &appv1.TreeEntityEvidence{}
	}
	documentIDs := make([]string, 0, len(evidence.Sources))
	seenDocumentIDs := make(map[string]struct{}, len(evidence.Sources))
	for _, source := range evidence.Sources {
		if source == nil || source.DocumentID == "" {
			continue
		}
		if _, ok := seenDocumentIDs[source.DocumentID]; ok {
			continue
		}
		seenDocumentIDs[source.DocumentID] = struct{}{}
		documentIDs = append(documentIDs, source.DocumentID)
	}
	return &appv1.TreeEntityEvidence{
		SourceDocumentIds: documentIDs,
		SourceRefs:        toProtoItemSourceRefs(evidence.SourceRefs),
	}
}

func toProtoItemSourceRefs(sourceRefs []*domain.ItemSourceRef) []*appv1.ItemSourceRef {
	refs := make([]*appv1.ItemSourceRef, 0, len(sourceRefs))
	for _, source := range sourceRefs {
		if source == nil {
			continue
		}
		refs = append(refs, &appv1.ItemSourceRef{
			SourceRefId:  source.SourceRefID,
			ItemId:       source.ItemID,
			Type:         source.Type,
			Url:          source.URL,
			Repo:         source.Repo,
			Ref:          source.Ref,
			Path:         source.Path,
			LineStart:    int32(source.LineStart),
			LineEnd:      int32(source.LineEnd),
			Kind:         source.Kind,
			ExternalId:   source.ExternalID,
			Title:        source.Title,
			Confidence:   source.Confidence,
			SnapshotRef:  source.SnapshotRef,
			ContentHash:  source.ContentHash,
			MetadataJson: source.MetadataJSON,
		})
	}
	return refs
}

func toProtoUser(user *domain.User) *appv1.User {
	if user == nil {
		return nil
	}
	return &appv1.User{
		UserId:      user.UserID,
		AccountId:   user.AccountID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}
