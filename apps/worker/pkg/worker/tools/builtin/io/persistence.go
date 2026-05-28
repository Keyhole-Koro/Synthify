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

		// Phase 1: Sort items topologically to ensure parents are created before children
		sortedItems := sortItemsTopologically(args.Items)

		itemIDs := make(map[string]string, len(args.Items))
		workspaceRootID, err := b.Repo.GetWorkspaceRootItemID(ctx, args.WorkspaceID)
		if err != nil {
			workspaceRootID = ""
		}
		// document_root_item: the first item the LLM produces whose
		// ParentLocalID is empty (= sits directly under the workspace
		// root) is registered as the document's root. Subsequent
		// top-level items, if any, become regular nodes under the
		// workspace root — the schema's 1:1 link permits only one
		// document_root per document.
		documentRootCreated := false
		created := 0
		for _, item := range sortedItems {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				title = item.LocalID
			}
			isTopLevel := item.ParentLocalID == ""
			parentID := workspaceRootID
			if mapped := itemIDs[item.ParentLocalID]; mapped != "" {
				parentID = mapped
			}

			var createdItem *domain.Item
			if isTopLevel && !documentRootCreated {
				createdItem, err = b.Repo.CreateDocumentRootItemWithCapability(
					ctx,
					capability,
					args.JobID,
					args.DocumentID,
					args.WorkspaceID,
					title,
					item.Description,
					workspaceRootID,
				)
				if err != nil {
					return nil, core.Usage{}, fmt.Errorf("create document root item %q: %w", title, err)
				}
				documentRootCreated = true
			} else {
				createdItem, err = b.Repo.CreateStructuredItemWithCapability(
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
				if err != nil {
					return nil, core.Usage{}, fmt.Errorf("create item %q: %w", title, err)
				}
			}
			itemIDs[item.LocalID] = createdItem.ItemID
			for _, chunkID := range item.SourceChunkIDs {
				if err := b.Repo.UpsertItemSource(ctx, createdItem.ItemID, args.DocumentID, item.FileID, chunkID, item.Description, 0.75); err != nil {
					return nil, core.Usage{}, err
				}
			}
			created++
		}

		// Phase 2: Rewrite HTML references from local IDs to persisted ULIDs
		for _, item := range sortedItems {
			persistedID := itemIDs[item.LocalID]
			content := item.Content
			if !strings.Contains(content, "data-paper-id=") {
				continue
			}

			rewritten := content
			changed := false
			for localID, realID := range itemIDs {
				// Match both double and single quotes
				searchDQ := fmt.Sprintf("data-paper-id=\"%s\"", localID)
				replaceDQ := fmt.Sprintf("data-paper-id=\"%s\"", realID)
				if strings.Contains(rewritten, searchDQ) {
					rewritten = strings.ReplaceAll(rewritten, searchDQ, replaceDQ)
					changed = true
				}
				searchSQ := fmt.Sprintf("data-paper-id='%s'", localID)
				replaceSQ := fmt.Sprintf("data-paper-id='%s'", realID)
				if strings.Contains(rewritten, searchSQ) {
					rewritten = strings.ReplaceAll(rewritten, searchSQ, replaceSQ)
					changed = true
				}
			}

			if changed {
				_ = b.Repo.UpdateItemSummaryHTMLWithCapability(ctx, capability, args.JobID, persistedID, rewritten)
			}
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

func sortItemsTopologically(items []PersistenceItem) []PersistenceItem {
	itemMap := make(map[string]PersistenceItem)
	inDegree := make(map[string]int)
	children := make(map[string][]string)

	allLocalIDs := make(map[string]bool)
	for _, item := range items {
		itemMap[item.LocalID] = item
		allLocalIDs[item.LocalID] = true
	}

	for _, item := range items {
		pID := item.ParentLocalID
		if pID != "" && allLocalIDs[pID] {
			inDegree[item.LocalID]++
			children[pID] = append(children[pID], item.LocalID)
		}
	}

	var queue []string
	for _, item := range items {
		if inDegree[item.LocalID] == 0 {
			queue = append(queue, item.LocalID)
		}
	}

	var result []PersistenceItem
	visited := make(map[string]bool)

	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]

		if visited[currID] {
			continue
		}
		visited[currID] = true
		result = append(result, itemMap[currID])

		for _, childID := range children[currID] {
			inDegree[childID]--
			if inDegree[childID] == 0 {
				queue = append(queue, childID)
			}
		}
	}

	// Handle cycles or missing parents by adding remaining items to keep them in the tree
	if len(result) < len(items) {
		for _, item := range items {
			if !visited[item.LocalID] {
				result = append(result, item)
			}
		}
	}

	return result
}
