// Package chat generates one dialogue turn and publishes it to Firestore.
//
// A turn is produced one section at a time rather than in a single call.
// Structured output cannot stream — a partial ContentNode[] is invalid JSON, so
// chunks cannot be assembled incrementally — and generating the whole answer in
// one call means nothing appears until it finishes. One section per call fills
// that gap without pretending to stream.
package chat

import (
	"encoding/json"
	"fmt"
)

// ContentNode mirrors the paper-in-paper type of the same name. The source of
// truth for the shape is the vendored TypeScript definition at
// apps/web/vender/paper-in-paper/src/lib/core/types.ts; this is the Go side of
// the same wire format, kept deliberately loose (a map, not a union) because
// the worker only validates and forwards — the browser is what renders it.
type ContentNode map[string]any

// Node kinds the browser can render. Anything outside this set is dropped or
// demoted to text rather than forwarded: a node the renderer does not
// understand would break the whole paper, not just itself.
var knownNodeTypes = map[string]bool{
	"text":       true,
	"paragraph":  true,
	"bold":       true,
	"paper-link": true,
	"card":       true,
	"section":    true,
	"list":       true,
	"table":      true,
	"callout":    true,
}

// Kinds that carry a paperId and therefore need it checked against the
// candidates offered for this turn.
var paperRefTypes = map[string]bool{
	"paper-link": true,
	"card":       true,
}

// nodesWithChildren maps a node type to the field holding its child nodes, so
// sanitization can recurse without special-casing each kind at the call site.
var childrenField = map[string]string{
	"paragraph": "children",
	"bold":      "children",
	"section":   "children",
	"callout":   "children",
}

// Sanitize drops what the browser cannot render and rewrites references the
// model invented.
//
// Two things are enforced. Unknown node types are removed. Paper references
// pointing outside allowedPaperIDs are demoted to plain text keeping their
// label, because a link to a paper that does not exist looks clickable and then
// does nothing — worse than no link at all.
func Sanitize(nodes []ContentNode, allowedPaperIDs map[string]bool) []ContentNode {
	out := make([]ContentNode, 0, len(nodes))
	for _, node := range nodes {
		clean, ok := sanitizeNode(node, allowedPaperIDs)
		if !ok {
			continue
		}
		out = append(out, clean)
	}
	return out
}

func sanitizeNode(node ContentNode, allowed map[string]bool) (ContentNode, bool) {
	typ, _ := node["type"].(string)
	if !knownNodeTypes[typ] {
		return nil, false
	}

	if paperRefTypes[typ] {
		id, _ := node["paperId"].(string)
		if !allowed[id] {
			return demoteToText(node, typ), true
		}
	}

	if field, ok := childrenField[typ]; ok {
		if raw, present := node[field]; present {
			node[field] = sanitizeChildren(raw, allowed)
		}
	}

	if typ == "list" {
		if items, ok := node["items"].([]any); ok {
			cleaned := make([]any, 0, len(items))
			for _, item := range items {
				cleaned = append(cleaned, sanitizeChildren(item, allowed))
			}
			node["items"] = cleaned
		}
	}

	return node, true
}

// demoteToText turns a dangling reference into plain text, preserving whatever
// the model meant to show the user.
func demoteToText(node ContentNode, typ string) ContentNode {
	label, _ := node["label"].(string)
	if typ == "card" {
		if label == "" {
			label, _ = node["title"].(string)
		}
	}
	return ContentNode{"type": "text", "value": label}
}

func sanitizeChildren(raw any, allowed map[string]bool) []any {
	children, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(children))
	for _, child := range children {
		m, ok := child.(map[string]any)
		if !ok {
			continue
		}
		clean, keep := sanitizeNode(ContentNode(m), allowed)
		if !keep {
			continue
		}
		out = append(out, map[string]any(clean))
	}
	return out
}

// sectionResponse is what one generation call returns.
type sectionResponse struct {
	Nodes []ContentNode `json:"nodes"`
	// Done lets the model stop early instead of padding out to the section
	// cap when the answer is already complete.
	Done bool `json:"done"`
	// SelectedContextIds is only meaningful on the first section, where the
	// model picks which candidates it actually used.
	SelectedContextIDs []string `json:"selectedContextIds"`
	// Commands is a flat list; the worker splits it into auto-dispatchable
	// and Apply-gated halves rather than letting the model decide which is
	// which.
	Commands []Command `json:"commands"`
}

func parseSection(raw json.RawMessage) (sectionResponse, error) {
	var res sectionResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return sectionResponse{}, fmt.Errorf("decode section: %w", err)
	}
	return res, nil
}

// sectionSchema is the response schema handed to the model. It is intentionally
// shallower than the full recursive ContentNode union: a deeply recursive JSON
// schema constrains models unevenly, and Sanitize already rejects anything the
// browser cannot render. The schema steers, the sanitizer enforces.
func sectionSchema() map[string]any {
	node := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":        map[string]any{"type": "string"},
			"value":       map[string]any{"type": "string"},
			"label":       map[string]any{"type": "string"},
			"paperId":     map[string]any{"type": "string"},
			"title":       map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"children":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"items":       map[string]any{"type": "array", "items": map[string]any{"type": "array"}},
			"headers":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"rows":        map[string]any{"type": "array", "items": map[string]any{"type": "array"}},
		},
		"required": []string{"type"},
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nodes":              map[string]any{"type": "array", "items": node},
			"done":               map[string]any{"type": "boolean"},
			"selectedContextIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"commands":           commandSchema(),
		},
		"required": []string{"nodes", "done"},
	}
}
