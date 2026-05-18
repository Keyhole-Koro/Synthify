package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/synthify/backend/packages/shared/domain"
)

// Golden is the persisted expected output for a case. It holds only the
// strict-judgement fields (contract §4.1): item count, title set, parent
// structure, source_chunk_ids, max depth. Full "content" HTML and free-text
// descriptions are intentionally excluded.
type Golden struct {
	ItemCount int          `json:"item_count"`
	MaxDepth  int          `json:"max_depth"`
	Items     []GoldenItem `json:"items"`
}

// GoldenItem captures the structural identity of one item. Title is included
// (title set), local_id/parent_local_id give parent structure, and
// source_chunk_ids are compared as a set.
type GoldenItem struct {
	LocalID        string   `json:"local_id"`
	ParentLocalID  string   `json:"parent_local_id"`
	Title          string   `json:"title"`
	SourceChunkIDs []string `json:"source_chunk_ids"`
}

func goldenFromItems(items []domain.GeneratedTreeItem) Golden {
	g := Golden{
		ItemCount: len(items),
		MaxDepth:  maxDepth(items),
		Items:     make([]GoldenItem, 0, len(items)),
	}
	for _, it := range items {
		ids := append([]string(nil), it.SourceChunkIDs...)
		sort.Strings(ids)
		g.Items = append(g.Items, GoldenItem{
			LocalID:        it.LocalID,
			ParentLocalID:  it.ParentLocalID,
			Title:          it.Title,
			SourceChunkIDs: ids,
		})
	}
	sort.Slice(g.Items, func(i, j int) bool {
		return g.Items[i].LocalID < g.Items[j].LocalID
	})
	return g
}

func goldenPath(dir, caseName string) string {
	return filepath.Join(dir, caseName+".json")
}

func writeGolden(dir, caseName string, items []domain.GeneratedTreeItem) error {
	if dir == "" {
		return fmt.Errorf("--update-golden requires --golden directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(goldenFromItems(items), "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(goldenPath(dir, caseName), b, 0o644)
}

// compareGolden returns nil when items match the stored golden on all strict
// fields, or a GoldenDiff summarizing the mismatches. The diff never contains
// full "content" HTML (contract §6).
func compareGolden(dir, caseName string, items []domain.GeneratedTreeItem) (*GoldenDiff, error) {
	raw, err := os.ReadFile(goldenPath(dir, caseName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("golden %s.json not found; create it with --update-golden", caseName)
		}
		return nil, err
	}
	var want Golden
	if err := json.Unmarshal(raw, &want); err != nil {
		return nil, fmt.Errorf("parse golden %s.json: %w", caseName, err)
	}

	got := goldenFromItems(items)
	var fields []GoldenFieldDiff

	if got.ItemCount != want.ItemCount {
		fields = append(fields, GoldenFieldDiff{
			Field:    "item_count",
			Expected: fmt.Sprintf("%d", want.ItemCount),
			Actual:   fmt.Sprintf("%d", got.ItemCount),
		})
	}
	if got.MaxDepth != want.MaxDepth {
		fields = append(fields, GoldenFieldDiff{
			Field:    "max_depth",
			Expected: fmt.Sprintf("%d", want.MaxDepth),
			Actual:   fmt.Sprintf("%d", got.MaxDepth),
		})
	}

	if d := titleSetDiff(want.Items, got.Items); d != nil {
		fields = append(fields, *d)
	}
	if d := parentStructureDiff(want.Items, got.Items); d != nil {
		fields = append(fields, *d)
	}
	if d := sourceChunkDiff(want.Items, got.Items); d != nil {
		fields = append(fields, *d)
	}

	if len(fields) == 0 {
		return nil, nil
	}
	return &GoldenDiff{Fields: fields}, nil
}

func titleSetDiff(want, got []GoldenItem) *GoldenFieldDiff {
	ws := titleSet(want)
	gs := titleSet(got)
	if setsEqual(ws, gs) {
		return nil
	}
	return &GoldenFieldDiff{
		Field:    "title_set",
		Expected: strings.Join(sortedKeys(ws), " | "),
		Actual:   strings.Join(sortedKeys(gs), " | "),
	}
}

func parentStructureDiff(want, got []GoldenItem) *GoldenFieldDiff {
	we := structureEdges(want)
	ge := structureEdges(got)
	if setsEqual(we, ge) {
		return nil
	}
	return &GoldenFieldDiff{
		Field:    "parent_structure",
		Expected: strings.Join(sortedKeys(we), " | "),
		Actual:   strings.Join(sortedKeys(ge), " | "),
	}
}

func sourceChunkDiff(want, got []GoldenItem) *GoldenFieldDiff {
	ws := sourceChunkSet(want)
	gs := sourceChunkSet(got)
	if setsEqual(ws, gs) {
		return nil
	}
	return &GoldenFieldDiff{
		Field:    "source_chunk_ids",
		Expected: strings.Join(sortedKeys(ws), " | "),
		Actual:   strings.Join(sortedKeys(gs), " | "),
	}
}

func titleSet(items []GoldenItem) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, it := range items {
		s[it.Title] = struct{}{}
	}
	return s
}

func structureEdges(items []GoldenItem) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, it := range items {
		s[it.LocalID+"<-"+it.ParentLocalID] = struct{}{}
	}
	return s
}

func sourceChunkSet(items []GoldenItem) map[string]struct{} {
	s := make(map[string]struct{})
	for _, it := range items {
		for _, id := range it.SourceChunkIDs {
			s[it.LocalID+":"+id] = struct{}{}
		}
	}
	return s
}

func setsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(s map[string]struct{}) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
