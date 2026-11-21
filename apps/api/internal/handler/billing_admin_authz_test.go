package handler

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/auth"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// GrantCredit adds credit to any account, so only admin users may call it.
// Every other caller must be rejected before the billing service is reached.

// unreachableBillingUsecase stands in for the billing service where the handler
// must reject the call first. Its embedded BillingUsecase is nil, so any method
// call panics.
type unreachableBillingUsecase struct {
	BillingUsecase
}

func TestBillingHandler_GrantCredit_NonAdminCallers_RejectedBeforeService(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want connect.Code
	}{
		{
			name: "unauthenticated",
			ctx:  context.Background(),
			want: connect.CodeUnauthenticated,
		},
		{
			name: "share token only",
			ctx:  auth.ContextWithShareToken(context.Background(), "share-token-1"),
			want: connect.CodeUnauthenticated,
		},
		{
			name: "internal service token",
			ctx: auth.ContextWithPrincipal(context.Background(), auth.Principal{
				Kind:      auth.PrincipalKindService,
				SubjectID: auth.InternalServiceSubjectID,
			}),
			want: connect.CodeUnauthenticated,
		},
		{
			name: "user without the admin role",
			ctx: auth.ContextWithPrincipal(context.Background(), auth.Principal{
				Kind:      auth.PrincipalKindUser,
				SubjectID: "u1",
				Email:     "u1@example.com",
			}),
			want: connect.CodePermissionDenied,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewBillingHandler(unreachableBillingUsecase{})

			var (
				resp *connect.Response[appv1.GrantCreditResponse]
				err  error
			)
			requireNoBillingServiceCall(t, func() {
				resp, err = h.GrantCredit(tc.ctx, newGrantCreditRequest())
			})

			assert.Nil(t, resp)
			assertConnectCode(t, err, tc.want)
		})
	}
}

// Control for the test above: the same request from an admin user is served,
// so the rejections come from the role check and not from the request itself.
func TestBillingHandler_GrantCredit_AdminUser_ReachesService(t *testing.T) {
	h := NewBillingHandler(&billingHandlerTestUsecase{})
	ctx := auth.ContextWithPrincipal(context.Background(), auth.Principal{
		Kind:      auth.PrincipalKindUser,
		SubjectID: "admin-1",
		Email:     "admin@example.com",
		Admin:     true,
	})

	resp, err := h.GrantCredit(ctx, newGrantCreditRequest())

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "test-credit", resp.Msg.GetCreditId())
}

func newGrantCreditRequest() *connect.Request[appv1.GrantCreditRequest] {
	return connect.NewRequest(&appv1.GrantCreditRequest{
		AccountId:   "acct-1",
		AmountMinor: 500,
		Note:        "goodwill",
	})
}

// requireNoBillingServiceCall runs call and fails the test, with a clear message,
// if call reached an unreachableBillingUsecase (which panics on every method).
func requireNoBillingServiceCall(t *testing.T, call func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the handler reached the billing service: %v", r)
		}
	}()
	call()
}
