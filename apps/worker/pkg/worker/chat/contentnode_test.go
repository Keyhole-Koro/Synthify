package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitize_KeepsKnownNodes(t *testing.T) {
	nodes := []ContentNode{
		{"type": "text", "value": "hello"},
		{"type": "paragraph", "children": []any{}},
		{"type": "table", "headers": []any{"a"}, "rows": []any{}},
	}

	got := Sanitize(nodes, nil)

	assert.Len(t, got, 3)
}

// A node type the renderer does not know would break the whole paper, not just
// itself, so it is dropped rather than forwarded.
func TestSanitize_DropsUnknownNodeType(t *testing.T) {
	nodes := []ContentNode{
		{"type": "text", "value": "kept"},
		{"type": "script", "value": "alert(1)"},
		{"type": "iframe", "src": "evil"},
	}

	got := Sanitize(nodes, nil)

	require.Len(t, got, 1)
	assert.Equal(t, "text", got[0]["type"])
}

// A link to a paper that does not exist looks clickable and then does nothing,
// which is worse than no link, so it becomes plain text keeping its label.
func TestSanitize_DemotesDanglingPaperLink(t *testing.T) {
	nodes := []ContentNode{
		{"type": "paper-link", "paperId": "does-not-exist", "label": "See background"},
	}

	got := Sanitize(nodes, map[string]bool{"real": true})

	require.Len(t, got, 1)
	assert.Equal(t, "text", got[0]["type"])
	assert.Equal(t, "See background", got[0]["value"], "the label survives")
}

func TestSanitize_KeepsValidPaperLink(t *testing.T) {
	nodes := []ContentNode{
		{"type": "paper-link", "paperId": "real", "label": "See background"},
	}

	got := Sanitize(nodes, map[string]bool{"real": true})

	require.Len(t, got, 1)
	assert.Equal(t, "paper-link", got[0]["type"])
}

func TestSanitize_DemotesDanglingCardUsingItsTitle(t *testing.T) {
	nodes := []ContentNode{
		{"type": "card", "paperId": "ghost", "title": "Background", "description": "..."},
	}

	got := Sanitize(nodes, map[string]bool{})

	require.Len(t, got, 1)
	assert.Equal(t, "text", got[0]["type"])
	assert.Equal(t, "Background", got[0]["value"])
}

// Nesting is where a bad node is easiest to miss: the outer node is valid, so a
// shallow check would pass it straight through.
func TestSanitize_RecursesIntoChildren(t *testing.T) {
	nodes := []ContentNode{{
		"type": "paragraph",
		"children": []any{
			map[string]any{"type": "text", "value": "ok"},
			map[string]any{"type": "script", "value": "bad"},
			map[string]any{"type": "paper-link", "paperId": "ghost", "label": "dangling"},
		},
	}}

	got := Sanitize(nodes, map[string]bool{})

	require.Len(t, got, 1)
	children, ok := got[0]["children"].([]any)
	require.True(t, ok)
	require.Len(t, children, 2, "the script node is gone")

	second := children[1].(map[string]any)
	assert.Equal(t, "text", second["type"], "the dangling link was demoted, not dropped")
}

func TestSanitize_RecursesIntoListItems(t *testing.T) {
	nodes := []ContentNode{{
		"type": "list",
		"items": []any{
			[]any{
				map[string]any{"type": "text", "value": "ok"},
				map[string]any{"type": "script", "value": "bad"},
			},
		},
	}}

	got := Sanitize(nodes, nil)

	require.Len(t, got, 1)
	items, ok := got[0]["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	first, ok := items[0].([]any)
	require.True(t, ok)
	assert.Len(t, first, 1, "the script node inside the list item is gone")
}

func TestSanitize_EmptyInputYieldsEmptyNotNil(t *testing.T) {
	got := Sanitize(nil, nil)

	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestParseSection_RejectsMalformedJSON(t *testing.T) {
	_, err := parseSection([]byte(`{"nodes": [`))

	require.Error(t, err)
}

func TestParseSection_ReadsDoneAndSelectedIDs(t *testing.T) {
	got, err := parseSection([]byte(`{"nodes":[],"done":true,"selectedContextIds":["a","b"]}`))

	require.NoError(t, err)
	assert.True(t, got.Done)
	assert.Equal(t, []string{"a", "b"}, got.SelectedContextIDs)
}
