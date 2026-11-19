package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClaimsAuthenticator(adminCSV, allowedCSV string) *FirebaseAuthenticator {
	return &FirebaseAuthenticator{
		adminEmails:   parseEmailSet(adminCSV),
		allowedEmails: parseEmailSet(allowedCSV),
	}
}

func TestPrincipalFromClaims_MapsUserClaims(t *testing.T) {
	a := newClaimsAuthenticator("", "")

	got, err := a.principalFromClaims("uid_1", map[string]any{
		"email":          "User@Example.com",
		"email_verified": true,
		"name":           "User One",
	})

	require.NoError(t, err)
	assert.Equal(t, Principal{
		Kind:        PrincipalKindUser,
		SubjectID:   "uid_1",
		Email:       "User@Example.com",
		DisplayName: "User One",
	}, got)
}

func TestPrincipalFromClaims_AdminRole(t *testing.T) {
	a := newClaimsAuthenticator(" Admin@Example.com ", "")
	for _, tc := range []struct {
		name   string
		claims map[string]any
		want   bool
	}{
		{"verified admin email", map[string]any{"email": "admin@example.com", "email_verified": true}, true},
		{"verified admin email in other case", map[string]any{"email": " ADMIN@example.com", "email_verified": true}, true},
		{"verified other email", map[string]any{"email": "user@example.com", "email_verified": true}, false},
		{"no email", map[string]any{"email_verified": true}, false},
		{"empty email", map[string]any{"email": "", "email_verified": true}, false},
		{"non-string email", map[string]any{"email": 42, "email_verified": true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.principalFromClaims("uid_1", tc.claims)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Admin)
		})
	}
}

func TestPrincipalFromClaims_Allowlist(t *testing.T) {
	a := newClaimsAuthenticator("", "allowed@example.com")
	for _, tc := range []struct {
		name    string
		claims  map[string]any
		allowed bool
	}{
		{"verified allowed email", map[string]any{"email": "Allowed@Example.com", "email_verified": true}, true},
		{"verified other email", map[string]any{"email": "user@example.com", "email_verified": true}, false},
		{"no email", map[string]any{"email_verified": true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.principalFromClaims("uid_1", tc.claims)

			if tc.allowed {
				require.NoError(t, err)
				assert.Equal(t, "uid_1", got.SubjectID)
				return
			}
			require.ErrorIs(t, err, ErrPermissionDenied)
			assert.Equal(t, Principal{}, got)
		})
	}
}

func TestPrincipalFromClaims_NoAllowlistAdmitsAnyUser(t *testing.T) {
	a := newClaimsAuthenticator("admin@example.com", "")

	got, err := a.principalFromClaims("uid_1", map[string]any{"email": "user@example.com", "email_verified": true})

	require.NoError(t, err)
	assert.Equal(t, "uid_1", got.SubjectID)
	assert.False(t, got.Admin)
}
