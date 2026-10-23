package stif

import (
	"strings"
	"testing"
)

const sampleDocument = `#STIF/1
@tree workspace="ws-local" mode="append" scope="canonical"

::item /overview
title: "Workspace Overview"
description: "The overview of this workspace"
state: human_curated
cross_document: true
order: 10
---
<article>
  <h1>Overview</h1>
  <p>Rich HTML content goes here.</p>
</article>
::css
article { line-height: 1.7; }
::end

::item /overview/api
title: "API Design"
description: "The important parts of the API design"
parent: /overview
state: pending_review
source:
  - type: github
    url: https://github.com/example/synthify/blob/abc123/docs/api.md#L12-L48
    repo: example/synthify
    ref: abc123
    path: docs/api.md
    lines: [12, 48]
    confidence: 0.86
---
<p>Notes about the API design.</p>
::end
`

func parseOK(t *testing.T, src string) *Document {
	t.Helper()
	doc, diags := Parse(src)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error diagnostic: %s (line %d): %s", d.Code, d.Line, d.Message)
		}
	}
	return doc
}

func findItem(t *testing.T, doc *Document, localPath string) Item {
	t.Helper()
	for _, item := range doc.Items {
		if item.LocalPath == localPath {
			return item
		}
	}
	t.Fatalf("item %s not found; got %v", localPath, itemPaths(doc))
	return Item{}
}

func itemPaths(doc *Document) []string {
	paths := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		paths = append(paths, item.LocalPath)
	}
	return paths
}

func hasDiagnostic(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestParseSampleDocument(t *testing.T) {
	doc := parseOK(t, sampleDocument)

	if doc.WorkspaceID != "ws-local" {
		t.Errorf("workspace = %q, want %q", doc.WorkspaceID, "ws-local")
	}
	if doc.Mode != ModeAppend {
		t.Errorf("mode = %q, want %q", doc.Mode, ModeAppend)
	}
	if len(doc.Items) != 2 {
		t.Fatalf("got %d items, want 2: %v", len(doc.Items), itemPaths(doc))
	}

	overview := findItem(t, doc, "/overview")
	if overview.Title != "Workspace Overview" {
		t.Errorf("title = %q", overview.Title)
	}
	if overview.State != StateHumanCurated {
		t.Errorf("state = %q", overview.State)
	}
	if !overview.CrossDocument {
		t.Error("cross_document = false, want true")
	}
	if overview.Order != 10 {
		t.Errorf("order = %d, want 10", overview.Order)
	}
	if overview.Depth != 0 {
		t.Errorf("depth = %d, want 0", overview.Depth)
	}
	if !strings.Contains(overview.Content, "<h1>Overview</h1>") {
		t.Errorf("content lost its markup: %q", overview.Content)
	}
	if !strings.Contains(overview.OverrideCSS, "line-height") {
		t.Errorf("override css = %q", overview.OverrideCSS)
	}

	api := findItem(t, doc, "/overview/api")
	if api.ParentLocalPath != "/overview" {
		t.Errorf("parent = %q, want /overview", api.ParentLocalPath)
	}
	if api.State != StatePendingReview {
		t.Errorf("state = %q", api.State)
	}
	if api.Depth != 1 {
		t.Errorf("depth = %d, want 1", api.Depth)
	}
	if len(api.Sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(api.Sources))
	}
	source := api.Sources[0]
	if source.Type != "github" || source.Repo != "example/synthify" || source.Ref != "abc123" {
		t.Errorf("source = %+v", source)
	}
	if source.Path != "docs/api.md" || source.LineStart != 12 || source.LineEnd != 48 {
		t.Errorf("source location = %+v", source)
	}
	if source.Confidence != 0.86 {
		t.Errorf("confidence = %v, want 0.86", source.Confidence)
	}
}

func TestParseDefaults(t *testing.T) {
	doc := parseOK(t, "#STIF/1\n::item /a\ntitle: A\n::end\n")

	item := findItem(t, doc, "/a")
	if item.State != DefaultState {
		t.Errorf("state = %q, want %q", item.State, DefaultState)
	}
	if item.Scope != DefaultScope {
		t.Errorf("scope = %q, want %q", item.Scope, DefaultScope)
	}
	if doc.Mode != ModeAppend {
		t.Errorf("mode = %q, want %q", doc.Mode, ModeAppend)
	}
}

func TestParseRejectsMissingHeader(t *testing.T) {
	_, diags := Parse("::item /a\ntitle: A\n::end\n")
	if !hasDiagnostic(diags, "missing_header") {
		t.Fatalf("want missing_header, got %+v", diags)
	}
}

