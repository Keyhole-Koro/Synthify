package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allowed(ids ...string) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func TestClassifyCommands_SplitsViewFromStructural(t *testing.T) {
	cmds := []Command{
		{"type": "FOCUS_NODE", "nodeId": "a"},
		{"type": "OPEN_NODE", "parentId": "a", "childId": "b"},
		{"type": "PATCH_NODE", "nodeId": "a"},
		{"type": "CREATE_CHILD_NODE", "parentId": "a"},
	}

	auto, proposals := ClassifyCommands(cmds, allowed("a", "b"))

	assert.Len(t, auto, 2, "view operations dispatch on their own")
	assert.Len(t, proposals, 2, "structural changes wait for Apply")
}

// DELETE_NODE is excluded from the tool schema, so one appearing means the
// model went outside what it was offered — the case that must never execute.
func TestClassifyCommands_DropsDeleteNode(t *testing.T) {
	auto, proposals := ClassifyCommands(
		[]Command{{"type": "DELETE_NODE", "nodeId": "a"}},
		allowed("a"),
	)

	assert.Empty(t, auto)
	assert.Empty(t, proposals, "DELETE_NODE must not reach either field")
}

func TestClassifyCommands_DropsInternalSyncCommands(t *testing.T) {
	cmds := []Command{
		{"type": "__SYNC_PAPER_MAP"},
		{"type": "__SYNC_OPEN_STATE"},
		{"type": "__SYNC_UNPLACED"},
	}

	auto, proposals := ClassifyCommands(cmds, allowed())

	assert.Empty(t, auto)
	assert.Empty(t, proposals)
}

func TestClassifyCommands_DropsUnknownType(t *testing.T) {
	auto, proposals := ClassifyCommands(
		[]Command{{"type": "DROP_DATABASE", "nodeId": "a"}, {"type": ""}},
		allowed("a"),
	)

	assert.Empty(t, auto)
	assert.Empty(t, proposals)
}

// A hallucinated id would otherwise make the browser focus or patch something
// arbitrary.
func TestClassifyCommands_DropsCommandsNamingUnknownPapers(t *testing.T) {
	cmds := []Command{
		{"type": "FOCUS_NODE", "nodeId": "ghost"},
		{"type": "FOCUS_NODE", "nodeId": "real"},
	}

	auto, _ := ClassifyCommands(cmds, allowed("real"))

	require.Len(t, auto, 1)
	assert.Equal(t, "real", auto[0]["nodeId"])
}

// OPEN_NODE names both the child and the parent to open it under; a wrong
// parent targets the right paper in the wrong place.
func TestClassifyCommands_RequiresBothEndsOfOpenNode(t *testing.T) {
	cmds := []Command{
		{"type": "OPEN_NODE", "parentId": "ghost", "childId": "real"},
		{"type": "OPEN_NODE", "childId": "real"},
		{"type": "OPEN_NODE", "parentId": "real", "childId": "other"},
	}

	auto, _ := ClassifyCommands(cmds, allowed("real", "other"))

	require.Len(t, auto, 1)
	assert.Equal(t, "other", auto[0]["childId"])
}

func TestClassifyCommands_DropsCommandMissingItsNodeReference(t *testing.T) {
	auto, _ := ClassifyCommands([]Command{{"type": "FOCUS_NODE"}}, allowed("real"))

	assert.Empty(t, auto)
}

// MERGE_PAPERS and UPSERT_PAPERS carry their own payloads rather than a single
// node reference; Apply validates those against the real tree.
func TestClassifyCommands_KeepsPayloadCommandsAsProposals(t *testing.T) {
	cmds := []Command{
		{"type": "MERGE_PAPERS", "papers": []any{}},
		{"type": "UPSERT_PAPERS", "papers": []any{}},
	}

	auto, proposals := ClassifyCommands(cmds, allowed())

	assert.Empty(t, auto)
	assert.Len(t, proposals, 2)
}

func TestMarshalCommands_EmptyYieldsEmptyString(t *testing.T) {
	got, err := marshalCommands(nil)

	require.NoError(t, err)
	// Empty string so the optional field is omitted rather than written as an
	// array that says nothing.
	assert.Equal(t, "", got)
}

func TestMarshalCommands_EncodesList(t *testing.T) {
	got, err := marshalCommands([]Command{{"type": "FOCUS_NODE", "nodeId": "a"}})

	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"FOCUS_NODE","nodeId":"a"}]`, got)
}

// The schema is what steers the model; advertising a destructive command would
// invite one even though the classifier drops it.
func TestCommandSchema_OmitsDestructiveAndInternalCommands(t *testing.T) {
	items, ok := commandSchema()["items"].(map[string]any)
	require.True(t, ok)
	props, ok := items["properties"].(map[string]any)
	require.True(t, ok)
	typ, ok := props["type"].(map[string]any)
	require.True(t, ok)
	values, ok := typ["enum"].([]string)
	require.True(t, ok)

	for _, v := range values {
		assert.NotEqual(t, "DELETE_NODE", v)
		assert.NotContains(t, v, "__SYNC")
	}
	assert.Contains(t, values, "OPEN_NODE")
}
