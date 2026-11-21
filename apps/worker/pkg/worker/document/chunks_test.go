package document

import (
	"strings"
	"testing"
)

func TestBuildChunks(t *testing.T) {
	chunks := BuildChunks("doc_1", "# Intro\nfirst\n\n2. Details\nsecond")
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].ChunkID != "doc_1_chunk_0" || chunks[0].Heading != "Intro" {
		t.Fatalf("first chunk = %+v", chunks[0])
	}
	if chunks[1].Heading != "2. Details" || chunks[1].Text != "2. Details\nsecond" {
		t.Fatalf("second chunk = %+v", chunks[1])
	}
	if got := BuildChunks("doc_1", "  "); got != nil {
		t.Fatalf("empty document chunks = %#v, want nil", got)
	}
}

func TestSplitSectionsUsesIntroductionWhenThereIsNoHeading(t *testing.T) {
	sections := SplitSections("plain text\nwith two lines")
	if len(sections) != 1 || sections[0].Heading != "Introduction" {
		t.Fatalf("sections = %+v", sections)
	}
}

func TestSplitSectionsSplitsHeadingsAndLongText(t *testing.T) {
	sections := SplitSections("# One\nbody\n# Two\nbody")
	if len(sections) != 2 || sections[0].Heading != "One" || sections[1].Heading != "Two" {
		t.Fatalf("sections = %+v", sections)
	}
	long := strings.Repeat("あ\n", 1800)
	if got := SplitSections(long); len(got) < 2 {
		t.Fatalf("long text produced %d section(s), want split", len(got))
	}
	if got := SplitSections(strings.Repeat("あ", 3600)); len(got) < 2 {
		t.Fatalf("long line produced %d section(s), want split", len(got))
	}
}

func TestLooksLikeHeadingAndNormalizeHeading(t *testing.T) {
	if !LooksLikeHeading("  1. Overview") || !LooksLikeHeading("# Overview") || !LooksLikeHeading("Summary:") {
		t.Fatal("expected supported heading forms")
	}
	if LooksLikeHeading(strings.Repeat("a", 121)) || LooksLikeHeading("") {
		t.Fatal("expected invalid heading forms to be rejected")
	}
	if got := NormalizeHeading(" ##  Overview: "); got != "Overview" {
		t.Fatalf("NormalizeHeading() = %q, want Overview", got)
	}
}
