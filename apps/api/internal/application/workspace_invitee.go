package application

import "context"

// VerifiedEmails tells whether a user's sign-in account has verified an email
// address. auth.FirebaseEmailVerifier implements it.
type VerifiedEmails interface {
	HasVerifiedEmail(ctx context.Context, userID, email string) (bool, error)
}

// inviteeHasVerifiedEmail reports whether an invitation to email may go to
// userID, the user that GetUserByEmail found for it.
//
// users.email is copied from the ID token at every sign-in without looking at
// email_verified, and it is not unique. With email and password sign-up
// enabled, anyone can create an account under an address that is not theirs
// and sign in once; an invitation sent to that address would then make them a
// member of the workspace. So an invitation binds only to a user whose sign-in
// account has verified the invited address now, the same rule the API applies
// to the admin list and the allowlist. Without a VerifiedEmails nobody can be
// invited.
func (s *WorkspaceService) inviteeHasVerifiedEmail(ctx context.Context, userID, email string) (bool, error) {
	if s.emails == nil {
		return false, nil
	}
	return s.emails.HasVerifiedEmail(ctx, userID, email)
}
