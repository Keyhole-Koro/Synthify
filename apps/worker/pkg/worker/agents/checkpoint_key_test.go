package agents

import "testing"

func TestCheckpointKey_FixedStageTools(t *testing.T) {
	cases := map[string]string{
		"generate_brief":          "briefing",
		"generate_knowledge_tree": "knowledge_tree",
		"persist_knowledge_tree":  "persistence",
	}
	for tool, want := range cases {
		if got := checkpointKey(tool, nil); got != want {
			t.Errorf("checkpointKey(%q) = %q, want %q", tool, got, want)
		}
	}
}

func TestCheckpointKey_NonStageToolIsEmpty(t *testing.T) {
	if got := checkpointKey("grep_search", map[string]any{"pattern": "x"}); got != "" {
		t.Errorf("checkpointKey(grep_search) = %q, want empty", got)
	}
}

func TestCheckpointKey_HTMLSummaryIsPerItem(t *testing.T) {
	args := map[string]any{
		"item": map[string]any{"local_id": "item_2", "title": "Flowcharts"},
	}
	got := checkpointKey("generate_html_summary", args)
	if want := "html_summary/item_2"; got != want {
		t.Errorf("checkpointKey = %q, want %q", got, want)
	}
}

// Two different items must map to different keys, or one would overwrite the
// other's checkpoint and a resume could not skip both independently.
func TestCheckpointKey_DistinctItemsDistinctKeys(t *testing.T) {
	k1 := checkpointKey("generate_html_summary", map[string]any{"item": map[string]any{"local_id": "item_1"}})
	k2 := checkpointKey("generate_html_summary", map[string]any{"item": map[string]any{"local_id": "item_2"}})
	if k1 == k2 {
		t.Fatalf("items collided on key %q", k1)
	}
}

// Read and write paths must produce the same key for the same call, otherwise a
// recorded checkpoint is never found on resume.
func TestCheckpointKey_StableAcrossCalls(t *testing.T) {
	args := map[string]any{"item": map[string]any{"local_id": "item_7"}}
	if checkpointKey("generate_html_summary", args) != checkpointKey("generate_html_summary", args) {
		t.Error("checkpointKey is not stable for identical args")
	}
}

// A per-item tool call with no identifiable item falls back to the bare stage
// rather than producing an empty (non-checkpointing) key.
func TestCheckpointKey_HTMLSummaryWithoutItemFallsBack(t *testing.T) {
	if got := checkpointKey("generate_html_summary", map[string]any{}); got != "html_summary" {
		t.Errorf("checkpointKey = %q, want fallback \"html_summary\"", got)
	}
}
