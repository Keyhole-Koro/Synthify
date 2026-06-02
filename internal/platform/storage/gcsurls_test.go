package storage

import "testing"

func TestSourceObjectName(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"report.pdf", "doc1/source/original.pdf"},
		{"REPORT.PDF", "doc1/source/original.pdf"}, // extension lowercased
		{"archive.tar.gz", "doc1/source/original.gz"},
		{"noext", "doc1/source/original"},
		{"", "doc1/source/original"},
	}
	for _, tc := range cases {
		if got := SourceObjectName("doc1", tc.filename); got != tc.want {
			t.Errorf("SourceObjectName(doc1, %q) = %q, want %q", tc.filename, got, tc.want)
		}
	}
}

// User-supplied names must never inject path segments into the object key; only
// the extension is used.
func TestSourceObjectName_IgnoresPathInjection(t *testing.T) {
	got := SourceObjectName("doc1", "../../etc/passwd")
	if got != "doc1/source/original" {
		t.Errorf("path injection leaked: got %q", got)
	}
}

func TestSourcePrefixAndDocumentPrefix(t *testing.T) {
	if got := SourcePrefix("doc1"); got != "doc1/source/" {
		t.Errorf("SourcePrefix = %q", got)
	}
	if got := DocumentPrefix("doc1"); got != "doc1/" {
		t.Errorf("DocumentPrefix = %q", got)
	}
}
