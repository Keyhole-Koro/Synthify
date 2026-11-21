// Package auth provides authentication mechanisms, principal context propagation,
// and role/permission enforcement for the Synthify API.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
)

// InternalServiceSubjectID is the subject ID assigned to verified internal service principals.
const InternalServiceSubjectID = "internal-service"

var (
	// ErrUnauthenticated is returned when authentication credentials are missing or invalid.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrPermissionDenied is returned when an authenticated principal lacks required access rights.
	ErrPermissionDenied = errors.New("permission denied")
)

// FirebaseAuthenticatorConfig defines settings for Firebase Admin SDK and access control lists.
type FirebaseAuthenticatorConfig struct {
	ProjectID        string
	ServiceToken     string
	AdminEmailsCSV   string
	AllowedEmailsCSV string
}

// FirebaseAuthenticator verifies Firebase user ID tokens and internal service shared secrets.
type FirebaseAuthenticator struct {
	client               *firebaseauth.Client
	expectedServiceToken string
	adminEmails          map[string]bool
	allowedEmails        map[string]bool
}

// NewFirebaseAuthenticator initializes a Firebase Admin Auth client and parses email allowlists.
func NewFirebaseAuthenticator(cfg FirebaseAuthenticatorConfig) (*FirebaseAuthenticator, error) {
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		return nil, errors.New("FIREBASE_PROJECT_ID is required")
	}
	app, err := firebase.NewApp(context.Background(), &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	client, err := app.Auth(context.Background())
	if err != nil {
		return nil, err
	}
	return &FirebaseAuthenticator{
		client:               client,
		expectedServiceToken: strings.TrimSpace(cfg.ServiceToken),
		adminEmails:          parseEmailSet(cfg.AdminEmailsCSV),
		allowedEmails:        parseEmailSet(cfg.AllowedEmailsCSV),
	}, nil
}

// AuthenticateBearer verifies an incoming Firebase ID token, validates allowed email domains,
// and extracts user identity and administrator privileges.
func (a *FirebaseAuthenticator) AuthenticateBearer(ctx context.Context, token string) (Principal, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	idToken, err := a.client.VerifyIDToken(ctx, token)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: verify bearer token: %v", ErrUnauthenticated, err)
	}
	email, _ := idToken.Claims["email"].(string)
	displayName, _ := idToken.Claims["name"].(string)
	normalizedEmail := lowerTrim(email)
	if len(a.allowedEmails) > 0 && !a.allowedEmails[normalizedEmail] {
		return Principal{}, fmt.Errorf("%w: email not allowed", ErrPermissionDenied)
	}
	return Principal{
		Kind:        PrincipalKindUser,
		SubjectID:   idToken.UID,
		Email:       email,
		DisplayName: displayName,
		Admin:       normalizedEmail != "" && a.adminEmails[normalizedEmail],
	}, nil
}

// AuthenticateServiceToken validates an internal service token against the expected configured secret
// using constant-time comparison.
func (a *FirebaseAuthenticator) AuthenticateServiceToken(ctx context.Context, token string) (Principal, error) {
	_ = ctx
	token = strings.TrimSpace(token)
	if token == "" || a.expectedServiceToken == "" {
		return Principal{}, ErrUnauthenticated
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(a.expectedServiceToken)) != 1 {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{
		Kind:      PrincipalKindService,
		SubjectID: InternalServiceSubjectID,
	}, nil
}

// lowerTrim trims leading/trailing whitespace and converts the string to lowercase.
func lowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// parseEmailSet converts a comma-separated email list into a normalized lowercase lookup set.
func parseEmailSet(csv string) map[string]bool {
	result := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		if e := lowerTrim(raw); e != "" {
			result[e] = true
		}
	}
	return result
}
