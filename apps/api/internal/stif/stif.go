// Package stif parses STIF (Synthify Tree Item Format) documents into items
// ready to be appended to a workspace tree.
//
// The format is specified in docs/improvements/stif-tree-item-import-format.md.
// This package implements Phase 1: parse, validate, sanitize. It performs no
// I/O and knows nothing about the database or the API — the application layer
// maps the result onto domain items.
//
// Parsing never returns an error value. Everything that can go wrong is
// reported as a Diagnostic so a single pass can surface every problem in the
// document at once, which is what the import preview needs. Callers must
// refuse to commit when HasErrors reports true.
package stif

// Severity classifies a Diagnostic. Warnings describe something that was
// adjusted (a stripped script tag, a clamped confidence); errors describe
// something that makes the document unimportable.
type Severity int

const (
	SeverityWarning Severity = iota + 1
	SeverityError
)

// Diagnostic is one problem found in the source document.
type Diagnostic struct {
	Severity Severity
	// Line is 1-indexed into the source document, or 0 when the diagnostic is
	// about the document as a whole.
	Line int
	// Code is a stable identifier so the UI can group or translate messages.
	Code string
	// Message is human-readable English, matching the rest of the API surface.
	Message string
	// LocalPath names the item block the diagnostic belongs to, when it has one.
	LocalPath string
}

// Source is a parsed `source:` entry: a reference handle for the evidence
// behind an item, not the evidence itself. Retrieval happens later, on demand.
type Source struct {
	Type       string
	URL        string
	Repo       string
	Ref        string
	Path       string
	LineStart  int
	LineEnd    int
	Kind       string
	ExternalID string
	Title      string
	Confidence float64
}

// Item is one parsed `::item` block.
type Item struct {
	// LocalPath is the document-scoped identifier (e.g. "/overview/api"). It is
	// never a database id; ids are assigned at import time.
	LocalPath string
	// ParentLocalPath is empty for a root item.
	ParentLocalPath string
	Title           string
	Description     string
	Content         string
	OverrideCSS     string
	// State and Scope hold validated STIF vocabulary ("human_curated",
	// "canonical", ...), defaulted when the block omitted them.
	State         string
	Scope         string
	CrossDocument bool
	Order         int
	MergeKey      string
	Sources       []Source
	// Line is the 1-indexed line of the `::item` header.
	Line int
	// Depth is 0 for a root item. Filled in during resolution.
	Depth int
}

// Document is a parsed STIF file. Items are ordered parents-before-children,
// siblings by `order` then by their position in the file, so an importer can
// insert them in slice order and get both the parent links and the sibling
// ordering right.
type Document struct {
	Version     string
	WorkspaceID string
	Mode        string
	Scope       string
	Items       []Item
}

// Governance states and scopes accepted by the format.
const (
	StateSystemGenerated = "system_generated"
	StatePendingReview   = "pending_review"
	StateHumanCurated    = "human_curated"
	StateLocked          = "locked"

	ScopeDocument  = "document"
	ScopeCanonical = "canonical"

	ModeAppend  = "append"
	ModeReplace = "replace"
	ModeMerge   = "merge"
)

// Phase 1 defaults, per the spec: an item that does not say otherwise is
// human-authored and canonical.
const (
	DefaultState = StateHumanCurated
	DefaultScope = ScopeCanonical
	DefaultMode  = ModeAppend
)

var validStates = map[string]bool{
	StateSystemGenerated: true,
	StatePendingReview:   true,
	StateHumanCurated:    true,
	StateLocked:          true,
}

var validScopes = map[string]bool{
	ScopeDocument:  true,
	ScopeCanonical: true,
}

var validSourceTypes = map[string]bool{
	"github":     true,
	"web":        true,
	"local_file": true,
	"chat":       true,
	"document":   true,
}

// HasErrors reports whether any diagnostic blocks the import.
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

type diagList struct {
	items []Diagnostic
}

func (d *diagList) errorf(line int, localPath, code, message string) {
	d.items = append(d.items, Diagnostic{
		Severity:  SeverityError,
		Line:      line,
		Code:      code,
		Message:   message,
		LocalPath: localPath,
	})
}

func (d *diagList) warnf(line int, localPath, code, message string) {
	d.items = append(d.items, Diagnostic{
		Severity:  SeverityWarning,
		Line:      line,
		Code:      code,
		Message:   message,
		LocalPath: localPath,
	})
}