func TestParseRejectsUnsupportedVersion(t *testing.T) {
	_, diags := Parse("#STIF/2\n::item /a\ntitle: A\n::end\n")
	if !hasDiagnostic(diags, "unsupported_version") {
		t.Fatalf("want unsupported_version, got %+v", diags)
	}
}

func TestParseRejectsNonAppendModes(t *testing.T) {
	for _, mode := range []string{ModeReplace, ModeMerge} {
		src := "#STIF/1\n@tree mode=\"" + mode + "\"\n::item /a\ntitle: A\n::end\n"
		_, diags := Parse(src)
		if !hasDiagnostic(diags, "unsupported_mode") {
			t.Errorf("mode %q: want unsupported_mode, got %+v", mode, diags)
		}
	}
}

func TestParseRejectsMissingTitle(t *testing.T) {
	_, diags := Parse("#STIF/1\n::item /a\ndescription: no title here\n::end\n")
	if !hasDiagnostic(diags, "missing_title") {
		t.Fatalf("want missing_title, got %+v", diags)
	}
}

func TestParseRejectsDuplicateLocalPath(t *testing.T) {
	src := "#STIF/1\n::item /a\ntitle: A\n::end\n::item /a\ntitle: A again\n::end\n"
	doc, diags := Parse(src)
	if !hasDiagnostic(diags, "duplicate_local_path") {
		t.Fatalf("want duplicate_local_path, got %+v", diags)
	}
	if len(doc.Items) != 1 {
		t.Errorf("got %d items, want the duplicate dropped", len(doc.Items))
	}
}

func TestParseRejectsUnresolvedParent(t *testing.T) {
	src := "#STIF/1\n::item /a\ntitle: A\nparent: /nope\n::end\n"
	_, diags := Parse(src)
	if !hasDiagnostic(diags, "unresolved_parent") {
		t.Fatalf("want unresolved_parent, got %+v", diags)
	}
}

func TestParseRejectsParentCycle(t *testing.T) {
	src := "#STIF/1\n" +
		"::item /a\ntitle: A\nparent: /b\n::end\n" +
		"::item /b\ntitle: B\nparent: /a\n::end\n"
	doc, diags := Parse(src)
	if !hasDiagnostic(diags, "parent_cycle") {
		t.Fatalf("want parent_cycle, got %+v", diags)
	}
	// Every item still has to appear exactly once so the preview can show what
	// the document contained.
	if len(doc.Items) != 2 {
		t.Errorf("got %d items, want 2: %v", len(doc.Items), itemPaths(doc))
	}
}

func TestParseRejectsSelfParent(t *testing.T) {
	_, diags := Parse("#STIF/1\n::item /a\ntitle: A\nparent: /a\n::end\n")
	if !hasDiagnostic(diags, "self_parent") {
		t.Fatalf("want self_parent, got %+v", diags)
	}
}

func TestParseRejectsUnterminatedBlock(t *testing.T) {
	_, diags := Parse("#STIF/1\n::item /a\ntitle: A\n---\n<p>dangling</p>\n")
	if !hasDiagnostic(diags, "unterminated_block") {
		t.Fatalf("want unterminated_block, got %+v", diags)
	}
}

func TestParseInfersParentFromLocalPath(t *testing.T) {
	src := "#STIF/1\n" +
		"::item /overview\ntitle: Overview\n::end\n" +
		"::item /overview/api\ntitle: API\n::end\n"
	doc := parseOK(t, src)

	if got := findItem(t, doc, "/overview/api").ParentLocalPath; got != "/overview" {
		t.Errorf("inferred parent = %q, want /overview", got)
	}
}

func TestParseWarnsWhenImpliedParentIsMissing(t *testing.T) {
	doc, diags := Parse("#STIF/1\n::item /overview/api\ntitle: API\n::end\n")
	if !hasDiagnostic(diags, "implied_parent_missing") {
		t.Fatalf("want implied_parent_missing, got %+v", diags)
	}
	if HasErrors(diags) {
		t.Error("a missing implied parent must not block the import")
	}
	if got := findItem(t, doc, "/overview/api").ParentLocalPath; got != "" {
		t.Errorf("parent = %q, want the item imported as a root", got)
	}
}

// Sibling order is stored implicitly as created_at, so the parser has to hand
// the importer its items in the order they should be written.
func TestParseOrdersSiblingsByOrderThenDocumentOrder(t *testing.T) {
	src := "#STIF/1\n" +
		"::item /root\ntitle: Root\n::end\n" +
		"::item /root/c\ntitle: C\norder: 30\n::end\n" +
		"::item /root/a\ntitle: A\norder: 10\n::end\n" +
		"::item /root/unordered\ntitle: Unordered\n::end\n" +
		"::item /root/b\ntitle: B\norder: 20\n::end\n"
	doc := parseOK(t, src)

	// The unordered item defaults to order 0, so it sorts ahead of 10/20/30
	// while keeping its document position relative to other unordered items.
	want := []string{"/root", "/root/unordered", "/root/a", "/root/b", "/root/c"}
	got := itemPaths(doc)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("insertion order = %v, want %v", got, want)
		}
	}
}

