package stif

import (
	"fmt"
	"sort"
	"strings"
)

// resolve turns a flat list of parsed blocks into an insertion-ordered tree:
// duplicates and unresolvable parents are rejected, parents are inferred from
// the local path shape when omitted, cycles are broken, and the result is
// emitted parents-first with siblings in `order` sequence.
//
// The importer relies on that ordering twice over: parent ids must exist
// before a child row is written, and sibling order is stored implicitly as
// created_at, so siblings must be inserted in the order the author asked for.
func resolve(items []Item, diags *diagList) []Item {
	byPath := make(map[string]int, len(items))
	unique := make([]Item, 0, len(items))
	for _, item := range items {
		if item.LocalPath == "" {
			continue
		}
		if first, exists := byPath[item.LocalPath]; exists {
			diags.errorf(item.Line, item.LocalPath, "duplicate_local_path",
				fmt.Sprintf("local path %s is already defined on line %d", item.LocalPath, unique[first].Line))
			continue
		}
		byPath[item.LocalPath] = len(unique)
		unique = append(unique, item)
	}

	resolveParents(unique, byPath, diags)
	breakCycles(unique, byPath, diags)
	return sortForInsertion(unique)
}

func resolveParents(items []Item, byPath map[string]int, diags *diagList) {
	for i := range items {
		item := &items[i]
		if item.ParentLocalPath != "" {
			if _, ok := byPath[item.ParentLocalPath]; !ok {
				diags.errorf(item.Line, item.LocalPath, "unresolved_parent",
					fmt.Sprintf("parent %s is not defined in this document", item.ParentLocalPath))
				item.ParentLocalPath = ""
			}
			if item.ParentLocalPath == item.LocalPath {
				diags.errorf(item.Line, item.LocalPath, "self_parent",
					fmt.Sprintf("%s cannot be its own parent", item.LocalPath))
				item.ParentLocalPath = ""
			}
			continue
		}
		// No explicit parent: a path-shaped local id implies its ancestor, so
		// /overview/api hangs under /overview when that block exists. When the
		// ancestor is absent the item becomes a root, which is worth saying out
		// loud because it is usually a missing block rather than an intent.
		if ancestor := parentPath(item.LocalPath); ancestor != "" {
			if _, ok := byPath[ancestor]; ok {
				item.ParentLocalPath = ancestor
				continue
			}
			diags.warnf(item.Line, item.LocalPath, "implied_parent_missing",
				fmt.Sprintf("%s has no parent field and its implied parent %s is not in this document; importing it as a root item", item.LocalPath, ancestor))
		}
	}
}

// parentPath returns "/a/b" for "/a/b/c", and "" for a single-segment path.
func parentPath(localPath string) string {
	idx := strings.LastIndex(localPath, "/")
	if idx <= 0 {
		return ""
	}
	return localPath[:idx]
}

// breakCycles detaches any item whose ancestor chain loops. Cycles cannot be
// written to a tree, and detaching (rather than dropping) keeps the rest of
// the document reviewable in the preview.
func breakCycles(items []Item, byPath map[string]int, diags *diagList) {
	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	state := make([]int, len(items))

	for i := range items {
		if state[i] != unvisited {
			continue
		}
		// Walk up from i marking the chain, so a loop shows up as a revisit of
		// a node still on the stack.
		var chain []int
		node := i
		for {
			if state[node] == done {
				break
			}
			if state[node] == onStack {
				diags.errorf(items[node].Line, items[node].LocalPath, "parent_cycle",
					fmt.Sprintf("%s is part of a parent cycle", items[node].LocalPath))
				items[node].ParentLocalPath = ""
				break
			}
			state[node] = onStack
			chain = append(chain, node)

			parent := items[node].ParentLocalPath
			if parent == "" {
				break
			}
			next, ok := byPath[parent]
			if !ok {
				break
			}
			node = next
		}
		for _, n := range chain {
			state[n] = done
		}
	}
}

// sortForInsertion emits a pre-order traversal: every parent precedes its
// children, and siblings are ordered by `order` then by their position in the
// file (a stable sort, so items without `order` keep document order).
func sortForInsertion(items []Item) []Item {
	children := make(map[string][]int, len(items))
	var roots []int
	for i := range items {
		if parent := items[i].ParentLocalPath; parent != "" {
			children[parent] = append(children[parent], i)
			continue
		}
		roots = append(roots, i)
	}

	byOrderThenFile := func(group []int) {
		sort.SliceStable(group, func(a, b int) bool {
			return items[group[a]].Order < items[group[b]].Order
		})
	}
	byOrderThenFile(roots)
	for _, group := range children {
		byOrderThenFile(group)
	}

	out := make([]Item, 0, len(items))
	var walk func(idx, depth int)
	walk = func(idx, depth int) {
		item := items[idx]
		item.Depth = depth
		out = append(out, item)
		for _, child := range children[item.LocalPath] {
			walk(child, depth+1)
		}
	}
	for _, root := range roots {
		walk(root, 0)
	}

	// Anything unreachable from a root sat in a cycle that breakCycles already
	// reported; keep it out of the insertion order rather than looping forever.
	if len(out) < len(items) {
		emitted := make(map[string]bool, len(out))
		for _, item := range out {
			emitted[item.LocalPath] = true
		}
		for i := range items {
			if !emitted[items[i].LocalPath] {
				out = append(out, items[i])
			}
		}
	}
	return out
}
