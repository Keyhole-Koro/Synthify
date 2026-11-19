package auth

import "fmt"

// principalFromClaims maps the claims of a verified Firebase ID token to a user
// Principal, applying the email allowlist and the admin list.
func (a *FirebaseAuthenticator) principalFromClaims(uid string, claims map[string]any) (Principal, error) {
	email, _ := claims["email"].(string)
	displayName, _ := claims["name"].(string)
	normalizedEmail := lowerTrim(email)
	if len(a.allowedEmails) > 0 && !a.allowedEmails[normalizedEmail] {
		return Principal{}, fmt.Errorf("%w: email not allowed", ErrPermissionDenied)
	}
	return Principal{
		Kind:        PrincipalKindUser,
		SubjectID:   uid,
		Email:       email,
		DisplayName: displayName,
		Admin:       normalizedEmail != "" && a.adminEmails[normalizedEmail],
	}, nil
}
