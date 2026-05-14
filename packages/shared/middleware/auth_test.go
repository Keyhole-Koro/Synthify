package middleware

import (
	"context"
	"testing"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer token", "Bearer abc123", "abc123"},
		{"lowercase bearer also valid", "bearer abc", "abc"},
		{"missing prefix", "abc123", ""},
		{"empty header", "", ""},
		{"value trimmed of spaces", "Bearer  spaced ", "spaced"},
		{"bearer with no value", "Bearer ", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bearerToken(tc.header)
			if got != tc.want {
				t.Errorf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestCurrentUser_WithUser_ReturnsUser(t *testing.T) {
	want := AuthUser{ID: "user_123", Email: "test@example.com"}
	ctx := context.WithValue(context.Background(), authUserContextKey, want)

	got, ok := CurrentUser(ctx)
	if !ok {
		t.Fatal("CurrentUser: ok=false, want true")
	}
	if got.ID != want.ID || got.Email != want.Email {
		t.Errorf("CurrentUser = %+v, want %+v", got, want)
	}
}

func TestCurrentUser_NoUser_ReturnsFalse(t *testing.T) {
	_, ok := CurrentUser(context.Background())
	if ok {
		t.Fatal("CurrentUser: ok=true, want false")
	}
}

func TestContextWithUser_RoundTrip(t *testing.T) {
	want := AuthUser{ID: "u42", Email: "u@example.com"}
	ctx := ContextWithUser(context.Background(), want)

	got, ok := CurrentUser(ctx)
	if !ok {
		t.Fatal("CurrentUser after ContextWithUser: ok=false")
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
