package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	firebaseauth "firebase.google.com/go/v4/auth"
)

// TestFirebaseEmailVerifier_OnTheAuthEmulator runs against the Firebase Auth
// emulator (project demo-synthify, as in compose.yaml) when
// FIREBASE_AUTH_EMULATOR_HOST is set, and skips otherwise.
func TestFirebaseEmailVerifier_OnTheAuthEmulator(t *testing.T) {
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	v, err := NewFirebaseEmailVerifier(ctx, "demo-synthify")
	if err != nil {
		t.Fatalf("NewFirebaseEmailVerifier: %v", err)
	}
	suffix := time.Now().UnixNano()
	addr := func(name string) string { return fmt.Sprintf("%s-%d@example.test", name, suffix) }
	create := func(email string, verified bool) string {
		t.Helper()
		u, err := v.client.CreateUser(ctx, (&firebaseauth.UserToCreate{}).Email(email).EmailVerified(verified).Password("Verified-1!"))
		if err != nil {
			t.Fatalf("create %s: %v", email, err)
		}
		return u.UID
	}

	owner := create(addr("owner"), true)
	// Someone signed up with another person's address and never verified it.
	squatter := create(addr("victim"), false)

	for _, tc := range []struct {
		name       string
		uid, email string
		want       bool
	}{
		{"verified address", owner, addr("owner"), true},
		{"case and spaces do not matter", owner, "  OWNER" + addr("owner")[len("owner"):] + " ", true},
		{"another address of a verified account", owner, addr("someone-else"), false},
		{"unverified address", squatter, addr("victim"), false},
		{"unknown account", "no-such-uid", addr("owner"), false},
		{"empty address", owner, "", false},
	} {
		got, err := v.HasVerifiedEmail(ctx, tc.uid, tc.email)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: HasVerifiedEmail = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Once the account verifies the address, it counts.
	if _, err := v.client.UpdateUser(ctx, squatter, (&firebaseauth.UserToUpdate{}).EmailVerified(true)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got, err := v.HasVerifiedEmail(ctx, squatter, addr("victim")); err != nil || !got {
		t.Errorf("after verification: HasVerifiedEmail = %v, %v; want true", got, err)
	}
}
