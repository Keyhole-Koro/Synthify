package chat

import "encoding/json"

// Command is a canvas operation the model asked for. Like ContentNode it stays
// a map rather than a union: the worker classifies and validates, the browser
// dispatches. The source of truth for the shape is the paper-in-paper Command
// type in apps/web/vender/paper-in-paper/src/lib/core/commands.ts.
type Command map[string]any

// autoCommands may be dispatched by the browser without asking. Every one of
// these is either reversible or loses no state: the worst a wrong guess does is
// move the reader's view, which they can undo by hand.
var autoCommands = map[string]bool{
	"OPEN_NODE":  true,
	"CLOSE_NODE": true,
	"FOCUS_NODE": true,
	"PIN_NODE":   true,
	"UNPIN_NODE": true,
}

// proposalCommands change the tree, so they are shown as a preview and wait for
// an explicit Apply. A model that is confidently wrong here would otherwise
// rewrite a reader's notes.
var proposalCommands = map[string]bool{
	"CREATE_CHILD_NODE": true,
	"PATCH_NODE":        true,
	"MOVE_NODE":         true,
	"MERGE_PAPERS":      true,
	"UPSERT_PAPERS":     true,
}

// Commands that carry a node the operation acts on, which must name a paper
// the model was actually shown.
var commandNodeField = map[string]string{
	"OPEN_NODE":         "childId",
	"CLOSE_NODE":        "childId",
	"FOCUS_NODE":        "nodeId",
	"PIN_NODE":          "nodeId",
	"UNPIN_NODE":        "nodeId",
	"CREATE_CHILD_NODE": "parentId",
	"PATCH_NODE":        "nodeId",
	"MOVE_NODE":         "nodeId",
}

// ClassifyCommands splits model-proposed commands into the two fields the
// contract defines, dropping anything that does not belong in either.
//
// The model's own labelling is not trusted: it proposes a flat list and the
// split happens here, so a structural change cannot reach the browser's
// auto-dispatch path by being mislabelled.
//
// DELETE_NODE is absent from both maps and is therefore dropped. It is also
// excluded from the tool schema, so seeing one means the model went outside
// what it was offered — exactly the case that must not be executed.
func ClassifyCommands(cmds []Command, allowedPaperIDs map[string]bool) (auto []Command, proposals []Command) {
	auto = make([]Command, 0, len(cmds))
	proposals = make([]Command, 0, len(cmds))

	for _, cmd := range cmds {
		typ, _ := cmd["type"].(string)
		if typ == "" {
			continue
		}
		if !referencesKnownPaper(cmd, typ, allowedPaperIDs) {
			continue
		}
		switch {
		case autoCommands[typ]:
			auto = append(auto, cmd)
		case proposalCommands[typ]:
			proposals = append(proposals, cmd)
		}
		// Anything else — DELETE_NODE, __SYNC_*, an invented type — falls
		// through and is dropped.
	}
	return auto, proposals
}

// referencesKnownPaper rejects a command aimed at a paper the model was never
// shown. Without this a hallucinated id would make the browser open or focus
// something arbitrary, or worse, patch it.
func referencesKnownPaper(cmd Command, typ string, allowed map[string]bool) bool {
	field, ok := commandNodeField[typ]
	if !ok {
		// No node reference to check (MERGE_PAPERS / UPSERT_PAPERS carry their
		// own payloads, which Apply validates against the real tree).
		return true
	}
	id, _ := cmd[field].(string)
	if id == "" {
		return false
	}
	if !allowed[id] {
		return false
	}
	// OPEN_NODE and CLOSE_NODE also name the parent to open under; a wrong one
	// would target the right paper in the wrong place.
	if typ == "OPEN_NODE" || typ == "CLOSE_NODE" {
		parentID, _ := cmd["parentId"].(string)
		if parentID == "" || !allowed[parentID] {
			return false
		}
	}
	return true
}

func marshalCommands(cmds []Command) (string, error) {
	if len(cmds) == 0 {
		// Empty rather than "[]" so the field is omitted from the document:
		// the schema makes it optional, and an empty array would have the
		// browser parse something that says nothing.
		return "", nil
	}
	raw, err := json.Marshal(cmds)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// commandSchema describes the command list handed to the model. Only the
// dispatchable types are advertised: DELETE_NODE and the internal __SYNC_*
// commands are deliberately absent, so a model following the schema never
// proposes one.
func commandSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type": map[string]any{
					"type": "string",
					"enum": []string{
						"OPEN_NODE", "CLOSE_NODE", "FOCUS_NODE", "PIN_NODE", "UNPIN_NODE",
						"CREATE_CHILD_NODE", "PATCH_NODE", "MOVE_NODE",
					},
				},
				"nodeId":      map[string]any{"type": "string"},
				"parentId":    map[string]any{"type": "string"},
				"childId":     map[string]any{"type": "string"},
				"title":       map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
			},
			"required": []string{"type"},
		},
	}
}
