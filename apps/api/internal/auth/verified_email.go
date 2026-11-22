package auth

import (
	"context"
	"errors"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
)

// FirebaseEmailVerifier asks Firebase Authentication whether an account has
// verified its email address.
//
// The email the API stores for a user is copied from the ID token at sign-in
// without looking at email_verified. With email and password sign-up enabled,
// anyone can create an account under someone else's address, so that stored
// address proves nothing. Decisions that hand out access by email (workspace
// invitations) ask Firebase instead, as principalFromClaims does for the admin
// list and the allowlist.
type FirebaseEmailVerifier struct {
	client *firebaseauth.Client
}

// NewFirebaseEmailVerifier returns a verifier for the Firebase project.
// Reading user accounts needs firebaseauth.users.get on that project (the
// API's custom role in terraform/manual/iam).
func NewFirebaseEmailVerifier(ctx context.Context, projectID string) (*FirebaseEmailVerifier, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("FIREBASE_PROJECT_ID is required")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}
	return &FirebaseEmailVerifier{client: client}, nil
}

// HasVerifiedEmail reports whether the Firebase account uid has email as its
// address now and has verified it. Emails are compared without regard to case
// or surrounding space. An unknown account is not an error; it has no verified
// address.
func (v *FirebaseEmailVerifier) HasVerifiedEmail(ctx context.Context, uid, email string) (bool, error) {
	email = lowerTrim(email)
	if uid == "" || email == "" {
		return false, nil
	}
	record, err := v.client.GetUser(ctx, uid)
	if err != nil {
		if firebaseauth.IsUserNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return record.EmailVerified && record.UserInfo != nil && lowerTrim(record.Email) == email, nil
}
