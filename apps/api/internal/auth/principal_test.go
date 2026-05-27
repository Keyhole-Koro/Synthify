package auth

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrincipalContextRoundTrip(t *testing.T) {
	want := Principal{Kind: PrincipalKindUser, SubjectID: "user_123", Email: "test@example.com", Admin: true}
	ctx := ContextWithPrincipal(context.Background(), want)

	got, ok := PrincipalFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, want, got)
}

func TestPrincipalFromContextNoPrincipal(t *testing.T) {
	_, ok := PrincipalFromContext(context.Background())
	assert.False(t, ok)
}

func TestRequireServicePrincipalRejectsUser(t *testing.T) {
	ctx := ContextWithPrincipal(context.Background(), Principal{Kind: PrincipalKindUser, SubjectID: "u1"})
	_, err := RequireServicePrincipal(ctx)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func TestRequireAdminPrincipalRequiresAdminUser(t *testing.T) {
	ctx := ContextWithPrincipal(context.Background(), Principal{Kind: PrincipalKindUser, SubjectID: "u1"})
	_, err := RequireAdminPrincipal(ctx)
	assertConnectCode(t, err, connect.CodePermissionDenied)
}

func assertConnectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	require.Error(t, err)
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, want, ce.Code())
}
