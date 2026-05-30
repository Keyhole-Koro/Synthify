package agents

import "testing"

func TestRepeatTracker_NudgesAfterThreshold(t *testing.T) {
	tr := newRepeatTracker()
	args := map[string]any{"file_uri": "x", "mime_type": "image/png"}

	// First repeatThreshold calls run normally.
	for i := 0; i < repeatThreshold; i++ {
		if nudge, _ := tr.observe("extract_text", args); nudge {
			t.Fatalf("call %d should not nudge", i+1)
		}
	}
	// The call after the threshold is nudged.
	nudge, count := tr.observe("extract_text", args)
	if !nudge {
		t.Fatalf("call %d should nudge", repeatThreshold+1)
	}
	if count != repeatThreshold+1 {
		t.Fatalf("expected count %d, got %d", repeatThreshold+1, count)
	}
}

func TestRepeatTracker_NudgesOncePerKey(t *testing.T) {
	tr := newRepeatTracker()
	args := map[string]any{"a": 1.0}

	nudged := 0
	for i := 0; i < repeatThreshold+5; i++ {
		if nudge, _ := tr.observe("t", args); nudge {
			nudged++
		}
	}
	if nudged != 1 {
		t.Fatalf("expected exactly one nudge per stuck key, got %d", nudged)
	}
}

func TestRepeatTracker_DifferentArgsResetStreak(t *testing.T) {
	tr := newRepeatTracker()

	// Alternating args never build a streak, so never nudge.
	for i := 0; i < 10; i++ {
		a := map[string]any{"page": float64(i)}
		if nudge, _ := tr.observe("extract_text", a); nudge {
			t.Fatalf("distinct args should never nudge (i=%d)", i)
		}
	}
}

func TestRepeatTracker_DifferentToolResetsStreak(t *testing.T) {
	tr := newRepeatTracker()
	args := map[string]any{"a": "b"}

	for i := 0; i < repeatThreshold; i++ {
		tr.observe("tool_a", args)
	}
	// Switching tool resets the streak even with identical args.
	if nudge, _ := tr.observe("tool_b", args); nudge {
		t.Fatal("switching tool should reset the streak")
	}
}

func TestCallKey_StableAcrossKeyOrder(t *testing.T) {
	// Maps with the same content must hash identically regardless of how Go
	// happens to order them.
	k1 := callKey("t", map[string]any{"a": "1", "b": "2", "c": map[string]any{"d": "3"}})
	k2 := callKey("t", map[string]any{"c": map[string]any{"d": "3"}, "b": "2", "a": "1"})
	if k1 != k2 {
		t.Fatalf("identical args produced different keys: %s vs %s", k1, k2)
	}
	if k1 == callKey("t", map[string]any{"a": "1", "b": "2"}) {
		t.Fatal("different args should produce different keys")
	}
}
