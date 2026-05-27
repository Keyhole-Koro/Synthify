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

const InternalServiceSubjectID = "internal-service"

var (
	ErrUnauthenticated  = errors.New("unauthenticated")
	ErrPermissionDenied = errors.New("permission denied")
)

type FirebaseAuthenticatorConfig struct {
	ProjectID        string
	ServiceToken     string
	AdminEmailsCSV   string
	AllowedEmailsCSV string
}

type FirebaseAuthenticator struct {
	client               *firebaseauth.Client
	expectedServiceToken string
	adminEmails          map[string]bool
	allowedEmails        map[string]bool
}

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

func lowerTrim(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func parseEmailSet(csv string) map[string]bool {
	result := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		if e := lowerTrim(raw); e != "" {
			result[e] = true
		}
	}
	return result
}
