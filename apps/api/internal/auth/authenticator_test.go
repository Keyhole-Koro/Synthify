package auth

import "testing"

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

func TestServiceTokenAuthenticator(t *testing.T) {
	authenticator := &FirebaseAuthenticator{expectedServiceToken: "secret"}
	principal, err := authenticator.AuthenticateServiceToken(t.Context(), "secret")
	if err != nil {
		t.Fatalf("AuthenticateServiceToken: %v", err)
	}
	if principal.Kind != PrincipalKindService || principal.SubjectID != InternalServiceSubjectID {
		t.Fatalf("principal = %+v", principal)
	}
}
