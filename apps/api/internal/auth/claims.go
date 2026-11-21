package auth

import "fmt"

// principalFromClaims maps the claims of a verified Firebase ID token to a user
// Principal, applying the email allowlist and the admin list.
//
// Only a verified email (email_verified: true) counts for those two decisions.
// Firebase does not check that whoever signs up with an email and password owns
// the address, so with that sign-in method enabled anyone could otherwise create
// an account under an admin's or an allowlisted address. Google sign-in returns
// verified addresses. The email itself is still passed on as before.
func (a *FirebaseAuthenticator) principalFromClaims(uid string, claims map[string]any) (Principal, error) {
	email, _ := claims["email"].(string)
	displayName, _ := claims["name"].(string)
	trustedEmail := ""
	if verified, _ := claims["email_verified"].(bool); verified {
		trustedEmail = lowerTrim(email)
	}
	if len(a.allowedEmails) > 0 && !a.allowedEmails[trustedEmail] {
		return Principal{}, fmt.Errorf("%w: email not allowed", ErrPermissionDenied)
	}
	return Principal{
		Kind:        PrincipalKindUser,
		SubjectID:   uid,
		Email:       email,
		DisplayName: displayName,
		Admin:       trustedEmail != "" && a.adminEmails[trustedEmail],
	}, nil
}
