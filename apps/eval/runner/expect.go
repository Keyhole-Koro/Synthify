package runner

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type CaseExpect struct {
	SchemaValid bool         `yaml:"schema_valid" json:"schema_valid"`
	JSON        []JSONExpect `yaml:"json,omitempty" json:"json,omitempty"`
}

type JSONExpect struct {
	Path  string `yaml:"path" json:"path"`
	Op    string `yaml:"op" json:"op"`
	Value any    `yaml:"value" json:"value"`
}

func evaluateExpect(expect CaseExpect, output json.RawMessage, schemaValid bool) (bool, error) {
	if expect.SchemaValid && !schemaValid {
		return false, nil
	}
	if len(expect.JSON) == 0 {
		return true, nil
	}
	var root any
	if err := json.Unmarshal(output, &root); err != nil {
		return false, fmt.Errorf("parse output for JSON expectations: %w", err)
	}
	for _, rule := range expect.JSON {
		ok, err := evaluateJSONRule(root, rule)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// evaluateJSONRule applies one JSONExpect to the output JSON.
//
// Op semantics (kept here because the eval-multitool design doc is removed —
// "the contract is the Go type and its doc"):
//
//   - count_gte: dual interpretation by path shape. If the path returns a
//     single value AND that value is itself an array (e.g. "$.items" → the
//     items array), count is the array length. Otherwise (e.g.
//     "$.items[*].title" → a list of titles), count is the number of returned
//     values. This dual mode lets "count of items" be written naturally as
//     "$.items count_gte N" rather than forcing "$.items[*]".
//   - contains_all: case-insensitive *substring* match (NOT exact set
//     containment, despite the name). For each wanted string w, pass requires
//     some returned string v to satisfy strings.Contains(lower(v), lower(w)).
//     This is intentional regression of the legacy missingTitles behaviour
//     (e.g. "認証" matches "Authentication 認証フロー"); rename to
//     contains_all_substring if/when strict equality is needed.
//   - tree_depth_lte: tree-specific depth. If any item has a numeric "level"
//     field, depth = max(level) across items that *have* level set (partial
//     adoption: an item without level is ignored, not assumed depth 0). If no
//     item carries level, fall back to walking parent_local_id chains. The
//     legacy maxDepth used "all-or-nothing" level adoption; current behaviour
//     is "any-or-walk" — equivalent for fully-leveled outputs (the only shape
//     produced today), divergent if level is partially missing. Document or
//     tighten if mixed-level outputs become real.
func evaluateJSONRule(root any, rule JSONExpect) (bool, error) {
	values, err := selectJSONPath(root, rule.Path)
	if err != nil {
		return false, err
	}
	switch rule.Op {
	case "count_gte":
		want, err := intValue(rule.Value)
		if err != nil {
			return false, fmt.Errorf("%s value: %w", rule.Op, err)
		}
		if len(values) == 1 {
			if a, ok := values[0].([]any); ok {
				return len(a) >= want, nil
			}
		}
		return len(values) >= want, nil
	case "contains_all":
		want, err := stringSliceValue(rule.Value)
		if err != nil {
			return false, fmt.Errorf("%s value: %w", rule.Op, err)
		}
		got := make([]string, 0, len(values))
		for _, v := range values {
			if s, ok := v.(string); ok {
				got = append(got, s)
			}
		}
		for _, w := range want {
			if !containsFold(got, w) {
				return false, nil
			}
		}
		return true, nil
	case "tree_depth_lte":
		want, err := intValue(rule.Value)
		if err != nil {
			return false, fmt.Errorf("%s value: %w", rule.Op, err)
		}
		depth, err := treeDepth(values)
		if err != nil {
			return false, err
		}
		return depth <= want, nil
	default:
		return false, fmt.Errorf("unsupported JSON expectation op %q", rule.Op)
	}
}

func selectJSONPath(root any, path string) ([]any, error) {
	path = strings.TrimSpace(path)
	if path == "$" {
		return []any{root}, nil
	}
	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("unsupported JSON path %q", path)
	}
	current := []any{root}
	for _, seg := range strings.Split(strings.TrimPrefix(path, "$."), ".") {
		if seg == "" {
			return nil, fmt.Errorf("unsupported JSON path %q", path)
		}
		next, err := selectSegment(current, seg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		current = next
	}
	return current, nil
}

func selectSegment(values []any, segment string) ([]any, error) {
	name := segment
	index := ""
	if open := strings.Index(segment, "["); open >= 0 {
		if !strings.HasSuffix(segment, "]") {
			return nil, fmt.Errorf("bad segment %q", segment)
		}
		name = segment[:open]
		index = segment[open+1 : len(segment)-1]
	}

	var selected []any
	for _, v := range values {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		child, ok := m[name]
		if !ok {
			continue
		}
		if index == "" {
			selected = append(selected, child)
			continue
		}
		a, ok := child.([]any)
		if !ok {
			continue
		}
		if index == "*" {
			selected = append(selected, a...)
			continue
		}
		i, err := strconv.Atoi(index)
		if err != nil {
			return nil, fmt.Errorf("bad array index %q", index)
		}
		if i >= 0 && i < len(a) {
			selected = append(selected, a[i])
		}
	}
	return selected, nil
}

func intValue(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		if math.Trunc(x) != x {
			return 0, fmt.Errorf("expected integer, got %v", x)
		}
		return int(x), nil
	case json.Number:
		i, err := x.Int64()
		return int(i), err
	default:
		return 0, fmt.Errorf("expected integer, got %T", v)
	}
}

func stringSliceValue(v any) ([]string, error) {
	switch x := v.(type) {
	case []string:
		return x, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected string item, got %T", item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected string array, got %T", v)
	}
}

func containsFold(values []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return true
	}
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), want) {
			return true
		}
	}
	return false
}

func treeDepth(values []any) (int, error) {
	if len(values) == 1 {
		if a, ok := values[0].([]any); ok {
			values = a
		}
	}
	maxLevel := 0
	byID := map[string]string{}
	for _, v := range values {
		m, ok := v.(map[string]any)
		if !ok {
			return 0, fmt.Errorf("tree_depth_lte expects object items")
		}
		if level, ok := m["level"]; ok {
			l, err := intValue(level)
			if err == nil && l > maxLevel {
				maxLevel = l
			}
		}
		id, _ := m["local_id"].(string)
		parent, _ := m["parent_local_id"].(string)
		if id != "" {
			byID[id] = parent
		}
	}
	if maxLevel > 0 {
		return maxLevel, nil
	}
	maxDepth := 0
	for id := range byID {
		depth := 1
		seen := map[string]bool{id: true}
		for parent := byID[id]; parent != "" && !seen[parent]; parent = byID[parent] {
			seen[parent] = true
			depth++
			if _, ok := byID[parent]; !ok {
				break
			}
		}
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth, nil
}
