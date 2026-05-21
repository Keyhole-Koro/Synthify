package io

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

type PersistenceArgs struct {
	JobID       string            `json:"job_id"`
	DocumentID  string            `json:"document_id"`
	WorkspaceID string            `json:"workspace_id"`
	Items       []PersistenceItem `json:"items"`
}

type PersistenceItem struct {
	domain.GeneratedTreeItem
	FileID string `json:"file_id"`
}

type PersistenceResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewPersistenceTool(b *base.Context) (core.Tool, error) {
	schema, err := core.IOSchemaFor[PersistenceArgs, PersistenceResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(ctx context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args PersistenceArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		if len(args.Items) == 0 {
			out, err := json.Marshal(PersistenceResult{Success: false, Message: "No items to persist"})
			return out, core.Usage{}, err
		}
		if b == nil || b.Repo == nil {
			return nil, core.Usage{}, fmt.Errorf("repository is not configured")
		}
		if err := b.IncrementItemCreations(ctx, len(args.Items)); err != nil {
			return nil, core.Usage{}, err
		}
		capability, err := b.Repo.GetJobCapability(ctx, args.JobID)
		if err != nil {
			return nil, core.Usage{}, fmt.Errorf("job capability not found: %s (%w)", args.JobID, err)
		}

		itemIDs := make(map[string]string, len(args.Items))
		rootID, err := b.Repo.GetWorkspaceRootItemID(ctx, args.WorkspaceID)
		if err != nil {
			rootID = ""
		}
		created := 0
		for _, item := range args.Items {
			parentID := rootID
			if mapped := itemIDs[item.ParentLocalID]; mapped != "" {
				parentID = mapped
			}
			title := strings.TrimSpace(item.Title)
			if title == "" {
				title = item.LocalID
			}
			createdItem := b.Repo.CreateStructuredItemWithCapability(
				ctx,
				capability,
				args.JobID,
				args.DocumentID,
				args.WorkspaceID,
				title,
				item.Level,
				item.Description,
				item.Content,
				item.OverrideCSS,
				"llm_worker",
				parentID,
				item.SourceChunkIDs,
			)
			if createdItem == nil {
				return nil, core.Usage{}, fmt.Errorf("failed to create item %q", title)
			}
			itemIDs[item.LocalID] = createdItem.ItemID
			for _, chunkID := range item.SourceChunkIDs {
				if err := b.Repo.UpsertItemSource(ctx, createdItem.ItemID, args.DocumentID, item.FileID, chunkID, item.Description, 0.75); err != nil {
					return nil, core.Usage{}, err
				}
			}
			created++
		}
		out, err := json.Marshal(PersistenceResult{Success: true, Message: fmt.Sprintf("Successfully persisted %d items", created)})
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "persist_knowledge_tree",
		Description: "Permanently saves the generated knowledge tree items to the database.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}
