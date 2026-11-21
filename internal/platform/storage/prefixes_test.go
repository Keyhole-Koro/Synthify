package storage

import "testing"

func TestDeletionPrefixes(t *testing.T) {
	cases := map[string]string{
		WorkspaceObjectPrefix("ws1"):        "ws1/",
		DocumentObjectPrefix("ws1", "doc1"): "ws1/doc1/",
		DocumentCachePrefix("doc1"):         ".cache/v1/doc1/",
		JobCheckpointPrefix("job1"):         ".checkpoints/job1/",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("prefix = %q, want %q", got, want)
		}
		if err := ValidateDeletionPrefix(got); err != nil {
			t.Errorf("ValidateDeletionPrefix(%q) = %v, want nil", got, err)
		}
	}
}

// An empty ID or a shared directory must never reach a bucket listing: each of
// these would match other owners' objects.
func TestValidateDeletionPrefix_RejectsPrefixesSpanningOwners(t *testing.T) {
	for _, prefix := range []string{
		"",
		"/",
		"ws1",                     // no trailing slash: would also match ws10/
		WorkspaceObjectPrefix(""), // "/"
		DocumentObjectPrefix("ws1", ""),
		DocumentObjectPrefix("", "doc1"),
		DocumentCachePrefix(""), // every document's cache
		JobCheckpointPrefix(""), // every job's checkpoints
		".cache/",
		".cache/v1/",
		".checkpoints/",
		"../ws1/",
		"ws1/../ws2/",
		"ws1//doc1/",
		"ws1/doc1/source/", // deeper than a document is not a deletion unit
		".cache/v2/doc1/",  // unknown cache version: refuse rather than guess
	} {
		if err := ValidateDeletionPrefix(prefix); err == nil {
			t.Errorf("ValidateDeletionPrefix(%q) = nil, want an error", prefix)
		}
	}
}