// Parents must precede their children: the importer needs the parent's row id
// before it can write the child.
func TestParseEmitsParentsBeforeChildren(t *testing.T) {
	src := "#STIF/1\n" +
		"::item /a/b/c\ntitle: C\nparent: /a/b\n::end\n" +
		"::item /a/b\ntitle: B\nparent: /a\n::end\n" +
		"::item /a\ntitle: A\n::end\n"
	doc := parseOK(t, src)

	seen := map[string]bool{}
	for _, item := range doc.Items {
		if item.ParentLocalPath != "" && !seen[item.ParentLocalPath] {
			t.Fatalf("%s emitted before its parent %s; order was %v", item.LocalPath, item.ParentLocalPath, itemPaths(doc))
		}
		seen[item.LocalPath] = true
	}
}

func TestParseStripsMarkdownFence(t *testing.T) {
	src := "```stif\n#STIF/1\n::item /a\ntitle: A\n::end\n```\n"
	doc := parseOK(t, src)
	if len(doc.Items) != 1 {
		t.Fatalf("got %d items, want 1: %v", len(doc.Items), itemPaths(doc))
	}
}

func TestParseAcceptsCRLF(t *testing.T) {
	src := "#STIF/1\r\n::item /a\r\ntitle: A\r\n---\r\n<p>x</p>\r\n::end\r\n"
	doc := parseOK(t, src)
	item := findItem(t, doc, "/a")
	if item.Title != "A" {
		t.Errorf("title = %q, want A", item.Title)
	}
	if item.Content != "<p>x</p>" {
		t.Errorf("content = %q", item.Content)
	}
}

func TestParseClampsConfidence(t *testing.T) {
	src := "#STIF/1\n::item /a\ntitle: A\nsource:\n  - type: web\n    confidence: 1.8\n::end\n"
	doc, diags := Parse(src)
	if !hasDiagnostic(diags, "confidence_out_of_range") {
		t.Fatalf("want confidence_out_of_range, got %+v", diags)
	}
	if got := findItem(t, doc, "/a").Sources[0].Confidence; got != 1 {
		t.Errorf("confidence = %v, want it clamped to 1", got)
	}
}

func TestParseRejectsUnknownSourceType(t *testing.T) {
	src := "#STIF/1\n::item /a\ntitle: A\nsource:\n  - type: telepathy\n::end\n"
	_, diags := Parse(src)
	if !hasDiagnostic(diags, "unknown_source_type") {
		t.Fatalf("want unknown_source_type, got %+v", diags)
	}
}

func TestParseReadsMultipleSources(t *testing.T) {
	src := "#STIF/1\n::item /a\ntitle: A\nsource:\n" +
		"  - type: github\n    repo: owner/repo\n" +
		"  - type: chat\n    id: conv-1\n" +
		"::end\n"
	doc := parseOK(t, src)

	sources := findItem(t, doc, "/a").Sources
	if len(sources) != 2 {
		t.Fatalf("got %d sources, want 2", len(sources))
	}
	if sources[0].Repo != "owner/repo" || sources[1].ExternalID != "conv-1" {
		t.Errorf("sources = %+v", sources)
	}
}

func TestParseWarnsOnUnknownMetadata(t *testing.T) {
	_, diags := Parse("#STIF/1\n::item /a\ntitle: A\nvibes: good\n::end\n")
	if !hasDiagnostic(diags, "unknown_metadata") {
		t.Fatalf("want unknown_metadata, got %+v", diags)
	}
	if HasErrors(diags) {
		t.Error("an unknown metadata key must not block the import")
	}
}

func TestParseWarnsThatMergeKeyIsInert(t *testing.T) {
	_, diags := Parse("#STIF/1\n::item /a\ntitle: A\nmerge_key: k\n::end\n")
	if !hasDiagnostic(diags, "merge_key_ignored") {
		t.Fatalf("want merge_key_ignored, got %+v", diags)
	}
}

func TestHasErrorsIgnoresWarnings(t *testing.T) {
	diags := []Diagnostic{{Severity: SeverityWarning, Code: "x"}}
	if HasErrors(diags) {
		t.Error("warnings must not count as errors")
	}
	if !HasErrors(append(diags, Diagnostic{Severity: SeverityError, Code: "y"})) {
		t.Error("an error diagnostic must be reported")
	}
}
