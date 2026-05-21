package middleware

import (
	"context"
	"testing"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer token", "Bearer abc123", "abc123"},
		{"lowercase bearer also valid", "bearer abc", "abc"},
		{"missing prefix", "abc123", ""},
		{"empty header", "", ""},
		{"value trimmed of spaces", "Bearer  spaced ", "spaced"},
		{"bearer with no value", "Bearer ", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bearerToken(tc.header)
			if got != tc.want {
				t.Errorf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestParseEmailSet(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want map[string]bool
	}{
		{"empty", "", map[string]bool{}},
		{"single", "a@x.com", map[string]bool{"a@x.com": true}},
		{"trims and lowercases", " A@X.com , b@y.com ", map[string]bool{"a@x.com": true, "b@y.com": true}},
		{"skips blanks", "a@x.com,,  ,b@y.com", map[string]bool{"a@x.com": true, "b@y.com": true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseEmailSet(tc.csv)
			if len(got) != len(tc.want) {
				t.Fatalf("parseEmailSet(%q) size = %d, want %d (%v)", tc.csv, len(got), len(tc.want), got)
			}
			for k := range tc.want {
				if !got[k] {
					t.Errorf("parseEmailSet(%q) missing %q", tc.csv, k)
				}
			}
		})
	}
}

// Mirrors the allowlist gate in WithAuth: empty set => allow anyone;
// non-empty => only listed emails (case-insensitive) pass.
func TestAllowlistGate(t *testing.T) {
	allowed := func(set map[string]bool, email string) bool {
		return len(set) == 0 || set[lowerTrim(email)]
	}
	locked := parseEmailSet("korokororin47@gmail.com")
	open := parseEmailSet("")

	cases := []struct {
		name  string
		set   map[string]bool
		email string
		want  bool
	}{
		{"open set allows anyone", open, "stranger@example.com", true},
		{"locked allows the listed email", locked, "korokororin47@gmail.com", true},
		{"locked allows case/space variant", locked, "  Korokororin47@Gmail.com ", true},
		{"locked rejects other email", locked, "intruder@example.com", false},
		{"locked rejects empty email", locked, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allowed(tc.set, tc.email); got != tc.want {
				t.Errorf("allowed(%q) = %v, want %v", tc.email, got, tc.want)
			}
		})
	}
}

func TestCurrentUser_WithUser_ReturnsUser(t *testing.T) {
	want := AuthUser{ID: "user_123", Email: "test@example.com"}
	ctx := context.WithValue(context.Background(), authUserContextKey, want)

	got, ok := CurrentUser(ctx)
	if !ok {
		t.Fatal("CurrentUser: ok=false, want true")
	}
	if got.ID != want.ID || got.Email != want.Email {
		t.Errorf("CurrentUser = %+v, want %+v", got, want)
	}
}

func TestCurrentUser_NoUser_ReturnsFalse(t *testing.T) {
	_, ok := CurrentUser(context.Background())
	if ok {
		t.Fatal("CurrentUser: ok=true, want false")
	}
}

func TestContextWithUser_RoundTrip(t *testing.T) {
	want := AuthUser{ID: "u42", Email: "u@example.com"}
	ctx := ContextWithUser(context.Background(), want)

	got, ok := CurrentUser(ctx)
	if !ok {
		t.Fatal("CurrentUser after ContextWithUser: ok=false")
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
