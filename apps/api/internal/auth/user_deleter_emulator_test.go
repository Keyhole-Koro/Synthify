package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Runs against the Firebase Auth emulator; skipped unless
// FIREBASE_AUTH_EMULATOR_HOST points at one (the compose stack starts one).
func TestFirebaseUserDeleter_Emulator(t *testing.T) {
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	deleter, err := NewFirebaseUserDeleter(ctx, "demo-synthify")
	require.NoError(t, err)
	email := fmt.Sprintf("delete-%d@example.test", time.Now().UnixNano())
	user, err := deleter.client.CreateUser(ctx, (&firebaseauth.UserToCreate{}).Email(email).Password("Delete-me-1!"))
	require.NoError(t, err)

	require.NoError(t, deleter.DeleteUser(ctx, user.UID))

	_, err = deleter.client.GetUser(ctx, user.UID)
	assert.True(t, firebaseauth.IsUserNotFound(err), "the user is gone: %v", err)
	require.NoError(t, deleter.DeleteUser(ctx, user.UID), "an already deleted user counts as deleted")
}
