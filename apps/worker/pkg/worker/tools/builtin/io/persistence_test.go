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

// persistDoc runs the persistence tool for one document and returns the
// resulting workspace items. Helper for the single-root tests.
func persistDoc(t *testing.T, store *mock.Store, wsID, jobID, docID string, items []PersistenceItem) []*domain.Item {
	t.Helper()
	store.SeedCapability(domain.JobCapability{
		CapabilityID:     "cap_" + jobID,
		JobID:            jobID,
		WorkspaceID:      wsID,
		MaxLLMCalls:      100,
		MaxToolRuns:      100,
		MaxItemCreations: 100,
		ExpiresAt:        time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	})
	b := &base.Context{Repo: store, Job: &base.JobContext{JobID: jobID, WorkspaceID: wsID, DocumentID: docID}}
	tool, err := NewPersistenceTool(b)
	require.NoError(t, err)
	argsJSON, _ := json.Marshal(PersistenceArgs{JobID: jobID, DocumentID: docID, WorkspaceID: wsID, Items: items})
	_, _, err = tool.Run(context.Background(), argsJSON)
	require.NoError(t, err)
	result, err := store.GetTreeByWorkspace(context.Background(), wsID)
	require.NoError(t, err)
	return result
}

func rootNodes(items []*domain.Item) []*domain.Item {
	var roots []*domain.Item
	for _, it := range items {
		if it.ParentID == "" {
			roots = append(roots, it)
		}
	}
	return roots
}

func TestPersistenceTool_SingleRoot_FirstDocument(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "user_1").Workspace.WorkspaceID

	// Two top-level items (both empty parent) in the first document: only the
	// first becomes the root; the second is forced under it.
	items := persistDoc(t, store, wsID, "job_1", "doc_1", []PersistenceItem{
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "a", Title: "Overview"}},
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "b", Title: "Second top-level"}},
	})

	roots := rootNodes(items)
	require.Len(t, roots, 1, "exactly one root node after first document")
	assert.Equal(t, "Overview", roots[0].Title)
	for _, it := range items {
		if it.Title == "Second top-level" {
			assert.Equal(t, roots[0].ItemID, it.ParentID, "second top-level item hangs off the root")
		}
	}
}

func TestPersistenceTool_SingleRoot_SecondDocumentReusesRoot(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "user_1").Workspace.WorkspaceID

	persistDoc(t, store, wsID, "job_1", "doc_1", []PersistenceItem{
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "a", Title: "Overview"}},
	})
	first := rootNodes(persistDocNoop(t, store, wsID))[0]

	// Second document: its top-level item must hang off the existing root, not
	// create a second root.
	items := persistDoc(t, store, wsID, "job_2", "doc_2", []PersistenceItem{
		{GeneratedTreeItem: domain.GeneratedTreeItem{LocalID: "c", Title: "New concept from doc 2"}},
	})

	roots := rootNodes(items)
	require.Len(t, roots, 1, "still exactly one root after the second document")
	assert.Equal(t, first.ItemID, roots[0].ItemID, "root is unchanged")
	for _, it := range items {
		if it.Title == "New concept from doc 2" {
			assert.Equal(t, first.ItemID, it.ParentID, "doc 2's top-level item hangs off the existing root")
		}
	}
}

func persistDocNoop(t *testing.T, store *mock.Store, wsID string) []*domain.Item {
	t.Helper()
	items, err := store.GetTreeByWorkspace(context.Background(), wsID)
	require.NoError(t, err)
	return items
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
