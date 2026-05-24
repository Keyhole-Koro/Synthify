package io

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

func TestPersistenceTool_TopologicalSort(t *testing.T) {
	items := []PersistenceItem{
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "child", ParentLocalID: "parent", Title: "Child"}},
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "parent", ParentLocalID: "", Title: "Parent"}},
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "grandchild", ParentLocalID: "child", Title: "Grandchild"}},
	}

	sorted := sortItemsTopologically(items)

	require.Len(t, sorted, 3)
	assert.Equal(t, "parent", sorted[0].LocalID)
	assert.Equal(t, "child", sorted[1].LocalID)
	assert.Equal(t, "grandchild", sorted[2].LocalID)
}

func TestPersistenceTool_HTMLRewriting(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()

	// 1. Setup Mock Workspace and Job
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, store, "user_1")
	wsID := fixture.Workspace.WorkspaceID
	jobID := "job_1"
	docID := "doc_1"

	// 2. Setup Capability (needed by PersistenceTool)
	store.SeedCapability(domain.JobCapability{
		CapabilityID:     "cap_" + jobID,
		JobID:            jobID,
		WorkspaceID:      wsID,
		MaxLLMCalls:      100,
		MaxToolRuns:      100,
		MaxItemCreations: 100,
		ExpiresAt:        time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	})

	b := &base.Context{
		Repo: store,
		Job: &base.JobContext{
			JobID:       jobID,
			WorkspaceID: wsID,
			DocumentID:  docID,
		},
	}

	tool, err := NewPersistenceTool(b)
	require.NoError(t, err)

	// 3. Define items with cross-references in HTML
	args := PersistenceArgs{
		JobID:       jobID,
		DocumentID:  docID,
		WorkspaceID: wsID,
		Items: []PersistenceItem{
			{
				GeneratedTreeItem: domain.GeneratedTreeItem{
					LocalID: "item_1",
					Title:   "Parent Item",
					Content: `<p>Link to <a data-paper-id="item_2">Child</a></p>`,
				},
			},
			{
				GeneratedTreeItem: domain.GeneratedTreeItem{
					LocalID:       "item_2",
					ParentLocalID: "item_1",
					Title:         "Child Item",
					Content:       `<p>Link back to <a data-paper-id="item_1">Parent</a></p>`,
				},
			},
		},
	}

	argsJSON, _ := json.Marshal(args)
	_, _, err = tool.Run(ctx, argsJSON)
	require.NoError(t, err)

	// 4. Verify items were created and HTML rewritten
	items, err := store.GetTreeByWorkspace(ctx, wsID)
	require.NoError(t, err)

	var parent, child *domain.Item
	for _, item := range items {
		if item.Title == "Parent Item" {
			parent = item
		} else if item.Title == "Child Item" {
			child = item
		}
	}

	require.NotNil(t, parent)
	require.NotNil(t, child)

	// Parent should link to child's REAL ID (which in mock is "item-Child Item")
	assert.Contains(t, parent.Content, `data-paper-id="`+child.ItemID+`"`)
	assert.NotContains(t, parent.Content, `data-paper-id="item_2"`)

	// Child should link to parent's REAL ID (which in mock is "item-Parent Item")
	assert.Contains(t, child.Content, `data-paper-id="`+parent.ItemID+`"`)
	assert.NotContains(t, child.Content, `data-paper-id="item_1"`)
}
