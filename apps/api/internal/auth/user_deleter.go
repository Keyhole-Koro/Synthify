package auth

import (
	"context"
	"errors"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
)

// FirebaseUserDeleter deletes Firebase Authentication users when an account
// is deleted. The Admin SDK call needs firebaseauth.users.delete on the
// project (roles/firebaseauth.admin); against the Auth emulator
// (FIREBASE_AUTH_EMULATOR_HOST) it needs no credentials.
type FirebaseUserDeleter struct {
	client *firebaseauth.Client
}

func NewFirebaseUserDeleter(ctx context.Context, projectID string) (*FirebaseUserDeleter, error) {
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
	return &FirebaseUserDeleter{client: client}, nil
}

// DeleteUser deletes the user. One that is already gone counts as deleted, so
// a retried account deletion can finish.
func (d *FirebaseUserDeleter) DeleteUser(ctx context.Context, uid string) error {
	if err := d.client.DeleteUser(ctx, uid); err != nil && !firebaseauth.IsUserNotFound(err) {
		return err
	}
	return nil
}
